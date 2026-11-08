package users

import "context"

type Repository interface {
	Register(ctx context.Context, user User) error
}
