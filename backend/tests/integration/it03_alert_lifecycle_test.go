//go:build integration

package integration

import (
	"context"
	"fmt"
	"marketmind-tests/integration/utils"
	"marketmind-tests/internal/testutil"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type AlertResponse struct {
	ID             int64  `json:"alert_id,string"`
	Type           string `json:"type"`
	Value          string `json:"value"`
	Direction      string `json:"direction"`
	ReferencePrice string `json:"reference_price"`
	TargetPrice    string `json:"target_price"`
	IsActive       bool   `json:"is_active"`
	QuoteAsset     string `json:"quote_asset"`
}

func Test03_Alert_Lifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	env := testutil.New(t, ctx)
	utils.SeedBTCUSDTMarket(t, ctx, env.DB)
	user := env.RegisterUser(t, ctx)

	var created subscriptionResponse
	user.API.RequestJSON(t, ctx, http.MethodPost, utils.SubscriptionsPath, map[string]any{
		"exchange_id": utils.BinanceExchangeID,
		"external_id": utils.BtcusdtExternalID,
	}, http.StatusCreated, &created)

	want := subscriptionResponse{
		ID: created.ID, BaseAsset: "BTC", QuoteAsset: "USDT", ExchangeName: "Binance",
	}

	if created.ID <= 0 || created != want {
		t.Fatalf("created subscription: got %+v, want a positive ID and %+v", created, want)
	}

	path := fmt.Sprintf("/api/v1/subscriptions/%d/alerts", created.ID)
	var alertCreated AlertResponse
	user.API.RequestJSON(t, ctx, http.MethodPost, path, map[string]any{
		"type":      "price",
		"value":     "70000",
		"direction": "gte",
	}, http.StatusCreated, &alertCreated)

	var listAfterCreate struct {
		Items []AlertResponse `json:"items"`
	}

	user.API.RequestJSON(t, ctx, http.MethodGet, path, nil, http.StatusOK, &listAfterCreate)

	if len(listAfterCreate.Items) != 1 {
		t.Fatalf("alerts after create: got %d, want 1", len(listAfterCreate.Items))
	}
	if listAfterCreate.Items[0] != alertCreated {
		t.Fatalf("alert after creation: got %+v, want %+v",
			listAfterCreate.Items[0], alertCreated)
	}

	AssertStoredAlert(t, ctx, env.DB, alertCreated.ID, StoredAlert{
		SubscriptionID: created.ID,
		Type:           "price",
		Value:          "70000",
		Direction:      "gte",
		IsActive:       true,
	})

	updatePath := fmt.Sprintf("%s/%d", path, alertCreated.ID)

	var changedAlert AlertResponse
	user.API.RequestJSON(t, ctx, http.MethodPatch, updatePath, map[string]any{
		"type":      "price",
		"value":     "80000",
		"direction": "gte",
	}, http.StatusOK, &changedAlert)

	if changedAlert.ID != alertCreated.ID ||
		changedAlert.Type != "price" ||
		changedAlert.Value != "80000" ||
		changedAlert.Direction != "gte" ||
		!changedAlert.IsActive {
		t.Fatalf("unexpected updated alert: %+v", changedAlert)
	}

	var listAfterChange struct {
		Items []AlertResponse `json:"items"`
	}

	user.API.RequestJSON(t, ctx, http.MethodGet, path, nil, http.StatusOK, &listAfterChange)

	if len(listAfterChange.Items) != 1 {
		t.Fatalf("alerts after update: got %d, want 1", len(listAfterChange.Items))
	}
	if listAfterChange.Items[0] != changedAlert {
		t.Fatalf("alert after update: got %+v, want %+v",
			listAfterChange.Items[0], changedAlert)
	}

	AssertStoredAlert(t, ctx, env.DB, alertCreated.ID, StoredAlert{
		SubscriptionID: created.ID,
		Type:           "price",
		Value:          "80000",
		Direction:      "gte",
		IsActive:       true,
	})

	user.API.RequestJSON(t, ctx, http.MethodDelete, updatePath, nil, http.StatusNoContent, nil)

	var list struct {
		Items []AlertResponse `json:"items"`
	}

	user.API.RequestJSON(t, ctx, http.MethodGet, path, nil, http.StatusOK, &list)

	if len(list.Items) != 0 {
		t.Fatalf("alerts after deletion: got %d, want 0", len(list.Items))
	}

	var exists bool
	err := env.DB.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM market_mind.alerts WHERE id = $1)", alertCreated.ID).Scan(&exists)
	if err != nil {
		t.Fatalf("check deleted alert in PostgreSQL: %v", err)
	}
	if exists {
		t.Fatalf("alert %d remains in PostgreSQL after deletion", alertCreated.ID)
	}
}

type StoredAlert struct {
	SubscriptionID int64
	Type           string
	Value          string
	Direction      string
	IsActive       bool
}

func AssertStoredAlert(t *testing.T, ctx context.Context, db *pgx.Conn, alertID int64, want StoredAlert) {
	t.Helper()
	if alertID <= 0 {
		t.Fatalf("invalid alert ID: %d", alertID)
	}

	var got StoredAlert
	err := db.QueryRow(ctx, `
        SELECT subscription_id, type, value::text, direction, is_active
        FROM market_mind.alerts
        WHERE id = $1
    `, alertID).Scan(&got.SubscriptionID, &got.Type, &got.Value, &got.Direction, &got.IsActive)
	if err != nil {
		t.Fatalf("read alert %d in PostgreSQL: %v", alertID, err)
	}
	if got != want {
		t.Fatalf("alert %d in PostgreSQL: got %+v, want %+v", alertID, got, want)
	}
}
