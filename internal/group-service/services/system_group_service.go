package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	permission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)

var ErrSystemGroupPermissionWriteFailed = errors.New("system group permission write failed")

type SystemGroupRepository interface {
	CreateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error)
	GetSystemGroupWithRelationships(ctx context.Context, systemID string, groupID string) (group.SystemGroup, group.SystemGroupRelationshipProjection, error)
	UpdateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error)
	ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error)
}

type SystemGroupPermissionClient interface {
	WriteRelationships(ctx context.Context, parameter permission.WriteRelationshipsParameter) (permission.WriteRelationshipsResult, error)
}

type SystemGroupOption func(*SystemGroupService)

type SystemGroupService struct {
	repository       SystemGroupRepository
	permissionClient SystemGroupPermissionClient
	idGenerator      func() string
	now              func() time.Time
	logger           *slog.Logger
}

func NewSystemGroupService(repository SystemGroupRepository, opts ...SystemGroupOption) *SystemGroupService {
	service := &SystemGroupService{
		repository:  repository,
		idGenerator: uuid.NewString,
		now: func() time.Time {
			return time.Now().UTC()
		},
		logger: slog.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func WithSystemGroupPermissionClient(permissionClient SystemGroupPermissionClient) SystemGroupOption {
	return func(s *SystemGroupService) {
		s.permissionClient = permissionClient
	}
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

func WithSystemGroupLogger(logger *slog.Logger) SystemGroupOption {
	return func(s *SystemGroupService) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func (s *SystemGroupService) CreateSystemGroup(ctx context.Context, input group.SystemGroupCreateInput) (group.SystemGroup, []string, error) {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return group.SystemGroup{}, nil, err
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
		return group.SystemGroup{}, nil, err
	}
	model, projection, permissionErrors, err := s.writeSystemGroupRelationships(ctx, model, projection)
	if err != nil {
		return group.SystemGroup{}, nil, err
	}
	saved, err := s.repository.CreateSystemGroup(ctx, model, projection)
	if err != nil {
		return group.SystemGroup{}, nil, fmt.Errorf("create system group: %w", err)
	}
	return saved, permissionErrors, nil
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

func (s *SystemGroupService) UpdateSystemGroup(ctx context.Context, input group.SystemGroupUpdateInput) (group.SystemGroup, []string, error) {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return group.SystemGroup{}, nil, err
	}

	current, currentProjection, err := s.repository.GetSystemGroupWithRelationships(ctx, input.SystemID, input.GroupID)
	if err != nil {
		return group.SystemGroup{}, nil, fmt.Errorf("get system group for update: %w", err)
	}

	model := group.SystemGroup{
		ID:            current.ID,
		SystemID:      current.SystemID,
		Name:          input.Name,
		GroupingRules: input.GroupingRules,
		CreatedAt:     current.CreatedAt,
		UpdatedAt:     now,
	}
	desiredProjection, err := buildSystemGroupRelationshipProjection(model.SystemID, model.ID, model.GroupingRules, now)
	if err != nil {
		return group.SystemGroup{}, nil, err
	}
	desiredProjection.CreatedAt = currentProjection.CreatedAt
	desiredProjection.UpdatedAt = now

	model, desiredProjection, permissionErrors, err := s.writeSystemGroupRelationshipUpdates(ctx, model, currentProjection, desiredProjection)
	if err != nil {
		return group.SystemGroup{}, nil, err
	}

	saved, err := s.repository.UpdateSystemGroup(ctx, model, desiredProjection)
	if err != nil {
		return group.SystemGroup{}, nil, fmt.Errorf("update system group: %w", err)
	}
	return saved, permissionErrors, nil
}

func (s *SystemGroupService) writeSystemGroupRelationships(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, group.SystemGroupRelationshipProjection, []string, error) {
	if s.permissionClient == nil {
		err := errors.New("permission client is not configured")
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, fmt.Errorf("%w: %w", ErrSystemGroupPermissionWriteFailed, err)
	}
	tasks, err := newSystemGroupRelationshipCreateTasks(projection)
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, err
	}
	result, err := s.permissionClient.WriteRelationships(ctx, permission.WriteRelationshipsParameter{Tasks: tasks})
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, fmt.Errorf("%w: %w", ErrSystemGroupPermissionWriteFailed, err)
	}
	if len(result.FailedTasks) == 0 {
		return model, projection, nil, nil
	}
	filteredProjection, permissionErrors, err := filterFailedSystemGroupRelationships(projection, result.FailedTasks)
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, err
	}
	adjustedModel, err := rebuildSystemGroupFromRelationshipProjection(model, filteredProjection)
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, err
	}
	s.logger.WarnContext(ctx, "permission API relationship write partially failed",
		"system_id", model.SystemID,
		"group_id", model.ID,
		"failed_task_count", len(result.FailedTasks),
		"errors", permissionErrors,
	)
	return adjustedModel, filteredProjection, permissionErrors, nil
}

