package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	mockfunctionconfig "github.com/hao0731/workspace-permission-management/internal/mock-function/config"
	"github.com/hao0731/workspace-permission-management/internal/mock-function/handlers"
	"github.com/hao0731/workspace-permission-management/internal/mock-function/services"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
	"github.com/labstack/echo/v5"
	"github.com/nats-io/nats.go"
)

type processIndicator struct{}

func (processIndicator) Name() string {
	return "process"
}

func (processIndicator) IsHealthy(context.Context) bool {
	return true
}

func main() {
	if err := run(); err != nil {
		slog.Error("mock function stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := mockfunctionconfig.Load()
	if err != nil {
		return err
	}
	logger := sharedlogger.New(cfg.Environment)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return err
	}
	defer nc.Close()

	producer, err := eventbus.NewJetStreamProducer(ctx, nc, logger)
	if err != nil {
		return err
	}
	upsertPublisher := newResourceUpsertPublisher(producer, eventbus.WithPublishTimeout(cfg.ResourceUpsertPublishTimeout))
	resourceService := services.NewResourceService(upsertPublisher, services.WithLogger(logger))
	eventHandler := handlers.NewResourceCreateEventHandler(resourceService, cfg.ResourceCreateSubjectAppNames(), logger)
	consumer, err := eventbus.NewJetStreamConsumer(ctx, nc, newResourceCreateEventbusConfig(cfg), eventHandler, logger)
	if err != nil {
		return err
	}

	e := echo.New()
	registerHealthRoutes(e)

	errCh := make(chan error, 2)
	go func() {
		startConfig := echo.StartConfig{
			Address:         cfg.HTTPAddr,
			GracefulTimeout: cfg.ShutdownTimeout,
		}
		if err := startConfig.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	go func() {
		errCh <- consumer.Run(ctx)
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			stop()
			return err
		}
	}
	return nil
}

func newResourceCreateEventbusConfig(cfg mockfunctionconfig.Config) eventbus.Config {
	return eventbus.Config{
		Stream:    cfg.ResourceCreate.Stream,
		Subjects:  []string{cfg.ResourceCreateConsumerSubject()},
		Durable:   cfg.ResourceCreate.Durable,
		BatchSize: cfg.ResourceCreate.FetchCount,
		MaxWait:   cfg.ResourceCreate.MaxWait,
	}
}

func registerHealthRoutes(e *echo.Echo) {
	health.NewHealthManager(processIndicator{}).RegisterRoutes(e)
}
