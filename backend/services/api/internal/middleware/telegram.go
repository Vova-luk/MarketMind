package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"marketmind/api/internal/utils/httpresponse"
	"marketmind/api/internal/utils/requestctx"
)

const (
	maxInitDataAge = 24 * time.Hour
	maxClockSkew   = time.Minute
)

func AuthenticateTelegram(botToken string) func(http.Handler) http.Handler {
	var secret []byte
	if botToken != "" {
		mac := hmac.New(sha256.New, []byte("WebAppData"))
		_, _ = mac.Write([]byte(botToken))
		secret = mac.Sum(nil)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if secret == nil {
				httpresponse.WriteError(w, http.StatusServiceUnavailable, "TELEGRAM_AUTH_UNAVAILABLE", "Telegram authentication is unavailable")
				return
			}
			if headers := r.Header.Values("Authorization"); len(headers) == 1 {
				scheme, data, _ := strings.Cut(headers[0], " ")
				if strings.EqualFold(scheme, "tma") {
					if id, ok := validateInitData(data, secret, time.Now()); ok {
						ctx := requestctx.WithTelegramUserID(r.Context(), id)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}
			w.Header().Set("WWW-Authenticate", "tma")
			httpresponse.WriteError(w, http.StatusUnauthorized, "TELEGRAM_AUTH_INVALID", "Missing or invalid Telegram initData")
		})
	}
}

func validateInitData(data string, secret []byte, now time.Time) (int64, bool) {
	fields, err := url.ParseQuery(data)
	if err != nil {
		return 0, false
	}
	hash, err := hex.DecodeString(fields.Get("hash"))
	if err != nil || len(hash) != sha256.Size {
		return 0, false
	}

	check := make([]string, 0, len(fields)-1)
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		values := fields[key]
		// Reject ambiguous input before building Telegram's newline-delimited string.
		if len(values) != 1 || key == "" || strings.ContainsAny(key, "=\r\n") || strings.ContainsAny(values[0], "\r\n") {
			return 0, false
		}
		if key != "hash" {
			check = append(check, key+"="+values[0])
		}
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.Join(check, "\n")))
	if !hmac.Equal(hash, mac.Sum(nil)) {
		return 0, false
	}

	authDate, err := strconv.ParseInt(fields.Get("auth_date"), 10, 64)
	if err != nil || authDate < now.Add(-maxInitDataAge).Unix() || authDate > now.Add(maxClockSkew).Unix() {
		return 0, false
	}
	var user struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(fields.Get("user")), &user); err != nil || user.ID <= 0 {
		return 0, false
	}
	return user.ID, true
}
