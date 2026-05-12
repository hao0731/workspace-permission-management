package transport

import (
	"encoding/json"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
)

func TestNewResourceCreateEvent(t *testing.T) {
	eventTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	data, err := NewResourceCreateEvent(workspace.ResourceCreateCommand{
		WorkspaceID:  "workspace-1",
		AppName:      "documents",
		ResourceName: "Docs",
		ResourceType: "document",
		EventID:      "event-1",
		EventTime:    eventTime,
	})
	if err != nil {
		t.Fatalf("NewResourceCreateEvent() error = %v", err)
	}
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event.Type() != "cmd.app.documents.resource.create" || event.Subject() != "workspace-1" {
		t.Fatalf("type=%q subject=%q", event.Type(), event.Subject())
	}
	if event.Source() != "workspace-service" || event.ID() != "event-1" {
		t.Fatalf("source=%q id=%q", event.Source(), event.ID())
	}
	var payload map[string]string
	if err := event.DataAs(&payload); err != nil {
		t.Fatalf("DataAs() error = %v", err)
	}
	if payload["workspace_id"] != "workspace-1" || payload["resource_name"] != "Docs" || payload["resource_type"] != "document" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestNewResourceCreateEventRejectsInvalidCommand(t *testing.T) {
	_, err := NewResourceCreateEvent(workspace.ResourceCreateCommand{})
	if err == nil {
		t.Fatal("NewResourceCreateEvent() error = nil, want error")
	}
}
