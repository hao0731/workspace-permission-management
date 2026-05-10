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

func TestGetQueryValidate(t *testing.T) {
	query := GetQuery{WorkspaceID: "workspace-1", GroupID: "group-1"}
	if err := query.Normalize().Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestGroupIdentityValidationRejectsBlankFields(t *testing.T) {
	tests := []struct {
		name        string
		validate    func() error
		wantMessage string
	}{
		{
			name: "get blank workspace",
			validate: func() error {
				return GetQuery{WorkspaceID: " ", GroupID: "group-1"}.Normalize().Validate()
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "get blank group id",
			validate: func() error {
				return GetQuery{WorkspaceID: "workspace-1", GroupID: " "}.Normalize().Validate()
			},
			wantMessage: "group id is required",
		},
		{
			name: "delete blank group id",
			validate: func() error {
				return DeleteInput{WorkspaceID: "workspace-1", GroupID: " "}.Normalize().Validate()
			},
			wantMessage: "group id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireInvalidInput(t, tt.validate(), tt.wantMessage)
		})
	}
}

func TestUpdateGroupingRuleInputValidate(t *testing.T) {
	input := UpdateGroupingRuleInput{
		WorkspaceID:    " workspace-1 ",
		GroupID:        " group-1 ",
		ExpirationDate: futureTime(),
		Rules: []Rule{{
			AttributeKey: " department ",
			Operator:     OperatorEq,
			Multi:        false,
			Value:        "ABCD-123",
		}},
	}.Normalize()

	if err := input.Validate(validationNow(), WithMaxGroupingRules(1)); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" {
		t.Fatalf("identity = %q/%q, want trimmed values", input.WorkspaceID, input.GroupID)
	}
	if input.Rules[0].AttributeKey != "department" {
		t.Fatalf("AttributeKey = %q, want department", input.Rules[0].AttributeKey)
	}
}

func TestUpdateGroupingRuleInputAllowsEmptyRulesAtDomainBoundary(t *testing.T) {
	input := UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		ExpirationDate: futureTime(),
		Rules:          nil,
	}

	if err := input.Validate(validationNow()); err != nil {
		t.Fatalf("Validate error = %v, want nil because active member count is repository-backed", err)
	}
}

func TestUpdateGroupingRuleInputRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		input       UpdateGroupingRuleInput
		wantMessage string
	}{
		{
			name:        "blank group id",
			input:       UpdateGroupingRuleInput{WorkspaceID: "workspace-1", GroupID: " ", ExpirationDate: futureTime()},
			wantMessage: "group id is required",
		},
		{
			name:        "missing expiration",
			input:       UpdateGroupingRuleInput{WorkspaceID: "workspace-1", GroupID: "group-1"},
			wantMessage: "grouping rule expiration date is required",
		},
		{
			name:        "past expiration",
			input:       UpdateGroupingRuleInput{WorkspaceID: "workspace-1", GroupID: "group-1", ExpirationDate: validationNow()},
			wantMessage: "grouping rule expiration date must be in the future",
		},
		{
			name: "too many rules",
			input: UpdateGroupingRuleInput{
				WorkspaceID:    "workspace-1",
				GroupID:        "group-1",
				ExpirationDate: futureTime(),
				Rules: []Rule{
					{AttributeKey: "department", Operator: OperatorEq, Multi: false, Value: "ABCD-123"},
					{AttributeKey: "level", Operator: OperatorGte, Multi: false, Value: 5},
				},
			},
			wantMessage: "grouping rules must not exceed 1 items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Normalize().Validate(validationNow(), WithMaxGroupingRules(1))
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestListIndividualMembersQueryValidate(t *testing.T) {
	query := ListIndividualMembersQuery{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		Limit:       20,
		Cursor: &IndividualMemberCursor{
			CreatedAt: validationNow(),
			ID:        " member-1 ",
		},
	}.Normalize()

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if query.WorkspaceID != "workspace-1" || query.GroupID != "group-1" || query.Cursor.ID != "member-1" {
		t.Fatalf("query = %+v, want trimmed identity and cursor", query)
	}
}

func TestListIndividualMembersQueryRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		query       ListIndividualMembersQuery
		wantMessage string
	}{
		{
			name:        "blank group id",
			query:       ListIndividualMembersQuery{WorkspaceID: "workspace-1", GroupID: " ", Limit: 20},
			wantMessage: "group id is required",
		},
		{
			name:        "zero limit",
			query:       ListIndividualMembersQuery{WorkspaceID: "workspace-1", GroupID: "group-1"},
			wantMessage: "limit must be greater than zero",
		},
		{
			name: "cursor missing created at",
			query: ListIndividualMembersQuery{
				WorkspaceID: "workspace-1",
				GroupID:     "group-1",
				Limit:       20,
				Cursor:      &IndividualMemberCursor{ID: "member-1"},
			},
			wantMessage: "cursor created_at is required",
		},
		{
			name: "cursor missing id",
			query: ListIndividualMembersQuery{
				WorkspaceID: "workspace-1",
				GroupID:     "group-1",
				Limit:       20,
				Cursor:      &IndividualMemberCursor{CreatedAt: validationNow()},
			},
			wantMessage: "cursor id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Normalize().Validate()
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestAddIndividualMembersInputValidate(t *testing.T) {
	input := AddIndividualMembersInput{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		IndividualMembers: []IndividualMember{{
			NTAccount:      " user2 ",
			ExpirationDate: futureTime(),
		}},
	}.Normalize()

	if err := input.Validate(validationNow(), WithMaxIndividualMembers(1)); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" {
		t.Fatalf("identity = %q/%q, want trimmed values", input.WorkspaceID, input.GroupID)
	}
	if input.IndividualMembers[0].NTAccount != "user2" {
		t.Fatalf("NTAccount = %q, want user2", input.IndividualMembers[0].NTAccount)
	}
}

func TestAddIndividualMembersInputRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name                 string
		input                AddIndividualMembersInput
		maxIndividualMembers int
		wantError            error
		wantMessage          string
	}{
		{
			name:        "blank workspace id",
			input:       AddIndividualMembersInput{WorkspaceID: " ", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: futureTime()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "workspace id is required",
		},
		{
			name:        "blank group id",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: " ", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: futureTime()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "group id is required",
		},
		{
			name:        "empty members",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1"},
			wantError:   ErrInvalidInput,
			wantMessage: "individual members are required",
		},
		{
			name:        "blank account",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: " ", ExpirationDate: futureTime()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "individual member nt account is required",
		},
		{
			name:                 "duplicate account",
			input:                AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: futureTime()}, {NTAccount: "user1", ExpirationDate: futureTime()}}},
			maxIndividualMembers: 2,
			wantError:            ErrDuplicateMember,
			wantMessage:          "duplicate individual member nt account",
		},
		{
			name:        "missing expiration",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1"}}},
			wantError:   ErrInvalidInput,
			wantMessage: "individual member expiration date is required",
		},
		{
			name:        "past expiration",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: validationNow()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "individual member expiration date must be in the future",
		},
		{
			name:        "limit exceeded",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: futureTime()}, {NTAccount: "user2", ExpirationDate: futureTime()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "individual members must not exceed 1 items",
		},
		{
			name:        "limit exceeded before duplicate account",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: futureTime()}, {NTAccount: "user1", ExpirationDate: futureTime()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "individual members must not exceed 1 items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxIndividualMembers := tt.maxIndividualMembers
			if maxIndividualMembers == 0 {
				maxIndividualMembers = 1
			}
			err := tt.input.Normalize().Validate(validationNow(), WithMaxIndividualMembers(maxIndividualMembers))
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("error = %v, want %v", err, tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("error = %q, want message containing %q", err.Error(), tt.wantMessage)
			}
		})
	}
}

func TestUpdateIndividualMemberExpirationInputValidate(t *testing.T) {
	input := UpdateIndividualMemberExpirationInput{
		WorkspaceID:    " workspace-1 ",
		GroupID:        " group-1 ",
		NTAccount:      " user2 ",
		ExpirationDate: futureTime(),
	}.Normalize()

	if err := input.Validate(validationNow()); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" || input.NTAccount != "user2" {
		t.Fatalf("input = %+v, want trimmed identity and account", input)
	}
}

func TestUpdateIndividualMemberExpirationInputRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		input       UpdateIndividualMemberExpirationInput
		wantMessage string
	}{
		{
			name:        "blank account",
			input:       UpdateIndividualMemberExpirationInput{WorkspaceID: "workspace-1", GroupID: "group-1", NTAccount: " ", ExpirationDate: futureTime()},
			wantMessage: "individual member nt account is required",
		},
		{
			name:        "missing expiration",
			input:       UpdateIndividualMemberExpirationInput{WorkspaceID: "workspace-1", GroupID: "group-1", NTAccount: "user1"},
			wantMessage: "individual member expiration date is required",
		},
		{
			name:        "past expiration",
			input:       UpdateIndividualMemberExpirationInput{WorkspaceID: "workspace-1", GroupID: "group-1", NTAccount: "user1", ExpirationDate: validationNow()},
			wantMessage: "individual member expiration date must be in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireInvalidInput(t, tt.input.Normalize().Validate(validationNow()), tt.wantMessage)
		})
	}
}

func TestDeleteIndividualMemberInputValidate(t *testing.T) {
	input := DeleteIndividualMemberInput{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		NTAccount:   " user2 ",
	}.Normalize()

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" || input.NTAccount != "user2" {
		t.Fatalf("input = %+v, want trimmed identity and account", input)
	}
}

func TestDeleteIndividualMemberInputRejectsBlankAccount(t *testing.T) {
	err := DeleteIndividualMemberInput{WorkspaceID: "workspace-1", GroupID: "group-1", NTAccount: " "}.Normalize().Validate()
	requireInvalidInput(t, err, "individual member nt account is required")
}
