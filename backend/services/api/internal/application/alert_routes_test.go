package application

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "marketmind/api/internal/domain/alerts"
	"marketmind/api/internal/domain/subscriptions"
	alerthandlers "marketmind/api/internal/handlers/alerts"
	userhandlers "marketmind/api/internal/handlers/users"
	alertservice "marketmind/api/internal/service/alerts"
	"marketmind/api/pkg/alerts"
)

type alertStoreStub struct {
	subscription domain.Subscription
	alert        domain.Alert
}

func (s *alertStoreStub) BeginSubscription(_ context.Context, telegramUserID, subscriptionID int64) (domain.Subscription, domain.Transaction, error) {
	if telegramUserID != 123 || subscriptionID != 18 {
		return domain.Subscription{}, nil, subscriptions.ErrNotFound
	}
	return s.subscription, s, nil
}

func (s *alertStoreStub) Commit(context.Context) error   { return nil }
func (s *alertStoreStub) Rollback(context.Context) error { return nil }

func (s *alertStoreStub) Get(_ context.Context, alertID int64) (domain.Alert, error) {
	if s.alert.ID == 0 || s.alert.ID != alertID {
		return domain.Alert{}, domain.ErrNotFound
	}
	return s.alert, nil
}

func (s *alertStoreStub) Insert(_ context.Context, alert domain.Alert) (int64, error) {
	alert.ID = 9
	s.alert = alert
	return alert.ID, nil
}

func (s *alertStoreStub) Update(_ context.Context, alert domain.Alert) error {
	s.alert = alert
	return nil
}

func (s *alertStoreStub) Delete(ctx context.Context, alertID int64) error {
	if _, err := s.Get(ctx, alertID); err != nil {
		return err
	}
	s.alert = domain.Alert{}
	return nil
}

func (s *alertStoreStub) CountActive(context.Context) (int, error) {
	if s.alert.IsActive {
		return 1, nil
	}
	return 0, nil
}

func TestAlertMutationRoutes(t *testing.T) {
	const botToken = "test-token"
	const collection = "/api/v1/subscriptions/18/alerts"
	const resource = collection + "/9"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := &alertStoreStub{subscription: domain.Subscription{
		Price: "83781.00000000000001", QuoteAsset: "USDT", ExchangeActive: true,
	}}
	router := NewRouter(logger, botToken, userhandlers.NewHandler(registerUserStub(nil), logger), nil, nil,
		alerthandlers.NewHandler(alertservice.NewService(store, 1), nil, logger), nil)
	request := func(method, path, body string, userID int64, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", signedTelegramHeader(botToken, userID))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, res.Code, status, res.Body)
		}
		return res
	}
	assertDirection := func(res *httptest.ResponseRecorder, target string, direction alerts.Direction) {
		t.Helper()
		var body struct {
			TargetPrice string           `json:"target_price"`
			Direction   alerts.Direction `json:"direction"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || body.TargetPrice != target || body.Direction != direction {
			t.Fatalf("want target %s and direction %s, got %s (%v)", target, direction, res.Body, err)
		}
		if store.alert.Direction != direction {
			t.Fatalf("stored direction %q, want %q", store.alert.Direction, direction)
		}
	}

	const createBody = `{"type":"percent","value":"5","direction":"lte"}`
	const firstTarget = "87970.0500000000000105"
	res := request(http.MethodPost, collection, createBody, 123, http.StatusCreated)
	assertDirection(res, firstTarget, alerts.LessOrEqual)
	if res.Header().Get("Location") != resource {
		t.Fatalf("unexpected Location: %q", res.Header().Get("Location"))
	}
	request(http.MethodPost, collection, createBody, 123, http.StatusConflict)
	request(http.MethodPost, collection, `{"type":"price","value":"0","direction":"gte"}`, 999, http.StatusBadRequest)
	request(http.MethodPost, collection, `{"type":"price","value":"100"}`, 123, http.StatusBadRequest)
	request(http.MethodPatch, resource, `{"type":"percent","value":"-100","direction":"lte"}`, 999, http.StatusBadRequest)
	request(http.MethodPatch, resource, `{"type":"percent","value":"5","direction":"up"}`, 123, http.StatusBadRequest)
	request(http.MethodPatch, resource, `{"is_active":null}`, 123, http.StatusBadRequest)
	request(http.MethodDelete, resource, "", 999, http.StatusNotFound)

	store.subscription.Price = "90000"
	store.subscription.ExchangeActive = false
	assertDirection(request(http.MethodPatch, resource, `{"type":"percent","value":"5.0","direction":"lte"}`, 123, http.StatusOK), firstTarget, alerts.LessOrEqual)
	assertDirection(request(http.MethodPatch, resource, `{"is_active":false}`, 123, http.StatusOK), firstTarget, alerts.LessOrEqual)
	request(http.MethodPatch, resource, `{"is_active":true}`, 123, http.StatusConflict)
	store.subscription.ExchangeActive = true
	assertDirection(request(http.MethodPatch, resource, `{"is_active":true}`, 123, http.StatusOK), firstTarget, alerts.LessOrEqual)
	assertDirection(request(http.MethodPatch, resource, `{"type":"percent","value":"5","direction":"gte"}`, 123, http.StatusOK), firstTarget, alerts.GreaterOrEqual)
	assertDirection(request(http.MethodPatch, resource, `{"type":"percent","value":"-5","direction":"gte"}`, 123, http.StatusOK), "85500", alerts.GreaterOrEqual)

	res = request(http.MethodDelete, resource, "", 123, http.StatusNoContent)
	if res.Body.Len() != 0 {
		t.Fatalf("delete returned a response body: %s", res.Body)
	}
	request(http.MethodDelete, resource, "", 123, http.StatusNotFound)
}
