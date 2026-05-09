package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/group-service/config"
	"github.com/hao0731/workspace-permission-management/internal/group-service/handlers"
	"github.com/hao0731/workspace-permission-management/internal/group-service/repositories"
	"github.com/hao0731/workspace-permission-management/internal/group-service/services"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
	"github.com/labstack/echo/v5"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
		slog.Error("group service stopped with error", "err", err)
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

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoDB.URI))
	if err != nil {
		return err
	}
	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if disconnectErr := mongoClient.Disconnect(disconnectCtx); disconnectErr != nil {
			logger.Warn("failed to disconnect mongodb", "err", disconnectErr)
		}
	}()

	db := mongoClient.Database(cfg.MongoDB.Database)
	repository := repositories.NewMongoGroupRepository(mongoClient, db)
	if ensureIndexErr := repository.EnsureIndexes(ctx); ensureIndexErr != nil {
		return ensureIndexErr
	}

	groupService := services.NewGroupService(repository, services.WithGroupValidationLimits(group.ValidationLimits{
		MaxIndividualMembers: cfg.Validation.MaxIndividualMembers,
		MaxGroupingRules:     cfg.Validation.MaxGroupingRules,
	}))

	e := echo.New()
	registerHealthRoutes(e)
	handlers.RegisterRoutes(e, handlers.NewGroupHandler(groupService, logger))

	errCh := make(chan error, 1)
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

func registerHealthRoutes(e *echo.Echo) {
	health.NewHealthManager(processIndicator{}).RegisterRoutes(e)
}
