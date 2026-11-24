//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
)

func TestIT11_MarketSearchPagination(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	user := testutil.New(t, ctx).RegisterUser(t, ctx)

	type marketOption struct {
		ExchangeID int    `json:"exchange_id"`
		ExternalID string `json:"external_id"`
		BaseAsset  string `json:"base_asset"`
		QuoteAsset string `json:"quote_asset"`
		PriceExact string `json:"price_exact"`
	}
	want := []marketOption{
		{ExchangeID: bybit.id, ExternalID: "IT11AUSDT", BaseAsset: "IT11A", QuoteAsset: "USDT", PriceExact: "1.25"},
		{ExchangeID: bybit.id, ExternalID: "IT11BUSDT", BaseAsset: "IT11B", QuoteAsset: "USDT", PriceExact: "2.50"},
		{ExchangeID: bybit.id, ExternalID: "IT11CUSDT", BaseAsset: "IT11C", QuoteAsset: "USDT", PriceExact: "3.75"},
	}
	for _, tc := range []struct {
		query string
		page  int
		items []marketOption
		next  int
	}{
		{query: "iT11", page: 1, items: want[:2], next: 2},
		{query: "iT11", page: 2, items: want[2:]},
		{query: "NOIT11MATCH", page: 1, items: []marketOption{}},
	} {
		var response struct {
			Items    []marketOption `json:"items"`
			NextPage *int           `json:"next_page"`
		}
		path := fmt.Sprintf("/api/v1/markets?exchange_id=%d&q=%s&page=%d&limit=2", bybit.id, tc.query, tc.page)
		user.API.RequestJSON(t, ctx, http.MethodGet, path, nil, http.StatusOK, &response)
		if response.Items == nil || !slices.Equal(response.Items, tc.items) {
			t.Fatalf("search %q page %d: got %#v, want %#v", tc.query, tc.page, response.Items, tc.items)
		}
		assertNextPage(t, tc.query, tc.page, response.NextPage, tc.next)
	}
}

func assertNextPage(t testing.TB, source string, page int, got *int, want int) {
	t.Helper()
	if got == nil {
		if want != 0 {
			t.Errorf("%s page %d: got null next_page, want %d", source, page, want)
		}
		return
	}
	if want == 0 || *got != want {
		t.Errorf("%s page %d: got next_page %d, want %d (0 means null)", source, page, *got, want)
	}
}
