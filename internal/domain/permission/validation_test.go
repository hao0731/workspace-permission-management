package permission

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validSaveInput() SaveInput {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return SaveInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		OfficePermission: &PermissionSection{
			BaselineRule: BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []ExtraRule{{
				RuleID:         "rule-office-1",
				GroupIDs:       []string{"group-1"},
				ActionID:       "edit",
				ResourceTags:   []string{"section_1"},
				ExpirationDate: expiration,
			}},
		},
		RemotePermission: &PermissionSection{
			BaselineRule: BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
			ExtraRules: []ExtraRule{{
				GroupIDs:       []string{"group-2"},
				ActionID:       "delete",
				ResourceTags:   []string{"remote"},
				ExpirationDate: expiration,
			}},
		},
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

func TestSaveInputValidateAcceptsValidInput(t *testing.T) {
	input := validSaveInput()

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestSaveInputValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*SaveInput)
		wantMessage string
	}{
		{
			name: "blank workspace id",
			mutate: func(input *SaveInput) {
				input.WorkspaceID = "   "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			mutate: func(input *SaveInput) {
				input.FunctionKey = "   "
			},
			wantMessage: "function key is required",
		},
		{
			name: "missing office permission",
			mutate: func(input *SaveInput) {
				input.OfficePermission = nil
			},
			wantMessage: "office permission is required",
		},
		{
			name: "missing remote permission",
			mutate: func(input *SaveInput) {
				input.RemotePermission = nil
			},
			wantMessage: "remote permission is required",
		},
		{
			name: "blank baseline action",
			mutate: func(input *SaveInput) {
				input.OfficePermission.BaselineRule.ActionID = "   "
			},
			wantMessage: "office baseline action id is required",
		},
		{
			name: "empty baseline tags",
			mutate: func(input *SaveInput) {
				input.OfficePermission.BaselineRule.ResourceTags = nil
			},
			wantMessage: "office baseline resource tags are required",
		},
		{
			name: "blank baseline tag",
			mutate: func(input *SaveInput) {
				input.OfficePermission.BaselineRule.ResourceTags = []string{"section_1", "   "}
			},
			wantMessage: "office baseline resource tags must be non-empty strings",
		},
		{
			name: "empty extra group ids",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].GroupIDs = nil
			},
			wantMessage: "office extra rule group ids are required",
		},
		{
			name: "blank extra group id",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].GroupIDs = []string{"group-1", "   "}
			},
			wantMessage: "office extra rule group ids must be non-empty strings",
		},
		{
			name: "blank extra action",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].ActionID = "   "
			},
			wantMessage: "office extra rule action id is required",
		},
		{
			name: "empty extra resource tags",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].ResourceTags = nil
			},
			wantMessage: "office extra rule resource tags are required",
		},
		{
			name: "zero extra expiration",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].ExpirationDate = time.Time{}
			},
			wantMessage: "office extra rule expiration date is required",
		},
		{
			name: "blank extra rule id",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].RuleID = "   "
			},
			wantMessage: "office extra rule rule id must be non-empty when provided",
		},
		{
			name: "duplicate provided rule id across sections",
			mutate: func(input *SaveInput) {
				input.RemotePermission.ExtraRules[0].RuleID = "rule-office-1"
			},
			wantMessage: "duplicate rule id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validSaveInput()
			tt.mutate(&input)

			err := input.Validate()

			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestGetQueryValidateAcceptsValidQuery(t *testing.T) {
	query := GetQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
	}

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestGetQueryValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		query       GetQuery
		wantMessage string
	}{
		{
			name: "blank workspace id",
			query: GetQuery{
				WorkspaceID: "   ",
				FunctionKey: "todo",
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			query: GetQuery{
				WorkspaceID: "workspace-1",
				FunctionKey: "   ",
			},
			wantMessage: "function key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}
