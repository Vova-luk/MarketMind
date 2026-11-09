package exchangedata

import (
	"context"
	"strings"

	"marketmind/api/internal/domain/exchanges"
	"marketmind/api/internal/domain/markets"
)

// Stub serves a fixed catalog until the Exchange Data search endpoint is available.
type Stub struct{}

func (Stub) Search(_ context.Context, exchange exchanges.Exchange, query string) ([]markets.Instrument, error) {
	fixtures := []struct{ base, quote, price string }{
		{"BTC", "USDT", "83781.50"},
		{"BTC", "EUR", "77000"},
		{"ETH", "USDT", "2400.25"},
		{"SOL", "USDT", "155.80"},
	}
	separator := ""
	if strings.EqualFold(exchange.Name, "OKX") {
		separator = "-"
	}
	query = strings.ToUpper(query)
	items := make([]markets.Instrument, 0, len(fixtures))
	for _, fixture := range fixtures {
		id := fixture.base + separator + fixture.quote
		label := fixture.base + "/" + fixture.quote
		if strings.Contains(id, query) || strings.Contains(label, query) {
			price := fixture.price
			items = append(items, markets.Instrument{
				ExternalID: id, BaseAsset: fixture.base, QuoteAsset: fixture.quote, Price: &price,
			})
		}
	}
	return items, nil
}
