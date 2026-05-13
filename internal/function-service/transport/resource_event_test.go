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

	got, err := ParseResourceUpsertEvent(eventJSON)
	if err != nil {
		t.Fatalf("ParseResourceUpsertEvent error = %v, want nil", err)
	}
	if got.ResourceID != "resource-1" || got.DisplayName != "Spec" || got.ResourceType != "document" {
		t.Fatalf("parsed event = %+v, want resource-1/Spec/document", got)
	}
	if got.WorkspaceID != "workspace-1" || got.FunctionKey != "todo" {
		t.Fatalf("parsed scope = %s/%s, want workspace-1/todo", got.WorkspaceID, got.FunctionKey)
	}
	if got.EventID != "event-1" {
		t.Fatalf("EventID = %q, want event-1", got.EventID)
	}
	if len(got.ResourceTags) != 1 || got.ResourceTags[0] != "section_1" {
		t.Fatalf("tags = %#v, want [section_1]", got.ResourceTags)
	}
	wantTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	if !got.EventTime.Equal(wantTime) {
		t.Fatalf("EventTime = %s, want %s", got.EventTime, wantTime)
	}
}

func TestParseResourceUpsertEventAcceptsSubjectPattern(t *testing.T) {
	eventJSON := []byte(`{
		"specversion":"1.0",
		"type":"app.documents.resource.upserted",
		"source":"documents-service",
		"subject":"resource-1",
		"id":"event-1",
		"time":"2026-05-05T07:31:00Z",
		"datacontenttype":"application/json",
		"data":{
			"resource_id":"resource-1",
			"display_name":"Spec",
			"resource_type":"document",
			"resource_tags":["section_1"],
			"function_key":"documents",
			"workspace_id":"workspace-1"
		}
	}`)

	got, err := ParseResourceUpsertEvent(eventJSON)
	if err != nil {
		t.Fatalf("ParseResourceUpsertEvent error = %v, want nil", err)
	}
	if got.FunctionKey != "documents" {
		t.Fatalf("FunctionKey = %q, want documents", got.FunctionKey)
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

	if _, err := ParseResourceUpsertEvent(eventJSON); err == nil {
		t.Fatal("ParseResourceUpsertEvent error = nil, want error")
	}
}

func TestParseResourceUpsertEventRejectsWildcardType(t *testing.T) {
	eventJSON := []byte(`{
		"specversion":"1.0",
		"type":"app.*.resource.upserted",
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

	if _, err := ParseResourceUpsertEvent(eventJSON); err == nil {
		t.Fatal("ParseResourceUpsertEvent error = nil, want error")
	}
}

func TestParseResourceUpsertEventRejectsFunctionKeyMismatch(t *testing.T) {
	eventJSON := []byte(`{
		"specversion":"1.0",
		"type":"app.documents.resource.upserted",
		"source":"documents-service",
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

	if _, err := ParseResourceUpsertEvent(eventJSON); err == nil {
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

	if _, err := ParseResourceUpsertEvent(eventJSON); err == nil {
		t.Fatal("ParseResourceUpsertEvent error = nil, want error")
	}
}
