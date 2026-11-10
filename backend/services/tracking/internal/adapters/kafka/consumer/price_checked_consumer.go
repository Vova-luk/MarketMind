package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"marketmind/contracts/events"
	"marketmind/tracking/internal/config"

	"github.com/segmentio/kafka-go"
)

type PriceCheckedHandler interface {
	HandlePricesChecked(ctx context.Context, event events.PricesCheckedEvent) error
}

type PriceCheckedConsumer struct {
	reader  *kafka.Reader
	topic   string
	handler PriceCheckedHandler
	logger  *slog.Logger
}

func NewPriceCheckedConsumer(cfg config.KafkaConfig, logger *slog.Logger) *PriceCheckedConsumer {
	if logger == nil {
		logger = slog.Default()
	}

	return &PriceCheckedConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Topic:   cfg.PriceCheckedTopic,
			Brokers: cfg.Brokers,
			GroupID: cfg.GroupID,
		}),
		topic: cfg.PriceCheckedTopic,

		logger: logger,
	}
}

func (c *PriceCheckedConsumer) Run(ctx context.Context) error {
	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return fmt.Errorf("fetch prices checked message: %w", err)
		}

		var event events.PricesCheckedEvent
		if err := json.Unmarshal(message.Value, &event); err != nil {
			return fmt.Errorf("decode prices checked message partition=%d offset=%d: %w", message.Partition, message.Offset, err)
		}

		if err := c.handler.HandlePricesChecked(ctx, event); err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return fmt.Errorf("handle prices checked event %q: %w", event.EventID, err)
		}

		if err := c.reader.CommitMessages(ctx, message); err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return fmt.Errorf(
				"commit prices checked message partition=%d offset=%d: %w", message.Partition, message.Offset, err)
		}

		c.logger.DebugContext(
			ctx,
			"prices checked event processed",
			"event_id", event.EventID,
			"items_count", len(event.Items),
			"partition", message.Partition,
			"offset", message.Offset,
		)
	}
}

func (c *PriceCheckedConsumer) Close() error {
	return c.reader.Close()
}
