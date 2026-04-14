package factory

import (
	"github.com/DemonHunter29/CheckUser/src/data"
	"github.com/DemonHunter29/CheckUser/src/data/cache"
	"github.com/DemonHunter29/CheckUser/src/data/connection"
	"github.com/DemonHunter29/CheckUser/src/data/dao"
	"github.com/DemonHunter29/CheckUser/src/data/repository"
	user_use_case "github.com/DemonHunter29/CheckUser/src/domain/usecase/user"
	"github.com/DemonHunter29/CheckUser/src/infra/handler"
	user_handler "github.com/DemonHunter29/CheckUser/src/infra/handler/user"
)

func MakeCheckUserHandler() handler.Handler {
	executor := data.NewBashExecutor()
	userDAO := dao.NewUserDAO(executor)
	userRepository := repository.NewSystemUserRepository(userDAO)
	deviceRepository := repository.NewSQLiteDeviceRepository()
	checkUserUseCase := user_use_case.NewCheckUserUseCase(userRepository, deviceRepository)
	return user_handler.NewCheckUserHandler(checkUserUseCase)
}

func MakeCountConnectionsHandler() handler.Handler {
	executor := data.NewBashExecutor()
	countSSH := connection.NewSSHConnection(executor)
	countSSH.SetNext(connection.NewOpenVPNConnection(connection.NewAUXOpenVPNConnection("127.0.0.1", 7505)))
	countConnectionCacheService := cache.NewCountConnectionCacheService()
	countConnectionsUseCase := user_use_case.NewCountConnectionsUseCase(countSSH, countConnectionCacheService)
	return user_handler.NewCountConnectionsHandler(countConnectionsUseCase)
}

func MakeDetailsUserHandler() handler.Handler {
	executor := data.NewBashExecutor()
	userDAO := dao.NewUserDAO(executor)
	userRepository := repository.NewSystemUserRepository(userDAO)
	countSSH := connection.NewSSHConnection(executor)
	countSSH.SetNext(connection.NewOpenVPNConnection(connection.NewAUXOpenVPNConnection("127.0.0.1", 7505)))
	checkUserUseCase := user_use_case.NewDetailUserUseCase(userRepository, countSSH)
	return user_handler.NewDetailUserHandler(checkUserUseCase)
}
