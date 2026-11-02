package config

import (
	"testing"
	"time"

	"marketmind/api/internal/server"
	"marketmind/utils/postgres"
)

func TestLoadExampleConfig(t *testing.T) {
	t.Setenv("MARKETMIND_DATABASE__PASSWORD", "test-password")
	t.Setenv("MARKETMIND_SERVER__PORT", "8181")
	t.Setenv("MARKETMIND_DATABASE__CONNECT_TIMEOUT", "3s")
	cfg, err := Load("../../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Int("server.port") != 8181 || cfg.Duration("database.connect_timeout") != 3*time.Second || cfg.String("database.password") != "test-password" {
		t.Fatal("environment overrides were not applied to nested config")
	}
	if cfg.String("server.host") != "0.0.0.0" {
		t.Fatal("server section is unavailable")
	}
	var database postgres.Config
	if err := cfg.Unmarshal("database", &database); err != nil {
		t.Fatal(err)
	}
	if database.ConnectTimeout != 3*time.Second || database.MaxConns != 10 || database.Password != "test-password" {
		t.Fatal("database YAML tags or environment overrides were not decoded")
	}
	var httpConfig server.Config
	if err := cfg.Unmarshal("server", &httpConfig); err != nil {
		t.Fatal(err)
	}
	if httpConfig.Port != 8181 {
		t.Fatal("server YAML tags or environment overrides were not decoded")
	}
}
