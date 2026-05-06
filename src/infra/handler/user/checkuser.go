package user_handler

import (
	"context"
	"errors"

	"github.com/DemonHunter29/CheckUser/src/data/dao"
	user_use_case "github.com/DemonHunter29/CheckUser/src/domain/usecase/user"
	"github.com/DemonHunter29/CheckUser/src/infra/handler"
)

type checkUserHandler struct {
	checkUserUseCase *user_use_case.CheckUserUseCase
}

func NewCheckUserHandler(checkUserUseCase *user_use_case.CheckUserUseCase) handler.Handler {
	return &checkUserHandler{checkUserUseCase}
}

func (h *checkUserHandler) Handle(ctx context.Context, request *handler.HttpRequest) (*handler.HttpResponse, error) {
	username := request.Query("username")
	deviceID := request.Query("deviceId")
	if username == "" || deviceID == "" {
		return nil, errors.New("Please provide a username and device ID")
	}

	// Se o identificador for um UUID Xray, resolve para o username Linux
	// via config.json do Xray (clients[].email). Permite que SSH e Xray
	// compartilhem o mesmo deviceId tracking pelo mesmo username.
	if resolved, ok := dao.ResolveUUID(username); ok {
		username = resolved
	}

	output, err := h.checkUserUseCase.Execute(ctx, username, deviceID)
	if err != nil {
		return nil, err
	}

	return &handler.HttpResponse{
		Status: 200,
		Body:   output,
	}, nil
}
