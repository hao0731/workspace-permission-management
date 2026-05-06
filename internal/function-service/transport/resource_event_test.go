package transport

import (
	"testing"
	"time"
)

func TestParseResourceUpsertEvent(t *testing.T) {
	eventJSON := []byte(`{
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
	}`)

	got, err := ParseResourceUpsertEvent(eventJSON, "app.todo.resource.upserted")
	if err != nil {
		t.Fatalf("ParseResourceUpsertEvent error = %v, want nil", err)
	}
	if got.ID != "resource-1" || got.DisplayName != "Spec" || got.Type != "document" {
		t.Fatalf("parsed input = %+v, want resource-1/Spec/document", got)
	}
	if got.WorkspaceID != "workspace-1" || got.FunctionKey != "todo" {
		t.Fatalf("parsed scope = %s/%s, want workspace-1/todo", got.WorkspaceID, got.FunctionKey)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "section_1" {
		t.Fatalf("tags = %#v, want [section_1]", got.Tags)
	}
	wantTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	if !got.EventTime.Equal(wantTime) {
		t.Fatalf("EventTime = %s, want %s", got.EventTime, wantTime)
	}
}

func TestParseResourceUpsertEventRejectsWrongType(t *testing.T) {
	eventJSON := []byte(`{
		"specversion":"1.0",
		"type":"wrong.type",
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
	}`)

	if _, err := ParseResourceUpsertEvent(eventJSON, "app.todo.resource.upserted"); err == nil {
		t.Fatal("ParseResourceUpsertEvent error = nil, want error")
	}
}

func TestParseResourceUpsertEventRejectsSubjectMismatch(t *testing.T) {
	eventJSON := []byte(`{
		"specversion":"1.0",
		"type":"app.todo.resource.upserted",
		"source":"todo-service",
		"subject":"different-resource",
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
	}`)

	if _, err := ParseResourceUpsertEvent(eventJSON, "app.todo.resource.upserted"); err == nil {
		t.Fatal("ParseResourceUpsertEvent error = nil, want error")
	}
}
