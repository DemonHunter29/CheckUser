package contract

import (
	"context"

	"github.com/DemonHunter29/CheckUser/src/domain/entity"
)

type UserDAO interface {
	FindByUsername(ctx context.Context, username string) (*entity.User, error)
}
