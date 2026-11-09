package exchanges

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("active exchange not found")

type Exchange struct {
	ID   int32
	Name string
}

type Repository interface {
	ListActive(ctx context.Context) ([]Exchange, error)
}
