package group

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func systemGroupNow() time.Time {
	return time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
}

func validSystemGroupCreateInput() SystemGroupCreateInput {
	return SystemGroupCreateInput{
		SystemID: " system-a ",
		Name:     " System Admins ",
		GroupingRules: []SystemGroupRule{
			{AttributeKey: GroupAttributeOrganization, Operator: OperatorEq, Multi: true, Value: []string{" ORG-200 ", "ORG-100", "ORG-100"}},
			{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: false, Value: " DL "},
			{AttributeKey: GroupAttributeJobLevel, Operator: OperatorEq, Multi: false, Value: " M2 "},
			{AttributeKey: GroupAttributeJobTag, Operator: OperatorEq, Multi: true, Value: []string{"a4_reviewer", "_internal_secretary_"}},
		},
	}
}

func requireSystemGroupInvalidInput(t *testing.T, err error, contains string) {
	t.Helper()
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if contains != "" && !strings.Contains(err.Error(), contains) {
		t.Fatalf("error = %q, want containing %q", err.Error(), contains)
	}
}

func TestSystemGroupCreateInputNormalize(t *testing.T) {
	input := validSystemGroupCreateInput().Normalize()

	if input.SystemID != "system-a" {
		t.Fatalf("SystemID = %q, want system-a", input.SystemID)
	}
	if input.Name != "System Admins" {
		t.Fatalf("Name = %q, want System Admins", input.Name)
	}
	if input.GroupingRules[0].Value.([]string)[0] != "ORG-200" {
		t.Fatalf("first org value = %q, want ORG-200", input.GroupingRules[0].Value.([]string)[0])
	}
	if input.GroupingRules[1].Value.(string) != "DL" {
		t.Fatalf("job type = %q, want DL", input.GroupingRules[1].Value.(string))
	}
}

func TestSystemGroupCreateInputValidateAcceptsValidAndEmptyRules(t *testing.T) {
	if err := validSystemGroupCreateInput().Normalize().Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	emptyRules := SystemGroupCreateInput{SystemID: "system-a", Name: "Everyone", GroupingRules: []SystemGroupRule{}}
	if err := emptyRules.Normalize().Validate(); err != nil {
		t.Fatalf("Validate empty rules error = %v, want nil", err)
	}
}

func TestSystemGroupCreateInputValidateRejectsInvalidIdentityAndName(t *testing.T) {
	tests := []struct {
		name   string
		input  SystemGroupCreateInput
		reason string
	}{
		{name: "empty system id", input: SystemGroupCreateInput{SystemID: " ", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "system id is required"},
		{name: "system id has whitespace", input: SystemGroupCreateInput{SystemID: "system a", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "system id must be a single subject token"},
		{name: "system id has dot", input: SystemGroupCreateInput{SystemID: "system.a", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "system id must be a single subject token"},
		{name: "empty name", input: SystemGroupCreateInput{SystemID: "system-a", Name: " ", GroupingRules: []SystemGroupRule{}}, reason: "name is required"},
		{name: "nil grouping rules", input: SystemGroupCreateInput{SystemID: "system-a", Name: "Group", GroupingRules: nil}, reason: "grouping_rules is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireSystemGroupInvalidInput(t, tt.input.Normalize().Validate(), tt.reason)
		})
	}
}

func TestSystemGroupCreateInputValidateRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name   string
		rules  []SystemGroupRule
		reason string
	}{
		{
			name:   "not eq rejected",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeOrganization, Operator: OperatorNotEq, Multi: true, Value: []string{"ORG-100"}}},
			reason: "system group rule operator must be eq",
		},
		{
			name:   "unknown attribute",
			rules:  []SystemGroupRule{{AttributeKey: "department", Operator: OperatorEq, Multi: false, Value: "D100"}},
			reason: "system group rule attribute_key is invalid",
		},
		{
			name:   "organization must be multi",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeOrganization, Operator: OperatorEq, Multi: false, Value: "ORG-100"}},
			reason: "organization rule must be multi",
		},
		{
			name:   "job type must be single",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: true, Value: []string{"DL"}}},
			reason: "job_type rule must not be multi",
		},
		{
			name:   "invalid job type value",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: false, Value: "CONTRACTOR"}},
			reason: "job_type value must be DL, IDL, or ALL",
		},
		{
			name: "duplicate job type",
			rules: []SystemGroupRule{
				{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: false, Value: "DL"},
				{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: false, Value: "IDL"},
			},
			reason: "only one job_type rule is allowed",
		},
		{
			name:   "empty string value",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeJobLevel, Operator: OperatorEq, Multi: false, Value: " "}},
			reason: "system group rule value must not be empty",
		},
		{
			name:   "empty array item",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeJobTag, Operator: OperatorEq, Multi: true, Value: []string{"a4", " "}}},
			reason: "system group rule value must not be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := SystemGroupCreateInput{SystemID: "system-a", Name: "Group", GroupingRules: tt.rules}
			requireSystemGroupInvalidInput(t, input.Normalize().Validate(), tt.reason)
		})
	}
}

