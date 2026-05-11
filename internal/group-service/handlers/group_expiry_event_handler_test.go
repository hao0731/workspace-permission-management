package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

func TestGroupExpiryEventHandlerHandle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		message    eventbus.Message
		service    *fakeGroupExpiryService
		wantResult eventbus.HandleResult
		wantCalls  int
	}{
		{
			name:       "invalid event terminates",
			message:    eventbus.Message{Subject: "app.todo.group.expiry.process", Data: []byte("{")},
			service:    &fakeGroupExpiryService{},
			wantResult: eventbus.HandleResultTerminate,
		},
		{
			name:       "service invalid input terminates",
			message:    validGroupExpiryMessage(t),
			service:    &fakeGroupExpiryService{err: group.ErrInvalidInput},
			wantResult: eventbus.HandleResultTerminate,
			wantCalls:  1,
		},
		{
			name:       "service failure retries",
			message:    validGroupExpiryMessage(t),
			service:    &fakeGroupExpiryService{err: errors.New("database unavailable")},
			wantResult: eventbus.HandleResultRetry,
			wantCalls:  1,
		},
		{
			name:       "expired status acks",
			message:    validGroupExpiryMessage(t),
			service:    &fakeGroupExpiryService{status: group.ExpireGroupingRuleStatusExpired},
			wantResult: eventbus.HandleResultAck,
			wantCalls:  1,
		},
		{
			name:       "stale task status acks",
			message:    validGroupExpiryMessage(t),
			service:    &fakeGroupExpiryService{status: group.ExpireGroupingRuleStatusStaleTask},
			wantResult: eventbus.HandleResultAck,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewGroupExpiryEventHandler(tt.service, "app.todo.group.expiry.process", newTestLogger())
			got := handler.Handle(context.Background(), tt.message)
			if got != tt.wantResult {
				t.Fatalf("Handle() = %s, want %s", got, tt.wantResult)
			}
			if tt.service.calls != tt.wantCalls {
				t.Fatalf("service calls = %d, want %d", tt.service.calls, tt.wantCalls)
			}
		})
	}
}

type fakeGroupExpiryService struct {
	status group.ExpireGroupingRuleStatus
	err    error
	calls  int
	input  group.ExpireGroupingRuleCommand
}

func (s *fakeGroupExpiryService) ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand) (group.ExpireGroupingRuleStatus, error) {
	s.calls++
	s.input = input
	if s.status == "" {
		s.status = group.ExpireGroupingRuleStatusExpired
	}
	return s.status, s.err
}

func validGroupExpiryMessage(t *testing.T) eventbus.Message {
	t.Helper()

	event := cloudevents.NewEvent()
	event.SetSpecVersion("1.0")
	event.SetType("app.todo.group.expiry.process")
	event.SetSource("group-expiry-scheduler")
	event.SetSubject("task-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))
	if err := event.SetData(cloudevents.ApplicationJSON, map[string]string{
		"task_id":           "task-1",
		"workspace_id":      "workspace-1",
		"group_id":          "group-1",
		"expiration_bucket": "2026-05-10",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return eventbus.Message{Subject: "app.todo.group.expiry.process", Data: data}
}
