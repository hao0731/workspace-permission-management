package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/mockfunction"
)

type fakeUpsertPublisher struct {
	events []mockfunction.ResourceUpsertEvent
	err    error
}

func (f *fakeUpsertPublisher) PublishResourceUpsert(_ context.Context, event mockfunction.ResourceUpsertEvent) error {
	f.events = append(f.events, event)
	return f.err
}

func TestResourceServiceHandleCreateCommandPublishesUpsert(t *testing.T) {
	publisher := &fakeUpsertPublisher{}
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	service := NewResourceService(publisher,
		WithClock(func() time.Time { return now }),
		WithIDGenerator(sequenceIDs("resource-1", "event-1")),
	)
	err := service.HandleResourceCreate(context.Background(), mockfunction.ResourceCreateCommand{
		WorkspaceID:  "workspace-1",
		AppName:      "documents",
		ResourceName: "Docs",
		ResourceType: "document",
	})
	if err != nil {
		t.Fatalf("HandleResourceCreate() error = %v", err)
	}
	if len(publisher.events) != 1 || publisher.events[0].FunctionKey != "documents" {
		t.Fatalf("events = %+v", publisher.events)
	}
	if publisher.events[0].ResourceID != "resource-1" || publisher.events[0].EventID != "event-1" {
		t.Fatalf("event ids = %+v", publisher.events[0])
	}
}

func TestResourceServiceHandleCreateCommandRejectsInvalidInput(t *testing.T) {
	publisher := &fakeUpsertPublisher{}
	service := NewResourceService(publisher)
	err := service.HandleResourceCreate(context.Background(), mockfunction.ResourceCreateCommand{})
	if !errors.Is(err, mockfunction.ErrInvalidInput) {
		t.Fatalf("HandleResourceCreate() error = %v, want ErrInvalidInput", err)
	}
	if len(publisher.events) != 0 {
		t.Fatalf("events = %+v, want none", publisher.events)
	}
}

func TestResourceServiceHandleCreateCommandReturnsPublishError(t *testing.T) {
	publisher := &fakeUpsertPublisher{err: errors.New("publish failed")}
	service := NewResourceService(publisher, WithIDGenerator(sequenceIDs("resource-1", "event-1")))
	err := service.HandleResourceCreate(context.Background(), mockfunction.ResourceCreateCommand{
		WorkspaceID:  "workspace-1",
		AppName:      "documents",
		ResourceName: "Docs",
		ResourceType: "document",
	})
	if err == nil {
		t.Fatal("HandleResourceCreate() error = nil, want error")
	}
}

func sequenceIDs(ids ...string) func() string {
	index := 0
	return func() string {
		if index >= len(ids) {
			return "extra-id"
		}
		id := ids[index]
		index++
		return id
	}
}
