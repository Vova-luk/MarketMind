package testutil

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type TestStack struct {
	options []string
}

// OpenTestStack checks the Compose project before any test can stop or restart services.
func OpenTestStack(t testing.TB, ctx context.Context) TestStack {
	t.Helper()
	composeFile := os.Getenv("TEST_COMPOSE_FILE")
	if composeFile == "" {
		composeFile = filepath.Join("..", "..", "deploy", "compose.test.yaml")
	}
	envFile := filepath.Join("..", "..", "deploy", "test.env")
	stack := TestStack{options: []string{"compose", "--env-file", envFile, "-f", composeFile}}
	var config struct {
		Name string `json:"name"`
	}
	command := exec.CommandContext(ctx, "docker", append(stack.options, "config", "--format", "json")...)
	output, err := command.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("resolve test Compose configuration: %v\n%s", err, exit.Stderr)
		}
		t.Fatalf("resolve test Compose configuration: %v", err)
	}
	if err := json.Unmarshal(output, &config); err != nil || config.Name != "marketmind-test" {
		t.Fatalf("expected Compose project marketmind-test, got %q (decode error: %v)", config.Name, err)
	}
	return stack
}

func (s TestStack) Restart(t testing.TB, ctx context.Context, services ...string) {
	t.Helper()
	s.run(t, ctx, append([]string{"restart"}, services...)...)
	s.StartAndWait(t, ctx, services...)
}

func (s TestStack) StopImmediately(t testing.TB, ctx context.Context, service string) {
	t.Helper()
	s.run(t, ctx, "stop", "--timeout", "0", service)
}

func (s TestStack) StartAndWait(t testing.TB, ctx context.Context, services ...string) {
	t.Helper()
	s.run(t, ctx, append([]string{"up", "--no-deps", "--no-build", "--wait", "--wait-timeout", "45"}, services...)...)
}

func (s TestStack) StartedAt(t testing.TB, ctx context.Context, service string) string {
	t.Helper()
	ids := strings.Fields(s.run(t, ctx, "ps", "--quiet", service))
	if len(ids) != 1 {
		t.Fatalf("expected one running test container for %s, got %d", service, len(ids))
	}
	command := exec.CommandContext(ctx, "docker", "inspect", "--type", "container", "--format", "{{.State.StartedAt}}", ids[0])
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect test container for %s: %v\n%s", service, err, output)
	}
	return strings.TrimSpace(string(output))
}

func (s TestStack) run(t testing.TB, ctx context.Context, args ...string) string {
	t.Helper()
	command := exec.CommandContext(ctx, "docker", append(s.options, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose %v: %v\n%s", args, err, output)
	}
	return string(output)
}
