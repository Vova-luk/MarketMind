package users

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	domainusers "marketmind/api/internal/domain/users"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Register(ctx context.Context, user domainusers.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO market_mind.users (telegram_user_id)
		VALUES ($1)
		ON CONFLICT (telegram_user_id) DO NOTHING
	`, user.TelegramUserID)
	if err != nil {
		return fmt.Errorf("register Telegram user: %w", err)
	}
	return nil
}

func (r *Repository) Exists(ctx context.Context, telegramUserID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM market_mind.users WHERE telegram_user_id = $1)
	`, telegramUserID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("look up Telegram user: %w", err)
	}
	return exists, nil
}
