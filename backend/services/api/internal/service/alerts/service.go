package alerts

import (
	"context"
	"fmt"
	"math/big"

	domain "marketmind/api/internal/domain/alerts"
)

const DefaultActiveLimit = 50

type Service struct {
	repository  domain.Repository
	activeLimit int
}

func NewService(repository domain.Repository, activeLimit int) *Service {
	if activeLimit <= 0 {
		activeLimit = DefaultActiveLimit
	}
	return &Service{repository: repository, activeLimit: activeLimit}
}

type Patch struct {
	Condition *domain.Condition
	IsActive  *bool
}

type Result struct {
	domain.Alert
	TargetPrice string
	QuoteAsset  string
}

func (s *Service) Create(ctx context.Context, telegramUserID, subscriptionID int64, condition domain.Condition) (Result, error) {
	subscription, tx, err := s.repository.BeginSubscription(ctx, telegramUserID, subscriptionID)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)

	if err := validateQuote(subscription); err != nil {
		return Result{}, err
	}
	alert := domain.Alert{Condition: condition, ReferencePrice: subscription.Price, IsActive: true}
	target, err := alert.TargetPrice()
	if err != nil {
		return Result{}, err
	}
	if err := validateTarget(target, alert.ReferencePrice); err != nil {
		return Result{}, err
	}
	if err := s.checkLimit(ctx, tx); err != nil {
		return Result{}, err
	}

	alert.ID, err = tx.Insert(ctx, alert)
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{Alert: alert, TargetPrice: target, QuoteAsset: subscription.QuoteAsset}, nil
}

func (s *Service) Update(ctx context.Context, telegramUserID, subscriptionID, alertID int64, patch Patch) (Result, error) {
	subscription, tx, err := s.repository.BeginSubscription(ctx, telegramUserID, subscriptionID)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)

	current, err := tx.Get(ctx, alertID)
	if err != nil {
		return Result{}, err
	}
	next := current
	if patch.IsActive != nil {
		next.IsActive = *patch.IsActive
	}
	enabling := next.IsActive && !current.IsActive
	targetChanged := patch.Condition != nil && !sameTargetInput(current.Condition, *patch.Condition)
	if patch.Condition != nil {
		next.Direction = patch.Condition.Direction
	}
	conditionChanged := targetChanged || next.Direction != current.Direction
	if conditionChanged || enabling {
		if err := validateQuote(subscription); err != nil {
			return Result{}, err
		}
	}
	if targetChanged {
		next.Condition = *patch.Condition
		next.ReferencePrice = subscription.Price
	}

	target, err := next.TargetPrice()
	if err != nil {
		return Result{}, err
	}
	if targetChanged {
		if err := validateTarget(target, next.ReferencePrice); err != nil {
			return Result{}, err
		}
	}
	if enabling {
		if err := s.checkLimit(ctx, tx); err != nil {
			return Result{}, err
		}
	}
	if conditionChanged || next.IsActive != current.IsActive {
		if err := tx.Update(ctx, next); err != nil {
			return Result{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{Alert: next, TargetPrice: target, QuoteAsset: subscription.QuoteAsset}, nil
}

func (s *Service) Delete(ctx context.Context, telegramUserID, subscriptionID, alertID int64) error {
	_, tx, err := s.repository.BeginSubscription(ctx, telegramUserID, subscriptionID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := tx.Delete(ctx, alertID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) checkLimit(ctx context.Context, tx domain.Transaction) error {
	count, err := tx.CountActive(ctx)
	if err != nil {
		return err
	}
	if count >= s.activeLimit {
		return domain.ErrActiveLimit
	}
	return nil
}

func validateQuote(subscription domain.Subscription) error {
	if !subscription.ExchangeActive {
		return domain.ErrExchangeOff
	}
	price, ok := domain.ParseDecimal(subscription.Price)
	if !ok || price.Sign() <= 0 {
		return domain.ErrQuoteMissing
	}
	return nil
}

func validateTarget(target, referencePrice string) error {
	value, _ := domain.ParseDecimal(target)
	reference, _ := domain.ParseDecimal(referencePrice)
	if value.Sign() <= 0 || value.Cmp(big.NewRat(500000, 1)) > 0 || value.Cmp(reference) == 0 {
		return fmt.Errorf("%w: target must be in (0, 500000] and differ from the reference price", domain.ErrInvalid)
	}
	return nil
}

func sameTargetInput(old, current domain.Condition) bool {
	if old.Type != current.Type {
		return false
	}
	oldValue, oldOK := domain.ParseDecimal(old.Value)
	newValue, newOK := domain.ParseDecimal(current.Value)
	return oldOK && newOK && oldValue.Cmp(newValue) == 0
}
