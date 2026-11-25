package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type Environment struct {
	DB       *pgx.Conn
	api      Client
	botToken string
}

// New connects to the configured test database and sets up an API client.
func New(t testing.TB, ctx context.Context) Environment {
	t.Helper()
	botToken := os.Getenv("MARKETMIND_TELEGRAM__BOT_TOKEN")
	if botToken == "" {
		t.Fatal("MARKETMIND_TELEGRAM__BOT_TOKEN must match the test API token")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL must point to the isolated database used by the test API")
	}
	baseURL := os.Getenv("TEST_API_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}

	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.Close(ctx); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	return Environment{
		DB:       db,
		api:      NewClient(baseURL),
		botToken: botToken,
	}
}
