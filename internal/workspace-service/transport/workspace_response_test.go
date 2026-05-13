package transport

import (
	"encoding/json"
	"testing"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
)

func TestNewWorkspaceCreateResponse(t *testing.T) {
	response := NewWorkspaceCreateResponse(workspace.Workspace{
		ID:             "workspace-1",
		Name:           "Planning",
		Description:    "Planning workspace",
		OwnerNTAccount: "user1",
	}, domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"})
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	workspaceBody, ok := body["workspace"].(map[string]any)
	if !ok {
		t.Fatalf("workspace body type = %T", body["workspace"])
	}
	if workspaceBody["id"] != "workspace-1" || workspaceBody["name"] != "Planning" {
		t.Fatalf("workspace = %#v", workspaceBody)
	}
	ownerBody, ok := workspaceBody["owner"].(map[string]any)
	if !ok {
		t.Fatalf("owner body type = %T", workspaceBody["owner"])
	}
	if ownerBody["nt_account"] != "user1" || ownerBody["display_name"] != "Test User 測試員" {
		t.Fatalf("owner = %#v", ownerBody)
	}
	if _, ok := workspaceBody["owner_nt_account"]; ok {
		t.Fatal("owner_nt_account is present, want omitted")
	}
	if _, ok := workspaceBody["created_at"]; ok {
		t.Fatal("created_at is present, want omitted")
	}
	if _, ok := workspaceBody["updated_at"]; ok {
		t.Fatal("updated_at is present, want omitted")
	}
}
