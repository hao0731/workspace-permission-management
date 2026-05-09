package transport

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func TestNewGroupCreateResponse(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	model := group.Group{
		ID:             "group-1",
		WorkspaceID:    "workspace-1",
		Name:           "Design Reviewers",
		NormalizedName: "Design Reviewers",
		Description:    "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: "department",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: expiration,
		},
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-1",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: expiration,
		}},
		CreatedAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	}

	response := NewGroupCreateResponse(model)
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
		t.Fatalf("group body = %#v, want object", body["group"])
	}
	if groupBody["id"] != "group-1" {
		t.Fatalf("id = %v, want group-1", groupBody["id"])
	}
	if _, ok := groupBody["workspace_id"]; ok {
		t.Fatal("workspace_id is present, want omitted")
	}
	if _, ok := groupBody["created_at"]; ok {
		t.Fatal("created_at is present, want omitted")
	}
	if groupBody["name"] != "Design Reviewers" {
		t.Fatalf("name = %v, want Design Reviewers", groupBody["name"])
	}
}
