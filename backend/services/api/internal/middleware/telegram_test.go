package middleware

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidateInitData(t *testing.T) {
	// Fixtures were signed independently with HMAC-SHA256 and a synthetic bot token.
	const signed = "hash=cac1c70fc6529fd52257aefa6d7ff11deda36386598243209d69bcd828c1e8d6" +
		"&user=%7B%22id%22%3A1234567890123%2C%22first_name%22%3A%22%D0%A1%D0%B0%D0%BD%D1%8F+%2B+test%22%7D" +
		"&signature=telegram-test-signature&auth_date=1794153600&start_param=test"
	const signedWithoutUserID = "hash=26665a0ffdc5957d0ca4d4aa4896aaa0776aa290cf0042410a72fa27bde8c204" +
		"&user=%7B%22id%22%3A0%7D&signature=telegram-test-signature&auth_date=1794153600&start_param=test"
	secret, err := hex.DecodeString("5cd51e13246054538a418839daa13309165ce73ea98ea1d193ab48a9990e117b")
	if err != nil {
		t.Fatal(err)
	}
	issued := time.Unix(1794153600, 0)
	for _, tc := range []struct {
		name string
		data string
		now  time.Time
		want int64
	}{
		{"valid including signature", signed, issued, 1234567890123},
		{"tampered user ID", strings.Replace(signed, "1234567890123", "1234567890124", 1), issued, 0},
		{"duplicate user", signed + "&user=%7B%22id%22%3A2%7D", issued, 0},
		{"invalid query encoding", signed + "&extra=%", issued, 0},
		{"invalid signed user ID", signedWithoutUserID, issued, 0},
		{"expired", signed, issued.Add(24*time.Hour + time.Second), 0},
		{"future", signed, issued.Add(-time.Minute - time.Second), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := validateInitData(tc.data, secret, tc.now)
			if id != tc.want || ok != (tc.want > 0) {
				t.Fatalf("validateInitData got (%d, %t), want ID %d", id, ok, tc.want)
			}
		})
	}
}

func TestAuthenticateTelegramRejectsUnsignedRequests(t *testing.T) {
	for _, tc := range []struct {
		name, token, authorization string
		want                       int
	}{
		{"missing config", "", "", http.StatusServiceUnavailable},
		{"missing header", "test-token", "", http.StatusUnauthorized},
		{"bare user ID", "test-token", "tma user=%7B%22id%22%3A1%7D", http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := AuthenticateTelegram(tc.token)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("unauthenticated request reached handler")
			}))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/users/me", nil)
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("got status %d, want %d", res.Code, tc.want)
			}
		})
	}
}
