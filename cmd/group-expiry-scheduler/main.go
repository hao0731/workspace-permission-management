package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-co-op/gocron/v2"
	schedulerconfig "github.com/hao0731/workspace-permission-management/internal/group-expiry-scheduler/config"
	schedulerservices "github.com/hao0731/workspace-permission-management/internal/group-expiry-scheduler/services"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
	sharedexpiry "github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
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
		slog.Error("group expiry scheduler stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := schedulerconfig.Load()
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

	repository := sharedexpiry.NewMongoRepository(mongoClient.Database(cfg.MongoDB.Database))
	if ensureIndexErr := repository.EnsureIndexes(ctx); ensureIndexErr != nil {
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
	publisher := newExpiryCommandPublisher(
		producer,
		cfg.GroupExpiry.Subject,
		cfg.IndividualMemberExpiry.Subject,
		withPublisherPublishOptions(eventbus.WithPublishTimeout(cfg.PublishTimeout)),
	)
	service := schedulerservices.NewSchedulerService(
		repository,
		publisher,
		schedulerservices.WithLogger(logger),
		schedulerservices.WithBatchSize(cfg.BatchSize),
		schedulerservices.WithBucketLocations(cfg.GroupExpiry.BucketLocation, cfg.IndividualMemberExpiry.BucketLocation),
	)
	runner := schedulerservices.NewJobRunner(func(jobCtx context.Context) {
		_, _ = service.Run(jobCtx)
	}, logger)

	scheduler, err := newGocronScheduler(cfg, runner.Run)
	if err != nil {
		return err
	}
	scheduler.Start()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if shutdownErr := scheduler.ShutdownWithContext(shutdownCtx); shutdownErr != nil {
			logger.Warn("failed to shutdown scheduler", "err", shutdownErr)
		}
	}()

	e := echo.New()
	registerHealthRoutes(e)

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
		return nil
	case err := <-errCh:
		if err != nil {
			stop()
			return err
		}
		return nil
	}
}

func newGocronScheduler(cfg schedulerconfig.Config, run func(context.Context)) (gocron.Scheduler, error) {
	scheduler, err := gocron.NewScheduler(
		gocron.WithLocation(cfg.Schedule.Location),
		gocron.WithStopTimeout(cfg.ShutdownTimeout),
	)
	if err != nil {
		return nil, err
	}
	if _, err := scheduler.NewJob(
		gocron.CronJob(cfg.Schedule.Expression, cfg.Schedule.WithSeconds),
		gocron.NewTask(func(ctx context.Context) {
			run(ctx)
		}),
	); err != nil {
		_ = scheduler.Shutdown()
		return nil, err
	}
	return scheduler, nil
}

func registerHealthRoutes(e *echo.Echo) {
	health.NewHealthManager(processIndicator{}).RegisterRoutes(e)
}