func validSystemGroupUpdateInput() SystemGroupUpdateInput {
	return SystemGroupUpdateInput{
		SystemID: " system-a ",
		GroupID:  " group-1 ",
		Name:     " System Admins Updated ",
		GroupingRules: []SystemGroupRule{
			{AttributeKey: GroupAttributeOrganization, Operator: OperatorEq, Multi: true, Value: []string{" ORG-300 ", "ORG-100"}},
			{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: false, Value: " IDL "},
		},
	}
}

func TestSystemGroupUpdateInputNormalize(t *testing.T) {
	input := validSystemGroupUpdateInput().Normalize()

	if input.SystemID != "system-a" {
		t.Fatalf("SystemID = %q, want system-a", input.SystemID)
	}
	if input.GroupID != "group-1" {
		t.Fatalf("GroupID = %q, want group-1", input.GroupID)
	}
	if input.Name != "System Admins Updated" {
		t.Fatalf("Name = %q, want System Admins Updated", input.Name)
	}
	values := input.GroupingRules[0].Value.([]string)
	if values[0] != "ORG-300" {
		t.Fatalf("first org value = %q, want ORG-300", values[0])
	}
	if input.GroupingRules[1].Value.(string) != "IDL" {
		t.Fatalf("job type = %q, want IDL", input.GroupingRules[1].Value.(string))
	}
}

func TestSystemGroupUpdateInputValidateAcceptsValidAndEmptyRules(t *testing.T) {
	if err := validSystemGroupUpdateInput().Normalize().Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}

	emptyRules := SystemGroupUpdateInput{
		SystemID:      "system-a",
		GroupID:       "group-1",
		Name:          "Everyone",
		GroupingRules: []SystemGroupRule{},
	}
	if err := emptyRules.Normalize().Validate(); err != nil {
		t.Fatalf("Validate empty rules error = %v, want nil", err)
	}
}

func TestSystemGroupUpdateInputValidateRejectsInvalidIdentityAndName(t *testing.T) {
	tests := []struct {
		name   string
		input  SystemGroupUpdateInput
		reason string
	}{
		{name: "empty system id", input: SystemGroupUpdateInput{SystemID: " ", GroupID: "group-1", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "system id is required"},
		{name: "empty group id", input: SystemGroupUpdateInput{SystemID: "system-a", GroupID: " ", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "group id is required"},
		{name: "group id has whitespace", input: SystemGroupUpdateInput{SystemID: "system-a", GroupID: "group 1", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "group id must be a single token"},
		{name: "empty name", input: SystemGroupUpdateInput{SystemID: "system-a", GroupID: "group-1", Name: " ", GroupingRules: []SystemGroupRule{}}, reason: "name is required"},
		{name: "nil grouping rules", input: SystemGroupUpdateInput{SystemID: "system-a", GroupID: "group-1", Name: "Group", GroupingRules: nil}, reason: "grouping_rules is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireSystemGroupInvalidInput(t, tt.input.Normalize().Validate(), tt.reason)
		})
	}
}

func TestSystemGroupListQueryValidate(t *testing.T) {
	query := SystemGroupListQuery{
		SystemID: " system-a ",
		Limit:    20,
		Cursor:   &SystemGroupCursor{CreatedAt: systemGroupNow(), ID: "group-1"},
	}.Normalize()

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if query.SystemID != "system-a" || query.Cursor.ID != "group-1" {
		t.Fatalf("query = %+v, want normalized", query)
	}
}

func TestSystemGroupListQueryValidateRejectsInvalidCursor(t *testing.T) {
	query := SystemGroupListQuery{SystemID: "system-a", Limit: 20, Cursor: &SystemGroupCursor{ID: "group-1"}}
	requireSystemGroupInvalidInput(t, query.Normalize().Validate(), "cursor created_at is required")
}
