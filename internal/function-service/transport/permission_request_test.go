package transport

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

func validPermissionRequestJSON() string {
	return `{
		"office_permission": {
			"baseline_rule": {
				"action_id": "view",
				"resource_tags": ["section_1"],
				"enabled": true
			},
			"extra_rules": [
				{
					"rule_id": "rule-1",
					"group_ids": ["group-1"],
					"action_id": "edit",
					"resource_tags": ["section_1"],
					"expiration_date": "2026-06-01T00:00:00Z"
				}
			]
		},
		"remote_permission": {
			"baseline_rule": {
				"action_id": "view",
				"resource_tags": ["remote"],
				"enabled": false
			},
			"extra_rules": []
		}
	}`
}

func TestDecodePermissionSaveRequest(t *testing.T) {
	req, err := DecodePermissionSaveRequest(strings.NewReader(validPermissionRequestJSON()))
	if err != nil {
		t.Fatalf("DecodePermissionSaveRequest error = %v, want nil", err)
	}
	if req.OfficePermission == nil || req.RemotePermission == nil {
		t.Fatal("permissions were not decoded")
	}
	if req.OfficePermission.BaselineRule == nil {
		t.Fatal("office baseline was not decoded")
	}
	if req.OfficePermission.BaselineRule.Enabled == nil || !*req.OfficePermission.BaselineRule.Enabled {
		t.Fatal("office enabled = nil/false, want true")
	}
	if req.RemotePermission.BaselineRule.Enabled == nil || *req.RemotePermission.BaselineRule.Enabled {
		t.Fatal("remote enabled = nil/true, want false")
	}
}

func TestDecodePermissionSaveRequestRejectsInvalidJSON(t *testing.T) {
	if _, err := DecodePermissionSaveRequest(strings.NewReader(`{"office_permission":`)); err == nil {
		t.Fatal("DecodePermissionSaveRequest error = nil, want error")
	}
}

func TestPermissionSaveRequestToDomain(t *testing.T) {
	req, err := DecodePermissionSaveRequest(strings.NewReader(validPermissionRequestJSON()))
	if err != nil {
		t.Fatalf("decode request: %v", err)
	}

	input, err := req.ToDomain("workspace-1", "todo")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.FunctionKey != "todo" {
		t.Fatalf("identity = %s/%s, want workspace-1/todo", input.WorkspaceID, input.FunctionKey)
	}
	if input.OfficePermission == nil || input.RemotePermission == nil {
		t.Fatal("domain permissions = nil, want values")
	}
	if !input.OfficePermission.BaselineRule.Enabled {
		t.Fatal("office enabled = false, want true")
	}
	if input.RemotePermission.BaselineRule.Enabled {
		t.Fatal("remote enabled = true, want false")
	}
	if len(input.OfficePermission.ExtraRules) != 1 {
		t.Fatalf("office extra rules len = %d, want 1", len(input.OfficePermission.ExtraRules))
	}
	if input.OfficePermission.ExtraRules[0].ExpirationDate != time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("expiration = %s, want 2026-06-01T00:00:00Z", input.OfficePermission.ExtraRules[0].ExpirationDate)
	}
}

func TestPermissionSaveRequestToDomainRejectsMissingEnabled(t *testing.T) {
	req := PermissionSaveRequest{
		OfficePermission: &PermissionSectionRequest{
			BaselineRule: &BaselineRuleRequest{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
			},
		},
		RemotePermission: &PermissionSectionRequest{
			BaselineRule: &BaselineRuleRequest{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      boolPtr(false),
			},
		},
	}

	_, err := req.ToDomain("workspace-1", "todo")
	if !errors.Is(err, permission.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestPermissionSaveRequestToDomainRejectsMissingSection(t *testing.T) {
	req := PermissionSaveRequest{}

	_, err := req.ToDomain("workspace-1", "todo")
	if !errors.Is(err, permission.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
