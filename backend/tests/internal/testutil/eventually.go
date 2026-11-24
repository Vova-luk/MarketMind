package testutil

import (
	"context"
	"testing"
	"time"
)

func Eventually(t testing.TB, ctx context.Context, description string, check func() bool) {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for !check() {
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s: %v", description, ctx.Err())
		case <-ticker.C:
		}
	}
}
