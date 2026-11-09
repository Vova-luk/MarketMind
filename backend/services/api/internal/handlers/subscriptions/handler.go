package subscriptions

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"

	domainsubscriptions "marketmind/api/internal/domain/subscriptions"
	"marketmind/api/internal/httpresponse"
	"marketmind/api/internal/requestctx"
)

type Handler struct {
	repository domainsubscriptions.Repository
	logger     *slog.Logger
}

func NewHandler(repository domainsubscriptions.Repository, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// RequireRegistered has already checked the authenticated identity.
	telegramUserID, _ := requestctx.TelegramUserID(ctx)
	subscriptions, err := h.repository.List(ctx, telegramUserID)
	if err != nil {
		h.logger.ErrorContext(ctx, "list subscriptions", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not list subscriptions")
		return
	}

	items := make([]subscriptionResponse, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		items = append(items, toSubscriptionResponse(subscription))
	}
	httpresponse.WriteJSON(w, http.StatusOK, struct {
		Items []subscriptionResponse `json:"items"`
	}{Items: items})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "subscription_id")
	subscriptionID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || subscriptionID <= 0 || strconv.FormatInt(subscriptionID, 10) != rawID {
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
