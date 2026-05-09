package group

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func futureTime() time.Time {
	return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
}

func validationNow() time.Time {
	return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
}

func validCreateInput() CreateInput {
	return CreateInput{
		WorkspaceID: "workspace-1",
		Name:        " Design Reviewers ",
		Description: "Employees who can review design documents.",
		GroupingRule: GroupingRule{
			Rules: []Rule{{
				AttributeKey: " department ",
				Operator:     OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: futureTime(),
		},
		IndividualMembers: []IndividualMember{{
			NTAccount:      " user1 ",
			ExpirationDate: futureTime(),
		}},
	}
}

func validRule(attributeKey string) Rule {
	return Rule{
		AttributeKey: attributeKey,
		Operator:     OperatorEq,
		Multi:        false,
		Value:        "ABCD-123",
	}
}

func requireInvalidInput(t *testing.T, err error, wantMessage string) {
	t.Helper()
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if !strings.Contains(err.Error(), wantMessage) {
		t.Fatalf("error = %q, want message containing %q", err.Error(), wantMessage)
	}
}

func TestCreateInputValidateRejectsLimitExceededWithOptions(t *testing.T) {
	tests := []struct {
		name                 string
		maxGroupingRules     int
		maxIndividualMembers int
		mutate               func(*CreateInput)
		wantMessage          string
	}{
		{
			name:                 "too many grouping rules",
			maxGroupingRules:     1,
			maxIndividualMembers: 1000,
			mutate: func(input *CreateInput) {
				input.GroupingRule.Rules = []Rule{
					validRule("department"),
					validRule("job_code"),
				}
			},
			wantMessage: "grouping rules must not exceed 1 items",
		},
		{
			name:                 "too many individual members",
			maxGroupingRules:     10,
			maxIndividualMembers: 1,
			mutate: func(input *CreateInput) {
				input.IndividualMembers = []IndividualMember{
					{NTAccount: "user1", ExpirationDate: futureTime()},
					{NTAccount: "user2", ExpirationDate: futureTime()},
				}
			},
			wantMessage: "individual members must not exceed 1 items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validCreateInput().Normalize()
			tt.mutate(&input)

			err := input.Validate(
				validationNow(),
				WithMaxGroupingRules(tt.maxGroupingRules),
				WithMaxIndividualMembers(tt.maxIndividualMembers),
			)
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestCreateInputNormalizeTrimsNamesAndKeys(t *testing.T) {
	input := validCreateInput().Normalize()

	if input.WorkspaceID != "workspace-1" {
		t.Fatalf("WorkspaceID = %q, want workspace-1", input.WorkspaceID)
	}
	if input.Name != "Design Reviewers" {
		t.Fatalf("Name = %q, want Design Reviewers", input.Name)
	}
	if input.GroupingRule.Rules[0].AttributeKey != "department" {
		t.Fatalf("AttributeKey = %q, want department", input.GroupingRule.Rules[0].AttributeKey)
	}
	if input.IndividualMembers[0].NTAccount != "user1" {
		t.Fatalf("NTAccount = %q, want user1", input.IndividualMembers[0].NTAccount)
	}
}

func TestCreateInputValidateAcceptsValidInput(t *testing.T) {
	input := validCreateInput().Normalize()

	if err := input.Validate(validationNow()); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestCreateInputValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*CreateInput)
		wantMessage string
	}{
		{
			name: "blank workspace id",
			mutate: func(input *CreateInput) {
				input.WorkspaceID = " "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank name",
			mutate: func(input *CreateInput) {
				input.Name = " "
			},
			wantMessage: "name is required",
		},
		{
			name: "zero grouping expiration",
			mutate: func(input *CreateInput) {
				input.GroupingRule.ExpirationDate = time.Time{}
			},
			wantMessage: "grouping rule expiration date is required",
		},
		{
			name: "past grouping expiration",
			mutate: func(input *CreateInput) {
				input.GroupingRule.ExpirationDate = validationNow()
			},
			wantMessage: "grouping rule expiration date must be in the future",
		},
		{
			name: "no membership source",
			mutate: func(input *CreateInput) {
				input.GroupingRule.Rules = nil
				input.IndividualMembers = nil
			},
			wantMessage: "at least one membership source is required",
		},
		{
			name: "blank member nt account",
			mutate: func(input *CreateInput) {
				input.IndividualMembers = []IndividualMember{{
					NTAccount:      " ",
					ExpirationDate: futureTime(),
				}}
			},
			wantMessage: "individual member nt account is required",
		},
		{
			name: "duplicate member nt account",
			mutate: func(input *CreateInput) {
				input.IndividualMembers = []IndividualMember{
					{NTAccount: "user1", ExpirationDate: futureTime()},
					{NTAccount: "user1", ExpirationDate: futureTime()},
				}
			},
			wantMessage: "duplicate individual member nt account",
		},
		{
			name: "past member expiration",
			mutate: func(input *CreateInput) {
				input.IndividualMembers = []IndividualMember{{
					NTAccount:      "user1",
					ExpirationDate: validationNow(),
				}}
			},
			wantMessage: "individual member expiration date must be in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validCreateInput().Normalize()
			tt.mutate(&input)

			err := input.Validate(validationNow())
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestRuleValidate(t *testing.T) {
	tests := []struct {
		name        string
		rule        Rule
		wantMessage string
	}{
		{
			name: "blank attribute",
			rule: Rule{
				AttributeKey: " ",
				Operator:     OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			},
			wantMessage: "rule attribute key is required",
		},
		{
			name: "invalid operator",
			rule: Rule{
				AttributeKey: "department",
				Operator:     Operator("contains"),
				Multi:        false,
				Value:        "ABCD-123",
			},
			wantMessage: "rule operator is invalid",
		},
		{
			name: "single value null",
			rule: Rule{
				AttributeKey: "department",
				Operator:     OperatorEq,
				Multi:        false,
				Value:        nil,
			},
			wantMessage: "single-value rule value is required",
		},
		{
			name: "single value array",
			rule: Rule{
				AttributeKey: "department",
				Operator:     OperatorEq,
				Multi:        false,
				Value:        []any{"ABCD-123"},
			},
			wantMessage: "single-value rule value must not be an array",
		},
		{
			name: "multi value scalar",
			rule: Rule{
				AttributeKey: "department",
				Operator:     OperatorEq,
				Multi:        true,
				Value:        "ABCD-123",
			},
			wantMessage: "multi-value rule value must be a non-empty array",
		},
		{
			name: "multi value empty array",
			rule: Rule{
				AttributeKey: "department",
				Operator:     OperatorEq,
				Multi:        true,
				Value:        []any{},
			},
			wantMessage: "multi-value rule value must be a non-empty array",
		},
		{
			name: "multi value null item",
			rule: Rule{
				AttributeKey: "department",
				Operator:     OperatorEq,
				Multi:        true,
				Value:        []any{"ABCD-123", nil},
			},
			wantMessage: "multi-value rule value items must not be null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestRuleValidateAcceptsAllowedOperators(t *testing.T) {
	operators := []Operator{
		OperatorEq,
		OperatorNotEq,
		OperatorGt,
		OperatorGte,
		OperatorLt,
		OperatorLte,
	}

	for _, operator := range operators {
		t.Run(string(operator), func(t *testing.T) {
			rule := Rule{
				AttributeKey: "department",
				Operator:     operator,
				Multi:        false,
				Value:        "ABCD-123",
			}

			if err := rule.Validate(); err != nil {
				t.Fatalf("Validate error = %v, want nil", err)
			}
		})
	}
}
