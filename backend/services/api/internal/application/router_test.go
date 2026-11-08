package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	domainusers "marketmind/api/internal/domain/users"
	userhandlers "marketmind/api/internal/handlers/users"
)

type registerUserStub func(context.Context, domainusers.User) error

func (s registerUserStub) Register(ctx context.Context, user domainusers.User) error {
	return s(ctx, user)
}

func TestRegisterUserRouteUsesSignedTelegramID(t *testing.T) {
	const botToken = "test-token"
	const telegramUserID int64 = 1234567890123
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repository := registerUserStub(func(_ context.Context, user domainusers.User) error {
		if user.TelegramUserID != telegramUserID {
			t.Fatalf("repository received Telegram ID %d, want %d", user.TelegramUserID, telegramUserID)
		}
		return nil
	})
	router := NewRouter(logger, userhandlers.NewHandler(repository, logger), botToken)

	userID := strconv.FormatInt(telegramUserID, 10)
	fields := url.Values{
		"auth_date": {strconv.FormatInt(time.Now().Unix(), 10)},
		"user":      {`{"id":` + userID + `}`},
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secret.Write([]byte(botToken))
	signature := hmac.New(sha256.New, secret.Sum(nil))
	_, _ = signature.Write([]byte("auth_date=" + fields.Get("auth_date") + "\nuser=" + fields.Get("user")))
	fields.Set("hash", hex.EncodeToString(signature.Sum(nil)))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/me", strings.NewReader(`{"telegram_user_id":"999"}`))
	req.Header.Set("Authorization", "tma "+fields.Encode())
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200. Body: %s", res.Code, res.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["telegram_user_id"] != userID {
		t.Fatalf("unexpected response: %s", res.Body)
	}
}
