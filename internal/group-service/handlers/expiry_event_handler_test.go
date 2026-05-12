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

type expiryHandlerCaseKind string

const (
	expiryHandlerInvalidEvent  expiryHandlerCaseKind = "invalid_event"
	expiryHandlerInvalidInput  expiryHandlerCaseKind = "invalid_input"
	expiryHandlerServiceError  expiryHandlerCaseKind = "service_error"
	expiryHandlerExpiredStatus expiryHandlerCaseKind = "expired_status"
	expiryHandlerStaleStatus   expiryHandlerCaseKind = "stale_status"
)

type expiryHandlerSuite struct {
	name         string
	validMessage func(*testing.T) eventbus.Message
	newHandler   func(expiryHandlerCaseKind) (eventbus.Handler, func() int)
}

func TestExpiryEventHandlersHandle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		kind       expiryHandlerCaseKind
		wantResult eventbus.HandleResult
		wantCalls  int
	}{
		{name: "invalid event terminates", kind: expiryHandlerInvalidEvent, wantResult: eventbus.HandleResultTerminate},
		{name: "service invalid input terminates", kind: expiryHandlerInvalidInput, wantResult: eventbus.HandleResultTerminate, wantCalls: 1},
		{name: "service failure retries", kind: expiryHandlerServiceError, wantResult: eventbus.HandleResultRetry, wantCalls: 1},
		{name: "expired status acks", kind: expiryHandlerExpiredStatus, wantResult: eventbus.HandleResultAck, wantCalls: 1},
		{name: "stale task status acks", kind: expiryHandlerStaleStatus, wantResult: eventbus.HandleResultAck, wantCalls: 1},
	}

	for _, suite := range expiryHandlerSuites() {
		t.Run(suite.name, func(t *testing.T) {
			for _, tt := range cases {
				t.Run(tt.name, func(t *testing.T) {
					handler, calls := suite.newHandler(tt.kind)
					message := suite.validMessage(t)
					if tt.kind == expiryHandlerInvalidEvent {
						message.Data = []byte("{")
					}

					got := handler.Handle(context.Background(), message)
					if got != tt.wantResult {
						t.Fatalf("Handle() = %s, want %s", got, tt.wantResult)
					}
					if calls() != tt.wantCalls {
						t.Fatalf("service calls = %d, want %d", calls(), tt.wantCalls)
					}
				})
			}
		})
	}
}

func expiryHandlerSuites() []expiryHandlerSuite {
	return []expiryHandlerSuite{
		{
			name:         "group expiry handler",
			validMessage: validGroupExpiryMessage,
			newHandler:   newGroupExpiryHandlerForCase,
		},
		{
			name:         "individual member expiry handler",
			validMessage: validIndividualMemberExpiryMessage,
			newHandler:   newIndividualMemberExpiryHandlerForCase,
		},
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

type fakeIndividualMemberExpiryService struct {
	status group.ExpireIndividualMemberStatus
	err    error
	calls  int
	input  group.ExpireIndividualMemberCommand
}

func (s *fakeIndividualMemberExpiryService) ExpireIndividualMember(ctx context.Context, input group.ExpireIndividualMemberCommand) (group.ExpireIndividualMemberStatus, error) {
	s.calls++
	s.input = input
	if s.status == "" {
		s.status = group.ExpireIndividualMemberStatusExpired
	}
	return s.status, s.err
}

func newGroupExpiryHandlerForCase(kind expiryHandlerCaseKind) (eventbus.Handler, func() int) {
	service := &fakeGroupExpiryService{}
	switch kind {
	case expiryHandlerInvalidInput:
		service.err = group.ErrInvalidInput
	case expiryHandlerServiceError:
		service.err = errors.New("database unavailable")
	case expiryHandlerStaleStatus:
		service.status = group.ExpireGroupingRuleStatusStaleTask
	default:
		service.status = group.ExpireGroupingRuleStatusExpired
	}
	return NewGroupExpiryEventHandler(service, "app.todo.group.expiry.process", newTestLogger()), func() int {
		return service.calls
	}
}

func newIndividualMemberExpiryHandlerForCase(kind expiryHandlerCaseKind) (eventbus.Handler, func() int) {
	service := &fakeIndividualMemberExpiryService{}
	switch kind {
	case expiryHandlerInvalidInput:
		service.err = group.ErrInvalidInput
	case expiryHandlerServiceError:
		service.err = errors.New("database unavailable")
	case expiryHandlerStaleStatus:
		service.status = group.ExpireIndividualMemberStatusStaleTask
	default:
		service.status = group.ExpireIndividualMemberStatusExpired
	}
	return NewIndividualMemberExpiryEventHandler(service, "app.todo.group.individual-member.expiry.process", newTestLogger()), func() int {
		return service.calls
	}
}

func validGroupExpiryMessage(t *testing.T) eventbus.Message {
	t.Helper()

	return validExpiryMessage(t, "app.todo.group.expiry.process", "group-expiry-scheduler", map[string]string{
		"task_id":           "task-1",
		"workspace_id":      "workspace-1",
		"group_id":          "group-1",
		"expiration_bucket": "2026-05-10",
	})
}

func validIndividualMemberExpiryMessage(t *testing.T) eventbus.Message {
	t.Helper()

	return validExpiryMessage(t, "app.todo.group.individual-member.expiry.process", "individual-member-expiry-scheduler", map[string]string{
		"task_id":           "task-1",
		"group_id":          "group-1",
		"nt_account":        "user1",
		"expiration_bucket": "2026-05-10",
	})
}

func validExpiryMessage(t *testing.T, eventType string, source string, payload map[string]string) eventbus.Message {
	t.Helper()

	event := cloudevents.NewEvent()
	event.SetSpecVersion("1.0")
	event.SetType(eventType)
	event.SetSource(source)
	event.SetSubject("task-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))
	if err := event.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return eventbus.Message{Subject: eventType, Data: data}
}
