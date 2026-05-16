package transport

import (
	"encoding/json"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func TestNewGroupExpiryCommandEvent(t *testing.T) {
	data, err := NewGroupExpiryCommandEvent(GroupExpiryCommand{
		TaskID:           "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-16",
	}, "app.todo.group.expiry.process", "event-1", time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewGroupExpiryCommandEvent error = %v, want nil", err)
	}
	event := parseAndAssertEvent(t, data, "app.todo.group.expiry.process")
	var payload GroupExpiryCommand
	if err := event.DataAs(&payload); err != nil {
		t.Fatalf("DataAs error = %v", err)
	}
	if payload.TaskID != "task-1" || payload.WorkspaceID != "workspace-1" || payload.GroupID != "group-1" || payload.ExpirationBucket != "2026-05-16" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestNewIndividualMemberExpiryCommandEvent(t *testing.T) {
	data, err := NewIndividualMemberExpiryCommandEvent(IndividualMemberExpiryCommand{
		TaskID:           "task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-05-16",
	}, "app.todo.group.individual-member.expiry.process", "event-1", time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewIndividualMemberExpiryCommandEvent error = %v, want nil", err)
	}
	event := parseAndAssertEvent(t, data, "app.todo.group.individual-member.expiry.process")
	var payload IndividualMemberExpiryCommand
	if err := event.DataAs(&payload); err != nil {
		t.Fatalf("DataAs error = %v", err)
	}
	if payload.TaskID != "task-1" || payload.GroupID != "group-1" || payload.NTAccount != "user1" || payload.ExpirationBucket != "2026-05-16" {
		t.Fatalf("payload = %+v", payload)
	}
}

func parseAndAssertEvent(t *testing.T, data []byte, eventType string) cloudevents.Event {
	t.Helper()
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	if event.Type() != eventType || event.Source() != "group-expiry-scheduler" || event.Subject() != "task-1" || event.ID() != "event-1" {
		t.Fatalf("event metadata = type:%q source:%q subject:%q id:%q", event.Type(), event.Source(), event.Subject(), event.ID())
	}
	return event
}
