package transport

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func TestDecodeGroupCreateRequestToDomain(t *testing.T) {
	body := strings.NewReader(`{
		"name": " Design Reviewers ",
		"description": "Employees who can review design documents.",
		"grouping_rule": {
			"rules": [
				{
					"attribute_key": " department ",
					"operator": "eq",
					"multi": false,
					"value": "ABCD-123"
				},
				{
					"attribute_key": "level",
					"operator": "gte",
					"multi": true,
					"value": [5, 6]
				}
			],
			"expiration_date": "2026-06-01T00:00:00Z"
		},
		"individual_members": [
			{
				"nt_account": " user1 ",
				"expiration_date": "2026-06-02T00:00:00Z"
			}
		]
	}`)

	request, err := DecodeGroupCreateRequest(body)
	if err != nil {
		t.Fatalf("DecodeGroupCreateRequest error = %v, want nil", err)
	}
	input, err := request.ToDomain("workspace-1")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}

	if input.WorkspaceID != "workspace-1" {
		t.Fatalf("WorkspaceID = %q, want workspace-1", input.WorkspaceID)
	}
	if input.Name != " Design Reviewers " {
		t.Fatalf("Name = %q, want original request value", input.Name)
	}
	if len(input.GroupingRule.Rules) != 2 {
		t.Fatalf("rules len = %d, want 2", len(input.GroupingRule.Rules))
	}
	if input.GroupingRule.Rules[0].Operator != group.OperatorEq {
		t.Fatalf("operator = %q, want eq", input.GroupingRule.Rules[0].Operator)
	}
	if input.GroupingRule.Rules[1].Multi != true {
		t.Fatal("second rule Multi = false, want true")
	}
	values, ok := input.GroupingRule.Rules[1].Value.([]any)
	if !ok || len(values) != 2 {
		t.Fatalf("second rule value = %#v, want two JSON array items", input.GroupingRule.Rules[1].Value)
	}
	wantGroupingExpiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !input.GroupingRule.ExpirationDate.Equal(wantGroupingExpiration) {
		t.Fatalf("grouping expiration = %s, want %s", input.GroupingRule.ExpirationDate, wantGroupingExpiration)
	}
	wantMemberExpiration := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	if !input.IndividualMembers[0].ExpirationDate.Equal(wantMemberExpiration) {
		t.Fatalf("member expiration = %s, want %s", input.IndividualMembers[0].ExpirationDate, wantMemberExpiration)
	}
}

func TestDecodeGroupCreateRequestRejectsMalformedJSON(t *testing.T) {
	_, err := DecodeGroupCreateRequest(strings.NewReader(`{"name":`))
	if err == nil {
		t.Fatal("DecodeGroupCreateRequest error = nil, want error")
	}
}

func TestGroupCreateRequestToDomainRejectsMissingGroupingRule(t *testing.T) {
	request := GroupCreateRequest{Name: "Design Reviewers"}

	_, err := request.ToDomain("workspace-1")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}

func TestGroupCreateRequestToDomainRejectsMissingRuleMulti(t *testing.T) {
	request := GroupCreateRequest{
		Name: "Design Reviewers",
		GroupingRule: &GroupingRuleRequest{
			Rules: []RuleRequest{{
				AttributeKey: "department",
				Operator:     "eq",
				Value:        "ABCD-123",
			}},
			ExpirationDate: JSONTime{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		},
	}

	_, err := request.ToDomain("workspace-1")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}

func TestGroupGroupingRulesRequestToDomain(t *testing.T) {
	multi := false
	request := GroupGroupingRulesRequest{
		Rules: []RuleRequest{{
			AttributeKey: "department",
			Operator:     "eq",
			Multi:        &multi,
			Value:        "ABCD-123",
		}},
		ExpirationDate: JSONTime{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
	}

	input, err := request.ToDomain("workspace-1", "group-1")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" {
		t.Fatalf("identity = %q/%q, want workspace-1/group-1", input.WorkspaceID, input.GroupID)
	}
	if len(input.Rules) != 1 || input.Rules[0].Operator != group.OperatorEq {
		t.Fatalf("rules = %+v, want one eq rule", input.Rules)
	}
}

func TestGroupGroupingRulesRequestRejectsMissingMulti(t *testing.T) {
	request := GroupGroupingRulesRequest{
		Rules: []RuleRequest{{
			AttributeKey: "department",
			Operator:     "eq",
			Value:        "ABCD-123",
		}},
		ExpirationDate: JSONTime{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
	}

	_, err := request.ToDomain("workspace-1", "group-1")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}

func TestJSONTimeRejectsInvalidTimestamp(t *testing.T) {
	_, err := DecodeGroupCreateRequest(strings.NewReader(`{
		"name": "Design Reviewers",
		"grouping_rule": {
			"rules": [],
			"expiration_date": "not-a-time"
		},
		"individual_members": []
	}`))
	if err == nil {
		t.Fatal("DecodeGroupCreateRequest error = nil, want error")
	}
}
