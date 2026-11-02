package application

import (
	"context"
	"fmt"

	"marketmind/api/internal/config"
	"marketmind/api/internal/server"
	"marketmind/utils/logger"
	"marketmind/utils/migrator"
	"marketmind/utils/postgres"
)

func (a *Application) initConfig(path string) error {
	reader, err := config.Load(path)
	if err != nil {
		return err
	}
	a.config = reader
	return nil
}

func (a *Application) initLogger() error {
	var cfg logger.Config
	if err := a.config.Unmarshal("logger", &cfg); err != nil {
		return fmt.Errorf("read logger config: %w", err)
	}
	log, closeLog, err := logger.New(cfg)
	if err != nil {
		return err
	}
	a.logger = log.With("service", "api")
	a.shutdownFuncs = append(a.shutdownFuncs, closeLog)
	return nil
}

func (a *Application) initDatabase(ctx context.Context) error {
	var cfg postgres.Config
	if err := a.config.Unmarshal("database", &cfg); err != nil {
		return fmt.Errorf("read database config: %w", err)
	}
	database, err := postgres.Connect(ctx, cfg)
	if err != nil {
		a.logger.Error("database initialization failed", "error", err)
		return err
	}
	a.database = database
	a.shutdownFuncs = append(a.shutdownFuncs, func() error {
		a.database.Close()
		return nil
	})
	a.logger.Info("database connected")
	return nil
}

func (a *Application) initMigrator(ctx context.Context) error {
	return migrator.Run(ctx, a.database, a.logger)
}

func (a *Application) initRouter() {
	a.router = NewRouter(a.logger)
}

func (a *Application) initServer() error {
	var cfg server.Config
	if err := a.config.Unmarshal("server", &cfg); err != nil {
		return fmt.Errorf("read server config: %w", err)
	}
	srv, err := server.New(cfg, a.router, a.logger)
	if err != nil {
		return err
	}
	a.server = srv
	a.shutdownFuncs = append(a.shutdownFuncs, srv.Shutdown)
	return nil
}
