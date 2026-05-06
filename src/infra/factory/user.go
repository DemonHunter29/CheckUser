package factory

import (
	"log"

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
// Sem TTL de staleness — o próprio server do tunnel (DTProto/HCP) é
// responsável por remover entries do stats.json quando o usuário desconecta.
// Aplicar TTL aqui causava falso "00/01" porque last_seen_at é atualizado
// em intervalos de keep-alive (que podem ser mais longos que o TTL).
const (
	dtprotoStatsPath = "/var/lib/proto-server/stats.json"
	hcpStatsPath     = "/var/lib/hcp-server/stats.json"
	xrayAPIAddr      = "127.0.0.1:1085"
)

// buildCountChain monta a cadeia: SSH → DTProto → HCP → Xray.
// Cada handler soma sua contagem antes de delegar ao próximo.
//
// OpenVPN foi retirado: o net.Dial pra 127.0.0.1:7505 não tem timeout e
// pode travar a chain inteira quando o port não responde (firewall DROP).
//
// Xray: consulta a API gRPC do Xray via polling de 30s. Se o Xray não
// estiver instalado, retorna 0 silenciosamente.
func buildCountChain(executor contract.Executor) contract.CountConnection {
	countSSH := connection.NewSSHConnection(executor)

	countDtProto := connection.NewStatsFileConnection(dtprotoStatsPath, 0)
	countSSH.SetNext(countDtProto)

	countHcp := connection.NewStatsFileConnection(hcpStatsPath, 0)
	countDtProto.SetNext(countHcp)

	countXray := connection.NewXrayConnection(xrayAPIAddr)
	countHcp.SetNext(countXray)

	log.Printf("[chain] SSH → DTProto(%s) → HCP(%s) → Xray(%s)",
		dtprotoStatsPath, hcpStatsPath, xrayAPIAddr)
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
