package user_use_case

import (
	"log"
	"time"

	"github.com/DemonHunter29/CheckUser/src/domain/contract"
	"golang.org/x/net/context"
)

type DetailUserOutput struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	ExpiresAt   string `json:"expires_at"`
	ExpiresDays int    `json:"expires_days"`
	Limit       int    `json:"limit"`
	Connections int    `json:"connections"`
}

type DetailUserUseCase struct {
	userRepository  contract.UserRepository
	countConnection contract.CountConnection
}

func NewDetailUserUseCase(
	userRepository contract.UserRepository,
	countConnection contract.CountConnection,
) *DetailUserUseCase {
	return &DetailUserUseCase{
		userRepository:  userRepository,
		countConnection: countConnection,
	}
}

func (c *DetailUserUseCase) Execute(ctx context.Context, username string) (*DetailUserOutput, error) {
	log.Printf("[details] Execute: username=%q", username)
	user, err := c.userRepository.FindByUsername(ctx, username)
	if err != nil {
		log.Printf("[details] FindByUsername error: %v", err)
		return nil, err
	}

	log.Printf("[details] calling countConnection.ByUsername for %q", user.Username)
	connections, err := c.countConnection.ByUsername(ctx, user.Username)
	log.Printf("[details] countConnection returned: connections=%d err=%v", connections, err)
	if err != nil {
		connections = 0
	}

	return &DetailUserOutput{
		ID:          user.ID,
		Username:    user.Username,
		ExpiresAt:   user.ExpiresAt.Format("02/01/2006"),
		Limit:       user.Limit,
		ExpiresDays: int(time.Until(user.ExpiresAt).Hours() / 24),
		Connections: connections,
	}, nil
}
