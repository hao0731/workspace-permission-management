package resource

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validUpsertInput() UpsertInput {
	return UpsertInput{
		ID:          "resource-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		DisplayName: "Spec",
		Type:        "document",
		Tags:        []string{"section_1"},
		EventTime:   time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
	}
}

func validDeleteInput() DeleteInput {
	return DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	}
}

func validListQuery() ListQuery {
	return ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       20,
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

func TestUpsertInputValidateAcceptsValidInput(t *testing.T) {
	input := validUpsertInput()

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestUpsertInputValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*UpsertInput)
		wantMessage string
	}{
		{
			name: "blank resource id",
			mutate: func(input *UpsertInput) {
				input.ID = "   "
			},
			wantMessage: "resource id is required",
		},
		{
			name: "blank workspace id",
			mutate: func(input *UpsertInput) {
				input.WorkspaceID = "   "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			mutate: func(input *UpsertInput) {
				input.FunctionKey = "   "
			},
			wantMessage: "function key is required",
		},
		{
			name: "blank display name",
			mutate: func(input *UpsertInput) {
				input.DisplayName = "   "
			},
			wantMessage: "display name is required",
		},
		{
			name: "blank resource type",
			mutate: func(input *UpsertInput) {
				input.Type = "   "
			},
			wantMessage: "resource type is required",
		},
		{
			name: "zero event time",
			mutate: func(input *UpsertInput) {
				input.EventTime = time.Time{}
			},
			wantMessage: "event time is required",
		},
		{
			name: "blank resource tag",
			mutate: func(input *UpsertInput) {
				input.Tags = []string{"section_1", "   "}
			},
			wantMessage: "resource tags must be non-empty strings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validUpsertInput()
			tt.mutate(&input)

			err := input.Validate()

			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestDeleteInputValidateAcceptsValidInput(t *testing.T) {
	input := validDeleteInput()

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestDeleteInputValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*DeleteInput)
		wantMessage string
	}{
		{
			name: "blank workspace id",
			mutate: func(input *DeleteInput) {
				input.WorkspaceID = "   "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			mutate: func(input *DeleteInput) {
				input.FunctionKey = "   "
			},
			wantMessage: "function key is required",
		},
		{
			name: "blank resource id",
			mutate: func(input *DeleteInput) {
				input.ResourceID = "   "
			},
			wantMessage: "resource id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validDeleteInput()
			tt.mutate(&input)

			err := input.Validate()

			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestListQueryValidateAcceptsValidQuery(t *testing.T) {
	query := validListQuery()

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestListQueryValidateAcceptsValidCursor(t *testing.T) {
	query := validListQuery()
	query.Cursor = &Cursor{
		CreatedAt: time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
		ID:        "resource-1",
	}

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestListQueryValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*ListQuery)
		wantMessage string
	}{
		{
			name: "blank workspace id",
			mutate: func(query *ListQuery) {
				query.WorkspaceID = "   "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			mutate: func(query *ListQuery) {
				query.FunctionKey = "   "
			},
			wantMessage: "function key is required",
		},
		{
			name: "zero limit",
			mutate: func(query *ListQuery) {
				query.Limit = 0
			},
			wantMessage: "limit must be greater than zero",
		},
		{
			name: "negative limit",
			mutate: func(query *ListQuery) {
				query.Limit = -1
			},
			wantMessage: "limit must be greater than zero",
		},
		{
			name: "cursor missing created_at",
			mutate: func(query *ListQuery) {
				query.Cursor = &Cursor{ID: "resource-1"}
			},
			wantMessage: "cursor created_at is required",
		},
		{
			name: "cursor missing id",
			mutate: func(query *ListQuery) {
				query.Cursor = &Cursor{
					CreatedAt: time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
					ID:        "   ",
				}
			},
			wantMessage: "cursor id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := validListQuery()
			tt.mutate(&query)

			err := query.Validate()

			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}
