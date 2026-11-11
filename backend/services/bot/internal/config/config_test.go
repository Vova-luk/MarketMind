package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func clearBotEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"BOT_SERVER_HOST", "BOT_SERVER_PORT", "BOT_SERVER_READ_TIMEOUT",
		"BOT_SERVER_WRITE_TIMEOUT", "BOT_SERVER_SHUTDOWN_TIMEOUT",
		"BOT_SERVER_HEARTBEAT_INTERVAL",
		"BOT_KAFKA_BROKERS", "BOT_KAFKA_GROUP_ID", "BOT_KAFKA_INPUT_TOPIC", "BOT_KAFKA_WORKERS",
		"BOT_TOKEN", "BOT_TELEGRAM_API_URL", "BOT_TELEGRAM_TIMEOUT", "BOT_MINI_APP_URL",
	} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadRequiresBotToken(t *testing.T) {
	clearBotEnv(t)
	if _, err := Load("testdata/config.yaml"); err == nil {
		t.Fatal("missing BOT_TOKEN was accepted")
	}
}

func TestLoadConfig(t *testing.T) {
	clearBotEnv(t)
	t.Setenv("BOT_TOKEN", "test-token")
	cfg, err := Load("testdata/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8183 || cfg.Server.ShutdownTimeout != 10*time.Second || cfg.Server.HeartbeatInterval != 12*time.Minute {
		t.Fatal("server YAML was not decoded")
	}
	if cfg.Kafka.GroupID != "marketmind-bot" || cfg.Kafka.InputTopic != "Price-Changed-Topic" || len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Workers != 1 {
		t.Fatal("Kafka consumer settings were not decoded")
	}
	if cfg.Telegram.RequestTimeout != 5*time.Second || cfg.Telegram.APIURL != "https://api.telegram.org" {
		t.Fatal("Telegram settings were not decoded")
	}
}

func TestLoadDotenvAndProcessPrecedence(t *testing.T) {
	clearBotEnv(t)
	path := filepath.Join(t.TempDir(), ".env")
	data := "BOT_SERVER_PORT=8283\nBOT_TOKEN=file-token\nBOT_TELEGRAM_TIMEOUT=8s\nBOT_KAFKA_BROKERS=broker-a:9092, broker-b:9092\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BOT_SERVER_PORT", "8383")
	t.Setenv("BOT_TOKEN", "process-token")
	t.Setenv("BOT_KAFKA_WORKERS", "3")
	t.Setenv("BOT_SERVER_HEARTBEAT_INTERVAL", "45m")
	cfg, err := Load("testdata/config.yaml", path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.HeartbeatInterval != 45*time.Minute {
		t.Fatal("environment heartbeat interval did not override YAML")
	}
	if cfg.Server.Port != 8383 || cfg.Telegram.Token != "process-token" || cfg.Telegram.RequestTimeout != 8*time.Second {
		t.Fatal("expected process > dotenv > YAML precedence")
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[1] != "broker-b:9092" || cfg.Kafka.Workers != 3 {
		t.Fatal("Kafka overrides were not applied")
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"BOT_SERVER_PORT", "invalid"},
		{"BOT_SERVER_PORT", "0"},
		{"BOT_SERVER_HEARTBEAT_INTERVAL", "-1m"},
		{"BOT_SERVER_READ_TIMEOUT", "invalid"},
		{"BOT_SERVER_SHUTDOWN_TIMEOUT", "0s"},
		{"BOT_KAFKA_BROKERS", "broker-a:9092,"},
		{"BOT_KAFKA_GROUP_ID", ""},
		{"BOT_KAFKA_INPUT_TOPIC", ""},
		{"BOT_KAFKA_WORKERS", "invalid"},
		{"BOT_KAFKA_WORKERS", "0"},
		{"BOT_TELEGRAM_API_URL", "invalid"},
		{"BOT_TELEGRAM_TIMEOUT", "0s"},
		{"BOT_MINI_APP_URL", "invalid"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			clearBotEnv(t)
			t.Setenv("BOT_TOKEN", "test-token")
			t.Setenv(tc.key, tc.value)
			if _, err := Load("testdata/config.yaml"); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func TestLoadRejectsInvalidFiles(t *testing.T) {
	clearBotEnv(t)
	if _, err := Load(""); err == nil {
		t.Fatal("empty path was accepted")
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing YAML was accepted")
	}
	if _, err := Load("testdata/config.yaml", filepath.Join(t.TempDir(), "missing.env")); err == nil {
		t.Fatal("missing explicit dotenv file was accepted")
	}
	for _, data := range []string{"server: [invalid", "server:\n  prot: 8183\n", "database:\n  host: localhost\n"} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("malformed YAML or unknown fields were accepted")
		}
	}
}
