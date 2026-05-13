package transport

import (
	"encoding/json"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func TestNewResourceUpsertEvent(t *testing.T) {
	data, subject, err := NewResourceUpsertEvent(resource.ResourceUpsertEvent{
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
		t.Fatalf("NewResourceUpsertEvent() error = %v", err)
	}
	if subject != "app.documents.resource.upserted" {
		t.Fatalf("subject = %q", subject)
	}
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event.Type() != "app.documents.resource.upserted" || event.Source() != "mock-function" || event.Subject() != "resource-1" {
		t.Fatalf("event type=%q source=%q subject=%q", event.Type(), event.Source(), event.Subject())
	}
	var payload map[string]any
	if err := event.DataAs(&payload); err != nil {
		t.Fatalf("DataAs() error = %v", err)
	}
	if payload["function_key"] != "documents" || payload["workspace_id"] != "workspace-1" {
		t.Fatalf("payload = %#v", payload)
	}
	tags, ok := payload["resource_tags"].([]any)
	if !ok || len(tags) != 0 {
		t.Fatalf("resource_tags = %#v", payload["resource_tags"])
	}
}

func TestNewResourceUpsertEventRejectsInvalidEvent(t *testing.T) {
	_, _, err := NewResourceUpsertEvent(resource.ResourceUpsertEvent{})
	if err == nil {
		t.Fatal("NewResourceUpsertEvent() error = nil, want error")
	}
}
