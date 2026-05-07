package connection

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/http"

	"golang.org/x/net/http2"

	"github.com/DemonHunter29/CheckUser/src/domain/contract"
)

const (
	// GetStatsOnline retorna o número exato de conexões ativas de um usuário.
	xrayOnlinePath = "/xray.app.stats.command.StatsService/GetStatsOnline"
	// GetAllOnlineUsers retorna lista de usuários com pelo menos 1 conexão ativa.
	xrayAllOnlinePath = "/xray.app.stats.command.StatsService/GetAllOnlineUsers"
)

type xrayConnection struct {
	addr      string
	transport *http2.Transport
	next      contract.CountConnection
}

func NewXrayConnection(addr string) contract.CountConnection {
	return &xrayConnection{
		addr: addr,
		transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}
}

func (x *xrayConnection) SetNext(next contract.CountConnection) {
	x.next = next
}

func (x *xrayConnection) ByUsername(ctx context.Context, username string) (int, error) {
	count, err := x.queryOnlineCount(ctx, username)
	if err != nil {
		log.Printf("[xray] online %s user=%s: %v", x.addr, username, err)
		count = 0
	} else {
		log.Printf("[xray] online %s user=%s: %d conexão(ões)", x.addr, username, count)
	}
	if x.next != nil {
		if n, err := x.next.ByUsername(ctx, username); err == nil {
			count += n
		}
	}
	return count, nil
}

func (x *xrayConnection) All(ctx context.Context) (int, error) {
	users, err := x.queryAllOnlineUsers(ctx)
	count := 0
	if err != nil {
		log.Printf("[xray] all online %s: %v", x.addr, err)
	} else {
		count = len(users)
	}
	if x.next != nil {
		if n, err := x.next.All(ctx); err == nil {
			count += n
		}
	}
	return count, nil
}

func (x *xrayConnection) Kill(ctx context.Context, username string) {
	// Xray não tem API de kill direto; a sessão encerra quando a conexão TCP fecha.
	if x.next != nil {
		x.next.Kill(ctx, username)
	}
}

// queryOnlineCount chama GetStatsOnline e retorna o número exato de conexões ativas.
func (x *xrayConnection) queryOnlineCount(ctx context.Context, username string) (int, error) {
	name := "user>>>" + username + ">>>online"
	body := xrayEncodeRequest(name, false)

	frame := make([]byte, 5+len(body))
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(body)))
	copy(frame[5:], body)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"http://"+x.addr+xrayOnlinePath, bytes.NewReader(frame))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("TE", "trailers")

	resp, err := (&http.Client{Transport: x.transport}).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if len(raw) < 5 {
		return 0, nil
	}

	msgLen := int(binary.BigEndian.Uint32(raw[1:5]))
	end := 5 + msgLen
	if end > len(raw) {
		end = len(raw)
	}

	// GetStatsResponse tem Stat stat = 1 (mesmo wire format que QueryStats).
	stats := xrayDecodeResponse(raw[5:end])
	if len(stats) == 0 {
		return 0, nil
	}
	return int(stats[0].value), nil
}

// queryAllOnlineUsers chama GetAllOnlineUsers e retorna a lista de nomes online.
func (x *xrayConnection) queryAllOnlineUsers(ctx context.Context) ([]string, error) {
	frame := make([]byte, 5) // request vazio — GetAllOnlineUsersRequest sem campos

	req, err := http.NewRequestWithContext(ctx, "POST",
		"http://"+x.addr+xrayAllOnlinePath, bytes.NewReader(frame))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("TE", "trailers")

	resp, err := (&http.Client{Transport: x.transport}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(raw) < 5 {
		return nil, nil
	}

	msgLen := int(binary.BigEndian.Uint32(raw[1:5]))
	end := 5 + msgLen
	if end > len(raw) {
		end = len(raw)
	}

	return xrayDecodeAllOnline(raw[5:end]), nil
}

// xrayDecodeAllOnline desserializa GetAllOnlineUsersResponse { repeated string users = 1 }.
func xrayDecodeAllOnline(data []byte) []string {
	var out []string
	for i := 0; i < len(data); {
		tag, n := pbReadVarint(data[i:])
		if n <= 0 {
			break
		}
		i += n
		fieldNum, wireType := tag>>3, tag&0x7
		if fieldNum == 1 && wireType == 2 {
			l, n := pbReadVarint(data[i:])
			if n <= 0 || i+n+int(l) > len(data) {
				break
			}
			i += n
			out = append(out, string(data[i:i+int(l)]))
			i += int(l)
		} else {
			i = pbSkip(data, i, wireType)
		}
	}
	return out
}

// ── gRPC / Protobuf ──────────────────────────────────────────────────────────

type xrayStat struct {
	name  string
	value int64
}

// xrayEncodeRequest serializa GetStatsRequest{name} em protobuf.
func xrayEncodeRequest(name string, reset bool) []byte {
	var b []byte
	b = append(b, 0x0A) // field 1, wire 2 (string)
	b = pbVarint(b, uint64(len(name)))
	b = append(b, name...)
	if reset {
		b = append(b, 0x10, 0x01) // field 2, wire 0 (bool=true)
	}
	return b
}

// xrayDecodeResponse desserializa GetStatsResponse/QueryStatsResponse → []xrayStat.
func xrayDecodeResponse(data []byte) []xrayStat {
	var out []xrayStat
	for i := 0; i < len(data); {
		tag, n := pbReadVarint(data[i:])
		if n <= 0 {
			break
		}
		i += n
		fieldNum, wireType := tag>>3, tag&0x7
		if fieldNum == 1 && wireType == 2 {
			l, n := pbReadVarint(data[i:])
			if n <= 0 || i+n+int(l) > len(data) {
				break
			}
			i += n
			s := xrayDecodeStat(data[i : i+int(l)])
			if s.name != "" {
				out = append(out, s)
			}
			i += int(l)
		} else {
			i = pbSkip(data, i, wireType)
		}
	}
	return out
}

// xrayDecodeStat desserializa Stat{name, value}.
func xrayDecodeStat(data []byte) xrayStat {
	var s xrayStat
	for i := 0; i < len(data); {
		tag, n := pbReadVarint(data[i:])
		if n <= 0 {
			break
		}
		i += n
		fieldNum, wireType := tag>>3, tag&0x7
		switch {
		case fieldNum == 1 && wireType == 2:
			l, n := pbReadVarint(data[i:])
			if n <= 0 || i+n+int(l) > len(data) {
				return s
			}
			i += n
			s.name = string(data[i : i+int(l)])
			i += int(l)
		case fieldNum == 2 && wireType == 0:
			v, n := pbReadVarint(data[i:])
			if n <= 0 {
				return s
			}
			i += n
			s.value = int64(v)
		default:
			i = pbSkip(data, i, wireType)
		}
	}
	return s
}

func pbSkip(data []byte, i int, wireType uint64) int {
	switch wireType {
	case 0:
		_, n := pbReadVarint(data[i:])
		if n <= 0 {
			return len(data)
		}
		return i + n
	case 2:
		l, n := pbReadVarint(data[i:])
		if n <= 0 {
			return len(data)
		}
		return i + n + int(l)
	default:
		return len(data)
	}
}

func pbVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func pbReadVarint(data []byte) (uint64, int) {
	var x uint64
	var s uint
	for i, b := range data {
		if i == 10 {
			return 0, -1
		}
		if b < 0x80 {
			return x | uint64(b)<<s, i + 1
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, 0
}
