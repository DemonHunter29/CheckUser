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
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"

	"github.com/DemonHunter29/CheckUser/src/domain/contract"
)

const xrayStatsPath = "/xray.app.stats.command.StatsService/QueryStats"

// xrayConnection conta conexões Xray via API gRPC (HTTP/2 cleartext) do Xray.
// Faz polling a cada 30s comparando snapshots de stats — não usa reset=true
// para não interferir com painéis externos.
//
// Um usuário é considerado ativo se seus bytes acumulados aumentaram desde o
// último poll (janela de 90s = 3 intervalos de poll).
type xrayConnection struct {
	addr      string
	transport *http2.Transport
	next      contract.CountConnection

	mu     sync.RWMutex
	online map[string]time.Time // username → última atividade detectada
	prev   map[string]int64     // username → total de bytes no último poll
	once   sync.Once
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
		online: make(map[string]time.Time),
		prev:   make(map[string]int64),
	}
}

func (x *xrayConnection) SetNext(next contract.CountConnection) {
	x.next = next
}

func (x *xrayConnection) ByUsername(ctx context.Context, username string) (int, error) {
	x.once.Do(func() { go x.pollLoop() })
	// Se o usuário não está no cache, faz um poll imediato antes de verificar.
	// Resolve o caso de checkuser chamado logo após a conexão, antes do poll
	// periódico de 30s detectar a atividade do usuário.
	if x.isActive(username) == 0 {
		x.poll()
	}
	count := x.isActive(username)
	if x.next != nil {
		if n, err := x.next.ByUsername(ctx, username); err == nil {
			count += n
		}
	}
	return count, nil
}

func (x *xrayConnection) All(ctx context.Context) (int, error) {
	x.once.Do(func() { go x.pollLoop() })
	x.mu.RLock()
	count := 0
	deadline := time.Now().Add(-90 * time.Second)
	for _, t := range x.online {
		if t.After(deadline) {
			count++
		}
	}
	x.mu.RUnlock()
	if x.next != nil {
		if n, err := x.next.All(ctx); err == nil {
			count += n
		}
	}
	return count, nil
}

func (x *xrayConnection) isActive(username string) int {
	x.mu.RLock()
	defer x.mu.RUnlock()
	if t, ok := x.online[username]; ok && time.Since(t) < 90*time.Second {
		return 1
	}
	return 0
}

func (x *xrayConnection) pollLoop() {
	x.poll()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		x.poll()
	}
}

func (x *xrayConnection) poll() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats, err := x.callQueryStats(ctx, "user>>>", false)
	if err != nil {
		log.Printf("[xray] poll %s: %v", x.addr, err)
		return
	}

	// Soma todos os bytes por username (pode haver múltiplos inbounds)
	current := make(map[string]int64, len(stats))
	for _, s := range stats {
		u := xrayExtractUsername(s.name)
		if u != "" {
			current[u] += s.value
		}
	}

	now := time.Now()
	x.mu.Lock()
	for u, val := range current {
		if val > x.prev[u] {
			x.online[u] = now
		}
		x.prev[u] = val
	}
	x.mu.Unlock()

	log.Printf("[xray] poll %s: %d usuários com stats", x.addr, len(current))
}

// xrayExtractUsername extrai o username de "user>>>email>>>traffic>>>..."
// Suporta email com @ (user>>>nome@tag>>>...) e sem @ (user>>>nome>>>...).
func xrayExtractUsername(name string) string {
	after, found := strings.CutPrefix(name, "user>>>")
	if !found {
		return ""
	}
	// Prefere o delimitador '@' (formato email@tag)
	if idx := strings.IndexByte(after, '@'); idx > 0 {
		return after[:idx]
	}
	// Fallback: sem '@', usa tudo até '>>>'
	if idx := strings.Index(after, ">>>"); idx > 0 {
		return after[:idx]
	}
	return ""
}

// ── gRPC / Protobuf ──────────────────────────────────────────────────────────

type xrayStat struct {
	name  string
	value int64
}

func (x *xrayConnection) callQueryStats(ctx context.Context, pattern string, reset bool) ([]xrayStat, error) {
	body := xrayEncodeRequest(pattern, reset)

	// gRPC frame: [compressed(1)] [length(4 BE)] [body]
	frame := make([]byte, 5+len(body))
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(body)))
	copy(frame[5:], body)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"http://"+x.addr+xrayStatsPath, bytes.NewReader(frame))
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
	return xrayDecodeResponse(raw[5:end]), nil
}

// xrayEncodeRequest serializa QueryStatsRequest{pattern, reset} em protobuf.
func xrayEncodeRequest(pattern string, reset bool) []byte {
	var b []byte
	b = append(b, 0x0A) // field 1, wire 2 (string)
	b = pbVarint(b, uint64(len(pattern)))
	b = append(b, pattern...)
	if reset {
		b = append(b, 0x10, 0x01) // field 2, wire 0 (bool=true)
	}
	return b
}

// xrayDecodeResponse desserializa QueryStatsResponse → []xrayStat.
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

// pbSkip avança i além do valor do campo com o wireType dado.
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
