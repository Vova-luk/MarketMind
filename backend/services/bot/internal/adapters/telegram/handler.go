package telegram

import (
	"context"
	"strings"
)

type Update struct {
	ID      int64    `json:"update_id"`
	Message *Message `json:"message"`
}

type Message struct {
	Text string `json:"text"`
	Chat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
}

type inlineKeyboard struct {
	Rows [][]webAppButton `json:"inline_keyboard"`
}

type webAppButton struct {
	Text   string     `json:"text"`
	WebApp webAppInfo `json:"web_app"`
}

type webAppInfo struct {
	URL string `json:"url"`
}

type Handler struct {
	client     *Client
	miniAppURL string
}

func NewHandler(client *Client, miniAppURL string) *Handler {
	return &Handler{client: client, miniAppURL: miniAppURL}
}

func (h *Handler) HandleUpdate(ctx context.Context, update Update) error {
	message := update.Message
	if message == nil || message.Chat.Type != "private" || message.Chat.ID == 0 {
		return nil
	}
	words := strings.Fields(message.Text)
	if len(words) == 0 {
		return nil
	}
	command, _, _ := strings.Cut(words[0], "@")
	if command != "/start" {
		return nil
	}
	return h.client.sendMessage(ctx, sendMessageRequest{
		ChatID: message.Chat.ID, ParseMode: "HTML",
		Text: "Добро пожаловать в <b>MarketMind</b>!\nОтслеживайте цены криптовалют и получайте уведомления об изменениях.\n\nНажмите кнопку ниже, чтобы открыть приложение.",
		ReplyMarkup: &inlineKeyboard{Rows: [][]webAppButton{{{
			Text: "Открыть MarketMind", WebApp: webAppInfo{URL: h.miniAppURL},
		}}}},
	})
}
