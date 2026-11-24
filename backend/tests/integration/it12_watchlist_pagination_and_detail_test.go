//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
)

func TestIT12_WatchlistPaginationAndDetail(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	env := testutil.New(t, ctx)
	var markets []marketFixture
	for range 3 {
		markets = append(markets, seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "100.125")))
	}
	owner := env.RegisterUser(t, ctx)
	otherUser := env.RegisterUser(t, ctx)
	first := addSubscription(t, ctx, owner, markets[0])
	addSubscription(t, ctx, otherUser, markets[0])
	second := addSubscription(t, ctx, owner, markets[1])
	third := addSubscription(t, ctx, owner, markets[2])
	alert := addRisingPriceAlert(t, ctx, owner, third.ID, "110")

	for i, want := range []struct {
		items []subscriptionResponse
		next  int
	}{
		{items: []subscriptionResponse{first, second}, next: 2},
		{items: []subscriptionResponse{third}},
	} {
		page := i + 1
		var response struct {
			Items    []subscriptionResponse `json:"items"`
			NextPage *int                   `json:"next_page"`
		}
		path := fmt.Sprintf("%s?page=%d&page_size=2", subscriptionsPath, page)
		owner.API.RequestJSON(t, ctx, http.MethodGet, path, nil, http.StatusOK, &response)
		if !slices.Equal(response.Items, want.items) {
			t.Fatalf("watchlist page %d: got %+v, want %+v", page, response.Items, want.items)
		}
		assertNextPage(t, "watchlist", page, response.NextPage, want.next)
	}

	var detail struct {
		subscriptionResponse
		Price      string `json:"price"`
		PriceExact string `json:"price_exact"`
		Alerts     []struct {
			ID          int64  `json:"alert_id,string"`
			TargetPrice string `json:"target_price"`
			IsActive    bool   `json:"is_active"`
		} `json:"alerts"`
	}
	owner.API.RequestJSON(t, ctx, http.MethodGet, fmt.Sprintf("%s/%d", subscriptionsPath, third.ID), nil, http.StatusOK, &detail)
	if detail.subscriptionResponse != third || detail.Price != "100.13" || detail.PriceExact != "100.125" {
		t.Fatalf("subscription detail: got %+v, want subscription %+v with displayed price 100.13 and exact price 100.125", detail, third)
	}
	if len(detail.Alerts) != 1 || detail.Alerts[0].ID != alert.ID || detail.Alerts[0].TargetPrice != "110.00" || !detail.Alerts[0].IsActive {
		t.Fatalf("subscription detail alerts: got %+v, want active alert %d at 110.00", detail.Alerts, alert.ID)
	}
}
