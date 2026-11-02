package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"marketmind/api/internal/server"
	"marketmind/utils/config"
)

type Application struct {
	config        *config.Reader
	logger        *slog.Logger
	database      *pgxpool.Pool
	router        http.Handler
	server        *server.Server
	shutdownFuncs []func() error
}

func NewApplication(ctx context.Context, configPath string) (_ *Application, err error) {
	a := &Application{}
	defer func() {
		if err != nil {
			err = errors.Join(err, a.shutdown())
		}
	}()
	if err = a.initConfig(configPath); err != nil {
		return nil, err
	}
	if err = a.initLogger(); err != nil {
		return nil, err
	}
	if err = a.initDatabase(ctx); err != nil {
		return nil, err
	}
	if err = a.initMigrator(ctx); err != nil {
		return nil, err
	}
	a.initRouter()
	if err = a.initServer(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Application) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	group, ctx := errgroup.WithContext(ctx)
	a.logger.Info("HTTP server starting", "address", a.server.Addr)
	group.Go(func() error {
		defer cancel()
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	})
	group.Go(func() error {
		<-ctx.Done()
		return a.shutdown()
	})
	return group.Wait()
}

func (a *Application) shutdown() error {
	var shutdownErr error
	for i := len(a.shutdownFuncs) - 1; i >= 0; i-- {
		shutdownErr = errors.Join(shutdownErr, a.shutdownFuncs[i]())
	}
	return shutdownErr
}
