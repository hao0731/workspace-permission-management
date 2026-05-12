package mockfunction

import (
	"errors"
	"testing"
	"time"
)

func TestResourceCreateCommandValidate(t *testing.T) {
	tests := []ResourceCreateCommand{
		{AppName: "documents", ResourceName: "Docs", ResourceType: "document"},
		{WorkspaceID: "workspace-1", ResourceName: "Docs", ResourceType: "document"},
		{WorkspaceID: "workspace-1", AppName: "documents", ResourceType: "document"},
		{WorkspaceID: "workspace-1", AppName: "documents", ResourceName: "Docs"},
	}
	for _, command := range tests {
		if err := command.Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate() error = %v, want ErrInvalidInput for %+v", err, command)
		}
	}
}

func TestResourceCreateCommandNormalize(t *testing.T) {
	command := ResourceCreateCommand{WorkspaceID: " workspace-1 ", AppName: " documents ", ResourceName: " Docs ", ResourceType: " document "}
	normalized := command.Normalize()
	if normalized.WorkspaceID != "workspace-1" || normalized.AppName != "documents" || normalized.ResourceName != "Docs" || normalized.ResourceType != "document" {
		t.Fatalf("Normalize() = %+v", normalized)
	}
}

func TestResourceUpsertEventValidate(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	tests := []ResourceUpsertEvent{
		{DisplayName: "Docs", ResourceType: "document", FunctionKey: "documents", WorkspaceID: "workspace-1", EventID: "event-1", EventTime: now},
		{ResourceID: "resource-1", ResourceType: "document", FunctionKey: "documents", WorkspaceID: "workspace-1", EventID: "event-1", EventTime: now},
		{ResourceID: "resource-1", DisplayName: "Docs", FunctionKey: "documents", WorkspaceID: "workspace-1", EventID: "event-1", EventTime: now},
		{ResourceID: "resource-1", DisplayName: "Docs", ResourceType: "document", WorkspaceID: "workspace-1", EventID: "event-1", EventTime: now},
		{ResourceID: "resource-1", DisplayName: "Docs", ResourceType: "document", FunctionKey: "documents", EventID: "event-1", EventTime: now},
		{ResourceID: "resource-1", DisplayName: "Docs", ResourceType: "document", FunctionKey: "documents", WorkspaceID: "workspace-1", EventTime: now},
		{ResourceID: "resource-1", DisplayName: "Docs", ResourceType: "document", FunctionKey: "documents", WorkspaceID: "workspace-1", EventID: "event-1"},
	}
	for _, event := range tests {
		if err := event.Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate() error = %v, want ErrInvalidInput for %+v", err, event)
		}
	}
}

func TestResourceUpsertEventSubject(t *testing.T) {
	event := ResourceUpsertEvent{FunctionKey: "documents"}
	if event.Subject() != "app.documents.resource.upserted" {
		t.Fatalf("Subject() = %q", event.Subject())
	}
}
