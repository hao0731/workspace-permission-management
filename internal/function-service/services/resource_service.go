package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type ResourceRepository interface {
	Upsert(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error)
	List(ctx context.Context, query resource.ListQuery) (resource.Page, error)
}

type ResourceService struct {
	repository ResourceRepository
}

func NewResourceService(repository ResourceRepository) *ResourceService {
	return &ResourceService{repository: repository}
}

func (s *ResourceService) UpsertResource(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	if err := validateUpsertInput(input); err != nil {
		return "", err
	}
	status, err := s.repository.Upsert(ctx, input)
	if err != nil {
		return "", fmt.Errorf("upsert resource: %w", err)
	}
	return status, nil
}

func (s *ResourceService) ListResources(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	if err := validateListQuery(query); err != nil {
		return resource.Page{}, err
	}
	page, err := s.repository.List(ctx, query)
	if err != nil {
		return resource.Page{}, fmt.Errorf("list resources: %w", err)
	}
	return page, nil
}

func validateUpsertInput(input resource.UpsertInput) error {
	if strings.TrimSpace(input.ID) == "" {
		return fmt.Errorf("%w: resource id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return fmt.Errorf("%w: function key is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		return fmt.Errorf("%w: display name is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.Type) == "" {
		return fmt.Errorf("%w: resource type is required", resource.ErrInvalidInput)
	}
	if input.EventTime.IsZero() {
		return fmt.Errorf("%w: event time is required", resource.ErrInvalidInput)
	}
	for _, tag := range input.Tags {
		if strings.TrimSpace(tag) == "" {
			return fmt.Errorf("%w: resource tags must be non-empty strings", resource.ErrInvalidInput)
		}
	}
	return nil
}

func validateListQuery(query resource.ListQuery) error {
	if strings.TrimSpace(query.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(query.FunctionKey) == "" {
		return fmt.Errorf("%w: function key is required", resource.ErrInvalidInput)
	}
	if query.Limit <= 0 {
		return fmt.Errorf("%w: limit must be greater than zero", resource.ErrInvalidInput)
	}
	if query.Cursor != nil {
		if query.Cursor.CreatedAt.IsZero() {
			return fmt.Errorf("%w: cursor created_at is required", resource.ErrInvalidInput)
		}
		if strings.TrimSpace(query.Cursor.ID) == "" {
			return fmt.Errorf("%w: cursor id is required", resource.ErrInvalidInput)
		}
	}
	return nil
}
