package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type Client struct {
	baseURL       string
	httpClient    *http.Client
	authorization string
}

func NewClient(baseURL string) Client {
	return Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func ClientFromEnv(t testing.TB, name string) Client {
	t.Helper()
	url := os.Getenv(name)
	if url == "" {
		t.Fatalf("%s must point to the test HTTP endpoint", name)
	}
	return NewClient(url)
}

// RequestJSON sends a request, checks its status and optionally decodes its response.
func (c Client) RequestJSON(t testing.TB, ctx context.Context, method, path string, payload any, wantStatus int, result any) {
	t.Helper()
	if err := c.DoJSON(ctx, method, path, payload, wantStatus, result); err != nil {
		t.Fatal(err)
	}
}

// DoJSON is also suitable for requests issued by concurrent test workers.
func (c Client) DoJSON(ctx context.Context, method, path string, payload any, wantStatus int, result any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode %s %s request: %w", method, path, err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create %s %s request: %w", method, path, err)
	}
	if c.authorization != "" {
		req.Header.Set("Authorization", c.authorization)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send %s %s request: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s %s response: %w", method, path, err)
	}
	if resp.StatusCode != wantStatus {
		return fmt.Errorf("%s %s: status=%d, want=%d, body=%s", method, path, resp.StatusCode, wantStatus, data)
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("decode %s %s response: %w; body=%s", method, path, err, data)
		}
	}
	return nil
}
