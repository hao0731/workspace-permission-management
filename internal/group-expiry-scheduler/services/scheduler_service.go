package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

type TaskRepository interface {
	ListDueGroupTasks(ctx context.Context, dueBucket string, cursor *expiry.Cursor, limit int) ([]expiry.GroupTask, error)
	ListDueIndividualMemberTasks(ctx context.Context, dueBucket string, cursor *expiry.Cursor, limit int) ([]expiry.IndividualMemberTask, error)
}

type CommandPublisher interface {
	PublishGroupExpiryCommand(ctx context.Context, task expiry.GroupTask) error
	PublishIndividualMemberExpiryCommand(ctx context.Context, task expiry.IndividualMemberTask) error
}

type DispatchStats struct {
	RunID                     string
	GroupDueBucket            string
	IndividualMemberDueBucket string
	GroupScanned              int
	GroupPublished            int
	GroupFailed               int
	IndividualMemberScanned   int
	IndividualMemberPublished int
	IndividualMemberFailed    int
	Duration                  time.Duration
}

type SchedulerService struct {
	repository                     TaskRepository
	publisher                      CommandPublisher
	logger                         *slog.Logger
	now                            func() time.Time
	runIDGenerator                 func() string
	groupBucketLocation            *time.Location
	individualMemberBucketLocation *time.Location
	batchSize                      int
}

type Option func(*SchedulerService)

