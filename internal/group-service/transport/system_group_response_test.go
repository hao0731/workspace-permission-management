package transport

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func transportSystemGroupModel() group.SystemGroup {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	return group.SystemGroup{
		ID:       "group-1",
		SystemID: "system-a",
		Name:     "System Admins",
		GroupingRules: []group.SystemGroupRule{{
			AttributeKey: group.GroupAttributeOrganization,
			Operator:     group.OperatorEq,
			Multi:        true,
			Value:        []string{"ORG-100", "ORG-200"},
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestNewSystemGroupCreateResponse(t *testing.T) {
	response := NewSystemGroupCreateResponse(transportSystemGroupModel())
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}
	groupBody, ok := body["group"].(map[string]any)
	if !ok {
		t.Fatalf("group = %#v, want object", body["group"])
	}
	if groupBody["name"] != "System Admins" {
		t.Fatalf("name = %v, want System Admins", groupBody["name"])
	}
	if _, ok := groupBody["system_id"]; ok {
		t.Fatal("system_id present, want omitted")
	}
	if groupBody["created_at"] == "" || groupBody["updated_at"] == "" {
		t.Fatalf("timestamps missing in %#v", groupBody)
	}
}

func TestNewSystemGroupCreatePartialResponse(t *testing.T) {
	response := NewSystemGroupCreatePartialResponse(transportSystemGroupModel(), []string{"organization rejected"})
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var body struct {
		Group  map[string]any `json:"group"`
		Errors []string       `json:"errors"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}
	if body.Group["name"] != "System Admins" {
		t.Fatalf("name = %v, want System Admins", body.Group["name"])
	}
	if len(body.Errors) != 1 || body.Errors[0] != "organization rejected" {
		t.Fatalf("errors = %#v, want organization rejected", body.Errors)
	}
}

func TestNewSystemGroupUpdateResponse(t *testing.T) {
	response := NewSystemGroupUpdateResponse(transportSystemGroupModel())
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}
	groupBody, ok := body["group"].(map[string]any)
	if !ok {
		t.Fatalf("group = %#v, want object", body["group"])
	}
	if groupBody["name"] != "System Admins" {
		t.Fatalf("name = %v, want System Admins", groupBody["name"])
	}
	if _, ok := groupBody["system_id"]; ok {
		t.Fatal("system_id present, want omitted")
	}
}

func TestNewSystemGroupUpdatePartialResponse(t *testing.T) {
	response := NewSystemGroupUpdatePartialResponse(transportSystemGroupModel(), []string{"delete rejected"})
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var body struct {
		Group  map[string]any `json:"group"`
		Errors []string       `json:"errors"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}
	if body.Group["id"] != "group-1" {
		t.Fatalf("group id = %v, want group-1", body.Group["id"])
	}
	if len(body.Errors) != 1 || body.Errors[0] != "delete rejected" {
		t.Fatalf("errors = %#v, want delete rejected", body.Errors)
	}
}

func TestNewSystemGroupListResponse(t *testing.T) {
	page := group.SystemGroupPage{
		Groups:      []group.SystemGroup{transportSystemGroupModel()},
		HasNextPage: true,
		NextCursor:  &group.SystemGroupCursor{CreatedAt: time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC), ID: "group-1"},
	}
	response, err := NewSystemGroupListResponse(page)
	if err != nil {
		t.Fatalf("NewSystemGroupListResponse error = %v, want nil", err)
	}
	if len(response.Groups) != 1 || response.Groups[0].ID != "group-1" {
		t.Fatalf("Groups = %+v, want group-1", response.Groups)
	}
	if !response.PageInfo.HasNextPage || response.PageInfo.NextToken == "" {
		t.Fatalf("PageInfo = %+v, want next token", response.PageInfo)
	}
}
