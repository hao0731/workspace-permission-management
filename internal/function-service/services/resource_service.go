package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type ResourceRepository interface {
	Upsert(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error)
	List(ctx context.Context, query resource.ListQuery) (resource.Page, error)
	Delete(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error)
}

type ResourceDeletedPublisher interface {
	PublishResourceDeleted(ctx context.Context, event resource.DeletedEvent) error
}

type Option func(*ResourceService)

func WithResourceDeletedPublisher(publisher ResourceDeletedPublisher) Option {
	return func(s *ResourceService) {
		s.deletedPublisher = publisher
	}
}

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

type ResourceService struct {
	repository       ResourceRepository
	deletedPublisher ResourceDeletedPublisher
	clock            func() time.Time
	idGenerator      func() string
}

func NewResourceService(repository ResourceRepository, opts ...Option) *ResourceService {
	service := &ResourceService{
		repository:  repository,
		clock:       func() time.Time { return time.Now().UTC() },
		idGenerator: uuid.NewString,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *ResourceService) UpsertResource(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	if err := input.Validate(); err != nil {
		return "", err
	}
	status, err := s.repository.Upsert(ctx, input)
	if err != nil {
		return "", fmt.Errorf("upsert resource: %w", err)
	}
	return status, nil
}

func (s *ResourceService) ListResources(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	if err := query.Validate(); err != nil {
		return resource.Page{}, err
	}
	page, err := s.repository.List(ctx, query)
	if err != nil {
		return resource.Page{}, fmt.Errorf("list resources: %w", err)
	}
	return page, nil
}

func (s *ResourceService) DeleteResource(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error) {
	if err := input.Validate(); err != nil {
		return "", err
	}
	status, err := s.repository.Delete(ctx, input)
	if err != nil {
		return "", fmt.Errorf("delete resource: %w", err)
	}
	if status == resource.DeleteStatusNotFound {
		return status, nil
	}
	if s.deletedPublisher == nil {
		return "", fmt.Errorf("publish resource deleted event: publisher is not configured")
	}
	if err := s.deletedPublisher.PublishResourceDeleted(ctx, resource.DeletedEvent{
		WorkspaceID: input.WorkspaceID,
		FunctionKey: input.FunctionKey,
		ResourceID:  input.ResourceID,
		EventID:     s.idGenerator(),
		EventTime:   s.clock(),
	}); err != nil {
		return "", fmt.Errorf("publish resource deleted event: %w", err)
	}
	return status, nil
}
