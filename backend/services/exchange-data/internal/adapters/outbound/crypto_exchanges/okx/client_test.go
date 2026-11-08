package okx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"marketmind/exchange-data/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func testClient() *OKXClient {
	return NewOKXClient(config.ExchangeConfig{
		BaseURL: "https://okx.example/", Timeout: time.Second, MaxAttempts: 3,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestQuotesAreFilteredAndMapped(t *testing.T) {
	client := testClient()
	var logs bytes.Buffer
	client.logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	body := &trackedBody{Reader: strings.NewReader(`{"code":"0","msg":"OK","data":[{"instId":"SOL-USDT","last":"100"},{"instId":"ETH-USDT","last":"abc"},{"instId":"BTC-USDT","last":"65000.50000000","askPx":"65001"}]}`)}
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "caller")
	calls := 0
	client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Host != "okx.example" || r.URL.Path != "/api/v5/market/tickers" || r.URL.RawQuery != "instType=SPOT" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if r.Context().Value(key{}) != "caller" {
			t.Error("caller context lost")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body, Request: r}, nil
	})
	before := time.Now()
	market, err := client.GetPrices(ctx, []string{" BTC-USDT ", "", "ETH-USDT", "MISSING", "BTC-USDT"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !body.closed || market.Exchange != "OKX" || len(market.Cryptocurrency) != 3 {
		t.Fatalf("calls=%d, closed=%v, market=%+v", calls, body.closed, market)
	}
	quotes := market.Cryptocurrency
	if quotes[0].Symbol != "BTC-USDT" || quotes[0].Price != "65000.50000000" || quotes[0].Error != "" {
		t.Errorf("last trade price must be preserved: %+v", quotes[0])
	}
	if quotes[1].Symbol != "ETH-USDT" || quotes[1].Price != "abc" || quotes[1].Error == "" {
		t.Errorf("invalid price not reported: %+v", quotes[1])
	}
	if quotes[2].Symbol != "MISSING" || quotes[2].Price != "" || quotes[2].Error == "" {
		t.Errorf("missing quote not reported: %+v", quotes[2])
	}
	if market.ReceivedAt.Before(before) || market.ReceivedAt.After(time.Now()) || market.ReceivedAt.Location() != time.UTC {
		t.Errorf("unexpected received time: %v", market.ReceivedAt)
	}
	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "DEBUG" || entry["valid"] != float64(1) || entry["failed"] != float64(2) {
		t.Errorf("unexpected summary log: %v", entry)
	}
}

func TestQuoteResponseFailures(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		wantError  bool
	}{
		{"malformed JSON", `{`, true},
		{"array envelope", `[]`, true},
		{"empty envelope", `{}`, true},
		{"missing success code", `{"data":[]}`, true},
		{"numeric success code", `{"code":0,"data":[]}`, true},
		{"missing data", `{"code":"0"}`, true},
		{"null envelope", `null`, true},
		{"null list", `{"code":"0","data":null}`, true},
		{"API error", `{"code":"50011","msg":"Too many visits!"}`, true},
		{"empty list", `{"code":"0","data":[]}`, false},
		{"unrequested symbol", `{"code":"0","data":[{"instId":"ETH-USDT","last":"1"}]}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := testClient()
			calls := 0
			body := &trackedBody{Reader: strings.NewReader(tt.body)}
			client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body, Request: r}, nil
			})
			market, err := client.GetPrices(context.Background(), []string{"BTC-USDT"})
			if tt.wantError {
				if err == nil || market != nil {
					t.Fatalf("market=%+v, error=%v", market, err)
				}
			} else {
				if err != nil || market == nil || len(market.Cryptocurrency) != 1 || market.Cryptocurrency[0].Error == "" {
					t.Fatalf("expected missing-symbol error, market=%+v, error=%v", market, err)
				}
			}
			if calls != 1 || !body.closed {
				t.Errorf("calls=%d, closed=%v", calls, body.closed)
			}
		})
	}
}

func TestInvalidPricesCarryPerQuoteErrors(t *testing.T) {
	for _, price := range []string{"", " ", "NaN", "+Inf", "-Inf", "0", "-1", "1e9999"} {
		t.Run(price, func(t *testing.T) {
			client := testClient()
			encoded, err := json.Marshal(map[string]any{"code": "0", "data": []map[string]string{{"instId": "BTC-USDT", "last": price}}})
			if err != nil {
				t.Fatal(err)
			}
			client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(encoded)), Request: r}, nil
			})
			market, err := client.GetPrices(context.Background(), []string{"BTC-USDT"})
			if err != nil {
				t.Fatal(err)
			}
			if market.Cryptocurrency[0].Error == "" || market.Cryptocurrency[0].Price != price {
				t.Errorf("unexpected quote: %+v", market.Cryptocurrency[0])
			}
		})
	}
}

func TestEmptySymbolsDoNotSendRequest(t *testing.T) {
	for _, symbols := range [][]string{nil, {}, {"", " \t"}} {
		client := testClient()
		client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected HTTP request"); return nil, nil })
		if market, err := client.GetPrices(context.Background(), symbols); err == nil || market != nil {
			t.Errorf("market=%+v, error=%v", market, err)
		}
	}
}

func TestHTTPRetryPolicy(t *testing.T) {
	for _, tt := range []struct {
		status, calls int
		success       bool
	}{
		{502, 3, true}, {503, 3, true}, {504, 3, true}, {400, 1, false}, {403, 1, false}, {429, 1, false},
	} {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			client := testClient()
			var logs bytes.Buffer
			client.logger = slog.New(slog.NewJSONHandler(&logs, nil))
			calls := 0
			var previous *trackedBody
			client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if previous != nil && !previous.closed {
					t.Error("previous response body not closed before retry")
				}
				calls++
				status, data := tt.status, "unavailable"
				if tt.success && calls == 3 {
					status, data = 200, `{"code":"0","data":[{"instId":"BTC-USDT","last":"1"}]}`
				}
				previous = &trackedBody{Reader: strings.NewReader(data)}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: previous, Request: r}, nil
			})
			market, err := client.GetPrices(context.Background(), []string{"BTC-USDT"})
			if (err == nil) != tt.success || (market != nil) != tt.success || calls != tt.calls || !previous.closed {
				t.Fatalf("market=%+v, error=%v, calls=%d, closed=%v", market, err, calls, previous.closed)
			}
			if strings.Contains(logs.String(), "Binance") {
				t.Error("wrong exchange in logs")
			}
			if tt.success && strings.Count(logs.String(), `"level":"WARN"`) != 2 {
				t.Errorf("unexpected retry logs: %s", logs.String())
			}
			if !tt.success && logs.Len() != 0 {
				t.Errorf("returned error should not be logged twice: %s", logs.String())
			}
		})
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestTransportFailurePolicy(t *testing.T) {
	for _, tt := range []struct {
		name  string
		err   error
		calls int
	}{{"timeout", timeoutError{}, 3}, {"permanent", errors.New("bad certificate"), 1}} {
		t.Run(tt.name, func(t *testing.T) {
			client := testClient()
			calls := 0
			client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, tt.err })
			market, err := client.GetPrices(context.Background(), []string{"BTC-USDT"})
			if market != nil || !errors.Is(err, tt.err) || calls != tt.calls {
				t.Fatalf("market=%+v, error=%v, calls=%d", market, err, calls)
			}
		})
	}
}

type cancelWriter func([]byte) (int, error)

func (f cancelWriter) Write(p []byte) (int, error) { return f(p) }

func TestCancellationDuringRetryWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := testClient()
	client.retryDelay = time.Hour
	client.logger = slog.New(slog.NewTextHandler(cancelWriter(func(p []byte) (int, error) { cancel(); return len(p), nil }), nil))
	calls := 0
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, timeoutError{} })
	market, err := client.GetPrices(ctx, []string{"BTC-USDT"})
	if market != nil || !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("market=%+v, error=%v, calls=%d", market, err, calls)
	}
}
