//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
)

func TestIT05_GRPCQuoteThroughAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	user := testutil.New(t, ctx).RegisterUser(t, ctx)
	
	type quote struct {
		ExchangeID  int    `json:"exchange_id"`
		ExternalID  string `json:"external_id"`
		BaseAsset   string `json:"base_asset"`
		QuoteAsset  string `json:"quote_asset"`
		DisplayName string `json:"display_name"`
		Price       string `json:"price"`
		PriceExact  string `json:"price_exact"`
	}
	var response struct {
		Items    []quote `json:"items"`
		NextPage *int32  `json:"next_page"`
	}
	path := fmt.Sprintf("/api/v1/markets?exchange_id=%d&q=BTCUSDT", bybit.id)
	user.API.RequestJSON(t, ctx, http.MethodGet, path, nil, http.StatusOK, &response)

	want := quote{
		ExchangeID: bybit.id, ExternalID: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT",
		DisplayName: "BTC/USDT", Price: "65000.50", PriceExact: "65000.501234",
	}
	if len(response.Items) != 1 {
		t.Fatalf("Bybit BTC/USDT search: got %+v, want exactly one quote", response.Items)
	}
	if response.Items[0] != want {
		t.Errorf("quote through API and gRPC: got %+v, want %+v", response.Items[0], want)
	}
	if response.NextPage != nil {
		t.Errorf("single quote: unexpected next page %d", *response.NextPage)
	}
}
