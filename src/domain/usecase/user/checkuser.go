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

	existingDevices, err := c.deviceRepository.CountByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	device := &entity.Device{
		ID:       deviceID,
		Username: username,
	}

	deviceExists := c.deviceRepository.Exists(ctx, device)
	limitReached := !deviceExists && user.LimitReached(existingDevices)

	if !deviceExists && !limitReached {
		if err := c.deviceRepository.Save(ctx, device); err != nil {
			return nil, err
		}
		existingDevices++
	}

	// Usa sessões ativas (SSH + DTProto + HCP) como count_connections.
	// Fallback pro count de devices se o counter falhar ou retornar 0.
	connections, err := c.countConnection.ByUsername(ctx, username)
	if err != nil || connections == 0 {
		connections = existingDevices
	}
	if limitReached {
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
