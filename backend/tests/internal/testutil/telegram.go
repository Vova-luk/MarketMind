package testutil

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"
)

type RegisteredUser struct {
	TelegramID int64
	API        Client
}

// RegisterUser creates a Telegram user through the API and schedules its cleanup.
func (e Environment) RegisterUser(t testing.TB, ctx context.Context) RegisteredUser {
	t.Helper()
	randomID, err := rand.Int(rand.Reader, big.NewInt(1<<52))
	if err != nil {
		t.Fatalf("generate Telegram user ID: %v", err)
	}
	user := RegisteredUser{TelegramID: randomID.Int64() + 1, API: e.api}

	var exists bool
	err = e.DB.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM market_mind.users WHERE telegram_user_id = $1)
	`, user.TelegramID).Scan(&exists)
	if err != nil {
		t.Fatalf("check test user ID: %v", err)
	}
	if exists {
		t.Fatalf("Telegram test user ID %d already exists", user.TelegramID)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := e.DB.Exec(ctx, "DELETE FROM market_mind.users WHERE telegram_user_id = $1", user.TelegramID); err != nil {
			t.Errorf("delete test user: %v", err)
		}
	})

	user.API.authorization = signedTelegramHeader(e.botToken, user.TelegramID)
	var response struct {
		TelegramID int64 `json:"telegram_user_id,string"`
	}
	user.API.RequestJSON(t, ctx, http.MethodPost, "/api/v1/users/me", nil, http.StatusOK, &response)
	if response.TelegramID != user.TelegramID {
		t.Fatalf("registered Telegram user ID: got %d, want %d", response.TelegramID, user.TelegramID)
	}
	return user
}

func signedTelegramHeader(botToken string, telegramUserID int64) string {
	fields := url.Values{
		"auth_date": {strconv.FormatInt(time.Now().Unix(), 10)},
		"user":      {`{"id":` + strconv.FormatInt(telegramUserID, 10) + `}`},
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secret.Write([]byte(botToken))
	signature := hmac.New(sha256.New, secret.Sum(nil))
	_, _ = signature.Write([]byte("auth_date=" + fields.Get("auth_date") + "\nuser=" + fields.Get("user")))
	fields.Set("hash", hex.EncodeToString(signature.Sum(nil)))
	return "tma " + fields.Encode()
}