func (s *SystemGroupService) writeSystemGroupRelationshipUpdates(ctx context.Context, model group.SystemGroup, currentProjection group.SystemGroupRelationshipProjection, desiredProjection group.SystemGroupRelationshipProjection) (group.SystemGroup, group.SystemGroupRelationshipProjection, []string, error) {
	tasks, err := newSystemGroupRelationshipUpdateTasks(currentProjection, desiredProjection)
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, err
	}
	if len(tasks) == 0 {
		return model, desiredProjection, nil, nil
	}
	if s.permissionClient == nil {
		configErr := errors.New("permission client is not configured")
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, fmt.Errorf("%w: %w", ErrSystemGroupPermissionWriteFailed, configErr)
	}
	result, err := s.permissionClient.WriteRelationships(ctx, permission.WriteRelationshipsParameter{Tasks: tasks})
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, fmt.Errorf("%w: %w", ErrSystemGroupPermissionWriteFailed, err)
	}
	if len(result.FailedTasks) == 0 {
		return model, desiredProjection, nil, nil
	}

	finalProjection, permissionErrors, err := applyFailedSystemGroupRelationshipUpdateTasks(currentProjection, desiredProjection, result.FailedTasks)
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, err
	}
	adjustedModel, err := rebuildSystemGroupFromRelationshipProjection(model, finalProjection)
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, err
	}
	adjustedModel.Name = model.Name
	adjustedModel.CreatedAt = model.CreatedAt
	adjustedModel.UpdatedAt = model.UpdatedAt

	s.logger.WarnContext(ctx, "permission API relationship update partially failed",
		"system_id", model.SystemID,
		"group_id", model.ID,
		"failed_task_count", len(result.FailedTasks),
		"errors", permissionErrors,
	)
	return adjustedModel, finalProjection, permissionErrors, nil
}

func newSystemGroupRelationshipCreateTasks(projection group.SystemGroupRelationshipProjection) ([]permission.RelationshipTask, error) {
	tasks := make([]permission.RelationshipTask, 0, len(projection.Relationships))
	for _, info := range projection.Relationships {
		relationship, err := permissionRelationshipValue(info.Relationship)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, permission.RelationshipTask{
			Operator:     permission.RelationshipOperationCreate,
			Relationship: relationship,
		})
	}
	return tasks, nil
}

func filterFailedSystemGroupRelationships(projection group.SystemGroupRelationshipProjection, failedTasks []permission.FailedRelationshipTask) (group.SystemGroupRelationshipProjection, []string, error) {
	failedChecksums := make(map[string]struct{}, len(failedTasks))
	permissionErrors := make([]string, 0, len(failedTasks))
	for _, task := range failedTasks {
		checksum, err := relationshipChecksum(task.Relationship)
		if err != nil {
			return group.SystemGroupRelationshipProjection{}, nil, err
		}
		failedChecksums[checksum] = struct{}{}
		permissionErrors = append(permissionErrors, task.Error)
	}

	filtered := projection
	filtered.Relationships = make([]group.RelationshipInfo, 0, len(projection.Relationships))
	for _, info := range projection.Relationships {
		if _, failed := failedChecksums[info.Checksum]; failed {
			continue
		}
		filtered.Relationships = append(filtered.Relationships, info)
	}
	return filtered, permissionErrors, nil
}
