package main

import (
	"context"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
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

func TestResourceCreatePublisherPublishesExpectedSubject(t *testing.T) {
	publisher := &fakeMessagePublisher{}
	resourcePublisher := newResourceCreatePublisher(publisher, eventbus.WithPublishTimeout(time.Second))
	err := resourcePublisher.PublishResourceCreateCommand(context.Background(), resource.ResourceCreateCommand{
		WorkspaceID:  "workspace-1",
		AppName:      "documents",
		ResourceName: "Docs",
		ResourceType: "document",
		EventID:      "event-1",
		EventTime:    time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PublishResourceCreateCommand() error = %v", err)
	}
	if publisher.subject != "cmd.app.documents.resource.create" {
		t.Fatalf("subject = %q", publisher.subject)
	}
	if len(publisher.data) == 0 {
		t.Fatal("data is empty")
	}
	if len(publisher.opts) != 1 {
		t.Fatalf("opts len = %d, want 1", len(publisher.opts))
	}
}
