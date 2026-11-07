package binance

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"marketmind/exchange-data/internal/config"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

func stubClient(t *testing.T, status int, body string, check func(*http.Request)) (*BinanceClient, *trackedBody) {
	t.Helper()
	responseBody := &trackedBody{Reader: strings.NewReader(body)}
	client := NewBinanceClient(config.ExchangeConfig{
		BaseURL:     "https://data-api.binance.vision",
		Timeout:     time.Second,
		MaxAttempts: 1,
	}, nil)
	client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if check != nil {
			check(r)
		}
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       responseBody,
			Request:    r,
		}, nil
	})
	return client, responseBody
}

func TestGetActualCryptocurrencyPrice(t *testing.T) {
	for _, tt := range []struct {
		name    string
		symbols []string
		want    []string
		body    string
		prices  []string
	}{
		{"single", []string{"BTCUSDT"}, []string{"BTCUSDT"}, `{"symbol":"BTCUSDT","price":"65000.50000000"}`, []string{"65000.50000000"}},
		{"multiple", []string{"BTCUSDT", "ETHUSDT"}, []string{"BTCUSDT", "ETHUSDT"}, `[{"symbol":"BTCUSDT","price":"65000.50"},{"symbol":"ETHUSDT","price":"0.00000001"}]`, []string{"65000.50", "0.00000001"}},
		{"filter to single", []string{"", " BTCUSDT ", "\t"}, []string{"BTCUSDT"}, `{"symbol":"BTCUSDT","price":"65000.50"}`, []string{"65000.50"}},
		{"filter to multiple", []string{" BTCUSDT ", "", " ETHUSDT "}, []string{"BTCUSDT", "ETHUSDT"}, `[{"symbol":"BTCUSDT","price":"65000.50"},{"symbol":"ETHUSDT","price":"2500.00"}]`, []string{"65000.50", "2500.00"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			type contextKey struct{}
			ctx := context.WithValue(context.Background(), contextKey{}, "request-context")
			calls := 0
			client, body := stubClient(t, http.StatusOK, tt.body, func(r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Scheme != "https" || r.URL.Host != "data-api.binance.vision" || r.URL.Path != "/api/v3/ticker/price" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.Context().Value(contextKey{}) != "request-context" {
					t.Error("request lost caller context")
				}
				query := r.URL.Query()
				if len(tt.want) == 1 {
					if !reflect.DeepEqual(query, url.Values{"symbol": {tt.want[0]}}) {
						t.Errorf("unexpected query: %v", query)
					}
				} else {
					var got []string
					if err := json.Unmarshal([]byte(query.Get("symbols")), &got); err != nil {
						t.Fatalf("symbols query is not JSON: %v", err)
					}
					if len(query) != 1 || !reflect.DeepEqual(got, tt.want) {
						t.Errorf("unexpected query: %v", query)
					}
				}
			})
			before := time.Now()
			market, err := client.GetActualCryptocurrencyPriceFromBinance(ctx, tt.symbols)
			after := time.Now()
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || !body.closed {
				t.Fatalf("HTTP calls=%d, response body closed=%v", calls, body.closed)
			}
			if market == nil {
				t.Fatal("expected market")
			}
			if market.Exchange != "Binance" || len(market.Cryptocurrency) != len(tt.want) {
				t.Fatalf("unexpected market: %+v", market)
			}
			if market.ReceivedAt.Before(before) || market.ReceivedAt.After(after) || market.ReceivedAt.Location() != time.UTC {
				t.Errorf("unexpected received time: %v", market.ReceivedAt)
			}
			for i, quote := range market.Cryptocurrency {
				if quote.Symbol != tt.want[i] || quote.Price != tt.prices[i] || quote.Error != "" {
					t.Errorf("unexpected quote %d: %+v", i, quote)
				}
			}
		})
	}
}

func TestGetActualCryptocurrencyPriceRejectsEmptySymbols(t *testing.T) {
	for _, symbols := range [][]string{nil, {}, {""}, {" ", "\t\n"}} {
		client := NewBinanceClient(config.ExchangeConfig{
			BaseURL:     "https://data-api.binance.vision",
			Timeout:     time.Second,
			MaxAttempts: 1,
		}, nil)
		client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("empty symbols must not cause an HTTP request")
			return nil, nil
		})
		market, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), symbols)
		if err == nil || market != nil {
			t.Errorf("symbols=%q: got market=%+v, error=%v", symbols, market, err)
		}
	}
}

