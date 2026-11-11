package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"marketmind/api/pkg/alerts"
	"marketmind/bot/internal/adapters/telegram"
	"marketmind/bot/internal/config"
	"marketmind/bot/internal/service"
	"marketmind/contracts/events"
)

type fetchFunc func(context.Context) (kafka.Message, error)

func (f fetchFunc) FetchMessage(ctx context.Context) (kafka.Message, error) { return f(ctx) }

func TestConsumeCommitsOnlyAfterWholeBatchIsDelivered(t *testing.T) {
	for _, retrySucceeds := range []bool{true, false} {
		name := "cancel after failed delivery"
		if retrySucceeds {
			name = "retry then commit"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var mu sync.Mutex
			var attempts []int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ChatID int64  `json:"chat_id"`
					Text   string `json:"text"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if r.Method != http.MethodPost || r.URL.Path != "/bottest-token/sendMessage" {
					t.Errorf("unexpected Telegram request: %s %s", r.Method, r.URL.Path)
				}
				const wantText = "Сработал алерт!\nМонета: BTC/USDT (Binance)\nЦена при настройке алерта: 100.00 USDT\nАктуальная цена: 106.13 USDT\nУсловие срабатывания: ≥ +5%"
				if request.Text != wantText {
					t.Errorf("unexpected message: %q", request.Text)
				}
				mu.Lock()
				attempts = append(attempts, request.ChatID)
				attempt := len(attempts)
				mu.Unlock()
				if attempt == 2 || (attempt == 4 && !retrySucceeds) {
					// HTTP 200 alone does not mean that Telegram accepted the message.
					_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"parameters":{"retry_after":1}}`)
					if attempt == 4 {
						cancel()
					}
					return
				}
				_, _ = io.WriteString(w, `{"ok":true}`)
			}))
			defer server.Close()

			client, err := telegram.NewClient(config.TelegramConfig{
				APIURL: server.URL, Token: "test-token", RequestTimeout: time.Second,
			})
			if err != nil {
				t.Fatal(err)
			}
			logger := slog.New(slog.DiscardHandler)
			consumer := NewPriceChangedConsumer(config.KafkaConfig{}, service.NewService(client, 2, logger), logger)
			item := events.PriceNotification{
				TelegramUserID: 1234567890123, BaseAsset: "BTC", QuoteAsset: "USDT", ExchangeName: "Binance",
				Type: alerts.Percent, Value: "5", Direction: alerts.GreaterOrEqual, ReferencePrice: "100", CurrentPrice: "106.126",
			}
			second := item
			second.TelegramUserID++
			third := second
			third.TelegramUserID++
			data, err := json.Marshal(events.PriceChangedEvent{EventID: "event-1", Items: []events.PriceNotification{item, second, third}})
			if err != nil {
				t.Fatal(err)
			}
			var fetched, commits int
			reader := fetchFunc(func(context.Context) (kafka.Message, error) {
				fetched++
				if fetched > 1 {
					return kafka.Message{}, errors.New("fetched another message before cancellation")
				}
				return kafka.Message{Value: data, Partition: 2, Offset: 10}, nil
			})
			err = consumer.consume(ctx, reader, func(message kafka.Message) error {
				commits++
				mu.Lock()
				defer mu.Unlock()
				if len(attempts) != 4 || message.Offset != 10 || message.Partition != 2 {
					t.Errorf("unexpected commit after %d attempts for %d/%d", len(attempts), message.Partition, message.Offset)
				}
				cancel()
				return nil
			})
			if !errors.Is(err, context.Canceled) || fetched != 1 {
				t.Fatalf("consume returned %v after %d fetches", err, fetched)
			}
			wantAttempts := []int64{item.TelegramUserID, second.TelegramUserID, third.TelegramUserID, second.TelegramUserID}
			wantCommits := 0
			if retrySucceeds {
				wantCommits = 1
			}
			mu.Lock()
			defer mu.Unlock()
			if commits != wantCommits || !slices.Equal(attempts, wantAttempts) {
				t.Fatalf("got %d commits and attempts %v, want %d and %v", commits, attempts, wantCommits, wantAttempts)
			}
		})
	}
}
