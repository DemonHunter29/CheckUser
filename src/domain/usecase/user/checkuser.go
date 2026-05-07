package user_use_case

import (
	"context"
	"time"

	"github.com/DemonHunter29/CheckUser/src/domain/contract"
	"github.com/DemonHunter29/CheckUser/src/domain/entity"
)

type CheckUserOutput struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	ExpiresAt   string `json:"expiration_date"`
	ExpiresDays int    `json:"expiration_days"`
	Limit       int    `json:"limit_connections"`
	Connections int    `json:"count_connections"`
}

type CheckUserUseCase struct {
	userRepository   contract.UserRepository
	deviceRepository contract.DeviceRepository
	countConnection  contract.CountConnection
}

func NewCheckUserUseCase(
	userRepository contract.UserRepository,
	deviceRepository contract.DeviceRepository,
	countConnection contract.CountConnection,
) *CheckUserUseCase {
	return &CheckUserUseCase{
		userRepository:   userRepository,
		deviceRepository: deviceRepository,
		countConnection:  countConnection,
	}
}

func (c *CheckUserUseCase) Execute(ctx context.Context, username, deviceID string) (*CheckUserOutput, error) {
	user, err := c.userRepository.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	device := &entity.Device{
		ID:       deviceID,
		Username: username,
	}

	// Sessões ativas (SSH + DTProto + HCP + Xray) → base do bloqueio
	connections, _ := c.countConnection.ByUsername(ctx, username)

	deviceExists := c.deviceRepository.Exists(ctx, device)

	// Bloqueia apenas se: conexões ativas atingiram o limite E o device não
	// está cadastrado. Device cadastrado sempre passa — é o "aparelho do dono".
	limitReached := user.Limit > 0 && connections >= user.Limit && !deviceExists

	// Registra novo device apenas quando não está bloqueado
	if !deviceExists && !limitReached {
		if err := c.deviceRepository.Save(ctx, device); err != nil {
			return nil, err
		}
	}

	if limitReached {
		// Sinaliza acima do limite para disparar o aviso no app
		connections = user.Limit + 1
	}

	return &CheckUserOutput{
		ID:          user.ID,
		Username:    user.Username,
		ExpiresAt:   user.ExpiresAt.Format("02/01/2006"),
		ExpiresDays: int(time.Until(user.ExpiresAt).Hours() / 24),
		Limit:       user.Limit,
		Connections: connections,
	}, nil
}
