package exchanges

import "context"

type Exchange struct {
	ID   int32
	Name string
}

type Repository interface {
	ListActive(ctx context.Context) ([]Exchange, error)
}
