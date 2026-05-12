package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/mockfunction"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type fakeResourceCreateService struct {
	err     error
	command mockfunction.ResourceCreateCommand
}

func (f *fakeResourceCreateService) HandleResourceCreate(_ context.Context, command mockfunction.ResourceCreateCommand) error {
	f.command = command
	return f.err
}

func TestResourceCreateEventHandlerAck(t *testing.T) {
	service := &fakeResourceCreateService{}
	handler := NewResourceCreateEventHandler(service, map[string]string{"cmd.app.documents.resource.create": "documents"}, slog.Default())

	result := handler.Handle(context.Background(), eventbus.Message{
		Subject: "cmd.app.documents.resource.create",
		Data:    validCreateEventData(t),
	})

	if result != eventbus.HandleResultAck {
		t.Fatalf("result = %q, want ack", result)
	}
	if service.command.AppName != "documents" {
		t.Fatalf("command = %+v", service.command)
	}
}

func TestResourceCreateEventHandlerTerminatesMalformedMessage(t *testing.T) {
	handler := NewResourceCreateEventHandler(&fakeResourceCreateService{}, map[string]string{"cmd.app.documents.resource.create": "documents"}, slog.Default())
	result := handler.Handle(context.Background(), eventbus.Message{Subject: "cmd.app.documents.resource.create", Data: []byte(`{`)})
	if result != eventbus.HandleResultTerminate {
		t.Fatalf("result = %q, want terminate", result)
	}
}

func TestResourceCreateEventHandlerTerminatesUnknownSubject(t *testing.T) {
	handler := NewResourceCreateEventHandler(&fakeResourceCreateService{}, map[string]string{"cmd.app.documents.resource.create": "documents"}, slog.Default())
	result := handler.Handle(context.Background(), eventbus.Message{Subject: "cmd.app.unknown.resource.create", Data: validCreateEventData(t)})
	if result != eventbus.HandleResultTerminate {
		t.Fatalf("result = %q, want terminate", result)
	}
}

func TestResourceCreateEventHandlerRetriesPublisherFailure(t *testing.T) {
	service := &fakeResourceCreateService{err: errors.New("publish failed")}
	handler := NewResourceCreateEventHandler(service, map[string]string{"cmd.app.documents.resource.create": "documents"}, slog.Default())
	result := handler.Handle(context.Background(), eventbus.Message{Subject: "cmd.app.documents.resource.create", Data: validCreateEventData(t)})
	if result != eventbus.HandleResultRetry {
		t.Fatalf("result = %q, want retry", result)
	}
}

func TestResourceCreateEventHandlerTerminatesInvalidServiceInput(t *testing.T) {
	service := &fakeResourceCreateService{err: mockfunction.ErrInvalidInput}
	handler := NewResourceCreateEventHandler(service, map[string]string{"cmd.app.documents.resource.create": "documents"}, slog.Default())
	result := handler.Handle(context.Background(), eventbus.Message{Subject: "cmd.app.documents.resource.create", Data: validCreateEventData(t)})
	if result != eventbus.HandleResultTerminate {
		t.Fatalf("result = %q, want terminate", result)
	}
}

func validCreateEventData(t *testing.T) []byte {
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
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return data
}
