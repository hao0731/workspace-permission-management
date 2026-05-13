package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type ResourceUpsertPublisher interface {
	PublishResourceUpsert(ctx context.Context, event resource.ResourceUpsertEvent) error
}

type Option func(*ResourceService)

func WithClock(clock func() time.Time) Option {
	return func(s *ResourceService) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func WithIDGenerator(generator func() string) Option {
	return func(s *ResourceService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *ResourceService) {
		if logger != nil {
			s.logger = logger
		}
	}
}

type ResourceService struct {
	publisher   ResourceUpsertPublisher
	clock       func() time.Time
	idGenerator func() string
	logger      *slog.Logger
}

func NewResourceService(publisher ResourceUpsertPublisher, opts ...Option) *ResourceService {
	service := &ResourceService{
		publisher:   publisher,
		clock:       func() time.Time { return time.Now().UTC() },
		idGenerator: uuid.NewString,
		logger:      slog.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *ResourceService) HandleResourceCreate(ctx context.Context, command resource.ResourceCreateCommand) error {
	command = command.Normalize()
	if err := command.Validate(); err != nil {
		return err
	}
	s.logger.Info("received resource create command",
		"workspace_id", command.WorkspaceID,
		"app_name", command.AppName,
		"resource_name", command.ResourceName,
		"resource_type", command.ResourceType,
	)
	event := resource.ResourceUpsertEvent{
		ResourceID:   s.idGenerator(),
		DisplayName:  command.ResourceName,
		ResourceType: command.ResourceType,
		ResourceTags: []string{},
		FunctionKey:  command.AppName,
		WorkspaceID:  command.WorkspaceID,
		EventID:      s.idGenerator(),
		EventTime:    s.clock(),
	}
	if s.publisher == nil {
		return fmt.Errorf("resource upsert publisher is not configured")
	}
	if err := s.publisher.PublishResourceUpsert(ctx, event); err != nil {
		return fmt.Errorf("publish resource upsert event: %w", err)
	}
	s.logger.Info("published resource upsert event",
		"workspace_id", event.WorkspaceID,
		"app_name", event.FunctionKey,
		"resource_id", event.ResourceID,
		"upsert_subject", event.Subject(),
		"upsert_event_id", event.EventID,
	)
	return nil
}
