package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type fakeSystemResourceRepository struct {
	transactionCalls int
	existing         []resource.ResourceDefinition
	latest           []resource.ResourceDefinition
	saved            []resource.ResourceDefinition
	attributes       resource.ResourceAttributes
	attributesFound  bool
	attributesSaved  resource.ResourceAttributes
	err              error
}

func (f *fakeSystemResourceRepository) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	f.transactionCalls++
	return fn(ctx)
}

func (f *fakeSystemResourceRepository) ListResourceDefinitions(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.saved != nil {
		return append([]resource.ResourceDefinition(nil), f.latest...), nil
	}
	return append([]resource.ResourceDefinition(nil), f.existing...), nil
}

func (f *fakeSystemResourceRepository) UpsertResourceDefinitions(ctx context.Context, definitions []resource.ResourceDefinition) ([]resource.ResourceDefinition, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.saved = append([]resource.ResourceDefinition(nil), definitions...)
	if f.latest == nil {
		f.latest = append(append([]resource.ResourceDefinition(nil), f.existing...), definitions...)
	}
	return append([]resource.ResourceDefinition(nil), definitions...), nil
}

func (f *fakeSystemResourceRepository) UpsertResourceAttributes(ctx context.Context, attributes resource.ResourceAttributes) (resource.ResourceAttributes, error) {
	f.attributesSaved = attributes
	return attributes, nil
}

func (f *fakeSystemResourceRepository) GetResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) (resource.ResourceAttributes, bool, error) {
	if f.err != nil {
		return resource.ResourceAttributes{}, false, f.err
	}
	return f.attributes, f.attributesFound, nil
}

func TestSystemResourceServiceSaveSystemResources(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeSystemResourceRepository{
		existing: []resource.ResourceDefinition{{ID: "existing-1", SystemID: "todo", Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private", CreatedAt: now, UpdatedAt: now}},
		latest: []resource.ResourceDefinition{
			{ID: "new-action", SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit", CreatedAt: now, UpdatedAt: now},
			{ID: "existing-1", SystemID: "todo", Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private", CreatedAt: now, UpdatedAt: now},
			{ID: "new-type", SystemID: "todo", Type: resource.ResourceDefinitionKindType, Label: "Repository", Key: "repo", CreatedAt: now, UpdatedAt: now},
		},
	}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20},
		WithSystemResourceClock(func() time.Time { return now }),
		WithSystemResourceIDGenerator(sequenceIDs("new-action", "new-type", "attributes-1")),
	)

	saved, err := service.SaveSystemResources(context.Background(), resource.ResourceDefinitionSaveInput{
		SystemID: "todo",
		Resources: []resource.ResourceDefinitionInput{
			{Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit"},
			{Type: resource.ResourceDefinitionKindType, Label: "Repository", Key: "repo"},
		},
	})
	if err != nil {
		t.Fatalf("SaveSystemResources error = %v, want nil", err)
	}
	if repo.transactionCalls != 1 {
		t.Fatalf("transaction calls = %d, want 1", repo.transactionCalls)
	}
	if len(saved) != 2 || saved[0].Key != "can_edit" || saved[1].Key != "repo" {
		t.Fatalf("saved = %+v, want request order can_edit/repo", saved)
	}
	wantAttributes := []resource.ResourceAttribute{resource.ResourceAttribute("can_edit_private_repo")}
	if !reflect.DeepEqual(repo.attributesSaved.Values, wantAttributes) {
		t.Fatalf("attributes = %#v, want %#v", repo.attributesSaved.Values, wantAttributes)
	}
}

func TestSystemResourceServiceDoesNotWriteAttributesWhenIncomplete(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeSystemResourceRepository{}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20},
		WithSystemResourceClock(func() time.Time { return now }),
		WithSystemResourceIDGenerator(sequenceIDs("definition-1", "attributes-1")),
	)

	_, err := service.SaveSystemResources(context.Background(), resource.ResourceDefinitionSaveInput{
		SystemID:  "todo",
		Resources: []resource.ResourceDefinitionInput{{Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit"}},
	})
	if err != nil {
		t.Fatalf("SaveSystemResources error = %v, want nil", err)
	}
	if len(repo.attributesSaved.Values) != 0 {
		t.Fatalf("attributes = %#v, want none", repo.attributesSaved.Values)
	}
}

func TestSystemResourceServiceRejectsLimitViolation(t *testing.T) {
	repo := &fakeSystemResourceRepository{
		existing: []resource.ResourceDefinition{{SystemID: "todo", Type: resource.ResourceDefinitionKindType, Key: "repo"}},
	}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 1, Actions: 5, Tags: 20})

	_, err := service.SaveSystemResources(context.Background(), resource.ResourceDefinitionSaveInput{
		SystemID:  "todo",
		Resources: []resource.ResourceDefinitionInput{{Type: resource.ResourceDefinitionKindType, Label: "Page", Key: "page"}},
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestSystemResourceServiceListAndGetAttributes(t *testing.T) {
	repo := &fakeSystemResourceRepository{
		existing: []resource.ResourceDefinition{{SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Key: "can_edit"}},
		attributes: resource.ResourceAttributes{
			SystemID: "todo",
			Values:   []resource.ResourceAttribute{resource.ResourceAttribute("can_edit_private_repo")},
		},
		attributesFound: true,
	}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20})

	definitions, err := service.ListSystemResources(context.Background(), resource.ResourceDefinitionsQuery{SystemID: "todo"})
	if err != nil {
		t.Fatalf("ListSystemResources error = %v, want nil", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("definitions len = %d, want 1", len(definitions))
	}
	attributes, err := service.GetSystemResourceAttributes(context.Background(), resource.ResourceAttributesQuery{SystemID: "todo"})
	if err != nil {
		t.Fatalf("GetSystemResourceAttributes error = %v, want nil", err)
	}
	if len(attributes) != 1 || attributes[0] != "can_edit_private_repo" {
		t.Fatalf("attributes = %#v, want can_edit_private_repo", attributes)
	}
}

func sequenceIDs(values ...string) func() string {
	index := 0
	return func() string {
		value := values[index]
		index++
		return value
	}
}
