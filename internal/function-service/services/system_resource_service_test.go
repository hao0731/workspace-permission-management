package services

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type fakeSystemResourceRepository struct {
	transactionCalls     int
	inTransaction        bool
	transactionCommitted bool
	existing             []resource.ResourceDefinition
	latest               []resource.ResourceDefinition
	saved                []resource.ResourceDefinition
	attributes           resource.ResourceAttributes
	attributesFound      bool
	attributesSaved      resource.ResourceAttributes
	err                  error
}

func (f *fakeSystemResourceRepository) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	f.transactionCalls++
	f.inTransaction = true
	err := fn(ctx)
	f.inTransaction = false
	if err != nil {
		return err
	}
	f.transactionCommitted = true
	return nil
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

type fakePermissionClient struct {
	repo                       *fakeSystemResourceRepository
	calls                      int
	systemID                   string
	attributes                 []resource.ResourceAttribute
	calledDuringTransaction    bool
	transactionCommittedAtCall bool
	err                        error
}

func (f *fakePermissionClient) RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error {
	f.calls++
	f.systemID = systemID
	f.attributes = append([]resource.ResourceAttribute(nil), resourceAttributes...)
	if f.repo != nil {
		f.calledDuringTransaction = f.repo.inTransaction
		f.transactionCommittedAtCall = f.repo.transactionCommitted
	}
	return f.err
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
	permissionClient := &fakePermissionClient{repo: repo}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20}, permissionClient,
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
	if permissionClient.calls != 1 {
		t.Fatalf("permission client calls = %d, want 1", permissionClient.calls)
	}
	if permissionClient.systemID != "todo" {
		t.Fatalf("permission systemID = %q, want todo", permissionClient.systemID)
	}
	if !reflect.DeepEqual(permissionClient.attributes, wantAttributes) {
		t.Fatalf("permission attributes = %#v, want %#v", permissionClient.attributes, wantAttributes)
	}
	if permissionClient.calledDuringTransaction {
		t.Fatal("permission client called during transaction, want post-commit call")
	}
	if !permissionClient.transactionCommittedAtCall {
		t.Fatal("permission client called before transaction commit")
	}
}

func TestSystemResourceServiceDoesNotWriteAttributesWhenIncomplete(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeSystemResourceRepository{}
	permissionClient := &fakePermissionClient{}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20}, permissionClient,
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
	if permissionClient.calls != 0 {
		t.Fatalf("permission client calls = %d, want 0", permissionClient.calls)
	}
}

func TestSystemResourceServiceRejectsLimitViolation(t *testing.T) {
	repo := &fakeSystemResourceRepository{
		existing: []resource.ResourceDefinition{{SystemID: "todo", Type: resource.ResourceDefinitionKindType, Key: "repo"}},
	}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 1, Actions: 5, Tags: 20}, &fakePermissionClient{})

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
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20}, &fakePermissionClient{})

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

func TestSystemResourceServiceReturnsPermissionRegistrationFailureAfterLocalCommit(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeSystemResourceRepository{
		latest: []resource.ResourceDefinition{
			{ID: "new-action", SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit", CreatedAt: now, UpdatedAt: now},
			{ID: "new-tag", SystemID: "todo", Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private", CreatedAt: now, UpdatedAt: now},
			{ID: "new-type", SystemID: "todo", Type: resource.ResourceDefinitionKindType, Label: "Repository", Key: "repo", CreatedAt: now, UpdatedAt: now},
		},
	}
	permissionClient := &fakePermissionClient{repo: repo, err: errors.New("permission unavailable")}
	var logBuffer bytes.Buffer
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20}, permissionClient,
		WithSystemResourceClock(func() time.Time { return now }),
		WithSystemResourceIDGenerator(sequenceIDs("new-action", "new-tag", "new-type", "attributes-1")),
		WithSystemResourceLogger(slog.New(slog.NewTextHandler(&logBuffer, nil))),
	)

	_, err := service.SaveSystemResources(context.Background(), resource.ResourceDefinitionSaveInput{
		SystemID: "todo",
		Resources: []resource.ResourceDefinitionInput{
			{Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit"},
			{Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private"},
			{Type: resource.ResourceDefinitionKindType, Label: "Repository", Key: "repo"},
		},
	})
	if !errors.Is(err, ErrPermissionRegistrationFailed) {
		t.Fatalf("error = %v, want ErrPermissionRegistrationFailed", err)
	}
	if !repo.transactionCommitted {
		t.Fatal("transactionCommitted = false, want true")
	}
	if len(repo.attributesSaved.Values) != 1 {
		t.Fatalf("saved attributes = %#v, want committed derived attribute", repo.attributesSaved.Values)
	}
	if permissionClient.calls != 1 {
		t.Fatalf("permission client calls = %d, want 1", permissionClient.calls)
	}
	output := logBuffer.String()
	if !strings.Contains(output, "failed to register resource attributes") {
		t.Fatalf("log output = %q, want permission failure message", output)
	}
	if !strings.Contains(output, "system_id=todo") {
		t.Fatalf("log output = %q, want system_id", output)
	}
	if !strings.Contains(output, "resource_attribute_count=1") {
		t.Fatalf("log output = %q, want resource_attribute_count", output)
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
