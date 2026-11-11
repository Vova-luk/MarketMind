package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server ServerConfig   `yaml:"server"`
	DB     DatabaseConfig `yaml:"database"`
	Kafka  KafkaConfig    `yaml:"kafka"`
}

type DatabaseConfig struct {
	Port           string        `yaml:"port"`
	Host           string        `yaml:"host"`
	User           string        `yaml:"user"`
	Password       string        `yaml:"password"`
	Name           string        `yaml:"name"`
	SSLMode        string        `yaml:"sslmode"`
	PoolMaxConns   int           `yaml:"max_pool_conns"`
	PoolMinConns   int           `yaml:"min_pool_conns"`
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
}

type KafkaConfig struct {
	Brokers           []string `yaml:"brokers"`
	GroupID           string   `yaml:"consumer_group"`
	PriceCheckedTopic string   `yaml:"input_topic"`
	PriceChangedTopic string   `yaml:"output_topic"`
}

type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

func (c Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if c.DB.Host == "" || c.DB.User == "" || c.DB.Password == "" || c.DB.Name == "" {
		return fmt.Errorf("database host, user, password and name are required")
	}
	port, err := strconv.Atoi(c.DB.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("database port must be between 1 and 65535")
	}
	if c.DB.PoolMaxConns < 1 || c.DB.PoolMinConns < 0 || c.DB.PoolMinConns > c.DB.PoolMaxConns {
		return fmt.Errorf("database pool limits must satisfy 0 <= min <= max and max > 0")
	}
	if c.DB.ConnectTimeout <= 0 {
		return fmt.Errorf("database connect timeout must be greater than 0")
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("Kafka brokers are required")
	}
	for _, broker := range c.Kafka.Brokers {
		if strings.TrimSpace(broker) == "" {
			return fmt.Errorf("Kafka brokers must not contain empty addresses")
		}
	}
	if strings.TrimSpace(c.Kafka.GroupID) == "" || strings.TrimSpace(c.Kafka.PriceCheckedTopic) == "" || strings.TrimSpace(c.Kafka.PriceChangedTopic) == "" {
		return fmt.Errorf("Kafka consumer group and topics are required")
	}
	return nil
}

func (c ServerConfig) Validate() error {
	switch {
	case c.Host == "":
		return fmt.Errorf("host can't be empty")
	case c.Port <= 0 || c.Port > 65535:
		return fmt.Errorf("port must be between 1 and 65535")
	case c.ReadTimeout <= 0:
		return fmt.Errorf("read timeout must be greater than 0")
	case c.WriteTimeout <= 0:
		return fmt.Errorf("write timeout must be greater than 0")
	case c.ShutdownTimeout <= 0:
		return fmt.Errorf("shutdown timeout must be greater than 0")
	}
	return nil
}
