package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type GroupRepository interface {
	Create(ctx context.Context, input group.Group) (group.Group, error)
}

type GroupOption func(*GroupService)

func WithGroupIDGenerator(generator func() string) GroupOption {
	return func(s *GroupService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}

func WithGroupClock(clock func() time.Time) GroupOption {
	return func(s *GroupService) {
		if clock != nil {
			s.now = clock
		}
	}
}

type GroupService struct {
	repository  GroupRepository
	idGenerator func() string
	now         func() time.Time
}

func NewGroupService(repository GroupRepository, opts ...GroupOption) *GroupService {
	service := &GroupService{
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

func (s *GroupService) CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error) {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(now); err != nil {
		return group.Group{}, err
	}

	model := group.Group{
		ID:             s.idGenerator(),
		WorkspaceID:    input.WorkspaceID,
		Name:           input.Name,
		NormalizedName: input.Name,
		Description:    input.Description,
		GroupingRule:   input.GroupingRule,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	model.IndividualMembers = s.newIndividualMembers(model.ID, input.IndividualMembers, now)

	saved, err := s.repository.Create(ctx, model)
	if err != nil {
		if errors.Is(err, group.ErrDuplicateName) {
			return group.Group{}, err
		}
		return group.Group{}, fmt.Errorf("create group: %w", err)
	}
	return saved, nil
}

func (s *GroupService) newIndividualMembers(groupID string, input []group.IndividualMember, now time.Time) []group.IndividualMember {
	members := make([]group.IndividualMember, 0, len(input))
	for _, member := range input {
		members = append(members, group.IndividualMember{
			ID:             s.idGenerator(),
			GroupID:        groupID,
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	return members
}
