package transport

import (
	"encoding/json"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func TestNewResourceDeletedEvent(t *testing.T) {
	eventTime := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)

	data, err := NewResourceDeletedEvent(resource.DeletedEvent{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
		EventID:     "event-1",
		EventTime:   eventTime,
	}, "app.todo.resource.deleted")
	if err != nil {
		t.Fatalf("NewResourceDeletedEvent error = %v, want nil", err)
	}

	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("unmarshal cloudevent: %v", err)
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("validate cloudevent: %v", err)
	}
	if event.SpecVersion() != "1.0" {
		t.Fatalf("SpecVersion = %q, want 1.0", event.SpecVersion())
	}
	if event.Type() != "app.todo.resource.deleted" {
		t.Fatalf("Type = %q, want app.todo.resource.deleted", event.Type())
	}
	if event.Source() != "function-service" {
		t.Fatalf("Source = %q, want function-service", event.Source())
	}
	if event.Subject() != "resource-1" {
		t.Fatalf("Subject = %q, want resource-1", event.Subject())
	}
	if event.ID() != "event-1" {
		t.Fatalf("ID = %q, want event-1", event.ID())
	}
	if !event.Time().Equal(eventTime) {
		t.Fatalf("Time = %s, want %s", event.Time(), eventTime)
	}
	if event.DataContentType() != "application/json" {
		t.Fatalf("DataContentType = %q, want application/json", event.DataContentType())
	}

	var payload map[string]string
	if err := event.DataAs(&payload); err != nil {
		t.Fatalf("DataAs error = %v, want nil", err)
	}
	if len(payload) != 3 {
		t.Fatalf("payload keys = %d, want 3", len(payload))
	}
	if payload["workspace_id"] != "workspace-1" {
		t.Fatalf("workspace_id = %q, want workspace-1", payload["workspace_id"])
	}
	if payload["function_key"] != "todo" {
		t.Fatalf("function_key = %q, want todo", payload["function_key"])
	}
	if payload["resource_id"] != "resource-1" {
		t.Fatalf("resource_id = %q, want resource-1", payload["resource_id"])
	}
}
