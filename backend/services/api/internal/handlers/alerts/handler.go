package alerts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	domain "marketmind/api/internal/domain/alerts"
	"marketmind/api/internal/domain/subscriptions"
	service "marketmind/api/internal/service/alerts"
	"marketmind/api/internal/utils/httpresponse"
	"marketmind/api/internal/utils/requestctx"
	"marketmind/api/internal/utils/requestparams"
)

type AlertWriter interface {
	Create(ctx context.Context, telegramUserID, subscriptionID int64, condition domain.Condition) (service.Result, error)
	Update(ctx context.Context, telegramUserID, subscriptionID, alertID int64, patch service.Patch) (service.Result, error)
	Delete(ctx context.Context, telegramUserID, subscriptionID, alertID int64) error
}

type Handler struct {
	service       AlertWriter
	subscriptions subscriptionReader
	logger        *slog.Logger
}

type subscriptionReader interface {
	ListAlerts(ctx context.Context, telegramUserID, subscriptionID int64, limit int, offset int64) ([]subscriptions.Alert, string, error)
}

func NewHandler(service AlertWriter, subscriptions subscriptionReader, logger *slog.Logger) *Handler {
	return &Handler{service: service, subscriptions: subscriptions, logger: logger}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	subscriptionID, ok := readSubscriptionID(w, r)
	if !ok {
		return
	}
	page, ok := requestparams.ParsePage(r.URL.RawQuery)
	if !ok {
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid pagination parameters")
		return
	}
	ctx := r.Context()
	telegramUserID, _ := requestctx.TelegramUserID(ctx)
	alerts, quoteAsset, err := h.subscriptions.ListAlerts(ctx, telegramUserID, subscriptionID, page.Size+1, page.Offset())
	if errors.Is(err, subscriptions.ErrNotFound) {
		httpresponse.WriteError(w, http.StatusNotFound, "SUBSCRIPTION_NOT_FOUND", "Subscription not found")
		return
	}
	if err != nil {
		h.logger.ErrorContext(ctx, "list alerts", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not list alerts")
		return
	}

	count := min(len(alerts), page.Size)
	items := make([]alertResponse, 0, count)
	for _, alert := range alerts[:count] {
		items = append(items, alertResponse{
			ID: alert.ID, Type: alert.Type, Value: alert.Value,
			Direction:      alert.Direction,
			ReferencePrice: alert.ReferencePrice, TargetPrice: alert.TargetPrice,
			IsActive: alert.IsActive, QuoteAsset: quoteAsset,
		})
	}
	httpresponse.WriteJSON(w, http.StatusOK, struct {
		Items    []alertResponse `json:"items"`
		NextPage *int            `json:"next_page"`
	}{Items: items, NextPage: page.NextPage(len(alerts))})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	subscriptionID, ok := readSubscriptionID(w, r)
	if !ok {
		return
	}
	change, ok := readChange(w, r, false)
	if !ok {
		return
	}
	ctx := r.Context()
	telegramUserID, _ := requestctx.TelegramUserID(ctx)
	result, err := h.service.Create(ctx, telegramUserID, subscriptionID, *change.Condition)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/v1/subscriptions/%d/alerts/%d", subscriptionID, result.ID))
	httpresponse.WriteJSON(w, http.StatusCreated, toResponse(result))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	subscriptionID, alertID, ok := alertIDs(w, r)
	if !ok {
		return
	}
	change, ok := readChange(w, r, true)
	if !ok {
		return
	}
	ctx := r.Context()
	telegramUserID, _ := requestctx.TelegramUserID(ctx)
	result, err := h.service.Update(ctx, telegramUserID, subscriptionID, alertID, change)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpresponse.WriteJSON(w, http.StatusOK, toResponse(result))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	subscriptionID, alertID, ok := alertIDs(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	telegramUserID, _ := requestctx.TelegramUserID(ctx)
	if err := h.service.Delete(ctx, telegramUserID, subscriptionID, alertID); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readSubscriptionID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := requestparams.PathID(r, "subscription_id")
	if !ok {
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid subscription ID")
	}
	return id, ok
}

func alertIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	subscriptionID, ok := readSubscriptionID(w, r)
	if !ok {
		return 0, 0, false
	}
	alertID, ok := requestparams.PathID(r, "alert_id")
	if !ok {
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid alert ID")
	}
	return subscriptionID, alertID, ok
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, subscriptions.ErrNotFound):
		httpresponse.WriteError(w, http.StatusNotFound, "SUBSCRIPTION_NOT_FOUND", "Subscription not found")
	case errors.Is(err, domain.ErrNotFound):
		httpresponse.WriteError(w, http.StatusNotFound, "ALERT_NOT_FOUND", "Alert not found")
	case errors.Is(err, domain.ErrInvalid):
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, domain.ErrExchangeOff):
		httpresponse.WriteError(w, http.StatusConflict, "EXCHANGE_INACTIVE", "Exchange is inactive")
	case errors.Is(err, domain.ErrActiveLimit):
		httpresponse.WriteError(w, http.StatusConflict, "ACTIVE_ALERT_LIMIT_REACHED", "Active alert limit reached")
	case errors.Is(err, domain.ErrQuoteMissing):
		httpresponse.WriteError(w, http.StatusServiceUnavailable, "QUOTE_UNAVAILABLE", "Market price is unavailable")
	default:
		h.logger.ErrorContext(r.Context(), "write alert", "method", r.Method, "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not write alert")
	}
}
