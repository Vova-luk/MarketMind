package users

import (
	"log/slog"
	"net/http"

	domainusers "marketmind/api/internal/domain/users"
	"marketmind/api/internal/utils/httpresponse"
	"marketmind/api/internal/utils/requestctx"
)

type Handler struct {
	repository domainusers.Repository
	logger     *slog.Logger
}

func NewHandler(repository domainusers.Repository, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	telegramUserID, ok := requestctx.TelegramUserID(ctx)
	if !ok {
		w.Header().Set("WWW-Authenticate", "tma")
		httpresponse.WriteError(w, http.StatusUnauthorized, "TELEGRAM_AUTH_INVALID", "Missing or invalid Telegram initData")
		return
	}
	user := domainusers.User{TelegramUserID: telegramUserID}
	if err := h.repository.Register(ctx, user); err != nil {
		h.logger.ErrorContext(ctx, "register user", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not register user")
		return
	}
	httpresponse.WriteJSON(w, http.StatusOK, registrationResponse{
		TelegramUserID: user.TelegramUserID,
	})
}

func (h *Handler) RequireRegistered(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		telegramUserID, ok := requestctx.TelegramUserID(ctx)
		if !ok {
			w.Header().Set("WWW-Authenticate", "tma")
			httpresponse.WriteError(w, http.StatusUnauthorized, "TELEGRAM_AUTH_INVALID", "Missing or invalid Telegram initData")
			return
		}
		exists, err := h.repository.Exists(ctx, telegramUserID)
		if err != nil {
			h.logger.ErrorContext(ctx, "check user registration", "error", err)
			httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not check user registration")
			return
		}
		if !exists {
			httpresponse.WriteError(w, http.StatusNotFound, "USER_NOT_REGISTERED", "User is not registered")
			return
		}
		next.ServeHTTP(w, r)
	})
}
