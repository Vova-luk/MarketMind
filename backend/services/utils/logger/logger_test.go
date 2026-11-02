package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileLoggerFiltersLevelAndAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "api.log")
	cfg := Config{Level: "warn", Output: "file", Format: "json", File: path}
	for range 2 {
		log, closeLog, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		log.Info("filtered")
		log.Warn("kept", "service", "api")
		if err := closeLog(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two appended warnings, got %s", data)
	}
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["level"] != "WARN" || entry["msg"] != "kept" || entry["service"] != "api" {
			t.Fatalf("unexpected log entry: %s", line)
		}
	}
}

func TestLoggerRejectsInvalidOutput(t *testing.T) {
	for _, cfg := range []Config{
		{Level: "unknown", Output: "stdout", Format: "text"},
		{Level: "info", Output: "stderr", Format: "text"},
		{Level: "info", Output: "stdout", Format: "unknown"},
		{Level: "info", Output: "file", Format: "text"},
	} {
		if _, _, err := New(cfg); err == nil {
			t.Fatalf("expected invalid logger config to fail: %+v", cfg)
		}
	}
}
