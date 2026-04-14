package contract

import (
	"context"

	"github.com/DemonHunter29/CheckUser/src/domain/entity"
)

type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*entity.User, error)
}
