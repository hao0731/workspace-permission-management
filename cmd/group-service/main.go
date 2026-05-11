package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hao0731/workspace-permission-management/internal/group-service/config"
	"github.com/hao0731/workspace-permission-management/internal/group-service/handlers"
	"github.com/hao0731/workspace-permission-management/internal/group-service/repositories"
	"github.com/hao0731/workspace-permission-management/internal/group-service/services"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
	"github.com/hao0731/workspace-permission-management/internal/shared/pagination"
	"github.com/labstack/echo/v5"
	"github.com/nats-io/nats.go"
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

	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return err
	}
	defer nc.Close()

	groupService := services.NewGroupService(repository,
		services.WithGroupValidationLimits(
			cfg.Validation.MaxIndividualMembers,
			cfg.Validation.MaxGroupingRules,
		),
		services.WithGroupExpiryBucketLocation(cfg.GroupExpiryCommand.BucketLocation),
	)

	eventHandler := handlers.NewGroupExpiryEventHandler(groupService, cfg.GroupExpiryCommand.Subject, logger)
	consumer, err := eventbus.NewJetStreamConsumer(ctx, nc, newGroupExpiryEventbusConfig(cfg), eventHandler, logger)
	if err != nil {
		return err
	}

	e := echo.New()
	registerHealthRoutes(e)
	handlers.RegisterRoutes(e, handlers.NewGroupHandler(groupService, logger, pagination.New()))

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

func newGroupExpiryEventbusConfig(cfg config.Config) eventbus.Config {
	return eventbus.Config{
		Stream:    cfg.GroupExpiryCommand.Stream,
		Subjects:  []string{cfg.GroupExpiryCommand.Subject},
		Durable:   cfg.GroupExpiryCommand.Durable,
		BatchSize: cfg.GroupExpiryCommand.FetchCount,
		MaxWait:   cfg.GroupExpiryCommand.MaxWait,
	}
}

func registerHealthRoutes(e *echo.Echo) {
	health.NewHealthManager(processIndicator{}).RegisterRoutes(e)
}
