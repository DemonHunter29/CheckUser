package connection

import (
	"context"
	"log"
	"regexp"

	"github.com/DemonHunter29/CheckUser/src/domain/contract"
)

type sshConnection struct {
	executor contract.Executor
	next     contract.CountConnection
}

func NewSSHConnection(executor contract.Executor) contract.CountConnection {
	return &sshConnection{executor: executor}
}

func (ssh *sshConnection) SetNext(connection contract.CountConnection) {
	ssh.next = connection
}

func (s *sshConnection) ByUsername(ctx context.Context, username string) (int, error) {
	cmd := "ps -u " + username
	result, _ := s.executor.Execute(ctx, cmd)
	// NÃO propaga erro de `ps`: exit 1 é normal quando o usuário não tem
	// processos (nenhum sshd spawned). Antes esse erro abortava o chain
	// inteiro e impedia os counters DTProto/HCP de serem chamados.

	sshdPattern := regexp.MustCompile(`.*sshd`)
	matches := sshdPattern.FindAllStringSubmatch(result, -1)
	totalConnections := len(matches)
	log.Printf("[ssh] ps -u %s: %d sshd process(es)", username, totalConnections)
	if s.next != nil {
		count, err := s.next.ByUsername(ctx, username)
		if err == nil {
			totalConnections += count
		}
	}

	return totalConnections, nil
}

func (s *sshConnection) All(ctx context.Context) (int, error) {
	cmd := "ps -ef"
	result, _ := s.executor.Execute(ctx, cmd)
	// Tolera erro — se o ps falhar, segue com totalConnections=0 do SSH e
	// delega pro próximo counter (DTProto/HCP).

	sshdPattern := regexp.MustCompile(`(?m)^(\S+)\s+\d+\s+\d+\s+\d+\s+\d+:\d+\s+.*\bsshd\b.*$`)
	processMatches := sshdPattern.FindAllStringSubmatch(string(result), -1)
	forbiddenUsernames := map[string]bool{
		"root":   true,
		"nobody": true,
		"grep":   true,
	}

	totalConnections := 0
	for _, match := range processMatches {
		username := match[1]
		if !forbiddenUsernames[username] {
			totalConnections++
		}
	}

	if s.next != nil {
		count, err := s.next.All(ctx)
		if err == nil {
			totalConnections += count
		}
	}

	return totalConnections, nil
}
