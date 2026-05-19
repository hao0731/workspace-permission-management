package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)

var ErrPermissionRegistrationFailed = errors.New("permission registration failed")

type SystemResourceRepository interface {
	RunInTransaction(ctx context.Context, fn func(context.Context) error) error
	ListResourceDefinitions(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error)
	UpsertResourceDefinitions(ctx context.Context, definitions []resource.ResourceDefinition) ([]resource.ResourceDefinition, error)
	UpsertResourceAttributes(ctx context.Context, attributes resource.ResourceAttributes) (resource.ResourceAttributes, error)
	GetResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) (resource.ResourceAttributes, bool, error)
}

type SystemResourceOption func(*SystemResourceService)

func WithSystemResourceClock(clock func() time.Time) SystemResourceOption {
	return func(s *SystemResourceService) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func WithSystemResourceIDGenerator(generator func() string) SystemResourceOption {
	return func(s *SystemResourceService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}

func WithSystemResourceLogger(logger *slog.Logger) SystemResourceOption {
	return func(s *SystemResourceService) {
		if logger != nil {
			s.logger = logger
		}
	}
}

type SystemResourceService struct {
	repository       SystemResourceRepository
	limits           resource.ResourceDefinitionLimits
	permissionClient clientpermission.Client
	clock            func() time.Time
	idGenerator      func() string
	logger           *slog.Logger
}

func NewSystemResourceService(repository SystemResourceRepository, limits resource.ResourceDefinitionLimits, permissionClient clientpermission.Client, opts ...SystemResourceOption) *SystemResourceService {
	service := &SystemResourceService{
		repository:       repository,
		limits:           limits,
		permissionClient: permissionClient,
		clock:            func() time.Time { return time.Now().UTC() },
		idGenerator:      uuid.NewString,
		logger:           slog.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *SystemResourceService) SaveSystemResources(ctx context.Context, input resource.ResourceDefinitionSaveInput) ([]resource.ResourceDefinition, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if err := s.limits.Validate(); err != nil {
		return nil, err
	}

	normalized := input.Normalize()
	now := s.clock()
	definitions := make([]resource.ResourceDefinition, 0, len(normalized.Resources))
	for _, item := range normalized.Resources {
		definitions = append(definitions, resource.ResourceDefinition{
			ID:          s.idGenerator(),
			SystemID:    normalized.SystemID,
			Type:        item.Type,
			Label:       item.Label,
			Key:         item.Key,
			Description: item.Description,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	attributeID := s.idGenerator()

	var saved []resource.ResourceDefinition
	var derivedAttributes []resource.ResourceAttribute
	if err := s.repository.RunInTransaction(ctx, func(tx context.Context) error {
		existing, err := s.repository.ListResourceDefinitions(tx, resource.ResourceDefinitionsQuery{SystemID: normalized.SystemID})
		if err != nil {
			return fmt.Errorf("list existing system resources: %w", err)
		}
		merged := mergeResourceDefinitions(existing, definitions)
		if countErr := resource.ValidateResourceDefinitionCounts(merged, s.limits); countErr != nil {
			return countErr
		}
		saved, err = s.repository.UpsertResourceDefinitions(tx, definitions)
		if err != nil {
			return fmt.Errorf("upsert system resources: %w", err)
		}
		latest, err := s.repository.ListResourceDefinitions(tx, resource.ResourceDefinitionsQuery{SystemID: normalized.SystemID})
		if err != nil {
			return fmt.Errorf("list latest system resources: %w", err)
		}
		attributes := deriveResourceAttributes(latest)
		if len(attributes) == 0 {
			return nil
		}
		derivedAttributes = append([]resource.ResourceAttribute(nil), attributes...)
		if _, err := s.repository.UpsertResourceAttributes(tx, resource.ResourceAttributes{
			ID:        attributeID,
			SystemID:  normalized.SystemID,
			Values:    attributes,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("upsert system resource attributes: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if len(derivedAttributes) > 0 {
		if err := s.registerResourceAttributes(ctx, normalized.SystemID, derivedAttributes); err != nil {
			return nil, err
		}
	}
	return saved, nil
}

func (s *SystemResourceService) registerResourceAttributes(ctx context.Context, systemID string, attributes []resource.ResourceAttribute) error {
	if s.permissionClient == nil {
		err := errors.New("permission client is not configured")
		s.logger.ErrorContext(ctx, "failed to register resource attributes",
			"err", err,
			"system_id", systemID,
			"resource_attribute_count", len(attributes),
		)
		return fmt.Errorf("%w: %w", ErrPermissionRegistrationFailed, err)
	}
	if err := s.permissionClient.RegisterResourceAttributes(ctx, systemID, attributes); err != nil {
		s.logger.ErrorContext(ctx, "failed to register resource attributes",
			"err", err,
			"system_id", systemID,
			"resource_attribute_count", len(attributes),
		)
		return fmt.Errorf("%w: %w", ErrPermissionRegistrationFailed, err)
	}
	return nil
}

func (s *SystemResourceService) ListSystemResources(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	definitions, err := s.repository.ListResourceDefinitions(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list system resources: %w", err)
	}
	return definitions, nil
}

func (s *SystemResourceService) GetSystemResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) ([]resource.ResourceAttribute, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	attributes, found, err := s.repository.GetResourceAttributes(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get system resource attributes: %w", err)
	}
	if !found {
		return []resource.ResourceAttribute{}, nil
	}
	return append([]resource.ResourceAttribute(nil), attributes.Values...), nil
}

func mergeResourceDefinitions(existing, updates []resource.ResourceDefinition) []resource.ResourceDefinition {
	byIdentity := map[string]resource.ResourceDefinition{}
	for _, definition := range existing {
		byIdentity[resourceDefinitionIdentity(definition.Type, definition.Key)] = definition
	}
	for _, definition := range updates {
		byIdentity[resourceDefinitionIdentity(definition.Type, definition.Key)] = definition
	}
	merged := make([]resource.ResourceDefinition, 0, len(byIdentity))
	for _, definition := range byIdentity {
		merged = append(merged, definition)
	}
	return merged
}

func deriveResourceAttributes(definitions []resource.ResourceDefinition) []resource.ResourceAttribute {
	actions := resourceDefinitionKeys(definitions, resource.ResourceDefinitionKindAction)
	tags := resourceDefinitionKeys(definitions, resource.ResourceDefinitionKindTag)
	types := resourceDefinitionKeys(definitions, resource.ResourceDefinitionKindType)
	if len(actions) == 0 || len(tags) == 0 || len(types) == 0 {
		return nil
	}
	attributes := make([]resource.ResourceAttribute, 0, len(actions)*len(tags)*len(types))
	for _, action := range actions {
		for _, tag := range tags {
			for _, resourceType := range types {
				attributes = append(attributes, resource.NewResourceAttribute(action, tag, resourceType))
			}
		}
	}
	return attributes
}

func resourceDefinitionKeys(definitions []resource.ResourceDefinition, kind resource.ResourceDefinitionType) []string {
	keys := make([]string, 0)
	for _, definition := range definitions {
		if definition.Type == kind {
			keys = append(keys, definition.Key)
		}
	}
	sort.Strings(keys)
	return keys
}

func resourceDefinitionIdentity(kind resource.ResourceDefinitionType, key string) string {
	return string(kind) + "\x00" + key
}
