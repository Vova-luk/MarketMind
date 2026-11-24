//go:build nonfunctional

package nonfunctional

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
)

func TestNF03_RESTLatency(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	fixture := createAlertFixture(t, ctx, testutil.New(t, ctx))
	paths := []string{subscriptionsPath, fixture.alertsPath()}
	for _, path := range paths {
		fixture.user.API.RequestJSON(t, ctx, http.MethodGet, path, nil, http.StatusOK, nil)
	}

	workers, requests := 4, 120
	if extendedRun() {
		workers, requests = 12, 720
	}
	type result struct {
		latency time.Duration
		err     error
	}
	results := make(chan result, requests)
	var wg sync.WaitGroup
	started := time.Now()
	for worker := range workers {
		wg.Go(func() {
			for n := worker; n < requests; n += workers {
				begin := time.Now()
				err := fixture.user.API.DoJSON(ctx, http.MethodGet, paths[n%len(paths)], nil, http.StatusOK, nil)
				results <- result{latency: time.Since(begin), err: err}
			}
		})
	}
	wg.Wait()
	elapsed := time.Since(started)
	close(results)

	samples := make([]time.Duration, 0, requests)
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		samples = append(samples, result.latency)
	}
	t.Logf("REST workload: %d concurrent clients, %d requests, 50%% watchlist and 50%% alert list, %.1f requests/s",
		workers, requests, float64(requests)/elapsed.Seconds())
	assertLatency(t, "REST response latency", samples, 300*time.Millisecond)
}
