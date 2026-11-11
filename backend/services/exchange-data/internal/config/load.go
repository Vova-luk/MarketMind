package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"go.yaml.in/yaml/v3"

	"marketmind/contracts/events"
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
		GRPC:      GRPCConfig{Host: "127.0.0.1", Port: 9090, ShutdownTimeout: 10 * time.Second},
		API:       APIConfig{Address: "127.0.0.1:50051", RequestTimeout: 5 * time.Second},
		Kafka:     KafkaConfig{Brokers: "localhost:29092,localhost:39092", PriceCheckedTopic: events.PricesCheckedTopic},
		Scheduler: SchedulerConfig{Interval: 10 * time.Second},
		Exchanges: ExchangesConfig{
			Binance: ExchangeConfig{BaseURL: "https://data-api.binance.vision", Timeout: 10 * time.Second, RetryDelay: 3 * time.Second, MaxAttempts: 3},
			Bybit:   ExchangeConfig{BaseURL: "https://api.bybit.com", Timeout: 5 * time.Second, MaxAttempts: 1},
			OKX:     ExchangeConfig{BaseURL: "https://www.okx.com", Timeout: 5 * time.Second, MaxAttempts: 1},
		},
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
	if value, ok := os.LookupEnv("EXCHANGE_DATA_SCHEDULER_DISABLED"); ok {
		disabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("EXCHANGE_DATA_SCHEDULER_DISABLED must be a boolean")
		}
		c.Scheduler.Disabled = disabled
	}
	for _, field := range []struct {
		name   string
		target *string
	}{
		{"EXCHANGE_DATA_SERVER_HOST", &c.Server.Host},
		{"EXCHANGE_DATA_GRPC_HOST", &c.GRPC.Host},
		{"EXCHANGE_DATA_API_ADDRESS", &c.API.Address},
		{"EXCHANGE_DATA_KAFKA_OUTPUT_TOPIC", &c.Kafka.PriceCheckedTopic},
		{"EXCHANGE_DATA_KAFKA_BROKERS", &c.Kafka.Brokers},
		{"EXCHANGE_DATA_BINANCE_BASE_URL", &c.Exchanges.Binance.BaseURL},
		{"EXCHANGE_DATA_BYBIT_BASE_URL", &c.Exchanges.Bybit.BaseURL},
		{"EXCHANGE_DATA_OKX_BASE_URL", &c.Exchanges.OKX.BaseURL},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			*field.target = value
		}
	}
	for _, field := range []struct {
		name   string
		target *int
	}{
		{"EXCHANGE_DATA_SERVER_PORT", &c.Server.Port},
		{"EXCHANGE_DATA_GRPC_PORT", &c.GRPC.Port},
		{"EXCHANGE_DATA_BINANCE_MAX_ATTEMPTS", &c.Exchanges.Binance.MaxAttempts},
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
		{"EXCHANGE_DATA_SERVER_READ_TIMEOUT", &c.Server.ReadTimeout},
		{"EXCHANGE_DATA_SERVER_WRITE_TIMEOUT", &c.Server.WriteTimeout},
		{"EXCHANGE_DATA_SERVER_SHUTDOWN_TIMEOUT", &c.Server.ShutdownTimeout},
		{"EXCHANGE_DATA_SERVER_HEARTBEAT_INTERVAL", &c.Server.HeartbeatInterval},
		{"EXCHANGE_DATA_GRPC_SHUTDOWN_TIMEOUT", &c.GRPC.ShutdownTimeout},
		{"EXCHANGE_DATA_API_REQUEST_TIMEOUT", &c.API.RequestTimeout},
		{"EXCHANGE_DATA_POLL_INTERVAL", &c.Scheduler.Interval},
		{"EXCHANGE_DATA_BINANCE_TIMEOUT", &c.Exchanges.Binance.Timeout},
		{"EXCHANGE_DATA_BINANCE_RETRY_DELAY", &c.Exchanges.Binance.RetryDelay},
		{"EXCHANGE_DATA_BYBIT_TIMEOUT", &c.Exchanges.Bybit.Timeout},
		{"EXCHANGE_DATA_OKX_TIMEOUT", &c.Exchanges.OKX.Timeout},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("%s must be a duration such as 5s", field.name)
			}
			*field.target = duration
		}
	}
	return nil
}
