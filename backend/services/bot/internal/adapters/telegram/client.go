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
	maxResponseSize = 1 << 20
	contentType     = "Content-Type"
	jsonContentType = "application/json"
	redactedToken   = "[redacted]"
)

type sendMessageRequest struct {
	ChatID      int64           `json:"chat_id"`
	Text        string          `json:"text"`
	ParseMode   string          `json:"parse_mode"`
	ReplyMarkup *inlineKeyboard `json:"reply_markup,omitempty"`
}

type responseParameters struct {
	RetryAfter int32 `json:"retry_after"`
}

type apiResponse struct {
	Result      json.RawMessage    `json:"result"`
	OK          bool               `json:"ok"`
	ErrorCode   int                `json:"error_code"`
	Description string             `json:"description"`
	Parameters  responseParameters `json:"parameters"`
}

type Client struct {
	http     *http.Client
	pollHTTP *http.Client
	endpoint string
	token    string
}

func NewClient(cfg config.TelegramConfig) (*Client, error) {
	token := strings.TrimSpace(cfg.Token)
	endpoint, err := url.JoinPath(cfg.APIURL, botPathPrefix+url.PathEscape(token))
	if err != nil {
		return nil, fmt.Errorf("build Telegram API URL: %w", err)
	}
	httpClient := &http.Client{
		Timeout: cfg.RequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	pollClient := *httpClient
	pollClient.Timeout += pollTimeout
	return &Client{
		http: httpClient, pollHTTP: &pollClient,
		endpoint: endpoint,
		token:    token,
	}, nil
}

type apiError struct {
	method      string
	status      int
	code        int
	description string
	retryAfter  time.Duration
}

func (e *apiError) Error() string {
	return fmt.Sprintf("Telegram %s: HTTP %d, API code %d (%s)", e.method, e.status, e.code, e.description)
}

func (e *apiError) RetryAfter() time.Duration { return e.retryAfter }

func (c *Client) SendMessage(ctx context.Context, userID int64, text string) error {
	return c.sendMessage(ctx, sendMessageRequest{ChatID: userID, Text: text, ParseMode: "HTML"})
}

func (c *Client) sendMessage(ctx context.Context, message sendMessageRequest) error {
	return c.call(ctx, c.http, sendMessagePath, message, nil)
}

func (c *Client) call(ctx context.Context, client *http.Client, method string, request, result any) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode Telegram %s request: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/"+method, bytes.NewReader(payload))
	if err != nil {
		return errors.New("build Telegram request")
	}
	req.Header.Set(contentType, jsonContentType)

	resp, err := client.Do(req)
	if err != nil {
		var requestErr *url.Error
		if errors.As(err, &requestErr) {
			err = requestErr.Err
		}
		return fmt.Errorf("call Telegram %s: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("read Telegram %s response: %w", method, err)
	}
	var response apiResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("decode Telegram %s response (HTTP %d): %w", method, resp.StatusCode, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !response.OK {
		return &apiError{
			method: method, status: resp.StatusCode, code: response.ErrorCode,
			description: strings.ReplaceAll(response.Description, c.token, redactedToken),
			retryAfter:  time.Duration(response.Parameters.RetryAfter) * time.Second,
		}
	}
	if result != nil {
		if err := json.Unmarshal(response.Result, result); err != nil {
			return fmt.Errorf("decode Telegram %s result: %w", method, err)
		}
	}
	return nil
}
