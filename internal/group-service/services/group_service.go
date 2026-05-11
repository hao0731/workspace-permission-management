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
	Get(ctx context.Context, query group.GetQuery) (*group.Group, error)
	Delete(ctx context.Context, input group.DeleteInput, deletedAt time.Time) error
	UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput, updatedAt time.Time) error
	ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error)
	AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error)
	UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput, updatedAt time.Time) error
	DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput, deletedAt time.Time) error
	ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireGroupingRuleStatus, error)
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

func WithGroupValidationLimits(maxIndividualMembers int, maxGroupingRules int) GroupOption {
	return func(s *GroupService) {
		s.validateOptions = []group.ValidateOption{
			group.WithMaxIndividualMembers(maxIndividualMembers),
			group.WithMaxGroupingRules(maxGroupingRules),
		}
	}
}

func WithGroupExpiryBucketLocation(location *time.Location) GroupOption {
	return func(s *GroupService) {
		if location != nil {
			s.expiryBucketLocation = location
		}
	}
}

type GroupService struct {
	repository           GroupRepository
	idGenerator          func() string
	now                  func() time.Time
	expiryBucketLocation *time.Location
	validateOptions      []group.ValidateOption
}

func NewGroupService(repository GroupRepository, opts ...GroupOption) *GroupService {
	service := &GroupService{
		repository:           repository,
		idGenerator:          uuid.NewString,
		expiryBucketLocation: time.UTC,
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
	if err := input.Validate(now, s.validateOptions...); err != nil {
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
	model.GroupingRule.ExpiredAt = nil
	model.ExpiryTask = s.newExpiryTask(model.WorkspaceID, model.ID, model.GroupingRule)

	saved, err := s.repository.Create(ctx, model)
	if err != nil {
		if errors.Is(err, group.ErrDuplicateName) {
			return group.Group{}, err
		}
		return group.Group{}, fmt.Errorf("create group: %w", err)
	}
	return saved, nil
}

func (s *GroupService) GetGroup(ctx context.Context, query group.GetQuery) (*group.Group, error) {
	query = query.Normalize()
	if err := query.Validate(); err != nil {
		return nil, err
	}
	model, err := s.repository.Get(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}
	return model, nil
}

func (s *GroupService) DeleteGroup(ctx context.Context, input group.DeleteInput) error {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, input, s.now().UTC()); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	return nil
}

func (s *GroupService) UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput) error {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(now, s.validateOptions...); err != nil {
		return err
	}
	input.ExpiryTask = s.newExpiryTask(input.WorkspaceID, input.GroupID, group.GroupingRule{
		Rules:          input.Rules,
		ExpirationDate: input.ExpirationDate,
	})
	if err := s.repository.UpdateGroupingRule(ctx, input, now); err != nil {
		if errors.Is(err, group.ErrNotFound) || errors.Is(err, group.ErrInvalidInput) {
			return err
		}
		return fmt.Errorf("update grouping rule: %w", err)
	}
	return nil
}

func (s *GroupService) ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand) (group.ExpireGroupingRuleStatus, error) {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return "", err
	}
	status, err := s.repository.ExpireGroupingRule(ctx, input, s.now().UTC(), s.expiryBucketLocation)
	if err != nil {
		return "", fmt.Errorf("expire grouping rule: %w", err)
	}
	return status, nil
}

func (s *GroupService) ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error) {
	query = query.Normalize()
	if err := query.Validate(); err != nil {
		return group.IndividualMemberPage{}, err
	}
	page, err := s.repository.ListIndividualMembers(ctx, query)
	if err != nil {
		return group.IndividualMemberPage{}, fmt.Errorf("list group individual members: %w", err)
	}
	return page, nil
}

func (s *GroupService) AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error) {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(now, s.validateOptions...); err != nil {
		return nil, err
	}
	input.IndividualMembers = s.newIndividualMembers(input.GroupID, input.IndividualMembers, now)
	members, err := s.repository.AddIndividualMembers(ctx, input)
	if err != nil {
		if errors.Is(err, group.ErrDuplicateMember) || errors.Is(err, group.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("add group individual members: %w", err)
	}
	return members, nil
}

func (s *GroupService) UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput) error {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(now); err != nil {
		return err
	}
	if err := s.repository.UpdateIndividualMemberExpiration(ctx, input, now); err != nil {
		if errors.Is(err, group.ErrNotFound) {
			return err
		}
		return fmt.Errorf("update group individual member expiration: %w", err)
	}
	return nil
}

func (s *GroupService) DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput) error {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}
	if err := s.repository.DeleteIndividualMember(ctx, input, s.now().UTC()); err != nil {
		return fmt.Errorf("delete group individual member: %w", err)
	}
	return nil
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

func (s *GroupService) newExpiryTask(workspaceID string, groupID string, groupingRule group.GroupingRule) *group.ExpiryTask {
	if len(groupingRule.Rules) == 0 {
		return nil
	}
	return &group.ExpiryTask{
		ID:               s.idGenerator(),
		WorkspaceID:      workspaceID,
		GroupID:          groupID,
		ExpirationBucket: group.ExpirationBucketFor(groupingRule.ExpirationDate, s.expiryBucketLocation),
	}
}
