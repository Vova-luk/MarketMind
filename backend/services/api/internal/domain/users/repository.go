package users

import "context"

type Repository interface {
	Register(ctx context.Context, user User) error
	Exists(ctx context.Context, telegramUserID int64) (bool, error)
}
