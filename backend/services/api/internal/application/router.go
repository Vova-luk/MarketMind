package application

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi"

	"marketmind/api/internal/handlers/alerts"
	"marketmind/api/internal/handlers/exchanges"
	"marketmind/api/internal/handlers/markets"
	"marketmind/api/internal/handlers/subscriptions"
	"marketmind/api/internal/handlers/users"
	"marketmind/api/internal/middleware"
)

func NewRouter(logger *slog.Logger, botToken string, userHandler *users.Handler, exchangeHandler *exchanges.Handler, subscriptionHandler *subscriptions.Handler, alertHandler *alerts.Handler, marketHandler *markets.Handler) *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(logger))
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(middleware.AuthenticateTelegram(botToken))
		api.Post("/users/me", userHandler.Register)
		api.Group(func(registered chi.Router) {
			registered.Use(userHandler.RequireRegistered)
			registered.Get("/markets", marketHandler.Search)
			registered.Get("/exchanges", exchangeHandler.List)
			registered.Get("/subscriptions", subscriptionHandler.List)
			registered.Post("/subscriptions", subscriptionHandler.Add)
			registered.Get("/subscriptions/{subscription_id}", subscriptionHandler.Get)
			registered.Delete("/subscriptions/{subscription_id}", subscriptionHandler.Delete)
			registered.Get("/subscriptions/{subscription_id}/alerts", alertHandler.List)
			registered.Post("/subscriptions/{subscription_id}/alerts", alertHandler.Create)
			registered.Patch("/subscriptions/{subscription_id}/alerts/{alert_id}", alertHandler.Update)
			registered.Delete("/subscriptions/{subscription_id}/alerts/{alert_id}", alertHandler.Delete)
		})
	})
	return router
}
