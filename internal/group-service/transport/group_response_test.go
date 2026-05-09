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

func TestNewGroupGetResponseFound(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	model := group.Group{
		ID:          "group-1",
		WorkspaceID: "workspace-1",
		Name:        "Design Reviewers",
		Description: "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: "department",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: expiration,
		},
	}

	response := NewGroupGetResponse(&model)
	if response.Group == nil {
		t.Fatal("Group = nil, want group")
	}
	if response.Group.ID != "group-1" {
		t.Fatalf("ID = %q, want group-1", response.Group.ID)
	}
}

func TestNewGroupGetResponseMissing(t *testing.T) {
	response := NewGroupGetResponse(nil)
	if response.Group != nil {
		t.Fatalf("Group = %+v, want nil", response.Group)
	}
}

func TestNewIndividualMemberListResponse(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	page := group.IndividualMemberPage{
		Members: []group.IndividualMember{{
			ID:             "member-1",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: expiration,
		}},
		HasNextPage: true,
		NextCursor: &group.IndividualMemberCursor{
			CreatedAt: time.Date(2026, 5, 9, 7, 31, 0, 0, time.UTC),
			ID:        "member-1",
		},
	}

	response, err := NewIndividualMemberListResponse(page)
	if err != nil {
		t.Fatalf("response error = %v, want nil", err)
	}
	if len(response.Members) != 1 || response.Members[0].NTAccount != "user1" {
		t.Fatalf("Members = %+v, want user1", response.Members)
	}
	if !response.PageInfo.HasNextPage || response.PageInfo.NextToken == "" {
		t.Fatalf("PageInfo = %+v, want next page token", response.PageInfo)
	}
}
