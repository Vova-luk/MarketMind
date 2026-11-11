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

// Load reads YAML and applies environment overrides. Optional envPaths are
// loaded with godotenv; existing process variables take precedence.
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
		DB: DatabaseConfig{
			SSLMode: "disable", PoolMaxConns: 10, PoolMinConns: 3,
			ConnectTimeout: 5 * time.Second,
		},
		Kafka: KafkaConfig{
			Brokers: []string{"localhost:29092", "localhost:39092"}, GroupID: "marketmind-tracking",
			PriceCheckedTopic: events.PricesCheckedTopic, PriceChangedTopic: events.PriceChangedTopic,
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
	for _, field := range []struct {
		name   string
		target *string
	}{
		{"DB_PORT", &c.DB.Port},
		{"DB_HOST", &c.DB.Host},
		{"DB_USER", &c.DB.User},
		{"DB_PASSWORD", &c.DB.Password},
		{"DB_NAME", &c.DB.Name},
		{"DB_SSLMODE", &c.DB.SSLMode},
		{"KAFKA_GROUP_ID", &c.Kafka.GroupID},
		{"KAFKA_TOPIC_PRICE_CHECKED", &c.Kafka.PriceCheckedTopic},
		{"KAFKA_TOPIC_PRICE_CHANGED", &c.Kafka.PriceChangedTopic},
		{"TRACKING_SERVER_HOST", &c.Server.Host},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			*field.target = value
		}
	}
	for _, field := range []struct {
		name   string
		target *int
	}{
		{"DB_MAX_POOL_CONNS", &c.DB.PoolMaxConns},
		{"DB_MIN_POOL_CONNS", &c.DB.PoolMinConns},
		{"TRACKING_SERVER_PORT", &c.Server.Port},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("%s must be an integer", field.name)
			}
			*field.target = parsed
		}
	}
	for _, field := range []struct {
		name   string
		target *time.Duration
	}{
		{"DB_CONNECT_TIMEOUT", &c.DB.ConnectTimeout},
		{"TRACKING_SERVER_READ_TIMEOUT", &c.Server.ReadTimeout},
		{"TRACKING_SERVER_WRITE_TIMEOUT", &c.Server.WriteTimeout},
		{"TRACKING_SERVER_SHUTDOWN_TIMEOUT", &c.Server.ShutdownTimeout},
		{"TRACKING_SERVER_HEARTBEAT_INTERVAL", &c.Server.HeartbeatInterval},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("%s must be a duration such as 5s", field.name)
			}
			*field.target = parsed
		}
	}
	if value, ok := os.LookupEnv("KAFKA_BROKERS"); ok {
		c.Kafka.Brokers = strings.Split(value, ",")
		for i := range c.Kafka.Brokers {
			c.Kafka.Brokers[i] = strings.TrimSpace(c.Kafka.Brokers[i])
		}
	}
	return nil
}
