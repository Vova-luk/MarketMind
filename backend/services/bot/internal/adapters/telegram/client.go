package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"marketmind/bot/internal/config"
)

const (
	botPathPrefix   = "bot"
	sendMessagePath = "sendMessage"
	maxResponseSize = 64 << 10
	contentType     = "Content-Type"
	jsonContentType = "application/json"
	redactedToken   = "[redacted]"
)

type sendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

type responseParameters struct {
	RetryAfter int32 `json:"retry_after"`
}

type sendMessageResponse struct {
	OK          bool               `json:"ok"`
	ErrorCode   int                `json:"error_code"`
	Description string             `json:"description"`
	Parameters  responseParameters `json:"parameters"`
}

type Client struct {
	http     *http.Client
	endpoint string
	token    string
}

func NewClient(cfg config.TelegramConfig) (*Client, error) {
	token := strings.TrimSpace(cfg.Token)
	endpoint, err := url.JoinPath(cfg.APIURL, botPathPrefix+url.PathEscape(token), sendMessagePath)
	if err != nil {
		return nil, fmt.Errorf("build Telegram sendMessage URL: %w", err)
	}
	return &Client{
		http: &http.Client{
			Timeout: cfg.RequestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		endpoint: endpoint,
		token:    token,
	}, nil
}

type apiError struct {
	status      int
	code        int
	description string
	retryAfter  time.Duration
}

func (e *apiError) Error() string {
	return fmt.Sprintf("Telegram sendMessage: HTTP %d, API code %d (%s)", e.status, e.code, e.description)
}

func (e *apiError) RetryAfter() time.Duration { return e.retryAfter }

func (c *Client) SendMessage(ctx context.Context, userID int64, text string) error {
	payload, err := json.Marshal(sendMessageRequest{ChatID: userID, Text: text})
	if err != nil {
		return fmt.Errorf("encode Telegram message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return errors.New("build Telegram sendMessage request")
	}
	req.Header.Set(contentType, jsonContentType)

	resp, err := c.http.Do(req)
	if err != nil {
		var requestErr *url.Error
		if errors.As(err, &requestErr) {
			err = requestErr.Err
		}
		return fmt.Errorf("call Telegram sendMessage: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("read Telegram sendMessage response: %w", err)
	}
	var result sendMessageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode Telegram sendMessage response (HTTP %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !result.OK {
		return &apiError{
			status: resp.StatusCode, code: result.ErrorCode,
			description: strings.ReplaceAll(result.Description, c.token, redactedToken),
			retryAfter:  time.Duration(result.Parameters.RetryAfter) * time.Second,
		}
	}
	return nil
}
