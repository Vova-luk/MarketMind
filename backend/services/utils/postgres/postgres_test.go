package postgres

import (
	"context"
	"testing"
	"time"
)

func TestConnectRejectsInvalidDatabaseConfig(t *testing.T) {
	cfg := Config{
		Host: "localhost", Port: 5432, User: "test", Password: "test-password",
		Database: "test", SSLMode: "disable", MaxConns: 10, ConnectTimeout: 5 * time.Second,
	}
	for name, change := range map[string]func(*Config){
		"password":  func(c *Config) { c.Password = "" },
		"max_conns": func(c *Config) { c.MaxConns = 0 },
		"sslmode":   func(c *Config) { c.SSLMode = "invalid" },
		"port":      func(c *Config) { c.Port = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := cfg
			change(&invalid)
			if _, err := Connect(context.Background(), invalid); err == nil {
				t.Fatal("expected invalid database config to fail before connecting")
			}
		})
	}
}
