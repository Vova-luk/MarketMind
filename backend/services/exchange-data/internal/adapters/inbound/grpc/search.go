package grpc

import (
	"context"
	"errors"
	"log/slog"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"marketmind/exchange-data/internal/service"
	"marketmind/gen/go/exchangedata"
)

type SearchHandler struct {
	exchangedata.UnimplementedMarketSearchServiceServer
	search *service.SearchService
	logger *slog.Logger
}

func NewSearchHandler(search *service.SearchService, logger *slog.Logger) *SearchHandler {
	return &SearchHandler{search: search, logger: logger}
}

func (h *SearchHandler) SearchMarkets(ctx context.Context, req *exchangedata.SearchMarketsRequest) (*exchangedata.SearchMarketsResponse, error) {
	page, limit := int32(1), int32(20)
	if req.Page != nil {
		page = req.GetPage()
	}
	if req.Limit != nil {
		limit = req.GetLimit()
	}
	if page < 1 || limit < 1 || limit > 100 || utf8.RuneCountInString(req.GetQuery()) > 120 {
		return nil, status.Error(codes.InvalidArgument, "Invalid market search parameters")
	}

	result, err := h.search.Search(ctx, req.GetExchangeName(), req.GetQuery(), page, limit)
	if errors.Is(err, service.ErrUnknownExchange) {
		return nil, status.Error(codes.InvalidArgument, "Unknown exchange")
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		h.logger.ErrorContext(ctx, "search exchange markets", "error", err)
		return nil, status.Error(codes.Unavailable, "Market catalog is unavailable")
	}

	response := &exchangedata.SearchMarketsResponse{
		Items:    make([]*exchangedata.Market, 0, len(result.Items)),
		NextPage: result.NextPage,
	}
	for _, item := range result.Items {
		response.Items = append(response.Items, &exchangedata.Market{
			ExternalId: item.ExternalID,
			BaseAsset:  item.BaseAsset,
			QuoteAsset: item.QuoteAsset,
			Price:      item.Price,
		})
	}
	return response, nil
}
