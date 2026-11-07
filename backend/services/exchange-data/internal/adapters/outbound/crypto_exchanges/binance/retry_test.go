package binance

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"marketmind/exchange-data/internal/config"
)

type requestTimeoutError struct{}

func (requestTimeoutError) Error() string   { return "request timed out" }
func (requestTimeoutError) Timeout() bool   { return true }
func (requestTimeoutError) Temporary() bool { return true }

func newRetryTestClient(attempts int, delay time.Duration) *BinanceClient {
	return NewBinanceClient(config.ExchangeConfig{
		BaseURL: "https://binance.example", Timeout: time.Second,
		MaxAttempts: attempts, RetryDelay: delay,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestRetriesTemporaryHTTPFailures(t *testing.T) {
	for _, status := range []int{502, 503, 504} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := newRetryTestClient(3, 0)
			calls := 0
			var bodies []*trackedBody
			client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if len(bodies) > 0 && !bodies[len(bodies)-1].closed {
					t.Error("previous response was not closed before retry")
				}
				calls++
				code, data := status, "unavailable"
				if calls == 3 {
					code, data = 200, `{"symbol":"BTCUSDT","price":"65000.50"}`
				}
				body := &trackedBody{Reader: strings.NewReader(data)}
				bodies = append(bodies, body)
				return &http.Response{StatusCode: code, Header: make(http.Header), Body: body, Request: r}, nil
			})
			market, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), []string{"BTCUSDT"})
			if err != nil || market == nil || calls != 3 {
				t.Fatalf("market=%+v, err=%v, calls=%d", market, err, calls)
			}
			for _, body := range bodies {
				if !body.closed {
					t.Error("response body leaked")
				}
			}
		})
	}
}

func TestRetryTimeoutThenSuccess(t *testing.T) {
	client := newRetryTestClient(3, 0)
	calls := 0
	client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls < 3 {
			return nil, requestTimeoutError{}
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"symbol":"BTCUSDT","price":"1.00"}`)), Request: r}, nil
	})
	market, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), []string{"BTCUSDT"})
	if err != nil || market == nil || calls != 3 {
		t.Fatalf("market=%+v, err=%v, calls=%d", market, err, calls)
	}
}

func TestRetryExhaustionPreservesError(t *testing.T) {
	client := newRetryTestClient(3, 0)
	want := requestTimeoutError{}
	calls := 0
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, want
	})
	market, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), []string{"BTCUSDT"})
	if market != nil || !errors.Is(err, want) || calls != 3 {
		t.Fatalf("market=%+v, err=%v, calls=%d", market, err, calls)
	}
}

func TestNoWaitAfterLastAttempt(t *testing.T) {
	client := newRetryTestClient(1, time.Hour)
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, requestTimeoutError{}
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := client.GetActualCryptocurrencyPriceFromBinance(ctx, []string{"BTCUSDT"})
	if !errors.Is(err, requestTimeoutError{}) || ctx.Err() != nil {
		t.Fatalf("last attempt must return immediately: err=%v, ctx=%v", err, ctx.Err())
	}
}

func TestPermanentFailuresAreNotRetried(t *testing.T) {
	for _, status := range []int{400, 401, 403, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := newRetryTestClient(3, 0)
			calls := 0
			body := &trackedBody{Reader: strings.NewReader(`{}`)}
			client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: body, Request: r}, nil
			})
			market, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), []string{"BTCUSDT"})
			if err == nil || market != nil || calls != 1 || !body.closed {
				t.Fatalf("market=%+v, err=%v, calls=%d, closed=%v", market, err, calls, body.closed)
			}
		})
	}
	client := newRetryTestClient(3, 0)
	want := errors.New("permanent transport failure")
	calls := 0
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, want
	})
	if _, err := client.GetActualCryptocurrencyPriceFromBinance(context.Background(), []string{"BTCUSDT"}); !errors.Is(err, want) || calls != 1 {
		t.Fatalf("err=%v, calls=%d", err, calls)
	}
}

type retryLogWriter func([]byte) (int, error)

func (f retryLogWriter) Write(p []byte) (int, error) { return f(p) }

func TestCancellationStopsRetryWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := newRetryTestClient(3, time.Hour)
	// Cancel exactly when the client announces a retry, before its wait.
	client.logger = slog.New(slog.NewTextHandler(retryLogWriter(func(p []byte) (int, error) {
		cancel()
		return len(p), nil
	}), nil))
	calls := 0
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, requestTimeoutError{}
	})
	_, err := client.GetActualCryptocurrencyPriceFromBinance(ctx, []string{"BTCUSDT"})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v, calls=%d", err, calls)
	}
}

func TestCanceledContextDoesNotSendRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := newRetryTestClient(3, 0)
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("request sent after cancellation")
		return nil, nil
	})
	if _, err := client.GetActualCryptocurrencyPriceFromBinance(ctx, []string{"BTCUSDT"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
