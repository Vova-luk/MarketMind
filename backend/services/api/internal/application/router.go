package application

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi"

	"marketmind/api/internal/handlers/users"
	"marketmind/api/internal/middleware"
)

func NewRouter(logger *slog.Logger, userHandler *users.Handler, botToken string) *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(logger))
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(middleware.AuthenticateTelegram(botToken))
		api.Post("/users/me", userHandler.Register)
	})
	return router
}
