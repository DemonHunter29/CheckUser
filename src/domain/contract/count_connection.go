package contract

import "context"

type CountConnection interface {
	SetNext(connection CountConnection)
	ByUsername(ctx context.Context, username string) (int, error)
	All(ctx context.Context) (int, error)
	// Kill encerra todas as sessões ativas do usuário neste protocolo.
	// Usado para expulsar conexões de devices não registrados quando o
	// device registrado reconecta acima do limite.
	Kill(ctx context.Context, username string)
}
