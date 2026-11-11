package subscriptions

import (
	"context"

	domain "marketmind/api/internal/domain/subscriptions"
)

const defaultSubscriptionLimit = 50

type Service struct {
	repository domain.AddRepository
	limit      int
}

func NewService(repository domain.AddRepository, limit int) *Service {
	if limit <= 0 {
		limit = defaultSubscriptionLimit
	}
	return &Service{repository: repository, limit: limit}
}

func (s *Service) Add(ctx context.Context, telegramUserID int64, key domain.MarketKey, seed domain.MarketSeed) (domain.Subscription, bool, error) {
	tx, err := s.repository.BeginAdd(ctx, telegramUserID)
	if err != nil {
		return domain.Subscription{}, false, err
	}
	defer tx.Rollback(ctx)

	exchange, err := tx.GetExchange(ctx, key.ExchangeID)
	if err != nil {
		return domain.Subscription{}, false, err
	}
	market, err := tx.FindMarket(ctx, key, exchange)
	if err != nil {
		return domain.Subscription{}, false, err
	}
	if market.Subscription.ID != 0 {
		return market.Subscription, false, nil
	}
	if !exchange.Active {
		return domain.Subscription{}, false, domain.ErrExchangeInactive
	}

	count, err := tx.CountSubscriptions(ctx)
	if err != nil {
		return domain.Subscription{}, false, err
	}
	if count >= s.limit {
		return domain.Subscription{}, false, domain.ErrLimitReached
	}

	if market.MarketID == 0 {
		if err := seed.Validate(); err != nil {
			return domain.Subscription{}, false, err
		}
		market, err = tx.InsertMarket(ctx, key, exchange, seed)
		if err != nil {
			return domain.Subscription{}, false, err
		}
	}
	market.Subscription.ID, err = tx.InsertSubscription(ctx, market.MarketID)
	if err != nil {
		return domain.Subscription{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Subscription{}, false, err
	}
	return market.Subscription, true, nil
}
