package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	functionconfig "github.com/hao0731/workspace-permission-management/internal/function-service/config"
	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	permissionapi "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
)

type fakeMessagePublisher struct {
	subject string
	data    []byte
	err     error
}

func (f *fakeMessagePublisher) Publish(ctx context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error {
	f.subject = subject
	f.data = append([]byte(nil), data...)
	return f.err
}

func TestLoggerNew(t *testing.T) {
	if sharedlogger.New(environment.Development) == nil {
		t.Fatal("development logger = nil, want logger")
	}
	if sharedlogger.New(environment.Production) == nil {
		t.Fatal("production logger = nil, want logger")
	}
}

func TestProcessIndicator(t *testing.T) {
	indicator := processIndicator{}
	if indicator.Name() != "process" {
		t.Fatalf("Name = %q, want process", indicator.Name())
	}
	if !indicator.IsHealthy(context.Background()) {
		t.Fatal("IsHealthy = false, want true")
	}
}

func TestLoggerNewReturnsSlogLogger(t *testing.T) {
	logger := sharedlogger.New(environment.Development)
	if logger == nil {
		t.Fatal("logger = nil, want slog logger")
	}
}

func TestNewPermissionClientReturnsAPIClient(t *testing.T) {
	client := newPermissionClient(functionconfig.PermissionAPIConfig{
		BaseURL:      "http://localhost:8086",
		APIKey:       "dev-permission-api-key",
		APIKeyHeader: "X-API-Key",
	})
	if _, ok := client.(*permissionapi.Client); !ok {
		t.Fatalf("permission client type = %T, want *api.Client", client)
	}
}

func TestResourceDeletedPublisherPublishesConfiguredSubject(t *testing.T) {
	messagePublisher := &fakeMessagePublisher{}
	publisher := newResourceDeletedPublisher(messagePublisher, "app.todo.resource.deleted")
	eventTime := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)

	err := publisher.PublishResourceDeleted(context.Background(), resource.DeletedEvent{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
		EventID:     "event-1",
		EventTime:   eventTime,
	})
	if err != nil {
		t.Fatalf("PublishResourceDeleted error = %v, want nil", err)
	}
	if messagePublisher.subject != "app.todo.resource.deleted" {
		t.Fatalf("subject = %q, want app.todo.resource.deleted", messagePublisher.subject)
	}

	var event cloudevents.Event
	if err := json.Unmarshal(messagePublisher.data, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event.Type() != "app.todo.resource.deleted" {
		t.Fatalf("event type = %q, want app.todo.resource.deleted", event.Type())
	}
	if event.Subject() != "resource-1" {
		t.Fatalf("event subject = %q, want resource-1", event.Subject())
	}
}

func TestResourceDeletedPublisherReturnsPublishError(t *testing.T) {
	messagePublisher := &fakeMessagePublisher{err: errors.New("publish failed")}
	publisher := newResourceDeletedPublisher(messagePublisher, "app.todo.resource.deleted")

	err := publisher.PublishResourceDeleted(context.Background(), resource.DeletedEvent{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
		EventID:     "event-1",
		EventTime:   time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("PublishResourceDeleted error = nil, want error")
	}
}
