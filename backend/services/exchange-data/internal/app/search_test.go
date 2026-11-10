package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"marketmind/exchange-data/internal/config"
	"marketmind/gen/go/exchangedata"
)

func TestSearchMarketsThroughGRPC(t *testing.T) {
	var binanceSymbols []string
	for i := 24; i >= 0; i-- {
		binanceSymbols = append(binanceSymbols, fmt.Sprintf(`{"symbol":"B%02dUSDT","baseAsset":"B%02d","quoteAsset":"USDT","status":"TRADING","isSpotTradingAllowed":true}`, i, i))
	}
	binanceSymbols = append(binanceSymbols, binanceSymbols[24],
		`{"symbol":"B00EUR","baseAsset":"B00","quoteAsset":"EUR","status":"HALT","isSpotTradingAllowed":true}`)
	fixtures := map[string]string{
		"/api/v3/exchangeInfo?permissions=SPOT&showPermissionSets=false&symbolStatus=TRADING": `{"symbols":[` + strings.Join(binanceSymbols, ",") + `]}`,
		"/api/v3/ticker/price?": `[{"symbol":"B00USDT","price":"0.12000000"},{"symbol":"B01USDT","price":"0"}]`,
		"/v5/market/instruments-info?category=spot&status=Trading": `{"retCode":0,"result":{"list":[{"symbol":"BTCUSDT","baseCoin":"BTC","quoteCoin":"USDT","status":"Trading"},{"symbol":"BTCEUR","baseCoin":"BTC","quoteCoin":"EUR","status":"PendingOpen"}]}}`,
		"/v5/market/tickers?category=spot":                         `{"retCode":0,"result":{"list":[{"symbol":"BTCUSDT","lastPrice":"65000.10"}]}}`,
		"/api/v5/public/instruments?instType=SPOT":                 `{"code":"0","data":[{"instId":"BTC-USDT","baseCcy":"BTC","quoteCcy":"USDT","state":"live"},{"instId":"BTC-EUR","baseCcy":"BTC","quoteCcy":"EUR","state":"suspend"}]}`,
		"/api/v5/market/tickers?instType=SPOT":                     `{"code":"0","data":[{"instId":"BTC-USDT","last":"65000.20"}]}`,
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path + "?" + r.URL.Query().Encode()
		if response, ok := fixtures[key]; ok && r.Method == http.MethodGet {
			_, _ = io.WriteString(w, response)
			return
		}
		t.Errorf("unexpected exchange request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(upstream.Close)
	ports := reserveSearchPorts(t)
	cfg, err := config.Load("../config/testdata/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Server.Host, cfg.Server.Port = "127.0.0.1", ports[0]
	cfg.GRPC.Host, cfg.GRPC.Port = "127.0.0.1", ports[1]
	cfg.Scheduler.Disabled = true
	cfg.Exchanges.Binance.BaseURL, cfg.Exchanges.Bybit.BaseURL, cfg.Exchanges.OKX.BaseURL = upstream.URL, upstream.URL, upstream.URL
	application, err := New(*cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	stopped := make(chan error, 1)
	go func() { stopped <- application.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-stopped; err != nil {
			t.Error(err)
		}
	})
	conn, err := grpc.NewClient(net.JoinHostPort(cfg.GRPC.Host, strconv.Itoa(cfg.GRPC.Port)), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := exchangedata.NewMarketSearchServiceClient(conn)
	search := func(req *exchangedata.SearchMarketsRequest) *exchangedata.SearchMarketsResponse {
		t.Helper()
		result, err := client.SearchMarkets(ctx, req, grpc.WaitForReady(true))
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	first := search(&exchangedata.SearchMarketsRequest{ExchangeName: " Binance ", Query: " b "})
	if len(first.Items) != 20 || first.GetNextPage() != 2 || first.Items[0].GetPrice() != "0.12000000" || first.Items[1].Price != nil {
		t.Fatalf("unexpected first page: %v", first)
	}
	for i, item := range first.Items {
		if want := fmt.Sprintf("B%02dUSDT", i); item.GetExternalId() != want {
			t.Fatalf("item %d: got %s, want %s", i, item.GetExternalId(), want)
		}
	}
	second := search(&exchangedata.SearchMarketsRequest{ExchangeName: "binance", Page: first.NextPage})
	if len(second.Items) != 5 || second.Items[0].GetExternalId() != "B20USDT" || second.NextPage != nil {
		t.Fatalf("unexpected second page with an empty query: %v", second)
	}
	page, customLimit := int32(2), int32(3)
	custom := search(&exchangedata.SearchMarketsRequest{ExchangeName: "binance", Query: "B", Page: &page, Limit: &customLimit})
	if len(custom.Items) != 3 || custom.Items[0].GetExternalId() != "B03USDT" || custom.Items[2].GetExternalId() != "B05USDT" || custom.GetNextPage() != 3 {
		t.Fatalf("unexpected page with a custom limit: %v", custom)
	}
	limit := int32(2)
	exact := search(&exchangedata.SearchMarketsRequest{ExchangeName: "binance", Query: "b00usdt", Limit: &limit})
	if len(exact.Items) != 1 || exact.Items[0].GetExternalId() != "B00USDT" || exact.NextPage != nil {
		t.Fatalf("unexpected exact match: %v", exact)
	}
	for _, fixture := range []struct{ exchange, externalID, price string }{
		{"bybit", "BTCUSDT", "65000.10"},
		{"okx", "BTC-USDT", "65000.20"},
	} {
		result := search(&exchangedata.SearchMarketsRequest{ExchangeName: fixture.exchange, Query: "tc/us"})
		if len(result.Items) != 1 || result.Items[0].GetExternalId() != fixture.externalID || result.Items[0].GetPrice() != fixture.price ||
			result.Items[0].GetBaseAsset() != "BTC" || result.Items[0].GetQuoteAsset() != "USDT" {
			t.Fatalf("unexpected %s matches: %v", fixture.exchange, result)
		}
	}
	badLimit := int32(101)
	if _, err := client.SearchMarkets(ctx, &exchangedata.SearchMarketsRequest{ExchangeName: "binance", Limit: &badLimit}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid limit returned %v", err)
	}
}

func reserveSearchPorts(t *testing.T) (ports [2]int) {
	t.Helper()
	for i := range ports {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		ports[i] = listener.Addr().(*net.TCPAddr).Port
	}
	return ports
}
