package application

import (
	"log/slog"
	"marketmind/api/internal/middleware"
	"net/http"

	"github.com/go-chi/chi"
)

func NewRouter(logger *slog.Logger) *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(logger))
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})
	return router
}