func NewSchedulerService(repository TaskRepository, publisher CommandPublisher, opts ...Option) *SchedulerService {
	service := &SchedulerService{
		repository:                     repository,
		publisher:                      publisher,
		logger:                         slog.Default(),
		now:                            func() time.Time { return time.Now().UTC() },
		runIDGenerator:                 uuid.NewString,
		groupBucketLocation:            time.UTC,
		individualMemberBucketLocation: time.UTC,
		batchSize:                      20,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *SchedulerService) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func WithClock(clock func() time.Time) Option {
	return func(s *SchedulerService) {
		if clock != nil {
			s.now = clock
		}
	}
}

func WithRunIDGenerator(generator func() string) Option {
	return func(s *SchedulerService) {
		if generator != nil {
			s.runIDGenerator = generator
		}
	}
}

func WithBucketLocations(groupLocation *time.Location, individualMemberLocation *time.Location) Option {
	return func(s *SchedulerService) {
		if groupLocation != nil {
			s.groupBucketLocation = groupLocation
		}
		if individualMemberLocation != nil {
			s.individualMemberBucketLocation = individualMemberLocation
		}
	}
}

func WithBatchSize(batchSize int) Option {
	return func(s *SchedulerService) {
		if batchSize > 0 {
			s.batchSize = batchSize
		}
	}
}

func (s *SchedulerService) Run(ctx context.Context) (DispatchStats, error) {
	startedAt := s.now().UTC()
	stats := DispatchStats{
		RunID:                     s.runIDGenerator(),
		GroupDueBucket:            group.ExpirationBucketFor(startedAt, s.groupBucketLocation),
		IndividualMemberDueBucket: group.ExpirationBucketFor(startedAt, s.individualMemberBucketLocation),
	}
	s.logger.Info("group expiry scheduler job started",
		"run_id", stats.RunID,
		"group_due_bucket", stats.GroupDueBucket,
		"individual_member_due_bucket", stats.IndividualMemberDueBucket,
	)

	if err := s.dispatchGroupTasks(ctx, &stats); err != nil {
		return s.failRun(startedAt, stats, err)
	}
	if err := s.dispatchIndividualMemberTasks(ctx, &stats); err != nil {
		return s.failRun(startedAt, stats, err)
	}

	stats.Duration = s.now().UTC().Sub(startedAt)
	s.logger.Info("group expiry scheduler job finished",
		"run_id", stats.RunID,
		"group_scanned", stats.GroupScanned,
		"group_published", stats.GroupPublished,
		"group_failed", stats.GroupFailed,
		"individual_member_scanned", stats.IndividualMemberScanned,
		"individual_member_published", stats.IndividualMemberPublished,
		"individual_member_failed", stats.IndividualMemberFailed,
		"duration", stats.Duration,
	)
	return stats, nil
}

func (s *SchedulerService) failRun(startedAt time.Time, stats DispatchStats, err error) (DispatchStats, error) {
	stats.Duration = s.now().UTC().Sub(startedAt)
	s.logger.Error("group expiry scheduler job failed", "err", err, "run_id", stats.RunID, "duration", stats.Duration)
	return stats, err
}

func (s *SchedulerService) dispatchGroupTasks(ctx context.Context, stats *DispatchStats) error {
	return dispatchTasks(ctx, s.logger, stats, s.batchSize, dispatchConfig[expiry.GroupTask]{
		dueBucket:     stats.GroupDueBucket,
		list:          s.repository.ListDueGroupTasks,
		publish:       s.publisher.PublishGroupExpiryCommand,
		cursor:        groupTaskCursor,
		listErr:       "list due group expiry tasks",
		warnMessage:   "failed to publish group expiry command",
		warnAttrs:     groupTaskWarnAttrs,
		recordScanned: recordGroupScanned,
		recordSuccess: recordGroupPublished,
		recordFailure: recordGroupFailed,
	})
}

func (s *SchedulerService) dispatchIndividualMemberTasks(ctx context.Context, stats *DispatchStats) error {
	return dispatchTasks(ctx, s.logger, stats, s.batchSize, dispatchConfig[expiry.IndividualMemberTask]{
		dueBucket:     stats.IndividualMemberDueBucket,
		list:          s.repository.ListDueIndividualMemberTasks,
		publish:       s.publisher.PublishIndividualMemberExpiryCommand,
		cursor:        individualMemberTaskCursor,
		listErr:       "list due individual member expiry tasks",
		warnMessage:   "failed to publish individual member expiry command",
		warnAttrs:     individualMemberTaskWarnAttrs,
		recordScanned: recordIndividualMemberScanned,
		recordSuccess: recordIndividualMemberPublished,
		recordFailure: recordIndividualMemberFailed,
	})
}

func groupTaskCursor(task expiry.GroupTask) expiry.Cursor {
	return expiry.Cursor{ExpirationBucket: task.ExpirationBucket, ID: task.ID}
}

func individualMemberTaskCursor(task expiry.IndividualMemberTask) expiry.Cursor {
	return expiry.Cursor{ExpirationBucket: task.ExpirationBucket, ID: task.ID}
}

func groupTaskWarnAttrs(task expiry.GroupTask) []any {
	return []any{
		"task_type", "group",
		"task_id", task.ID,
		"workspace_id", task.WorkspaceID,
		"group_id", task.GroupID,
		"expiration_bucket", task.ExpirationBucket,
	}
}

func individualMemberTaskWarnAttrs(task expiry.IndividualMemberTask) []any {
	return []any{
		"task_type", "individual_member",
		"task_id", task.ID,
		"group_id", task.GroupID,
		"nt_account", task.NTAccount,
		"expiration_bucket", task.ExpirationBucket,
	}
}

func recordGroupScanned(stats *DispatchStats) {
	stats.GroupScanned++
}

func recordGroupPublished(stats *DispatchStats) {
	stats.GroupPublished++
}

func recordGroupFailed(stats *DispatchStats) {
	stats.GroupFailed++
}

func recordIndividualMemberScanned(stats *DispatchStats) {
	stats.IndividualMemberScanned++
}

func recordIndividualMemberPublished(stats *DispatchStats) {
	stats.IndividualMemberPublished++
}

func recordIndividualMemberFailed(stats *DispatchStats) {
	stats.IndividualMemberFailed++
}

type dispatchConfig[T any] struct {
	dueBucket     string
	list          func(context.Context, string, *expiry.Cursor, int) ([]T, error)
	publish       func(context.Context, T) error
	cursor        func(T) expiry.Cursor
	listErr       string
	warnMessage   string
	warnAttrs     func(T) []any
	recordScanned func(*DispatchStats)
	recordSuccess func(*DispatchStats)
	recordFailure func(*DispatchStats)
}

func dispatchTasks[T any](ctx context.Context, logger *slog.Logger, stats *DispatchStats, batchSize int, cfg dispatchConfig[T]) error {
	var cursor *expiry.Cursor
	for {
		tasks, err := cfg.list(ctx, cfg.dueBucket, cursor, batchSize)
		if err != nil {
			return fmt.Errorf("%s: %w", cfg.listErr, err)
		}
		if len(tasks) == 0 {
			return nil
		}
		for _, task := range tasks {
			cfg.recordScanned(stats)
			if err := cfg.publish(ctx, task); err != nil {
				cfg.recordFailure(stats)
				taskAttrs := cfg.warnAttrs(task)
				attrs := make([]any, 0, 4+len(taskAttrs))
				attrs = append(attrs, "err", err, "run_id", stats.RunID)
				attrs = append(attrs, taskAttrs...)
				logger.Warn(cfg.warnMessage, attrs...)
				continue
			}
			cfg.recordSuccess(stats)
		}
		nextCursor := cfg.cursor(tasks[len(tasks)-1])
		cursor = &nextCursor
	}
}