func TestGetActualCryptocurrencyPriceResponseErrors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  int
		body    string
		symbols []string
	}{
		{"HTTP 400", 400, `{"code":-1121,"msg":"Invalid symbol."}`, []string{"INVALID"}},
		{"HTTP 429", 429, `{}`, []string{"BTCUSDT"}},
		{"HTTP 500", 500, `unavailable`, []string{"BTCUSDT"}},
		{"malformed object", 200, `{"symbol":`, []string{"BTCUSDT"}},
		{"malformed array", 200, `[`, []string{"BTCUSDT", "ETHUSDT"}},
		{"empty body", 200, ``, []string{"BTCUSDT"}},
		{"array instead of object", 200, `[]`, []string{"BTCUSDT"}},
		{"object instead of array", 200, `{}`, []string{"BTCUSDT", "ETHUSDT"}},
		{"empty array", 200, `[]`, []string{"BTCUSDT", "ETHUSDT"}},
		{"null array", 200, `null`, []string{"BTCUSDT", "ETHUSDT"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, body := stubClient(t, tt.status, tt.body, nil)
			market, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), tt.symbols)
			if err == nil || market != nil {
				t.Errorf("expected request failure, got market=%+v, error=%v", market, err)
			}
			if !body.closed {
				t.Error("response body was not closed")
			}
		})
	}
}

func TestGetActualCryptocurrencyPriceKeepsPartialResults(t *testing.T) {
	for _, price := range []string{"", "abc", " ", "NaN", "+Inf", "-Inf", "0", "-1", "1e9999"} {
		t.Run("price="+price, func(t *testing.T) {
			data, err := json.Marshal([]map[string]string{
				{"symbol": "BTCUSDT", "price": "65000.50"},
				{"symbol": "ETHUSDT", "price": price},
			})
			if err != nil {
				t.Fatal(err)
			}
			client, _ := stubClient(t, 200, string(data), nil)
			market, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), []string{"BTCUSDT", "ETHUSDT"})
			if err != nil {
				t.Fatalf("invalid quote must not discard successful quotes: %v", err)
			}
			if len(market.Cryptocurrency) != 2 {
				t.Fatalf("unexpected quotes: %+v", market.Cryptocurrency)
			}
			good, bad := market.Cryptocurrency[0], market.Cryptocurrency[1]
			if good.Symbol != "BTCUSDT" || good.Price != "65000.50" || good.Error != "" {
				t.Errorf("successful quote was changed: %+v", good)
			}
			if bad.Symbol != "ETHUSDT" || bad.Price != price || bad.Error == "" {
				t.Errorf("invalid quote must preserve data and report error: %+v", bad)
			}
		})
	}
}

func TestGetActualCryptocurrencyPriceMarksIncompleteSingleQuote(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"price":"65000.50"}`, `{"symbol":"BTCUSDT"}`} {
		client, _ := stubClient(t, 200, body, nil)
		market, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), []string{"BTCUSDT"})
		if err != nil {
			t.Fatalf("body=%s: expected per-quote error, got %v", body, err)
		}
		if len(market.Cryptocurrency) != 1 || market.Cryptocurrency[0].Error == "" {
			t.Errorf("body=%s: expected quote error, got %+v", body, market)
		}
	}
}

func TestGetActualCryptocurrencyPriceTransportError(t *testing.T) {
	want := errors.New("connection failed")
	client := NewBinanceClient(config.ExchangeConfig{
		BaseURL:     "https://data-api.binance.vision",
		Timeout:     time.Second,
		MaxAttempts: 1,
	}, nil)
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, want
	})
	market, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), []string{"BTCUSDT"})
	if market != nil || !errors.Is(err, want) {
		t.Fatalf("expected wrapped transport error, got market=%+v, error=%v", market, err)
	}
}

func TestGetActualCryptocurrencyPriceCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := NewBinanceClient(config.ExchangeConfig{
		BaseURL:     "https://data-api.binance.vision",
		Timeout:     time.Second,
		MaxAttempts: 1,
	}, nil)
	client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		cancel()
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	market, err := client.GetActualCryptocurrencyPriceFromBinance(ctx, []string{"BTCUSDT"})
	if market != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got market=%+v, error=%v", market, err)
	}
}

func TestBinanceClientUsesConfiguredAddress(t *testing.T) {
	client := NewBinanceClient(config.ExchangeConfig{
		BaseURL: "https://binance.example", Timeout: time.Second, MaxAttempts: 1,
	}, nil)
	client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "binance.example" || r.URL.Path != "/api/v3/ticker/price" {
			t.Errorf("configured base URL ignored: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"symbol":"BTCUSDT","price":"1.00"}`)), Request: r}, nil
	})
	if _, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), []string{"BTCUSDT"}); err != nil {
		t.Fatal(err)
	}
}

func TestBinanceClientUsesConfiguredTimeout(t *testing.T) {
	client := NewBinanceClient(config.ExchangeConfig{
		BaseURL: "https://binance.example", Timeout: 10 * time.Millisecond, MaxAttempts: 1,
	}, nil)
	client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	market, err := client.GetActualCryptocurrencyPriceFromBinance(ctx, []string{"BTCUSDT"})
	if market != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout, got market=%+v, error=%v", market, err)
	}
	if ctx.Err() != nil {
		t.Fatal("caller deadline expired instead of client timeout")
	}
}
