package transport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

func TestNewPermissionSaveResponse(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	response := NewPermissionSaveResponse(permission.Permission{
		ID:          "permission-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		OfficePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []permission.ExtraRule{{
				RuleID:         "rule-1",
				GroupIDs:       []string{"group-1"},
				ActionID:       "edit",
				ResourceTags:   []string{"section_1"},
				ExpirationDate: expiration,
			}},
		},
		RemotePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	})

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := body["permissions"]; !ok {
		t.Fatal("response missing permissions key")
	}
	if response.Permissions.OfficePermission.BaselineRule.Enabled != true {
		t.Fatal("office enabled = false, want true")
	}
	if response.Permissions.RemotePermission.BaselineRule.Enabled != false {
		t.Fatal("remote enabled = true, want false")
	}
	if got := response.Permissions.OfficePermission.ExtraRules[0].RuleID; got != "rule-1" {
		t.Fatalf("rule_id = %q, want rule-1", got)
	}
}

func TestNewPermissionGetResponse(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	response := NewPermissionGetResponse(permission.Permission{
		ID:          "permission-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		CreatedAt:   time.Date(2026, 5, 9, 1, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 9, 2, 0, 0, 0, time.UTC),
		OfficePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []permission.ExtraRule{{
				RuleID:         "rule-1",
				GroupIDs:       []string{"group-1"},
				ActionID:       "edit",
				ResourceTags:   []string{"section_1"},
				ExpirationDate: expiration,
			}},
		},
		RemotePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	})

	if response.Permissions == nil {
		t.Fatal("permissions = nil, want response object")
	}
	if response.Permissions.OfficePermission.ExtraRules[0].RuleID != "rule-1" {
		t.Fatalf("rule_id = %q, want rule-1", response.Permissions.OfficePermission.ExtraRules[0].RuleID)
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(data), "created_at") || strings.Contains(string(data), "updated_at") {
		t.Fatalf("response exposes persistence timestamps: %s", data)
	}
}

func TestNewPermissionGetNotFoundResponse(t *testing.T) {
	response := NewPermissionGetNotFoundResponse()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	got := string(data)
	want := `{"permissions":null}`
	if got != want {
		t.Fatalf("json = %s, want %s", got, want)
	}
}
