package subscriptions

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	domain "marketmind/api/internal/domain/subscriptions"
	"marketmind/api/internal/httpresponse"
	"marketmind/api/internal/requestctx"
)

func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	key, seed, ok := readAddRequest(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	telegramUserID, _ := requestctx.TelegramUserID(ctx)
	subscription, created, err := h.service.Add(ctx, telegramUserID, key, seed)
	if err != nil {
		h.writeAddError(w, r, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
		w.Header().Set("Location", fmt.Sprintf("/api/v1/subscriptions/%d", subscription.ID))
	}
	httpresponse.WriteJSON(w, status, toSubscriptionResponse(subscription))
}

func readAddRequest(w http.ResponseWriter, r *http.Request) (domain.MarketKey, domain.MarketSeed, bool) {
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) {
		httpresponse.WriteError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Expected application/json")
		return domain.MarketKey{}, domain.MarketSeed{}, false
	}

	var body struct {
		ExchangeID  int32  `json:"exchange_id"`
		ExternalID  string `json:"external_id"`
		BaseAsset   string `json:"base_asset"`
		QuoteAsset  string `json:"quote_asset"`
		Price       string `json:"price"`
		DisplayName string `json:"display_name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid subscription body")
		return domain.MarketKey{}, domain.MarketSeed{}, false
	}
	key := domain.MarketKey{ExchangeID: body.ExchangeID, ExternalID: body.ExternalID}
	if !key.Valid() {
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid exchange_id or external_id")
		return domain.MarketKey{}, domain.MarketSeed{}, false
	}
	return key, domain.MarketSeed{BaseAsset: body.BaseAsset, QuoteAsset: body.QuoteAsset, Price: body.Price}, true
}

func (h *Handler) writeAddError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrUserNotRegistered):
		httpresponse.WriteError(w, http.StatusNotFound, "USER_NOT_REGISTERED", "User is not registered")
	case errors.Is(err, domain.ErrExchangeNotFound):
		httpresponse.WriteError(w, http.StatusNotFound, "EXCHANGE_NOT_FOUND", "Exchange not found")
	case errors.Is(err, domain.ErrExchangeInactive):
		httpresponse.WriteError(w, http.StatusConflict, "EXCHANGE_INACTIVE", "Exchange is inactive")
	case errors.Is(err, domain.ErrLimitReached):
		httpresponse.WriteError(w, http.StatusConflict, "SUBSCRIPTION_LIMIT_REACHED", "Subscription limit reached")
	case errors.Is(err, domain.ErrMarketConflict):
		httpresponse.WriteError(w, http.StatusConflict, "MARKET_CONFLICT", "Pair already exists with another external ID")
	case errors.Is(err, domain.ErrInvalidMarketData):
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	default:
		h.logger.ErrorContext(r.Context(), "add subscription", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not add subscription")
	}
}
