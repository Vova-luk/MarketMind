package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"sync"
)

type telegramMessage struct {
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

type telegramInbox struct {
	mu       sync.Mutex
	messages map[int64][]telegramMessage
}

func registerTelegram(mux *http.ServeMux, token string) {
	inbox := &telegramInbox{messages: make(map[int64][]telegramMessage)}
	mux.HandleFunc("POST /bot"+url.PathEscape(token)+"/sendMessage", inbox.sendMessage)
	mux.HandleFunc("GET /messages/{chatID}", inbox.listMessages)
}

func (inbox *telegramInbox) sendMessage(w http.ResponseWriter, r *http.Request) {
	var message telegramMessage
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	if err := decoder.Decode(&message); err != nil || decoder.Decode(new(json.RawMessage)) != io.EOF || message.ChatID == 0 || message.Text == "" {
		http.Error(w, "invalid sendMessage payload", http.StatusBadRequest)
		return
	}
	inbox.mu.Lock()
	inbox.messages[message.ChatID] = append(inbox.messages[message.ChatID], message)
	inbox.mu.Unlock()
	writeJSON(w, struct {
		OK bool `json:"ok"`
	}{OK: true})
}

func (inbox *telegramInbox) listMessages(w http.ResponseWriter, r *http.Request) {
	chatID, err := strconv.ParseInt(r.PathValue("chatID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid chat ID", http.StatusBadRequest)
		return
	}
	inbox.mu.Lock()
	messages := slices.Clone(inbox.messages[chatID])
	inbox.mu.Unlock()
	writeJSON(w, struct {
		Messages []telegramMessage `json:"messages"`
	}{Messages: messages})
}
