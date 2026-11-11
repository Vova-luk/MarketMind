package application

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"marketmind/api/internal/adapters/exchangedata"
	"marketmind/api/internal/config"
	alerthandlers "marketmind/api/internal/handlers/alerts"
	exchangehandlers "marketmind/api/internal/handlers/exchanges"
	markethandlers "marketmind/api/internal/handlers/markets"
	subscriptionhandlers "marketmind/api/internal/handlers/subscriptions"
	userhandlers "marketmind/api/internal/handlers/users"
	alertrepository "marketmind/api/internal/repository/alerts"
	exchangerepository "marketmind/api/internal/repository/exchanges"
	subscriptionrepository "marketmind/api/internal/repository/subscriptions"
	userrepository "marketmind/api/internal/repository/users"
	"marketmind/api/internal/server"
	alertservice "marketmind/api/internal/service/alerts"
	marketservice "marketmind/api/internal/service/markets"
	subscriptionservice "marketmind/api/internal/service/subscriptions"
	"marketmind/gen/go/api"
	exchangedatapb "marketmind/gen/go/exchangedata"
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

func (a *Application) initRouter() error {
	address := cmp.Or(a.config.String("exchange_data.address"), "127.0.0.1:9090")
	timeout := cmp.Or(a.config.Duration("exchange_data.request_timeout"), 40*time.Second)
	if timeout < 0 {
		return fmt.Errorf("exchange_data.request_timeout must be positive")
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create Exchange-Data client: %w", err)
	}
	a.shutdownFuncs = append(a.shutdownFuncs, conn.Close)
	catalog := exchangedata.NewCatalog(exchangedatapb.NewMarketSearchServiceClient(conn), timeout)

	botToken := a.config.String("telegram.bot_token")
	users := userhandlers.NewHandler(userrepository.NewRepository(a.database), a.logger)
	exchangeRepo := exchangerepository.NewRepository(a.database)
	exchanges := exchangehandlers.NewHandler(exchangeRepo, a.logger)
	markets := markethandlers.NewHandler(marketservice.NewService(exchangeRepo, catalog), a.logger)
	subscriptionRepo := subscriptionrepository.NewRepository(a.database)
	addSubscriptions := subscriptionservice.NewService(subscriptionRepo, a.config.Int("subscriptions.limit"))
	subscriptions := subscriptionhandlers.NewHandler(subscriptionRepo, addSubscriptions, a.logger)
	alerts := alertservice.NewService(alertrepository.NewRepository(a.database), a.config.Int("alerts.active_limit"))
	a.router = NewRouter(a.logger, botToken, users, exchanges, subscriptions, alerthandlers.NewHandler(alerts, subscriptionRepo, a.logger), markets)
	return nil
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

func (a *Application) initGRPC() error {
	var cfg server.GRPCConfig
	if err := a.config.Unmarshal("grpc", &cfg); err != nil {
		return fmt.Errorf("read gRPC config: %w", err)
	}
	if cfg.Port == 0 {
		return nil
	}
	srv, err := server.NewGRPC(cfg)
	if err != nil {
		return err
	}
	repository := subscriptionrepository.NewRepository(a.database)
	handler := markethandlers.NewCatalogHandler(repository, a.logger)
	api.RegisterMarketCatalogServiceServer(srv, handler)
	a.grpcServer = srv
	a.shutdownFuncs = append(a.shutdownFuncs, srv.Shutdown)
	return nil
}
