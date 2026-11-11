package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.yaml.in/yaml/v3"

	"marketmind/contracts/events"
)

const (
	defaultKafkaBroker     = "localhost:9092"
	defaultKafkaGroupID    = "marketmind-bot"
	defaultKafkaWorkers    = 1
	defaultTelegramAPIURL  = "https://api.telegram.org"
	defaultTelegramTimeout = 5 * time.Second
)

const (
	envServerHost            = "BOT_SERVER_HOST"
	envServerPort            = "BOT_SERVER_PORT"
	envServerReadTimeout     = "BOT_SERVER_READ_TIMEOUT"
	envServerWriteTimeout    = "BOT_SERVER_WRITE_TIMEOUT"
	envServerShutdownTimeout = "BOT_SERVER_SHUTDOWN_TIMEOUT"
	envKafkaBrokers          = "BOT_KAFKA_BROKERS"
	envKafkaGroupID          = "BOT_KAFKA_GROUP_ID"
	envKafkaInputTopic       = "BOT_KAFKA_INPUT_TOPIC"
	envKafkaWorkers          = "BOT_KAFKA_WORKERS"
	envTelegramToken         = "BOT_TOKEN"
	envTelegramAPIURL        = "BOT_TELEGRAM_API_URL"
	envTelegramTimeout       = "BOT_TELEGRAM_TIMEOUT"
	envMiniAppURL            = "BOT_MINI_APP_URL"
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
			Brokers: []string{defaultKafkaBroker},
			GroupID: defaultKafkaGroupID, InputTopic: events.PriceChangedTopic, Workers: defaultKafkaWorkers,
		},
		Telegram: TelegramConfig{APIURL: defaultTelegramAPIURL, RequestTimeout: defaultTelegramTimeout},
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
		{envServerHost, &c.Server.Host},
		{envKafkaGroupID, &c.Kafka.GroupID},
		{envKafkaInputTopic, &c.Kafka.InputTopic},
		{envTelegramToken, &c.Telegram.Token},
		{envTelegramAPIURL, &c.Telegram.APIURL},
		{envMiniAppURL, &c.Telegram.MiniAppURL},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			*field.target = value
		}
	}
	for _, field := range []struct {
		name   string
		target *int
	}{
		{envServerPort, &c.Server.Port},
		{envKafkaWorkers, &c.Kafka.Workers},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			number, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("%s must be an integer", field.name)
			}
			*field.target = number
		}
	}
	for _, field := range []struct {
		name   string
		target *time.Duration
	}{
		{envServerReadTimeout, &c.Server.ReadTimeout},
		{envServerWriteTimeout, &c.Server.WriteTimeout},
		{envServerShutdownTimeout, &c.Server.ShutdownTimeout},
		{envTelegramTimeout, &c.Telegram.RequestTimeout},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("%s must be a duration such as 5s", field.name)
			}
			*field.target = duration
		}
	}
	if value, ok := os.LookupEnv(envKafkaBrokers); ok {
		c.Kafka.Brokers = strings.Split(value, ",")
		for i := range c.Kafka.Brokers {
			c.Kafka.Brokers[i] = strings.TrimSpace(c.Kafka.Brokers[i])
		}
	}
	return nil
}
