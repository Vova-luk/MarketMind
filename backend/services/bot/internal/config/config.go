package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	Telegram TelegramConfig `yaml:"telegram"`
}

type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type KafkaConfig struct {
	Brokers    []string `yaml:"brokers"`
	GroupID    string   `yaml:"consumer_group"`
	InputTopic string   `yaml:"input_topic"`
}

type TelegramConfig struct {
	// Token is optional until the Telegram adapter is connected.
	Token          string        `yaml:"token"`
	APIURL         string        `yaml:"api_url"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
	MiniAppURL     string        `yaml:"mini_app_url"`
}

func (c Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("Kafka brokers are required")
	}
	for _, broker := range c.Kafka.Brokers {
		if strings.TrimSpace(broker) == "" {
			return fmt.Errorf("Kafka brokers must not contain empty addresses")
		}
	}
	if strings.TrimSpace(c.Kafka.GroupID) == "" || strings.TrimSpace(c.Kafka.InputTopic) == "" {
		return fmt.Errorf("Kafka consumer group and input topic are required")
	}
	if !validHTTPURL(c.Telegram.APIURL) {
		return fmt.Errorf("Telegram api_url must be an absolute HTTP or HTTPS URL")
	}
	if c.Telegram.RequestTimeout <= 0 {
		return fmt.Errorf("Telegram request_timeout must be positive")
	}
	if c.Telegram.MiniAppURL != "" && !validHTTPURL(c.Telegram.MiniAppURL) {
		return fmt.Errorf("Telegram mini_app_url must be an absolute HTTP or HTTPS URL")
	}
	return nil
}

func (c ServerConfig) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if c.ReadTimeout <= 0 || c.WriteTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return fmt.Errorf("server timeouts must be positive")
	}
	return nil
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}
