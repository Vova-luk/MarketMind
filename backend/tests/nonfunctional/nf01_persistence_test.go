//go:build nonfunctional

package nonfunctional

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
)

func TestNF01_PersistenceAfterRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()

	stack := testutil.OpenTestStack(t, ctx)
	fixture := createAlertFixture(t, ctx, testutil.New(t, ctx))
	assertPersisted := func(t *testing.T) {
		t.Helper()
		var watchlist struct {
			Items []subscription `json:"items"`
		}
		fixture.user.API.RequestJSON(t, ctx, http.MethodGet, subscriptionsPath, nil, http.StatusOK, &watchlist)
		if !slices.Equal(watchlist.Items, []subscription{fixture.subscription}) {
			t.Errorf("watchlist after restart: got %+v, want [%+v]", watchlist.Items, fixture.subscription)
		}
		var response struct {
			Items []alert `json:"items"`
		}
		fixture.user.API.RequestJSON(t, ctx, http.MethodGet, fixture.alertsPath(), nil, http.StatusOK, &response)
		if !slices.Equal(response.Items, []alert{fixture.alert}) {
			t.Errorf("alerts after restart: got %+v, want [%+v]", response.Items, fixture.alert)
		}
	}

	t.Run("applications", func(t *testing.T) {
		stack.Restart(t, ctx, "api", "exchange-data", "tracking", "bot")
		assertPersisted(t)
	})
	if extendedRun() {
		t.Run("postgres", func(t *testing.T) {
			stack.Restart(t, ctx, "postgres")
			assertPersisted(t)
		})
	}
}
