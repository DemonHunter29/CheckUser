package factory

import (
	"log"
	"time"

	"github.com/DemonHunter29/CheckUser/src/data"
	"github.com/DemonHunter29/CheckUser/src/data/cache"
	"github.com/DemonHunter29/CheckUser/src/data/connection"
	"github.com/DemonHunter29/CheckUser/src/data/dao"
	"github.com/DemonHunter29/CheckUser/src/data/repository"
	"github.com/DemonHunter29/CheckUser/src/domain/contract"
	user_use_case "github.com/DemonHunter29/CheckUser/src/domain/usecase/user"
	"github.com/DemonHunter29/CheckUser/src/infra/handler"
	user_handler "github.com/DemonHunter29/CheckUser/src/infra/handler/user"
)

// Caminhos dos stats.json por protocolo. Se o arquivo não existir no server
// (protocolo não instalado) o counter silenciosamente retorna 0.
//
// TTL de 5 minutos: descarta entradas cujo last_seen_at é mais velho que isso.
// O keep-alive do HCP/DTProto atualiza last_seen_at a cada ~20s, então 5min
// só expira se a sessão foi abruptamente encerrada sem cleanup do stats.json.
const (
	dtprotoStatsPath    = "/var/lib/proto-server/stats.json"
	hcpStatsPath        = "/var/lib/hcp-server/stats.json"
	statsStaleAfter     = 5 * time.Minute
	xrayAPIAddrFallback = "127.0.0.1:1085"
)

// buildCountChain monta a cadeia: SSH → DTProto → HCP → Xray.
// Cada handler soma sua contagem antes de delegar ao próximo.
//
// OpenVPN foi retirado: o net.Dial pra 127.0.0.1:7505 não tem timeout e
// pode travar a chain inteira quando o port não responde (firewall DROP).
//
// Xray: porta lida do config.json (seção api.tag → inbound com essa tag).
// Fallback para 127.0.0.1:1085 se o config não for encontrado.
func buildCountChain(executor contract.Executor) contract.CountConnection {
	countSSH := connection.NewSSHConnection(executor)

	countDtProto := connection.NewStatsFileConnection(dtprotoStatsPath, statsStaleAfter)
	countSSH.SetNext(countDtProto)

	countHcp := connection.NewStatsFileConnection(hcpStatsPath, statsStaleAfter)
	countDtProto.SetNext(countHcp)

	xrayAddr := dao.ResolveXrayAPIAddr()
	if xrayAddr == "" {
		xrayAddr = xrayAPIAddrFallback
		log.Printf("[xray] config.json não encontrado — usando fallback %s", xrayAddr)
	} else {
		log.Printf("[xray] porta detectada via config.json: %s", xrayAddr)
	}
	countXray := connection.NewXrayConnection(xrayAddr)
	countHcp.SetNext(countXray)

	log.Printf("[chain] SSH → DTProto(%s) → HCP(%s) → Xray(%s)",
		dtprotoStatsPath, hcpStatsPath, xrayAddr)
	return countSSH
}

func MakeCheckUserHandler() handler.Handler {
	executor := data.NewBashExecutor()
	userDAO := dao.NewUserDAO(executor)
	userRepository := repository.NewSystemUserRepository(userDAO)
	deviceRepository := repository.NewSQLiteDeviceRepository()
	count := buildCountChain(executor)
	checkUserUseCase := user_use_case.NewCheckUserUseCase(userRepository, deviceRepository, count)
	return user_handler.NewCheckUserHandler(checkUserUseCase)
}

func MakeCountConnectionsHandler() handler.Handler {
	executor := data.NewBashExecutor()
	count := buildCountChain(executor)
	countConnectionCacheService := cache.NewCountConnectionCacheService()
	countConnectionsUseCase := user_use_case.NewCountConnectionsUseCase(count, countConnectionCacheService)
	return user_handler.NewCountConnectionsHandler(countConnectionsUseCase)
}

func MakeDetailsUserHandler() handler.Handler {
	executor := data.NewBashExecutor()
	userDAO := dao.NewUserDAO(executor)
	userRepository := repository.NewSystemUserRepository(userDAO)
	count := buildCountChain(executor)
	detailUserUseCase := user_use_case.NewDetailUserUseCase(userRepository, count)
	return user_handler.NewDetailUserHandler(detailUserUseCase)
}
