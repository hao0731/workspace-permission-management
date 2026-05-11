package transport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type expiryCommandEventSuite struct {
	name                  string
	expectedType          string
	source                string
	parse                 func([]byte, string) (any, error)
	assertCommand         func(*testing.T, any)
	setValidData          func(*testing.T, *cloudevents.Event)
	setEmptyRequiredField func(*testing.T, *cloudevents.Event)
	setInvalidBucket      func(*testing.T, *cloudevents.Event)
}

func TestParseExpiryCommandEvents(t *testing.T) {
	t.Parallel()

	for _, suite := range expiryCommandEventSuites() {
		t.Run(suite.name, func(t *testing.T) {
			event := newTestCommandCloudEvent(t, suite)
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}

			input, err := suite.parse(data, suite.expectedType)
			if err != nil {
				t.Fatalf("parse expiry command event error = %v, want nil", err)
			}
			suite.assertCommand(t, input)
		})
	}
}

func TestParseExpiryCommandEventsRejectInvalidEnvelope(t *testing.T) {
	t.Parallel()

	for _, suite := range expiryCommandEventSuites() {
		t.Run(suite.name, func(t *testing.T) {
			runInvalidEnvelopeCases(t, suite)
		})
	}
}

func TestParseExpiryCommandEventsRejectMalformedJSON(t *testing.T) {
	t.Parallel()

	for _, suite := range expiryCommandEventSuites() {
		t.Run(suite.name, func(t *testing.T) {
			_, err := suite.parse([]byte("{"), suite.expectedType)
			if err == nil || !strings.Contains(err.Error(), "parse cloudevent") {
				t.Fatalf("parse expiry command event error = %v, want parse error", err)
			}
		})
	}
}

func runInvalidEnvelopeCases(t *testing.T, suite expiryCommandEventSuite) {
	t.Helper()

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
			name: "empty required field",
			mutate: func(event *cloudevents.Event) {
				suite.setEmptyRequiredField(t, event)
			},
			wantErr: "empty required field",
		},
		{
			name: "invalid bucket",
			mutate: func(event *cloudevents.Event) {
				suite.setInvalidBucket(t, event)
			},
			wantErr: "expiration_bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := newTestCommandCloudEvent(t, suite)
			tt.mutate(&event)
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			_, err = suite.parse(data, suite.expectedType)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parse expiry command event error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func expiryCommandEventSuites() []expiryCommandEventSuite {
	return []expiryCommandEventSuite{
		groupExpiryCommandSuite(),
		individualMemberExpiryCommandSuite(),
	}
}

func groupExpiryCommandSuite() expiryCommandEventSuite {
	return expiryCommandEventSuite{
		name:                  "group expiry command",
		expectedType:          "app.todo.group.expiry.process",
		source:                "group-expiry-scheduler",
		parse:                 parseGroupExpiryCommandAsAny,
		assertCommand:         assertGroupExpiryCommand,
		setValidData:          setGroupExpiryData(groupExpiryData("workspace-1", "2026-05-10")),
		setEmptyRequiredField: setGroupExpiryData(groupExpiryData(" ", "2026-05-10")),
		setInvalidBucket:      setGroupExpiryData(groupExpiryData("workspace-1", "2026/05/10")),
	}
}

func individualMemberExpiryCommandSuite() expiryCommandEventSuite {
	return expiryCommandEventSuite{
		name:                  "individual member expiry command",
		expectedType:          "app.todo.group.individual-member.expiry.process",
		source:                "individual-member-expiry-scheduler",
		parse:                 parseIndividualMemberExpiryCommandAsAny,
		assertCommand:         assertIndividualMemberExpiryCommand,
		setValidData:          setIndividualMemberExpiryData("user1", "2026-05-10"),
		setEmptyRequiredField: setIndividualMemberExpiryData(" ", "2026-05-10"),
		setInvalidBucket:      setIndividualMemberExpiryData("user1", "2026/05/10"),
	}
}

func parseGroupExpiryCommandAsAny(data []byte, expectedType string) (any, error) {
	return ParseGroupExpiryCommandEvent(data, expectedType)
}

func parseIndividualMemberExpiryCommandAsAny(data []byte, expectedType string) (any, error) {
	return ParseIndividualMemberExpiryCommandEvent(data, expectedType)
}

func assertGroupExpiryCommand(t *testing.T, input any) {
	t.Helper()

	command, ok := input.(group.ExpireGroupingRuleCommand)
	if !ok {
		t.Fatalf("input type = %T, want ExpireGroupingRuleCommand", input)
	}
	if command.TaskID != "task-1" || command.WorkspaceID != "workspace-1" || command.GroupID != "group-1" || command.ExpirationBucket != "2026-05-10" {
		t.Fatalf("input = %+v", command)
	}
}

func assertIndividualMemberExpiryCommand(t *testing.T, input any) {
	t.Helper()

	command, ok := input.(group.ExpireIndividualMemberCommand)
	if !ok {
		t.Fatalf("input type = %T, want ExpireIndividualMemberCommand", input)
	}
	if command.TaskID != "task-1" || command.GroupID != "group-1" || command.NTAccount != "user1" || command.ExpirationBucket != "2026-05-10" {
		t.Fatalf("input = %+v", command)
	}
}

func groupExpiryData(workspaceID string, bucket string) groupExpiryCommandData {
	return groupExpiryCommandData{
		TaskID:           "task-1",
		WorkspaceID:      workspaceID,
		GroupID:          "group-1",
		ExpirationBucket: bucket,
	}
}

func setGroupExpiryData(payload groupExpiryCommandData) func(*testing.T, *cloudevents.Event) {
	return func(t *testing.T, event *cloudevents.Event) {
		t.Helper()
		mustSetCommandEventData(t, event, payload)
	}
}

func setIndividualMemberExpiryData(ntAccount string, bucket string) func(*testing.T, *cloudevents.Event) {
	return func(t *testing.T, event *cloudevents.Event) {
		t.Helper()
		mustSetCommandEventData(t, event, individualMemberExpiryCommandData{
			TaskID:           "task-1",
			GroupID:          "group-1",
			NTAccount:        ntAccount,
			ExpirationBucket: bucket,
		})
	}
}

func newTestCommandCloudEvent(t *testing.T, suite expiryCommandEventSuite) cloudevents.Event {
	t.Helper()

	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudEventSpecVersion)
	event.SetType(suite.expectedType)
	event.SetSource(suite.source)
	event.SetSubject("task-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))
	suite.setValidData(t, &event)
	return event
}

func mustSetCommandEventData(t *testing.T, event *cloudevents.Event, payload any) {
	t.Helper()

	if err := event.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		t.Fatal(err)
	}
}
