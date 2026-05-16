package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

type fakeMessagePublisher struct {
	subject string
	data    []byte
	err     error
	opts    []eventbus.PublishOption
}

func (f *fakeMessagePublisher) Publish(_ context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error {
	f.subject = subject
	f.data = append([]byte(nil), data...)
	f.opts = append([]eventbus.PublishOption(nil), opts...)
	return f.err
}

func TestExpiryCommandPublisherPublishesGroupCommand(t *testing.T) {
	fake := &fakeMessagePublisher{}
	publisher := newExpiryCommandPublisher(fake, "app.todo.group.expiry.process", "app.todo.group.individual-member.expiry.process",
		withPublisherClock(func() time.Time { return time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC) }),
		withPublisherIDGenerator(func() string { return "event-1" }),
	)

	err := publisher.PublishGroupExpiryCommand(context.Background(), expiry.GroupTask{ID: "task-1", WorkspaceID: "workspace-1", GroupID: "group-1", ExpirationBucket: "2026-05-16"})
	if err != nil {
		t.Fatalf("PublishGroupExpiryCommand error = %v, want nil", err)
	}
	if fake.subject != "app.todo.group.expiry.process" {
		t.Fatalf("subject = %q, want group subject", fake.subject)
	}
	event := parsePublisherEvent(t, fake.data)
	if event.ID() != "event-1" || event.Subject() != "task-1" {
		t.Fatalf("event id/subject = %q/%q", event.ID(), event.Subject())
	}
}

func TestExpiryCommandPublisherPublishesIndividualMemberCommand(t *testing.T) {
	fake := &fakeMessagePublisher{}
	publisher := newExpiryCommandPublisher(fake, "app.todo.group.expiry.process", "app.todo.group.individual-member.expiry.process",
		withPublisherClock(func() time.Time { return time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC) }),
		withPublisherIDGenerator(func() string { return "event-1" }),
	)

	err := publisher.PublishIndividualMemberExpiryCommand(context.Background(), expiry.IndividualMemberTask{ID: "task-1", GroupID: "group-1", NTAccount: "user1", ExpirationBucket: "2026-05-16"})
	if err != nil {
		t.Fatalf("PublishIndividualMemberExpiryCommand error = %v, want nil", err)
	}
	if fake.subject != "app.todo.group.individual-member.expiry.process" {
		t.Fatalf("subject = %q, want member subject", fake.subject)
	}
}

func TestExpiryCommandPublisherReturnsPublishError(t *testing.T) {
	fake := &fakeMessagePublisher{err: errors.New("nats unavailable")}
	publisher := newExpiryCommandPublisher(fake, "group.subject", "member.subject")

	err := publisher.PublishGroupExpiryCommand(context.Background(), expiry.GroupTask{ID: "task-1"})
	if err == nil {
		t.Fatal("PublishGroupExpiryCommand error = nil, want error")
	}
}

func TestExpiryCommandPublisherAppliesPublishOptions(t *testing.T) {
	fake := &fakeMessagePublisher{}
	publisher := newExpiryCommandPublisher(
		fake,
		"group.subject",
		"member.subject",
		withPublisherPublishOptions(eventbus.WithPublishTimeout(time.Second)),
	)

	err := publisher.PublishIndividualMemberExpiryCommand(context.Background(), expiry.IndividualMemberTask{ID: "task-1"})
	if err != nil {
		t.Fatalf("PublishIndividualMemberExpiryCommand error = %v, want nil", err)
	}
	if len(fake.opts) != 1 {
		t.Fatalf("publish options len = %d, want 1", len(fake.opts))
	}
}

func parsePublisherEvent(t *testing.T, data []byte) cloudevents.Event {
	t.Helper()
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}
	return event
}
