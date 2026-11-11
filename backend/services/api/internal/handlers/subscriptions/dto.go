package subscriptions

import (
	"time"

	domainsubscriptions "marketmind/api/internal/domain/subscriptions"
	"marketmind/api/pkg/alerts"
	"marketmind/utils/priceformat"
)

type subscriptionResponse struct {
	ID           int64   `json:"subscription_id,string"`
	BaseAsset    string  `json:"base_asset"`
	QuoteAsset   string  `json:"quote_asset"`
	ExchangeName string  `json:"exchange_name"`
	Price        *string `json:"price"`
	PriceExact   *string `json:"price_exact"`
}

type detailResponse struct {
	subscriptionResponse
	UpdatedAt time.Time       `json:"updated_at"`
	Alerts    []alertResponse `json:"alerts"`
}

type alertResponse struct {
	ID             int64            `json:"alert_id,string"`
	Type           alerts.Kind      `json:"type"`
	Value          string           `json:"value"`
	Direction      alerts.Direction `json:"direction"`
	ReferencePrice string           `json:"reference_price"`
	TargetPrice    string           `json:"target_price"`
	IsActive       bool             `json:"is_active"`
}

func toSubscriptionResponse(subscription domainsubscriptions.Subscription) subscriptionResponse {
	return subscriptionResponse{
		ID:           subscription.ID,
		BaseAsset:    subscription.BaseAsset,
		QuoteAsset:   subscription.QuoteAsset,
		ExchangeName: subscription.ExchangeName,
		Price:        priceformat.OptionalTwoDecimals(subscription.Price),
		PriceExact:   subscription.Price,
	}
}

func toDetailResponse(detail domainsubscriptions.Detail) detailResponse {
	alerts := make([]alertResponse, 0, len(detail.Alerts))
	for _, alert := range detail.Alerts {
		response := alertResponse(alert)
		response.ReferencePrice = priceformat.TwoDecimals(response.ReferencePrice)
		response.TargetPrice = priceformat.TwoDecimals(response.TargetPrice)
		alerts = append(alerts, response)
	}
	return detailResponse{
		subscriptionResponse: toSubscriptionResponse(detail.Subscription),
		UpdatedAt:            detail.UpdatedAt.UTC(),
		Alerts:               alerts,
	}
}
