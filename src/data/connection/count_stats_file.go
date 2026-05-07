package connection

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/DemonHunter29/CheckUser/src/domain/contract"
)

// StatsEntry reflete o formato de stats.json gerado pelos servers
// DTProto (/var/lib/proto-server/stats.json) e HCP (/var/lib/hcp/stats.json).
//
//	{
//	  "10.20.0.2": {
//	    "id": "theroms",
//	    "traffic_up": 22796,
//	    "traffic_down": 49451,
//	    "connected_at": "2026-04-14 19:56:59",
//	    "last_seen_at": "2026-04-14 19:57:03"
//	  }
//	}
type StatsEntry struct {
	ID         string `json:"id"`
	TrafficUp   int64  `json:"traffic_up"`
	TrafficDown int64  `json:"traffic_down"`
	ConnectedAt string `json:"connected_at"`
	LastSeenAt  string `json:"last_seen_at"`
}

// statsFileConnection conta conexões de um usuário pelos entries em um
// stats.json externo. Usado pra DTProto e HCP — ambos compartilham o mesmo
// formato e convivem com usuários SSH do sistema.
type statsFileConnection struct {
	path       string
	staleAfter time.Duration // 0 = não ignorar por idade
	next       contract.CountConnection
}

// NewStatsFileConnection cria um counter que lê um stats.json no caminho
// fornecido. `staleAfter` descarta entries com last_seen_at mais velho que
// esse período (evita contar sessões zumbi se o stats.json não for
// garbage-collected). Use 0 pra desativar.
func NewStatsFileConnection(path string, staleAfter time.Duration) contract.CountConnection {
	return &statsFileConnection{path: path, staleAfter: staleAfter}
}

func (s *statsFileConnection) SetNext(next contract.CountConnection) {
	s.next = next
}

func (s *statsFileConnection) Kill(ctx context.Context, username string) {
	// DTProto/HCP: sem mecanismo de kill direto; a sessão expira naturalmente.
	if s.next != nil {
		s.next.Kill(ctx, username)
	}
}

func (s *statsFileConnection) ByUsername(ctx context.Context, username string) (int, error) {
	count := s.countInFile(username)
	if s.next != nil {
		if n, err := s.next.ByUsername(ctx, username); err == nil {
			count += n
		}
	}
	return count, nil
}

func (s *statsFileConnection) All(ctx context.Context) (int, error) {
	total := s.countAllInFile()
	if s.next != nil {
		if n, err := s.next.All(ctx); err == nil {
			total += n
		}
	}
	return total, nil
}

// countInFile conta entries cujo id == username e que não estão stale.
// Se o arquivo não existir (server não instalado) retorna 0 silenciosamente.
func (s *statsFileConnection) countInFile(username string) int {
	entries, ok := s.readFile()
	if !ok {
		log.Printf("[stats] %s: file missing/unreadable", s.path)
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.ID == username && !s.isStale(e) {
			n++
		}
	}
	log.Printf("[stats] %s: matched %d entries for user=%q (total entries=%d)",
		s.path, n, username, len(entries))
	return n
}

func (s *statsFileConnection) countAllInFile() int {
	entries, ok := s.readFile()
	if !ok {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.ID != "" && !s.isStale(e) {
			n++
		}
	}
	return n
}

func (s *statsFileConnection) readFile() (map[string]StatsEntry, bool) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, false // arquivo ausente = protocolo não instalado
	}
	var m map[string]StatsEntry
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

func (s *statsFileConnection) isStale(e StatsEntry) bool {
	if s.staleAfter <= 0 || e.LastSeenAt == "" {
		return false
	}
	t, err := time.Parse("2006-01-02 15:04:05", e.LastSeenAt)
	if err != nil {
		return false
	}
	return time.Since(t) > s.staleAfter
}
