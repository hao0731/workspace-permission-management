package main

import (
	"context"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/mockfunction"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type fakeMessagePublisher struct {
	subject string
	data    []byte
	opts    []eventbus.PublishOption
}

func (f *fakeMessagePublisher) Publish(_ context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error {
	f.subject = subject
	f.data = append([]byte(nil), data...)
	f.opts = append([]eventbus.PublishOption(nil), opts...)
	return nil
}

func TestResourceUpsertPublisherPublishesDerivedSubject(t *testing.T) {
	publisher := &fakeMessagePublisher{}
	upsertPublisher := newResourceUpsertPublisher(publisher, eventbus.WithPublishTimeout(time.Second))
	err := upsertPublisher.PublishResourceUpsert(context.Background(), mockfunction.ResourceUpsertEvent{
		ResourceID:   "resource-1",
		DisplayName:  "Docs",
		ResourceType: "document",
		ResourceTags: []string{},
		FunctionKey:  "documents",
		WorkspaceID:  "workspace-1",
		EventID:      "event-1",
		EventTime:    time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PublishResourceUpsert() error = %v", err)
	}
	if publisher.subject != "app.documents.resource.upserted" {
		t.Fatalf("subject = %q", publisher.subject)
	}
	if len(publisher.data) == 0 {
		t.Fatal("data is empty")
	}
	if len(publisher.opts) != 1 {
		t.Fatalf("opts len = %d, want 1", len(publisher.opts))
	}
}
