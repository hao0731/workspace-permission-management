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

func validResourceDefinitionSaveInput() ResourceDefinitionSaveInput {
	return ResourceDefinitionSaveInput{
		SystemID: "todo",
		Resources: []ResourceDefinitionInput{
			{Type: ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit", Description: "Allows editing."},
			{Type: ResourceDefinitionKindTag, Label: "Private", Key: "private"},
			{Type: ResourceDefinitionKindType, Label: "Repository", Key: "repo"},
		},
	}
}

func TestResourceDefinitionSaveInputValidateAcceptsValidInput(t *testing.T) {
	input := validResourceDefinitionSaveInput()

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestResourceDefinitionSaveInputNormalizeTrimsValues(t *testing.T) {
	input := ResourceDefinitionSaveInput{
		SystemID: " todo ",
		Resources: []ResourceDefinitionInput{{
			Type:        ResourceDefinitionType(" action "),
			Label:       " Can Edit ",
			Key:         " can_edit ",
			Description: " Allows editing. ",
		}},
	}

	normalized := input.Normalize()
	if normalized.SystemID != "todo" {
		t.Fatalf("SystemID = %q, want todo", normalized.SystemID)
	}
	got := normalized.Resources[0]
	if got.Type != ResourceDefinitionKindAction || got.Label != "Can Edit" || got.Key != "can_edit" || got.Description != "Allows editing." {
		t.Fatalf("resource = %+v, want trimmed action", got)
	}
}

func TestResourceDefinitionSaveInputValidateRejectsInvalidFields(t *testing.T) {
	longLabel := strings.Repeat("測", 21)
	longDescription := strings.Repeat("a", 2001)
	tests := []struct {
		name        string
		mutate      func(*ResourceDefinitionSaveInput)
		wantMessage string
	}{
		{name: "blank system id", mutate: func(i *ResourceDefinitionSaveInput) { i.SystemID = "   " }, wantMessage: "system id is required"},
		{name: "dotted system id", mutate: func(i *ResourceDefinitionSaveInput) { i.SystemID = "app.todo" }, wantMessage: "system id must be a single subject token"},
		{name: "whitespace system id", mutate: func(i *ResourceDefinitionSaveInput) { i.SystemID = "todo app" }, wantMessage: "system id must be a single subject token"},
		{name: "empty resources", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources = nil }, wantMessage: "resources are required"},
		{name: "invalid resource type", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Type = "scope" }, wantMessage: "resource type must be type, tag, or action"},
		{name: "blank key", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Key = "   " }, wantMessage: "resource key is required"},
		{name: "uppercase key", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Key = "Can_Edit" }, wantMessage: "resource key must contain only lower-case letters, numbers, and underscores"},
		{name: "long key", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Key = "abcdefghijklmnop" }, wantMessage: "resource key must be at most 15 characters"},
		{name: "blank label", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Label = "   " }, wantMessage: "resource label is required"},
		{name: "long unicode label", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Label = longLabel }, wantMessage: "resource label must be at most 20 characters"},
		{name: "long description", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Description = longDescription }, wantMessage: "resource description must be at most 2000 characters"},
		{name: "duplicate request identity", mutate: func(i *ResourceDefinitionSaveInput) {
			i.Resources = append(i.Resources, ResourceDefinitionInput{Type: ResourceDefinitionKindAction, Label: "Can Update", Key: "can_edit"})
		}, wantMessage: "duplicate resource definition action/can_edit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validResourceDefinitionSaveInput()
			tt.mutate(&input)
			requireInvalidInput(t, input.Validate(), tt.wantMessage)
		})
	}
}

func TestResourceDefinitionLimitsValidateAndCounts(t *testing.T) {
	limits := ResourceDefinitionLimits{Types: 1, Actions: 1, Tags: 1}
	definitions := []ResourceDefinition{
		{SystemID: "todo", Type: ResourceDefinitionKindType, Key: "repo"},
		{SystemID: "todo", Type: ResourceDefinitionKindType, Key: "page"},
	}

	err := ValidateResourceDefinitionCounts(definitions, limits)
	requireInvalidInput(t, err, "resource type limit exceeded")
}

func TestResourceDefinitionLimitsRejectsInvalidLimit(t *testing.T) {
	err := ResourceDefinitionLimits{Types: 0, Actions: 1, Tags: 1}.Validate()
	requireInvalidInput(t, err, "resource type limit must be greater than zero")
}

func TestNewResourceAttribute(t *testing.T) {
	got := NewResourceAttribute("can_edit", "private", "repo")
	want := ResourceAttribute("can_edit_private_repo")

	if got != want {
		t.Fatalf("ResourceAttribute = %q, want %q", got, want)
	}
}
