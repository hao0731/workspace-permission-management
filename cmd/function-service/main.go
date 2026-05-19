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
	"github.com/nats-io/nats.go"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/config"
	"github.com/hao0731/workspace-permission-management/internal/function-service/handlers"
	"github.com/hao0731/workspace-permission-management/internal/function-service/repositories"
	"github.com/hao0731/workspace-permission-management/internal/function-service/services"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
	permissionapi "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
	"github.com/hao0731/workspace-permission-management/internal/shared/pagination"
)

type processIndicator struct{}

func (processIndicator) Name() string {
	return "process"
}

func (processIndicator) IsHealthy(context.Context) bool {
	return true
}

func newPermissionClient(cfg config.PermissionAPIConfig) clientpermission.Client {
	return permissionapi.New(cfg.BaseURL, cfg.APIKey, cfg.APIKeyHeader)
}

func main() {
	if err := run(); err != nil {
		slog.Error("function service stopped with error", "err", err)
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
	repository := repositories.NewMongoResourceRepository(db)
	if ensureIndexErr := repository.EnsureIndexes(ctx); ensureIndexErr != nil {
		return ensureIndexErr
	}
	permissionRepository := repositories.NewMongoPermissionRepository(db)
	if ensureIndexErr := permissionRepository.EnsureIndexes(ctx); ensureIndexErr != nil {
		return ensureIndexErr
	}
	systemResourceRepository := repositories.NewMongoSystemResourceRepository(db)
	if ensureIndexErr := systemResourceRepository.EnsureIndexes(ctx); ensureIndexErr != nil {
		return ensureIndexErr
	}

	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return err
	}
	defer nc.Close()

	producer, err := eventbus.NewJetStreamProducer(ctx, nc, logger)
	if err != nil {
		return err
	}

	resourceService := services.NewResourceService(repository,
		services.WithResourceDeletedPublisher(newResourceDeletedPublisher(producer, cfg.ResourceDeletedSubject)),
	)
	permissionService := services.NewPermissionService(permissionRepository)
	permissionClient := newPermissionClient(cfg.PermissionAPI)
	systemResourceService := services.NewSystemResourceService(systemResourceRepository, resource.ResourceDefinitionLimits{
		Types:   cfg.SystemResourceLimits.Type,
		Actions: cfg.SystemResourceLimits.Action,
		Tags:    cfg.SystemResourceLimits.Tag,
	}, permissionClient)

	eventHandler := handlers.NewResourceEventHandler(resourceService, logger)
	consumer, err := eventbus.NewJetStreamConsumer(ctx, nc, eventbus.Config{
		Stream:    cfg.JetStream.Stream,
		Subjects:  []string{transport.ResourceUpsertEventTypePattern},
		Durable:   cfg.JetStream.Durable,
		BatchSize: cfg.JetStream.FetchCount,
		MaxWait:   cfg.JetStream.MaxWait,
	}, eventHandler, logger)
	if err != nil {
		return err
	}

	e := echo.New()
	health.NewHealthManager(processIndicator{}).RegisterRoutes(e)
	handlers.RegisterRoutes(e, handlers.NewResourceHandler(resourceService, logger, pagination.New()))
	handlers.RegisterPermissionRoutes(e, handlers.NewPermissionHandler(permissionService, logger))
	handlers.RegisterSystemResourceRoutes(e, handlers.NewSystemResourceHandler(systemResourceService, logger))

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
