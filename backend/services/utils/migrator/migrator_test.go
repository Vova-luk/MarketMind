package migrator

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"

	"marketmind/utils/migrations"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestEmbeddedMigrationsAreDiscovered(t *testing.T) {
	db, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := newProvider(db, migrations.Files, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	sources := provider.ListSources()
	if len(sources) != 6 {
		t.Fatalf("expected six migrations, got %d", len(sources))
	}
	for i, source := range sources {
		if source.Version != int64(i+1) {
			t.Fatalf("unexpected migration version: %d", source.Version)
		}
	}
}

func TestProviderRejectsMissingOrDuplicateMigrations(t *testing.T) {
	db, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := newProvider(db, fstest.MapFS{}, testLogger()); !errors.Is(err, goose.ErrNoMigrations) {
		t.Fatalf("expected missing migrations to fail, got %v", err)
	}
	files := fstest.MapFS{
		"001_up.sql":   {Data: []byte("-- +goose Up\nSELECT 1;\n")},
		"001_down.sql": {Data: []byte("-- +goose Up\nSELECT 1;\n")},
	}
	if _, err := newProvider(db, files, testLogger()); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate versions to fail, got %v", err)
	}
}

func TestRunRejectsNilPool(t *testing.T) {
	if err := Run(context.Background(), nil, nil); err == nil {
		t.Fatal("expected a nil pool error")
	}
}
