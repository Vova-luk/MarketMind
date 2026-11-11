package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"marketmind/bot/internal/app"
	"marketmind/bot/internal/config"
)

const (
	serviceName       = "bot"
	defaultConfigPath = "bot/config/config.yaml"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("service", serviceName)
	if err := run(logger); err != nil {
		logger.Error("bot stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	configPath := flag.String("config", defaultConfigPath, "path to YAML configuration")
	envPath := flag.String("env-file", "", "optional path to dotenv file")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("unexpected positional arguments; use -config and -env-file")
	}
	var envPaths []string
	if *envPath != "" {
		envPaths = append(envPaths, *envPath)
	}
	cfg, err := config.Load(*configPath, envPaths...)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	application, err := app.New(*cfg, logger)
	if err != nil {
		return err
	}
	return application.Run(ctx)
}
