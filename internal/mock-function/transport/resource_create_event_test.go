package transport

import (
	"encoding/json"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func TestParseResourceCreateCommandEvent(t *testing.T) {
	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetType("cmd.app.documents.resource.create")
	event.SetSource("workspace-service")
	event.SetSubject("workspace-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC))
	if err := event.SetData(cloudevents.ApplicationJSON, map[string]string{
		"workspace_id":  "workspace-1",
		"resource_name": "Docs",
		"resource_type": "document",
	}); err != nil {
		t.Fatalf("SetData() error = %v", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	command, err := ParseResourceCreateCommandEvent(data, "cmd.app.documents.resource.create", map[string]string{
		"cmd.app.documents.resource.create": "documents",
	})
	if err != nil {
		t.Fatalf("ParseResourceCreateCommandEvent() error = %v", err)
	}
	if command.AppName != "documents" || command.WorkspaceID != "workspace-1" {
		t.Fatalf("command = %+v", command)
	}
	if command.EventID != "event-1" {
		t.Fatalf("EventID = %q, want event-1", command.EventID)
	}
	wantTime := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	if !command.EventTime.Equal(wantTime) {
		t.Fatalf("EventTime = %s, want %s", command.EventTime, wantTime)
	}
}

func TestParseResourceCreateCommandEventRejectsUnknownSubject(t *testing.T) {
	_, err := ParseResourceCreateCommandEvent([]byte(`{}`), "cmd.app.unknown.resource.create", map[string]string{})
	if err == nil {
		t.Fatal("ParseResourceCreateCommandEvent() error = nil, want error")
	}
}

func TestParseResourceCreateCommandEventRejectsSubjectMismatch(t *testing.T) {
	event := newTestCreateEvent(t)
	event.SetSubject("other-workspace")
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	_, err = ParseResourceCreateCommandEvent(data, "cmd.app.documents.resource.create", map[string]string{
		"cmd.app.documents.resource.create": "documents",
	})
	if err == nil {
		t.Fatal("ParseResourceCreateCommandEvent() error = nil, want error")
	}
}

func newTestCreateEvent(t *testing.T) cloudevents.Event {
	t.Helper()
	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetType("cmd.app.documents.resource.create")
	event.SetSource("workspace-service")
	event.SetSubject("workspace-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC))
	if err := event.SetData(cloudevents.ApplicationJSON, map[string]string{
		"workspace_id":  "workspace-1",
		"resource_name": "Docs",
		"resource_type": "document",
	}); err != nil {
		t.Fatalf("SetData() error = %v", err)
	}
	return event
}
