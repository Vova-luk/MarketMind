package service

import (
	"fmt"
	"strings"

	"marketmind/api/pkg/alerts"
	"marketmind/contracts/events"
	"marketmind/utils/priceformat"
)

const (
	notificationTemplate = "Сработал алерт!\n" +
		"Монета: %s\n" +
		"Цена при настройке алерта: %s %s\n" +
		"Актуальная цена: %s %s\n" +
		"Условие срабатывания: %s"
	marketTemplate             = "%s/%s"
	marketWithExchangeTemplate = "%s (%s)"
	priceConditionTemplate     = "%s %s %s"
	percentConditionTemplate   = "%s %s%%"
	greaterOrEqualSymbol       = "≥"
	lessOrEqualSymbol          = "≤"
	positiveSign               = "+"
	negativeSign               = "-"
)

func notificationText(item events.PriceNotification) (string, error) {
	for _, field := range []struct{ name, value string }{
		{"base_asset", item.BaseAsset},
		{"quote_asset", item.QuoteAsset},
		{"reference_price", item.ReferencePrice},
		{"current_price", item.CurrentPrice},
		{"value", item.Value},
	} {
		if strings.TrimSpace(field.value) == "" {
			return "", fmt.Errorf("%s is required", field.name)
		}
	}
	if item.TelegramUserID <= 0 {
		return "", fmt.Errorf("telegram_user_id must be positive")
	}

	var operator string
	switch item.Direction {
	case alerts.GreaterOrEqual:
		operator = greaterOrEqualSymbol
	case alerts.LessOrEqual:
		operator = lessOrEqualSymbol
	default:
		return "", fmt.Errorf("unknown alert direction")
	}
	var condition string
	switch item.Type {
	case alerts.Price:
		condition = fmt.Sprintf(priceConditionTemplate, operator, priceformat.TwoDecimals(item.Value), item.QuoteAsset)
	case alerts.Percent:
		value := item.Value
		if !strings.HasPrefix(value, negativeSign) && !strings.HasPrefix(value, positiveSign) {
			value = positiveSign + value
		}
		condition = fmt.Sprintf(percentConditionTemplate, operator, value)
	default:
		return "", fmt.Errorf("unknown alert type")
	}

	market := fmt.Sprintf(marketTemplate, item.BaseAsset, item.QuoteAsset)
	if item.ExchangeName != "" {
		market = fmt.Sprintf(marketWithExchangeTemplate, market, item.ExchangeName)
	}
	return fmt.Sprintf(notificationTemplate, market, priceformat.TwoDecimals(item.ReferencePrice), item.QuoteAsset,
		priceformat.TwoDecimals(item.CurrentPrice), item.QuoteAsset, condition), nil
}
