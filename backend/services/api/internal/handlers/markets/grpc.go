package markets

import (
	"context"
	"log/slog"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"marketmind/api/internal/domain/markets"
	"marketmind/gen/go/api"
)

type TrackedMarketReader interface {
	// ListTrackedMarkets returns markets ordered by exchange name and market ID.
	ListTrackedMarkets(context.Context) ([]markets.TrackedMarket, error)
}

type CatalogHandler struct {
	api.UnimplementedMarketCatalogServiceServer
	repository TrackedMarketReader
	logger     *slog.Logger
}

func NewCatalogHandler(repository TrackedMarketReader, logger *slog.Logger) *CatalogHandler {
	return &CatalogHandler{repository: repository, logger: logger}
}

func (h *CatalogHandler) GetMarketsToCheck(ctx context.Context, _ *api.GetMarketsToCheckRequest) (*api.GetMarketsToCheckResponse, error) {
	tracked, err := h.repository.ListTrackedMarkets(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		h.logger.ErrorContext(ctx, "list tracked markets", "error", err)
		return nil, status.Error(codes.Unavailable, "Could not list tracked markets")
	}

	response := &api.GetMarketsToCheckResponse{}
	var exchange *api.ExchangeMarkets
	for _, market := range tracked {
		if exchange == nil || exchange.ExchangeName != market.ExchangeName {
			exchange = &api.ExchangeMarkets{ExchangeName: market.ExchangeName}
			response.Items = append(response.Items, exchange)
		}
		exchange.Markets = append(exchange.Markets, &api.MarketToCheck{
			MarketId:   strconv.FormatInt(market.MarketID, 10),
			ExternalId: market.ExternalID,
		})
	}
	return response, nil
}
