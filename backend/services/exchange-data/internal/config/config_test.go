package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func clearExchangeEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"EXCHANGE_DATA_SERVER_HOST", "EXCHANGE_DATA_SERVER_PORT", "EXCHANGE_DATA_SERVER_READ_TIMEOUT",
		"EXCHANGE_DATA_SERVER_WRITE_TIMEOUT", "EXCHANGE_DATA_SERVER_SHUTDOWN_TIMEOUT",
		"EXCHANGE_DATA_GRPC_HOST", "EXCHANGE_DATA_GRPC_PORT", "EXCHANGE_DATA_GRPC_SHUTDOWN_TIMEOUT",
		"EXCHANGE_DATA_KAFKA_BROKERS", "EXCHANGE_DATA_KAFKA_OUTPUT_TOPIC", "EXCHANGE_DATA_POLL_INTERVAL",
		"EXCHANGE_DATA_BINANCE_BASE_URL", "EXCHANGE_DATA_BINANCE_TIMEOUT",
		"EXCHANGE_DATA_BINANCE_RETRY_DELAY", "EXCHANGE_DATA_BINANCE_MAX_ATTEMPTS",
		"EXCHANGE_DATA_BYBIT_BASE_URL", "EXCHANGE_DATA_BYBIT_TIMEOUT",
		"EXCHANGE_DATA_OKX_BASE_URL", "EXCHANGE_DATA_OKX_TIMEOUT",
	} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadConfigWithoutExternalConnections(t *testing.T) {
	clearExchangeEnv(t)
	cfg, err := Load("testdata/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8184 || cfg.Server.ShutdownTimeout != 10*time.Second || cfg.GRPC.Port != 9090 {
		t.Fatal("server settings were not decoded")
	}
	if cfg.Kafka.OutputTopic != "Price-Checked-Topic" || len(cfg.Kafka.Brokers) != 1 {
		t.Fatal("Kafka producer settings were not decoded")
	}
	if cfg.Scheduler.Interval != 10*time.Second || cfg.Exchanges.Binance.BaseURL != "https://data-api.binance.vision" || cfg.Exchanges.Bybit.Timeout != 5*time.Second || cfg.Exchanges.OKX.BaseURL != "https://www.okx.com" {
		t.Fatal("scheduler or exchange settings were not decoded")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearExchangeEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "server:\n  host: 127.0.0.1\n  port: 8184\n  read_timeout: 5s\n  write_timeout: 10s\n  shutdown_timeout: 10s\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("integration defaults must allow HTTP scaffold startup: %v", err)
	}
}

func TestLoadDotenvAndProcessPrecedence(t *testing.T) {
	clearExchangeEnv(t)
	path := filepath.Join(t.TempDir(), ".env")
	data := "EXCHANGE_DATA_SERVER_PORT=8284\nEXCHANGE_DATA_GRPC_PORT=9190\nEXCHANGE_DATA_POLL_INTERVAL=15s\nEXCHANGE_DATA_KAFKA_BROKERS=broker-a:9092, broker-b:9092\nEXCHANGE_DATA_OKX_TIMEOUT=8s\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXCHANGE_DATA_SERVER_PORT", "8384")
	cfg, err := Load("testdata/config.yaml", path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8384 || cfg.GRPC.Port != 9190 || cfg.Scheduler.Interval != 15*time.Second || cfg.Exchanges.OKX.Timeout != 8*time.Second {
		t.Fatal("expected process > dotenv > YAML precedence")
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[1] != "broker-b:9092" {
		t.Fatal("broker CSV was not decoded or trimmed")
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"EXCHANGE_DATA_SERVER_PORT", "invalid"},
		{"EXCHANGE_DATA_SERVER_PORT", "0"},
		{"EXCHANGE_DATA_GRPC_PORT", "invalid"},
		{"EXCHANGE_DATA_GRPC_PORT", "8184"},
		{"EXCHANGE_DATA_GRPC_SHUTDOWN_TIMEOUT", "0s"},
		{"EXCHANGE_DATA_SERVER_READ_TIMEOUT", "invalid"},
		{"EXCHANGE_DATA_KAFKA_BROKERS", "broker-a:9092,"},
		{"EXCHANGE_DATA_KAFKA_OUTPUT_TOPIC", ""},
		{"EXCHANGE_DATA_POLL_INTERVAL", "0s"},
		{"EXCHANGE_DATA_BINANCE_BASE_URL", "invalid"},
		{"EXCHANGE_DATA_BINANCE_TIMEOUT", "0s"},
		{"EXCHANGE_DATA_BINANCE_RETRY_DELAY", "-1s"},
		{"EXCHANGE_DATA_BINANCE_RETRY_DELAY", "invalid"},
		{"EXCHANGE_DATA_BINANCE_MAX_ATTEMPTS", "0"},
		{"EXCHANGE_DATA_BINANCE_MAX_ATTEMPTS", "-1"},
		{"EXCHANGE_DATA_BINANCE_MAX_ATTEMPTS", "invalid"},
		{"EXCHANGE_DATA_BYBIT_BASE_URL", "invalid"},
		{"EXCHANGE_DATA_OKX_TIMEOUT", "0s"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			clearExchangeEnv(t)
			t.Setenv(tc.key, tc.value)
			if _, err := Load("testdata/config.yaml"); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func TestLoadRejectsInvalidFiles(t *testing.T) {
	clearExchangeEnv(t)
	if _, err := Load(""); err == nil {
		t.Fatal("empty path was accepted")
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing YAML was accepted")
	}
	if _, err := Load("testdata/config.yaml", filepath.Join(t.TempDir(), "missing.env")); err == nil {
		t.Fatal("missing explicit dotenv file was accepted")
	}
	for _, data := range []string{"server: [invalid", "server:\n  prot: 8184\n", "database:\n  host: localhost\n", "kafka:\n  consumer_group: unexpected\n"} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("malformed YAML or unexpected service settings were accepted")
		}
	}
}

func TestBinanceSettingsPrecedence(t *testing.T) {
	clearExchangeEnv(t)
	cfg, err := Load("testdata/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Exchanges.Binance.Timeout != 5*time.Second || cfg.Exchanges.Binance.RetryDelay != 2*time.Second || cfg.Exchanges.Binance.MaxAttempts != 4 {
		t.Fatalf("Binance YAML settings not loaded: %+v", cfg.Exchanges.Binance)
	}
	path := filepath.Join(t.TempDir(), ".env")
	data := "EXCHANGE_DATA_BINANCE_BASE_URL=https://binance.example\nEXCHANGE_DATA_BINANCE_TIMEOUT=7s\nEXCHANGE_DATA_BINANCE_RETRY_DELAY=4s\nEXCHANGE_DATA_BINANCE_MAX_ATTEMPTS=5\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXCHANGE_DATA_BINANCE_MAX_ATTEMPTS", "2")
	cfg, err = Load("testdata/config.yaml", path)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Exchanges.Binance
	if got.BaseURL != "https://binance.example" || got.Timeout != 7*time.Second || got.RetryDelay != 4*time.Second || got.MaxAttempts != 2 {
		t.Fatalf("expected process > dotenv > YAML: %+v", got)
	}
}

func TestProjectConfigLoads(t *testing.T) {
	clearExchangeEnv(t)
	cfg, err := Load("../../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Exchanges.Binance.BaseURL != "https://data-api.binance.vision" || cfg.Exchanges.Binance.RetryDelay != 3*time.Second || cfg.Exchanges.Binance.MaxAttempts != 3 {
		t.Fatalf("unexpected project Binance settings: %+v", cfg.Exchanges.Binance)
	}
}
