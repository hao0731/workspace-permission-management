package resource

import (
	"errors"
	"strings"
	"testing"
	"time"
)

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

func validResourceUpsertEvent() ResourceUpsertEvent {
	return ResourceUpsertEvent{
		ResourceID:   "resource-1",
		WorkspaceID:  "workspace-1",
		FunctionKey:  "todo",
		DisplayName:  "Spec",
		ResourceType: "document",
		ResourceTags: []string{"section_1"},
		EventID:      "event-1",
		EventTime:    time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
	}
}

func validResourceCreateCommand() ResourceCreateCommand {
	return ResourceCreateCommand{
		WorkspaceID:  "workspace-1",
		AppName:      "documents",
		ResourceName: "Docs",
		ResourceType: "document",
		EventID:      "event-1",
		EventTime:    time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
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

func TestResourceCreateCommandValidateAcceptsValidCommand(t *testing.T) {
	if err := validResourceCreateCommand().Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestResourceCreateCommandValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*ResourceCreateCommand)
		wantMessage string
	}{
		{
			name: "blank workspace id",
			mutate: func(c *ResourceCreateCommand) {
				c.WorkspaceID = "   "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank app name",
			mutate: func(c *ResourceCreateCommand) {
				c.AppName = "   "
			},
			wantMessage: "app name is required",
		},
		{
			name: "blank resource name",
			mutate: func(c *ResourceCreateCommand) {
				c.ResourceName = "   "
			},
			wantMessage: "resource name is required",
		},
		{
			name: "blank resource type",
			mutate: func(c *ResourceCreateCommand) {
				c.ResourceType = "   "
			},
			wantMessage: "resource type is required",
		},
		{
			name: "blank event id",
			mutate: func(c *ResourceCreateCommand) {
				c.EventID = "   "
			},
			wantMessage: "event id is required",
		},
		{
			name: "zero event time",
			mutate: func(c *ResourceCreateCommand) {
				c.EventTime = time.Time{}
			},
			wantMessage: "event time is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := validResourceCreateCommand()
			tt.mutate(&command)
			requireInvalidInput(t, command.Validate(), tt.wantMessage)
		})
	}
}

func TestResourceCreateCommandNormalizeAndSubject(t *testing.T) {
	command := ResourceCreateCommand{
		WorkspaceID:  " workspace-1 ",
		AppName:      " documents ",
		ResourceName: " Docs ",
		ResourceType: " document ",
		EventID:      " event-1 ",
	}
	normalized := command.Normalize()
	if normalized.WorkspaceID != "workspace-1" || normalized.AppName != "documents" || normalized.ResourceName != "Docs" || normalized.ResourceType != "document" || normalized.EventID != "event-1" {
		t.Fatalf("Normalize() = %+v", normalized)
	}
	if normalized.Subject() != "cmd.app.documents.resource.create" {
		t.Fatalf("Subject() = %q", normalized.Subject())
	}
}

func TestResourceUpsertEventValidateAcceptsValidEvent(t *testing.T) {
	if err := validResourceUpsertEvent().Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestResourceUpsertEventValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*ResourceUpsertEvent)
		wantMessage string
	}{
		{
			name: "blank resource id",
			mutate: func(e *ResourceUpsertEvent) {
				e.ResourceID = "   "
			},
			wantMessage: "resource id is required",
		},
		{
			name: "blank workspace id",
			mutate: func(e *ResourceUpsertEvent) {
				e.WorkspaceID = "   "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			mutate: func(e *ResourceUpsertEvent) {
				e.FunctionKey = "   "
			},
			wantMessage: "function key is required",
		},
		{
			name: "blank display name",
			mutate: func(e *ResourceUpsertEvent) {
				e.DisplayName = "   "
			},
			wantMessage: "display name is required",
		},
		{
			name: "blank resource type",
			mutate: func(e *ResourceUpsertEvent) {
				e.ResourceType = "   "
			},
			wantMessage: "resource type is required",
		},
		{
			name: "blank event id",
			mutate: func(e *ResourceUpsertEvent) {
				e.EventID = "   "
			},
			wantMessage: "event id is required",
		},
		{
			name: "zero event time",
			mutate: func(e *ResourceUpsertEvent) {
				e.EventTime = time.Time{}
			},
			wantMessage: "event time is required",
		},
		{
			name: "blank resource tag",
			mutate: func(e *ResourceUpsertEvent) {
				e.ResourceTags = []string{"section_1", "   "}
			},
			wantMessage: "resource tags must be non-empty strings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := validResourceUpsertEvent()
			tt.mutate(&event)

			requireInvalidInput(t, event.Validate(), tt.wantMessage)
		})
	}
}

func TestResourceUpsertEventNormalizeAndSubject(t *testing.T) {
	event := ResourceUpsertEvent{
		ResourceID:   " resource-1 ",
		DisplayName:  " Spec ",
		ResourceType: " document ",
		FunctionKey:  " todo ",
		WorkspaceID:  " workspace-1 ",
		EventID:      " event-1 ",
	}
	normalized := event.Normalize()
	if normalized.ResourceID != "resource-1" || normalized.DisplayName != "Spec" || normalized.ResourceType != "document" || normalized.FunctionKey != "todo" || normalized.WorkspaceID != "workspace-1" || normalized.EventID != "event-1" {
		t.Fatalf("Normalize() = %+v", normalized)
	}
	if normalized.Subject() != "app.todo.resource.upserted" {
		t.Fatalf("Subject() = %q", normalized.Subject())
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
