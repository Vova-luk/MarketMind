package providers

import (
	"fmt"
	kafkaproducer "marketmind/exchange-data/internal/adapters/outbound/kafka_producer"
	"marketmind/exchange-data/internal/config"
)

type KafkaProvider struct {
	PriceCheckedProducer *kafkaproducer.PriceCheckedProducer
}

func NewKafkaProvider(cfg config.KafkaConfig) (*KafkaProvider, error) {
	if len(cfg.BrokerList()) == 0 {
		return nil, fmt.Errorf("kafka brokers are not configured")
	}

	if cfg.PriceCheckedTopic == "" {
		return nil, fmt.Errorf("task checked prices topic is not configured")
	}

	return &KafkaProvider{
		PriceCheckedProducer: kafkaproducer.NewPriceCheckedProducer(
			cfg.PriceCheckedTopic,
			cfg.BrokerList(),
		),
	}, nil
}

func (p *KafkaProvider) Close() error {
	if p == nil {
		return nil
	}

	if p.PriceCheckedProducer != nil {
		if err := p.PriceCheckedProducer.Close(); err != nil {
			return err
		}
	}

	return nil
}
