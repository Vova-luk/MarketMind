package requestctx

import "context"

type telegramUserIDKey struct{}

func WithTelegramUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, telegramUserIDKey{}, id)
}

func TelegramUserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(telegramUserIDKey{}).(int64)
	return id, ok
}
