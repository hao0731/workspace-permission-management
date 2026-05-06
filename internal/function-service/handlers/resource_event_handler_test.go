package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type fakeEventResourceService struct {
	input resource.UpsertInput
	err   error
}

func (f *fakeEventResourceService) UpsertResource(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	f.input = input
	if f.err != nil {
		return "", f.err
	}
	return resource.UpsertStatusInserted, nil
}

func TestResourceEventHandlerAck(t *testing.T) {
	service := &fakeEventResourceService{}
	handler := NewResourceEventHandler(service, "app.todo.resource.upserted", nil)

	result := handler.Handle(context.Background(), eventbus.Message{
		Subject: "app.todo.resource.upserted",
		Data: []byte(`{
			"specversion":"1.0",
			"type":"app.todo.resource.upserted",
			"source":"todo-service",
			"subject":"resource-1",
			"id":"event-1",
			"time":"2026-05-05T07:31:00Z",
			"datacontenttype":"application/json",
			"data":{
				"resource_id":"resource-1",
				"display_name":"Spec",
				"resource_type":"document",
				"resource_tags":["section_1"],
				"function_key":"todo",
				"workspace_id":"workspace-1"
			}
		}`),
	})

	if result != eventbus.HandleResultAck {
		t.Fatalf("result = %q, want ack", result)
	}
	if service.input.ID != "resource-1" {
		t.Fatalf("service input ID = %q, want resource-1", service.input.ID)
	}
}

func TestResourceEventHandlerTerminatesPoisonMessage(t *testing.T) {
	handler := NewResourceEventHandler(&fakeEventResourceService{}, "app.todo.resource.upserted", nil)

	result := handler.Handle(context.Background(), eventbus.Message{
		Subject: "app.todo.resource.upserted",
		Data:    []byte(`{"bad":`),
	})

	if result != eventbus.HandleResultTerminate {
		t.Fatalf("result = %q, want terminate", result)
	}
}

func TestResourceEventHandlerRetriesTransientServiceError(t *testing.T) {
	service := &fakeEventResourceService{err: ErrRetryableEvent}
	handler := NewResourceEventHandler(service, "app.todo.resource.upserted", nil)

	result := handler.Handle(context.Background(), eventbus.Message{
		Subject: "app.todo.resource.upserted",
		Data: []byte(`{
			"specversion":"1.0",
			"type":"app.todo.resource.upserted",
			"source":"todo-service",
			"subject":"resource-1",
			"id":"event-1",
			"time":"2026-05-05T07:31:00Z",
			"datacontenttype":"application/json",
			"data":{
				"resource_id":"resource-1",
				"display_name":"Spec",
				"resource_type":"document",
				"resource_tags":["section_1"],
				"function_key":"todo",
				"workspace_id":"workspace-1"
			}
		}`),
	})

	if result != eventbus.HandleResultRetry {
		t.Fatalf("result = %q, want retry", result)
	}
}

func TestResourceEventHandlerTerminatesInvalidServiceInput(t *testing.T) {
	service := &fakeEventResourceService{err: resource.ErrInvalidInput}
	handler := NewResourceEventHandler(service, "app.todo.resource.upserted", nil)

	result := handler.Handle(context.Background(), eventbus.Message{
		Subject: "app.todo.resource.upserted",
		Data: []byte(`{
			"specversion":"1.0",
			"type":"app.todo.resource.upserted",
			"source":"todo-service",
			"subject":"resource-1",
			"id":"event-1",
			"time":"2026-05-05T07:31:00Z",
			"datacontenttype":"application/json",
			"data":{
				"resource_id":"resource-1",
				"display_name":"Spec",
				"resource_type":"document",
				"resource_tags":["section_1"],
				"function_key":"todo",
				"workspace_id":"workspace-1"
			}
		}`),
	})

	if result != eventbus.HandleResultTerminate {
		t.Fatalf("result = %q, want terminate", result)
	}
}

func TestIsRetryableEventError(t *testing.T) {
	if !errors.Is(ErrRetryableEvent, ErrRetryableEvent) {
		t.Fatal("ErrRetryableEvent must compare with errors.Is")
	}
}
