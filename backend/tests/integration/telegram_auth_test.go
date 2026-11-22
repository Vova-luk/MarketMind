package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"time"
)

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
