package transport

import (
	"errors"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func TestDecodeSystemGroupCreateRequestToDomain(t *testing.T) {
	request, err := DecodeSystemGroupCreateRequest(strings.NewReader(`{
		"name": " System Admins ",
		"grouping_rules": [
			{"attribute_key": "organization", "operator": "eq", "multi": true, "value": [" ORG-100 ", "ORG-200"]},
			{"attribute_key": "job_type", "operator": "eq", "multi": false, "value": " DL "},
			{"attribute_key": "job_level", "operator": "eq", "multi": false, "value": " M2 "},
			{"attribute_key": "job_tag", "operator": "eq", "multi": true, "value": ["a4_reviewer", "_internal_secretary_"]}
		]
	}`))
	if err != nil {
		t.Fatalf("DecodeSystemGroupCreateRequest error = %v, want nil", err)
	}
	input, err := request.ToDomain("system-a")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.SystemID != "system-a" || input.Name != " System Admins " {
		t.Fatalf("input identity/name = %+v, want original request values", input)
	}
	if len(input.GroupingRules) != 4 {
		t.Fatalf("rules len = %d, want 4", len(input.GroupingRules))
	}
	if input.GroupingRules[0].AttributeKey != group.GroupAttributeOrganization {
		t.Fatalf("first attribute = %q, want organization", input.GroupingRules[0].AttributeKey)
	}
	if values, ok := input.GroupingRules[0].Value.([]string); !ok || values[0] != " ORG-100 " {
		t.Fatalf("organization values = %#v, want string slice preserving transport value", input.GroupingRules[0].Value)
	}
}

func TestDecodeSystemGroupCreateRequestRejectsMalformedJSON(t *testing.T) {
	_, err := DecodeSystemGroupCreateRequest(strings.NewReader(`{"name":`))
	if err == nil {
		t.Fatal("DecodeSystemGroupCreateRequest error = nil, want error")
	}
}

func TestSystemGroupCreateRequestToDomainRejectsMissingGroupingRules(t *testing.T) {
	request := SystemGroupCreateRequest{Name: "System Admins"}
	_, err := request.ToDomain("system-a")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}

func TestSystemGroupCreateRequestToDomainRejectsMissingMulti(t *testing.T) {
	request := SystemGroupCreateRequest{
		Name: "System Admins",
		GroupingRules: []SystemGroupRuleRequest{{
			AttributeKey: "organization",
			Operator:     "eq",
			Value:        []string{"ORG-100"},
		}},
	}
	_, err := request.ToDomain("system-a")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}
