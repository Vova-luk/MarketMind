package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"marketmind/bot/internal/config"
	"marketmind/contracts/events"
)

const (
	partitionRetryDelay  = time.Second
	disabledLagPolling   = -1
	groupUnavailable     = "Kafka consumer group unavailable"
	partitionInterrupted = "Kafka partition interrupted"
	invalidRecord        = "invalid price changed message, partition paused"
)

type PriceChangedHandler interface {
	HandlePriceChanged(ctx context.Context, event events.PriceChangedEvent) error
}

type PriceChangedConsumer struct {
	config  config.KafkaConfig
	handler PriceChangedHandler
	logger  *slog.Logger
}

func NewPriceChangedConsumer(cfg config.KafkaConfig, handler PriceChangedHandler, logger *slog.Logger) *PriceChangedConsumer {
	return &PriceChangedConsumer{config: cfg, handler: handler, logger: logger}
}

func (c *PriceChangedConsumer) Run(ctx context.Context) error {
	group, err := kafka.NewConsumerGroup(kafka.ConsumerGroupConfig{
		Brokers: c.config.Brokers,
		ID:      c.config.GroupID,
		Topics:  []string{c.config.InputTopic},

		StartOffset:           kafka.FirstOffset,
		WatchPartitionChanges: true,
	})
	if err != nil {
		return fmt.Errorf("create Kafka consumer group: %w", err)
	}
	defer group.Close()

	for ctx.Err() == nil {
		generation, err := group.Next(ctx)
		if err != nil {
			if ctx.Err() == nil {
				c.logger.WarnContext(ctx, groupUnavailable, "error", err)
			}
			continue
		}
		for _, assignment := range generation.Assignments[c.config.InputTopic] {
			generation.Start(func(ctx context.Context) {
				if err := c.consumePartition(ctx, generation, assignment); err != nil && ctx.Err() == nil {
					c.logger.WarnContext(ctx, partitionInterrupted,
						"topic", c.config.InputTopic, "partition", assignment.ID, "error", err)
					timer := time.NewTimer(partitionRetryDelay)
					defer timer.Stop()
					select {
					case <-ctx.Done():
					case <-timer.C:
					}
				}
			})
		}
	}
	return nil
}

func (c *PriceChangedConsumer) consumePartition(ctx context.Context, generation *kafka.Generation, assignment kafka.PartitionAssignment) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	topic := c.config.InputTopic
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:         c.config.Brokers,
		Topic:           topic,
		Partition:       assignment.ID,
		ReadLagInterval: disabledLagPolling,
	})
	defer reader.Close()
	if err := reader.SetOffset(assignment.Offset); err != nil {
		return fmt.Errorf("set partition offset: %w", err)
	}

	// Use the assignment's generation to fence commits after a rebalance.
	return c.consume(ctx, reader, func(message kafka.Message) error {
		return generation.CommitOffsets(map[string]map[int]int64{
			topic: {assignment.ID: message.Offset + 1},
		})
	})
}

type messageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
}

func (c *PriceChangedConsumer) consume(ctx context.Context, reader messageReader, commit func(kafka.Message) error) error {
	for ctx.Err() == nil {
		message, err := reader.FetchMessage(ctx)
		if err != nil {
			return fmt.Errorf("fetch price changed message: %w", err)
		}
		if err := c.handle(ctx, message); err != nil {
			if ctx.Err() == nil {
				c.logger.ErrorContext(ctx, invalidRecord,
					"topic", message.Topic, "partition", message.Partition, "offset", message.Offset, "error", err)
				// Committing any subsequent offset here would acknowledge the invalid record as well.
				<-ctx.Done()
			}
			return ctx.Err()
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := commit(message); err != nil {
			return fmt.Errorf("commit price changed message at offset %d: %w", message.Offset, err)
		}
	}
	return ctx.Err()
}

func (c *PriceChangedConsumer) handle(ctx context.Context, message kafka.Message) error {
	var event events.PriceChangedEvent
	if err := json.Unmarshal(message.Value, &event); err != nil {
		return fmt.Errorf("decode price changed event: %w", err)
	}
	return c.handler.HandlePriceChanged(ctx, event)
}
