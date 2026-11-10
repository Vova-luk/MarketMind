package exchanges

import (
	"log/slog"
	"net/http"

	domainexchanges "marketmind/api/internal/domain/exchanges"
	"marketmind/api/internal/utils/httpresponse"
)

type Handler struct {
	repository domainexchanges.Repository
	logger     *slog.Logger
}

func NewHandler(repository domainexchanges.Repository, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

type exchangeResponse struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	exchanges, err := h.repository.ListActive(ctx)
	if err != nil {
		h.logger.ErrorContext(ctx, "list exchanges", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not list exchanges")
		return
	}

	items := make([]exchangeResponse, 0, len(exchanges))
	for _, exchange := range exchanges {
		items = append(items, exchangeResponse{ID: exchange.ID, Name: exchange.Name})
	}
	httpresponse.WriteJSON(w, http.StatusOK, struct {
		Items []exchangeResponse `json:"items"`
	}{Items: items})
}
