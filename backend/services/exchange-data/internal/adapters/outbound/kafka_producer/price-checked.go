package kafkaproducer

import (
	"context"
	"encoding/json"
	"fmt"
	"marketmind/contracts/events"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

type PriceCheckedProducer struct {
	writer *kafka.Writer
	topic  string
}

func NewPriceCheckedProducer(topic string, brokers []string) *PriceCheckedProducer {
	return &PriceCheckedProducer{
		writer: &kafka.Writer{
			Topic:                  topic,
			Addr:                   kafka.TCP(brokers...),
			Balancer:               &kafka.Hash{},
			RequiredAcks:           kafka.RequireAll,
			MaxAttempts:            3,
			ReadTimeout:            5 * time.Second,
			WriteTimeout:           5 * time.Second,
			BatchTimeout:           100 * time.Millisecond,
			AllowAutoTopicCreation: true,
		},
		topic: topic,
	}
}

func (p *PriceCheckedProducer) SendPriceChecked(ctx context.Context, event events.PricesCheckedEvent) error {
	if len(event.Items) == 0 {
		return fmt.Errorf("event items must not be empty")
	}

	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal prices checked event: %w", err)
	}

	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte("prices-checked"),
		Value: value,
	}); err != nil {
		return fmt.Errorf("publish prices checked event: %w", err)
	}

	return nil
}

func (p *PriceCheckedProducer) Close() error {
	return p.writer.Close()
}
