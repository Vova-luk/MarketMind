package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"marketmind/api/internal/application"
	"marketmind/utils/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "services/api/config/config.yaml", "path to YAML configuration")
	envPath := flag.String("env-file", ".env", "path to dotenv file; empty disables loading")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("unexpected positional arguments; use -config and -env-file")
	}
	if err := config.LoadEnv(*envPath); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := application.NewApplication(ctx, *configPath)
	if err != nil {
		return err
	}
	return app.Run(ctx)
}
