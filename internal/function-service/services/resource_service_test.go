package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type fakeResourceRepository struct {
	upsertStatus resource.UpsertStatus
	upsertInput  resource.UpsertInput
	upsertErr    error
	listQuery    resource.ListQuery
	listPage     resource.Page
	listErr      error
	deleteStatus resource.DeleteStatus
	deleteInput  resource.DeleteInput
	deleteErr    error
}

func (f *fakeResourceRepository) Upsert(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	f.upsertInput = input
	if f.upsertErr != nil {
		return "", f.upsertErr
	}
	return f.upsertStatus, nil
}

func (f *fakeResourceRepository) List(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	f.listQuery = query
	if f.listErr != nil {
		return resource.Page{}, f.listErr
	}
	return f.listPage, nil
}

func (f *fakeResourceRepository) Delete(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error) {
	f.deleteInput = input
	if f.deleteErr != nil {
		return "", f.deleteErr
	}
	return f.deleteStatus, nil
}

type fakeResourceDeletedPublisher struct {
	event resource.DeletedEvent
	calls int
	err   error
}

func (f *fakeResourceDeletedPublisher) PublishResourceDeleted(ctx context.Context, event resource.DeletedEvent) error {
	f.calls++
	f.event = event
	return f.err
}

func TestResourceServiceUpsertResource(t *testing.T) {
	eventTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	repo := &fakeResourceRepository{upsertStatus: resource.UpsertStatusInserted}
	service := NewResourceService(repo)

	got, err := service.UpsertResource(context.Background(), resource.UpsertInput{
		ID:          "resource-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		DisplayName: "Spec",
		Type:        "document",
		Tags:        []string{"section_1"},
		EventTime:   eventTime,
	})
	if err != nil {
		t.Fatalf("UpsertResource error = %v, want nil", err)
	}
	if got != resource.UpsertStatusInserted {
		t.Fatalf("status = %q, want %q", got, resource.UpsertStatusInserted)
	}
	if repo.upsertInput.ID != "resource-1" {
		t.Fatalf("repo input ID = %q, want resource-1", repo.upsertInput.ID)
	}
	if repo.upsertInput.EventTime != eventTime {
		t.Fatalf("repo input EventTime = %s, want %s", repo.upsertInput.EventTime, eventTime)
	}
}

func TestResourceServiceRejectsInvalidUpsertInput(t *testing.T) {
	service := NewResourceService(&fakeResourceRepository{})

	_, err := service.UpsertResource(context.Background(), resource.UpsertInput{
		ID:          "",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		DisplayName: "Spec",
		Type:        "document",
		Tags:        []string{"section_1"},
		EventTime:   time.Now(),
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestResourceServiceListResources(t *testing.T) {
	cursorTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	repo := &fakeResourceRepository{
		listPage: resource.Page{
			Resources:   []resource.Resource{{ID: "resource-1"}},
			HasNextPage: true,
			NextCursor:  &resource.Cursor{CreatedAt: cursorTime, ID: "resource-1"},
		},
	}
	service := NewResourceService(repo)

	page, err := service.ListResources(context.Background(), resource.ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       20,
	})
	if err != nil {
		t.Fatalf("ListResources error = %v, want nil", err)
	}
	if len(page.Resources) != 1 {
		t.Fatalf("resources len = %d, want 1", len(page.Resources))
	}
	if !page.HasNextPage {
		t.Fatal("HasNextPage = false, want true")
	}
	if repo.listQuery.WorkspaceID != "workspace-1" || repo.listQuery.FunctionKey != "todo" {
		t.Fatalf("repo query = %+v, want workspace-1/todo", repo.listQuery)
	}
}

func TestResourceServiceRejectsInvalidListQuery(t *testing.T) {
	service := NewResourceService(&fakeResourceRepository{})

	_, err := service.ListResources(context.Background(), resource.ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       0,
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestResourceServiceDeleteResourcePublishesAfterDelete(t *testing.T) {
	eventTime := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := &fakeResourceRepository{deleteStatus: resource.DeleteStatusDeleted}
	publisher := &fakeResourceDeletedPublisher{}
	service := NewResourceService(repo,
		WithResourceDeletedPublisher(publisher),
		WithClock(func() time.Time { return eventTime }),
		WithIDGenerator(func() string { return "event-1" }),
	)

	status, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	})
	if err != nil {
		t.Fatalf("DeleteResource error = %v, want nil", err)
	}
	if status != resource.DeleteStatusDeleted {
		t.Fatalf("status = %q, want %q", status, resource.DeleteStatusDeleted)
	}
	if repo.deleteInput.ResourceID != "resource-1" {
		t.Fatalf("repo delete input = %+v, want resource-1", repo.deleteInput)
	}
	if publisher.calls != 1 {
		t.Fatalf("publisher calls = %d, want 1", publisher.calls)
	}
	if publisher.event.EventID != "event-1" {
		t.Fatalf("event id = %q, want event-1", publisher.event.EventID)
	}
	if !publisher.event.EventTime.Equal(eventTime) {
		t.Fatalf("event time = %s, want %s", publisher.event.EventTime, eventTime)
	}
	if publisher.event.WorkspaceID != "workspace-1" || publisher.event.FunctionKey != "todo" || publisher.event.ResourceID != "resource-1" {
		t.Fatalf("event = %+v, want workspace-1/todo/resource-1", publisher.event)
	}
}

func TestResourceServiceDeleteResourceMissingDoesNotPublish(t *testing.T) {
	repo := &fakeResourceRepository{deleteStatus: resource.DeleteStatusNotFound}
	publisher := &fakeResourceDeletedPublisher{}
	service := NewResourceService(repo, WithResourceDeletedPublisher(publisher))

	status, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	})
	if err != nil {
		t.Fatalf("DeleteResource error = %v, want nil", err)
	}
	if status != resource.DeleteStatusNotFound {
		t.Fatalf("status = %q, want %q", status, resource.DeleteStatusNotFound)
	}
	if publisher.calls != 0 {
		t.Fatalf("publisher calls = %d, want 0", publisher.calls)
	}
}

func TestResourceServiceDeleteResourceRejectsInvalidInput(t *testing.T) {
	service := NewResourceService(&fakeResourceRepository{})

	_, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "",
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestResourceServiceDeleteResourceReturnsRepositoryError(t *testing.T) {
	repo := &fakeResourceRepository{deleteErr: errors.New("database unavailable")}
	service := NewResourceService(repo, WithResourceDeletedPublisher(&fakeResourceDeletedPublisher{}))

	_, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	})
	if err == nil {
		t.Fatal("DeleteResource error = nil, want error")
	}
}

func TestResourceServiceDeleteResourceReturnsPublishError(t *testing.T) {
	repo := &fakeResourceRepository{deleteStatus: resource.DeleteStatusDeleted}
	publisher := &fakeResourceDeletedPublisher{err: errors.New("publish failed")}
	service := NewResourceService(repo, WithResourceDeletedPublisher(publisher))

	_, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	})
	if err == nil {
		t.Fatal("DeleteResource error = nil, want error")
	}
	if publisher.calls != 1 {
		t.Fatalf("publisher calls = %d, want 1", publisher.calls)
	}
}
