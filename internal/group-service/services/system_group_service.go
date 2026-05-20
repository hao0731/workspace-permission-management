package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type SystemGroupRepository interface {
	CreateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error)
	ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error)
}

type SystemGroupOption func(*SystemGroupService)

type SystemGroupService struct {
	repository  SystemGroupRepository
	idGenerator func() string
	now         func() time.Time
}

func NewSystemGroupService(repository SystemGroupRepository, opts ...SystemGroupOption) *SystemGroupService {
	service := &SystemGroupService{
		repository:  repository,
		idGenerator: uuid.NewString,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func WithSystemGroupIDGenerator(generator func() string) SystemGroupOption {
	return func(s *SystemGroupService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}

func WithSystemGroupClock(clock func() time.Time) SystemGroupOption {
	return func(s *SystemGroupService) {
		if clock != nil {
			s.now = clock
		}
	}
}

func (s *SystemGroupService) CreateSystemGroup(ctx context.Context, input group.SystemGroupCreateInput) (group.SystemGroup, error) {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return group.SystemGroup{}, err
	}
	model := group.SystemGroup{
		ID:            s.idGenerator(),
		SystemID:      input.SystemID,
		Name:          input.Name,
		GroupingRules: input.GroupingRules,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	projection, err := buildSystemGroupRelationshipProjection(model.SystemID, model.ID, model.GroupingRules, now)
	if err != nil {
		return group.SystemGroup{}, err
	}
	saved, err := s.repository.CreateSystemGroup(ctx, model, projection)
	if err != nil {
		return group.SystemGroup{}, fmt.Errorf("create system group: %w", err)
	}
	return saved, nil
}

func (s *SystemGroupService) ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error) {
	query = query.Normalize()
	if err := query.Validate(); err != nil {
		return group.SystemGroupPage{}, err
	}
	page, err := s.repository.ListSystemGroups(ctx, query)
	if err != nil {
		return group.SystemGroupPage{}, fmt.Errorf("list system groups: %w", err)
	}
	return page, nil
}
