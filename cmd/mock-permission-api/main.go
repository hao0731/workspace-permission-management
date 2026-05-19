package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/labstack/echo/v5"

	"github.com/hao0731/workspace-permission-management/internal/mock-permission-api/config"
	"github.com/hao0731/workspace-permission-management/internal/mock-permission-api/handlers"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
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
		slog.Error("mock permission API stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := sharedlogger.New(cfg.Environment)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	e := echo.New()
	registerHealthRoutes(e)
	handlers.RegisterRoutes(e, handlers.NewSchemaHandler(logger))

	startConfig := echo.StartConfig{
		Address:         cfg.HTTPAddr,
		GracefulTimeout: cfg.ShutdownTimeout,
	}
	if err := startConfig.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func registerHealthRoutes(e *echo.Echo) {
	health.NewHealthManager(processIndicator{}).RegisterRoutes(e)
}
