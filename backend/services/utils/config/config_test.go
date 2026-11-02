package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadReadsPathsAndIndependentSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8080\nhandlers:\n  prices:\n    timeout: 2s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_TEST_SERVER__PORT", "9090")
	t.Setenv("CONFIG_TEST_HANDLERS__PRICES__TIMEOUT", "3s")
	t.Setenv("CONFIG_TEST_HANDLERS__PRICES__PAGE_SIZE", "25")
	reader, err := Load(path, "CONFIG_TEST_")
	if err != nil {
		t.Fatal(err)
	}
	if reader.Int("server.port") != 9090 || reader.Duration("handlers.prices.timeout") != 3*time.Second {
		t.Fatal("prefixed environment did not override YAML")
	}
	var prices struct {
		Timeout  time.Duration `yaml:"timeout"`
		PageSize int           `yaml:"page_size"`
	}
	if err := reader.Unmarshal("handlers.prices", &prices); err != nil {
		t.Fatal(err)
	}
	if prices.Timeout != 3*time.Second || prices.PageSize != 25 {
		t.Fatalf("unexpected handler section: %+v", prices)
	}
}

func TestLoadRejectsInvalidYAMLAndMissingFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if _, err := Load(path, ""); err == nil {
		t.Fatal("expected missing file to fail")
	}
	if err := os.WriteFile(path, []byte("port: [invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, ""); err == nil {
		t.Fatal("expected invalid YAML to fail")
	}
}

func TestLoadEnvPreservesProcessEnvironment(t *testing.T) {
	t.Setenv("MARKETMIND_TEST_VALUE", "process")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("MARKETMIND_TEST_VALUE=file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("MARKETMIND_TEST_VALUE"); got != "process" {
		t.Fatalf("process environment overwritten: %q", got)
	}
}
