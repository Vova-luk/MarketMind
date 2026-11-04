package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.yaml.in/yaml/v3"
)

// Load applies defaults, YAML and environment overrides, then validates Config.
// Optional dotenv files never override existing process variables.
func Load(path string, envPaths ...string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("config path is required")
	}
	if len(envPaths) > 0 {
		if err := godotenv.Load(envPaths...); err != nil {
			return nil, fmt.Errorf("load dotenv: %w", err)
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open YAML config: %w", err)
	}
	defer file.Close()
	cfg := Config{
		Kafka: KafkaConfig{
			Brokers: []string{"localhost:9092"},
			GroupID: "marketmind-bot", InputTopic: "Price-Changed-Topic",
		},
		Telegram: TelegramConfig{APIURL: "https://api.telegram.org", RequestTimeout: 5 * time.Second},
	}
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode YAML config: %w", err)
	}
	if err := cfg.applyEnv(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) applyEnv() error {
	for _, field := range []struct {
		name   string
		target *string
	}{
		{"BOT_SERVER_HOST", &c.Server.Host},
		{"BOT_KAFKA_GROUP_ID", &c.Kafka.GroupID},
		{"BOT_KAFKA_INPUT_TOPIC", &c.Kafka.InputTopic},
		{"BOT_TOKEN", &c.Telegram.Token},
		{"BOT_TELEGRAM_API_URL", &c.Telegram.APIURL},
		{"BOT_MINI_APP_URL", &c.Telegram.MiniAppURL},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			*field.target = value
		}
	}
	if value, ok := os.LookupEnv("BOT_SERVER_PORT"); ok {
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("BOT_SERVER_PORT must be an integer")
		}
		c.Server.Port = port
	}
	for _, field := range []struct {
		name   string
		target *time.Duration
	}{
		{"BOT_SERVER_READ_TIMEOUT", &c.Server.ReadTimeout},
		{"BOT_SERVER_WRITE_TIMEOUT", &c.Server.WriteTimeout},
		{"BOT_SERVER_SHUTDOWN_TIMEOUT", &c.Server.ShutdownTimeout},
		{"BOT_TELEGRAM_TIMEOUT", &c.Telegram.RequestTimeout},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("%s must be a duration such as 5s", field.name)
			}
			*field.target = duration
		}
	}
	if value, ok := os.LookupEnv("BOT_KAFKA_BROKERS"); ok {
		c.Kafka.Brokers = strings.Split(value, ",")
		for i := range c.Kafka.Brokers {
			c.Kafka.Brokers[i] = strings.TrimSpace(c.Kafka.Brokers[i])
		}
	}
	return nil
}
