package transport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func TestParseGroupExpiryCommandEvent(t *testing.T) {
	t.Parallel()

	event := newGroupExpiryCommandEvent(t)
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	input, err := ParseGroupExpiryCommandEvent(data, "app.todo.group.expiry.process")
	if err != nil {
		t.Fatalf("ParseGroupExpiryCommandEvent() error = %v, want nil", err)
	}
	if input.TaskID != "task-1" || input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" || input.ExpirationBucket != "2026-05-10" {
		t.Fatalf("input = %+v", input)
	}
}

func TestParseGroupExpiryCommandEventRejectsInvalidEnvelope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*cloudevents.Event)
		wantErr string
	}{
		{
			name: "wrong type",
			mutate: func(event *cloudevents.Event) {
				event.SetType("app.todo.other")
			},
			wantErr: "does not match expected",
		},
		{
			name: "non json content type",
			mutate: func(event *cloudevents.Event) {
				event.SetDataContentType("text/plain")
			},
			wantErr: "datacontenttype",
		},
		{
			name: "missing time",
			mutate: func(event *cloudevents.Event) {
				event.SetTime(time.Time{})
			},
			wantErr: "time is required",
		},
		{
			name: "subject mismatch",
			mutate: func(event *cloudevents.Event) {
				event.SetSubject("different-task")
			},
			wantErr: "subject must match data.task_id",
		},
		{
			name: "empty workspace",
			mutate: func(event *cloudevents.Event) {
				mustSetGroupExpiryData(t, event, groupExpiryCommandData{
					TaskID:           "task-1",
					WorkspaceID:      " ",
					GroupID:          "group-1",
					ExpirationBucket: "2026-05-10",
				})
			},
			wantErr: "empty required field",
		},
		{
			name: "invalid bucket",
			mutate: func(event *cloudevents.Event) {
				mustSetGroupExpiryData(t, event, groupExpiryCommandData{
					TaskID:           "task-1",
					WorkspaceID:      "workspace-1",
					GroupID:          "group-1",
					ExpirationBucket: "2026/05/10",
				})
			},
			wantErr: "expiration_bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := newGroupExpiryCommandEvent(t)
			tt.mutate(&event)
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ParseGroupExpiryCommandEvent(data, "app.todo.group.expiry.process")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseGroupExpiryCommandEvent() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseGroupExpiryCommandEventRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseGroupExpiryCommandEvent([]byte("{"), "app.todo.group.expiry.process")
	if err == nil || !strings.Contains(err.Error(), "parse cloudevent") {
		t.Fatalf("ParseGroupExpiryCommandEvent() error = %v, want parse error", err)
	}
}

func newGroupExpiryCommandEvent(t *testing.T) cloudevents.Event {
	t.Helper()

	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudEventSpecVersion)
	event.SetType("app.todo.group.expiry.process")
	event.SetSource("group-expiry-scheduler")
	event.SetSubject("task-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))
	mustSetGroupExpiryData(t, &event, groupExpiryCommandData{
		TaskID:           "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-10",
	})
	return event
}

func mustSetGroupExpiryData(t *testing.T, event *cloudevents.Event, payload groupExpiryCommandData) {
	t.Helper()

	if err := event.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		t.Fatal(err)
	}
}
