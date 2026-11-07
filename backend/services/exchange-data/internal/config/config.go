package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	GRPC      GRPCConfig      `yaml:"grpc"`
	Kafka     KafkaConfig     `yaml:"kafka"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Exchanges ExchangesConfig `yaml:"exchanges"`
}

type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type GRPCConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type KafkaConfig struct {
	Brokers     []string `yaml:"brokers"`
	OutputTopic string   `yaml:"output_topic"`
}

type SchedulerConfig struct {
	Interval time.Duration `yaml:"interval"`
}

type ExchangesConfig struct {
	Binance ExchangeConfig `yaml:"binance"`
	Bybit   ExchangeConfig `yaml:"bybit"`
	OKX     ExchangeConfig `yaml:"okx"`
}

type ExchangeConfig struct {
	BaseURL     string        `yaml:"base_url"`
	Timeout     time.Duration `yaml:"request_timeout"`
	RetryDelay  time.Duration `yaml:"retry_delay"`
	MaxAttempts int           `yaml:"max_attempts"`
}

func (c Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := validateAddress(c.GRPC.Host, c.GRPC.Port); err != nil {
		return fmt.Errorf("grpc: %w", err)
	}
	if c.GRPC.ShutdownTimeout <= 0 {
		return fmt.Errorf("grpc shutdown_timeout must be positive")
	}
	if c.GRPC.Port == c.Server.Port {
		return fmt.Errorf("HTTP and gRPC ports must be different")
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("Kafka brokers are required")
	}
	for _, broker := range c.Kafka.Brokers {
		if strings.TrimSpace(broker) == "" {
			return fmt.Errorf("Kafka brokers must not contain empty addresses")
		}
	}
	if strings.TrimSpace(c.Kafka.OutputTopic) == "" {
		return fmt.Errorf("Kafka output topic is required")
	}
	if c.Scheduler.Interval <= 0 {
		return fmt.Errorf("scheduler interval must be positive")
	}
	for _, exchange := range []struct {
		name string
		cfg  ExchangeConfig
	}{
		{"binance", c.Exchanges.Binance},
		{"bybit", c.Exchanges.Bybit},
		{"okx", c.Exchanges.OKX},
	} {
		parsed, err := url.Parse(exchange.cfg.BaseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("%s base_url must be an absolute HTTP or HTTPS URL", exchange.name)
		}
		if exchange.cfg.RetryDelay < 0 {
			return fmt.Errorf("%s retry_delay must not be negative", exchange.name)
		}
		if exchange.cfg.MaxAttempts < 1 {
			return fmt.Errorf("%s max_attempts must be at least 1", exchange.name)
		}
		if exchange.cfg.Timeout <= 0 {
			return fmt.Errorf("%s request_timeout must be positive", exchange.name)
		}
	}
	return nil
}

func (c ServerConfig) Validate() error {
	if err := validateAddress(c.Host, c.Port); err != nil {
		return err
	}
	if c.ReadTimeout <= 0 || c.WriteTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return fmt.Errorf("server timeouts must be positive")
	}
	return nil
}

func validateAddress(host string, port int) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("host is required")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}
