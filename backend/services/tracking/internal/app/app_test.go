package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type componentStub struct {
	run   func(context.Context) error
	close func() error
}

func (s componentStub) Run(ctx context.Context) error {
	return s.run(ctx)
}
func (s componentStub) Close() error {
	return s.close()
}

type poolStub struct{ close func() }

func (p poolStub) Close() { p.close() }

func TestRunStopsPeersAndClosesResources(t *testing.T) {
	for _, scenario := range []string{"cancel", "consumer error", "server error", "unexpected exit"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var stopped atomic.Int32
			wait := func(ctx context.Context) error { defer stopped.Add(1); <-ctx.Done(); return nil }
			want := errors.New("component failed")
			fail := func(context.Context) error { defer stopped.Add(1); return want }
			serverRun, consumerRun := wait, wait
			switch scenario {
			case "consumer error":
				consumerRun = fail
			case "server error":
				serverRun = fail
			case "unexpected exit":
				consumerRun = func(context.Context) error { stopped.Add(1); return nil }
			case "cancel":
				cancel()
			}
			var order []string
			closeResource := func(name string) func() error {
				return func() error {
					if stopped.Load() != 2 {
						t.Error("resources closed while a component was running")
					}
					order = append(order, name)
					return nil
				}
			}
			a := &Application{
				server:   componentStub{run: serverRun},
				consumer: componentStub{run: consumerRun, close: closeResource("consumer")},
				producer: componentStub{close: closeResource("producer")},
				pool:     poolStub{close: func() { order = append(order, "pool") }},
			}
			err := a.Run(ctx)
			if scenario == "cancel" && err != nil {
				t.Fatal(err)
			}
			if (scenario == "consumer error" || scenario == "server error") && !errors.Is(err, want) {
				t.Fatalf("lost error: %v", err)
			}
			if scenario == "unexpected exit" && err == nil {
				t.Fatal("unexpected exit was accepted")
			}
			if len(order) != 3 || order[0] != "consumer" || order[1] != "producer" || order[2] != "pool" {
				t.Fatalf("close order: %v", order)
			}
		})
	}
}
