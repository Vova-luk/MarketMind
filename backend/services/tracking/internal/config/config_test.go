package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DB_PORT", "DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"DB_MAX_POOL_CONNS", "DB_MIN_POOL_CONNS", "DB_CONNECT_TIMEOUT",
		"KAFKA_BROKERS", "KAFKA_GROUP_ID", "KAFKA_TOPIC_PRICE_CHECKED", "KAFKA_TOPIC_PRICE_CHANGED",
		"TRACKING_SERVER_HOST", "TRACKING_SERVER_PORT", "TRACKING_SERVER_READ_TIMEOUT",
		"TRACKING_SERVER_WRITE_TIMEOUT", "TRACKING_SERVER_SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadExampleYAML(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DB_PASSWORD", "test-password")
	cfg, err := Load("../../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8182 || cfg.Server.ReadTimeout != 5*time.Second {
		t.Fatal("server YAML was not decoded")
	}
	if cfg.DB.Port != "5432" || cfg.DB.Name != "marketmind" || cfg.DB.ConnectTimeout != 5*time.Second || cfg.DB.PoolMaxConns != 10 {
		t.Fatal("database YAML or defaults were not decoded")
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "localhost:29092" || cfg.Kafka.Brokers[1] != "localhost:39092" || cfg.Kafka.GroupID != "marketmind-tracking" || cfg.Kafka.PriceCheckedTopic != "Price-Checked-Topic" {
		t.Fatal("Kafka YAML was not decoded")
	}
}

func TestLoadDotenvAndProcessOverrides(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), ".env")
	data := "DB_PASSWORD=test-password\nTRACKING_SERVER_PORT=8282\nDB_CONNECT_TIMEOUT=8s\nKAFKA_BROKERS=broker-a:9092, broker-b:9092\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRACKING_SERVER_PORT", "8383")
	cfg, err := Load("../../config/config.yaml", path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8383 || cfg.DB.Password != "test-password" || cfg.DB.ConnectTimeout != 8*time.Second {
		t.Fatal("expected process > dotenv > YAML precedence")
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[1] != "broker-b:9092" {
		t.Fatal("broker list was not parsed or trimmed")
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"TRACKING_SERVER_PORT", "invalid"},
		{"TRACKING_SERVER_PORT", "0"},
		{"DB_CONNECT_TIMEOUT", "invalid"},
		{"DB_CONNECT_TIMEOUT", "0s"},
		{"DB_PASSWORD", ""},
		{"DB_MIN_POOL_CONNS", "11"},
		{"KAFKA_BROKERS", "broker-a:9092,"},
		{"KAFKA_GROUP_ID", ""},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("DB_PASSWORD", "test-password")
			t.Setenv(tc.key, tc.value)
			if _, err := Load("../../config/config.yaml"); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func TestLoadRejectsInvalidFiles(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DB_PASSWORD", "test-password")
	if _, err := Load(""); err == nil {
		t.Fatal("empty path was accepted")
	}
	if _, err := Load("../../config/config.yaml", filepath.Join(t.TempDir(), "missing.env")); err == nil {
		t.Fatal("missing explicit dotenv file was accepted")
	}
	for _, data := range []string{"server: [invalid", "server:\n  prot: 8182\n"} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("malformed YAML or unknown field was accepted")
		}
	}
}

func writePasswordConfig(t *testing.T, passwordLine string) string {
	t.Helper()
	data := `server:
  host: 127.0.0.1
  port: 8182
  read_timeout: 5s
  write_timeout: 10s
  shutdown_timeout: 10s

database:
  host: localhost
  port: 5432
  name: marketmind
  user: postgres
` + passwordLine
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMissingPasswordIsNotAccepted(t *testing.T) {
	for _, tc := range []struct{ name, passwordLine string }{
		{"omitted", ""},
		{"empty", "  password: \"\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			path := writePasswordConfig(t, tc.passwordLine)
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), "password") {
				t.Fatal("missing password should be reported")
			}
		})
	}
}

func TestLoadAcceptsPasswordFromYAML(t *testing.T) {
	clearConfigEnv(t)
	path := writePasswordConfig(t, "  password: yaml-test-password\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DB.Password != "yaml-test-password" {
		t.Fatal("password from YAML was not loaded")
	}
}
