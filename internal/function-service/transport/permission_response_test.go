package transport

import (
	"encoding/json"
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
