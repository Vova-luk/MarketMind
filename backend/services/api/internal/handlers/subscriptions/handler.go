package subscriptions

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	domainsubscriptions "marketmind/api/internal/domain/subscriptions"
	"marketmind/api/internal/utils/httpresponse"
	"marketmind/api/internal/utils/requestctx"
	"marketmind/api/internal/utils/requestparams"
)

type SubscriptionAdder interface {
	Add(ctx context.Context, telegramUserID int64, key domainsubscriptions.MarketKey, seed domainsubscriptions.MarketSeed) (domainsubscriptions.Subscription, bool, error)
}

type Handler struct {
	repository domainsubscriptions.Repository
	service    SubscriptionAdder
	logger     *slog.Logger
}

func NewHandler(repository domainsubscriptions.Repository, service SubscriptionAdder, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, service: service, logger: logger}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, ok := requestparams.ParsePage(r.URL.RawQuery)
	if !ok {
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid pagination parameters")
		return
	}
	ctx := r.Context()
	// RequireRegistered has already checked the authenticated identity.
	telegramUserID, _ := requestctx.TelegramUserID(ctx)
	subscriptions, err := h.repository.List(ctx, telegramUserID, page.Size+1, page.Offset())
	if err != nil {
		h.logger.ErrorContext(ctx, "list subscriptions", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not list subscriptions")
		return
	}

	count := min(len(subscriptions), page.Size)
	items := make([]subscriptionResponse, 0, count)
	for _, subscription := range subscriptions[:count] {
		items = append(items, toSubscriptionResponse(subscription))
	}
	httpresponse.WriteJSON(w, http.StatusOK, struct {
		Items    []subscriptionResponse `json:"items"`
		NextPage *int                   `json:"next_page"`
	}{Items: items, NextPage: page.NextPage(len(subscriptions))})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	subscriptionID, ok := requestparams.PathID(r, "subscription_id")
	if !ok {
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid subscription ID")
		return
	}
	ctx := r.Context()
	telegramUserID, _ := requestctx.TelegramUserID(ctx)
	detail, err := h.repository.Get(ctx, telegramUserID, subscriptionID)
	if errors.Is(err, domainsubscriptions.ErrNotFound) {
		httpresponse.WriteError(w, http.StatusNotFound, "SUBSCRIPTION_NOT_FOUND", "Subscription not found")
		return
	}
	if err != nil {
		h.logger.ErrorContext(ctx, "get subscription", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not get subscription")
		return
	}
	httpresponse.WriteJSON(w, http.StatusOK, toDetailResponse(detail))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	subscriptionID, ok := requestparams.PathID(r, "subscription_id")
	if !ok {
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid subscription ID")
		return
	}
	ctx := r.Context()
	telegramUserID, _ := requestctx.TelegramUserID(ctx)
	err := h.repository.Delete(ctx, telegramUserID, subscriptionID)
	if errors.Is(err, domainsubscriptions.ErrNotFound) {
		httpresponse.WriteError(w, http.StatusNotFound, "SUBSCRIPTION_NOT_FOUND", "Subscription not found")
		return
	}
	if err != nil {
		h.logger.ErrorContext(ctx, "delete subscription", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not delete subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
