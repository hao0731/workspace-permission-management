package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	clienthrpoc "github.com/hao0731/workspace-permission-management/internal/shared/interactions/hr/poc"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
	workspaceconfig "github.com/hao0731/workspace-permission-management/internal/workspace-service/config"
	"github.com/hao0731/workspace-permission-management/internal/workspace-service/handlers"
	"github.com/hao0731/workspace-permission-management/internal/workspace-service/repositories"
	"github.com/hao0731/workspace-permission-management/internal/workspace-service/services"
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
		slog.Error("workspace service stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := workspaceconfig.Load()
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
	repository := repositories.NewMongoWorkspaceRepository(db)
	if err := repository.EnsureIndexes(ctx); err != nil {
		return err
	}

	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return err
	}
	defer nc.Close()

	eventProducer, err := eventbus.NewJetStreamProducer(ctx, nc, logger)
	if err != nil {
		return err
	}
	commandPublisher := newResourceCreatePublisher(eventProducer, eventbus.WithPublishTimeout(cfg.CommandPublishTimeout))
	workspaceService := services.NewWorkspaceService(
		repository,
		clienthrpoc.New(cfg.HR.BaseURL),
		commandPublisher,
		services.WithResourceMappings(newServiceResourceMappings(cfg)),
		services.WithLogger(logger),
	)

	e := echo.New()
	registerHealthRoutes(e)
	handlers.RegisterRoutes(e, handlers.NewWorkspaceHandler(workspaceService, logger))

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

func newServiceResourceMappings(cfg workspaceconfig.Config) services.ResourceMappings {
	return services.ResourceMappings{
		Documents: services.ResourceMapping{
			AppName:      cfg.ResourceMappings.Documents.AppName,
			ResourceType: cfg.ResourceMappings.Documents.ResourceType,
		},
		Tasks: services.ResourceMapping{
			AppName:      cfg.ResourceMappings.Tasks.AppName,
			ResourceType: cfg.ResourceMappings.Tasks.ResourceType,
		},
		Drive: services.ResourceMapping{
			AppName:      cfg.ResourceMappings.Drive.AppName,
			ResourceType: cfg.ResourceMappings.Drive.ResourceType,
		},
	}
}
