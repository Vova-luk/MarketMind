package utils

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
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type timeoutError struct{}

func (timeoutError) Error() string   { return "transport timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func TestDoWithRetryPolicy(t *testing.T) {
	permanent := errors.New("permanent transport failure")
	for _, tt := range []struct {
		name      string
		statuses  []int
		fail      error
		success   bool
		wantCalls int
		wantLogs  int
	}{
		{"success", []int{200}, nil, true, 1, 0},
		{"502 then success", []int{502, 200}, nil, true, 2, 1},
		{"503 then success", []int{503, 200}, nil, true, 2, 1},
		{"504 then success", []int{504, 200}, nil, true, 2, 1},
		{"HTTP attempts exhausted", []int{503, 503, 503}, nil, false, 3, 2},
		{"timeout attempts exhausted", nil, timeoutError{}, false, 3, 2},
		{"permanent transport error", nil, permanent, false, 1, 0},
		{"400 not retried", []int{400}, nil, false, 1, 0},
		{"429 not retried", []int{429}, nil, false, 1, 0},
		{"500 not retried", []int{500}, nil, false, 1, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var logBuffer bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
			calls := 0
			var bodies []*trackedBody
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if len(bodies) > 0 && !bodies[len(bodies)-1].closed {
					t.Error("previous response must be closed before retry")
				}
				calls++
				if tt.fail != nil {
					return nil, tt.fail
				}
				if calls > len(tt.statuses) {
					t.Fatal("unexpected retry")
				}
				body := &trackedBody{Reader: strings.NewReader("response")}
				bodies = append(bodies, body)
				return &http.Response{StatusCode: tt.statuses[calls-1], Header: make(http.Header), Body: body, Request: r}, nil
			})}
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/prices", nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := DoWithRetry(req, client, RetryConfig{MaxAttempts: 3}, logger)
			if (err == nil) != tt.success || (resp != nil) != tt.success || calls != tt.wantCalls {
				t.Fatalf("resp=%v, err=%v, calls=%d", resp, err, calls)
			}
			if tt.fail != nil && !errors.Is(err, tt.fail) {
				t.Errorf("underlying error lost: %v", err)
			}
			if resp != nil {
				if bodies[len(bodies)-1].closed {
					t.Error("successful body must be left for caller")
				}
				resp.Body.Close()
			}
			for _, body := range bodies {
				if !body.closed {
					t.Error("response body leaked")
				}
			}
			decoder := json.NewDecoder(&logBuffer)
			logs := 0
			for {
				var entry map[string]any
				if err := decoder.Decode(&entry); err == io.EOF {
					break
				} else if err != nil {
					t.Fatal(err)
				}
				logs++
				if entry["msg"] != "HTTP request failed; retrying" || entry["level"] != "WARN" || entry["exchange"] != nil {
					t.Errorf("unexpected retry log: %v", entry)
				}
			}
			if logs != tt.wantLogs {
				t.Errorf("expected %d retry logs, got %d", tt.wantLogs, logs)
			}
		})
	}
}

func TestDoWithRetryStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("canceled operation must not send requests")
		return nil, nil
	})}
	if _, err := DoWithRetry(req, client, RetryConfig{MaxAttempts: 3}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestDoWithRetryHasNoDelayAfterLastAttempt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, timeoutError{} })}
	_, err = DoWithRetry(req, client, RetryConfig{MaxAttempts: 1, Delay: time.Hour}, nil)
	if !errors.Is(err, timeoutError{}) || ctx.Err() != nil {
		t.Fatalf("last failure must return without delay: %v", err)
	}
}

func TestDoWithRetryRejectsInvalidSettings(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid settings must not send requests")
		return nil, nil
	})}
	for _, cfg := range []RetryConfig{{MaxAttempts: 0}, {MaxAttempts: -1}, {MaxAttempts: 1, Delay: -time.Second}} {
		if resp, err := DoWithRetry(req, client, cfg, nil); err == nil || resp != nil {
			t.Errorf("invalid settings accepted: %+v", cfg)
		}
	}
}
