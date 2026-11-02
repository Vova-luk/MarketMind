package server

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

func TestServerUsesAndValidatesConfig(t *testing.T) {
	cfg := Config{
		Host: "0.0.0.0", Port: 8080,
		ReadTimeout: 10 * time.Second, ShutdownTimeout: 10 * time.Second,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := New(cfg, http.NewServeMux(), log)
	if err != nil {
		t.Fatal(err)
	}
	if srv.Addr != "0.0.0.0:8080" || srv.shutdownTimeout != 10*time.Second {
		t.Fatal("server configuration was not applied")
	}
	for name, change := range map[string]func(*Config){
		"port":             func(c *Config) { c.Port = 0 },
		"shutdown_timeout": func(c *Config) { c.ShutdownTimeout = 0 },
		"read_timeout":     func(c *Config) { c.ReadTimeout = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := cfg
			change(&invalid)
			if _, err := New(invalid, http.NewServeMux(), log); err == nil {
				t.Fatal("expected invalid server config to fail")
			}
		})
	}
}
