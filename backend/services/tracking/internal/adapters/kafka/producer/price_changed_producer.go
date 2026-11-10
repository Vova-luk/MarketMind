package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"marketmind/contracts/events"
	"marketmind/tracking/internal/config"
)

type PriceChangedProducer struct {
	writer *kafka.Writer
}

func NewPriceChangedProducer(cfg config.KafkaConfig) *PriceChangedProducer {
	return &PriceChangedProducer{writer: &kafka.Writer{
		Addr: kafka.TCP(cfg.Brokers...), Topic: cfg.PriceChangedTopic,
		Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll,
		MaxAttempts: 3, ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second, BatchTimeout: 100 * time.Millisecond,
	}}
}

func (p *PriceChangedProducer) SendPriceChangedEvent(ctx context.Context, event events.PriceChangedEvent) error {
	if event.EventID == "" || len(event.Items) == 0 {
		return fmt.Errorf("event ID and notifications are required")
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal price changed event: %w", err)
	}
	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key: []byte("price-changed"), Value: data,
	}); err != nil {
		return fmt.Errorf("publish price changed event: %w", err)
	}
	return nil
}

func (p *PriceChangedProducer) Close() error {
	return p.writer.Close()
}
