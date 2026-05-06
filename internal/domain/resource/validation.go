package resource

import (
	"fmt"
	"strings"
)

func (input UpsertInput) Validate() error {
	if strings.TrimSpace(input.ID) == "" {
		return invalidInput("resource id is required")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return invalidInput("function key is required")
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		return invalidInput("display name is required")
	}
	if strings.TrimSpace(input.Type) == "" {
		return invalidInput("resource type is required")
	}
	if input.EventTime.IsZero() {
		return invalidInput("event time is required")
	}
	for _, tag := range input.Tags {
		if strings.TrimSpace(tag) == "" {
			return invalidInput("resource tags must be non-empty strings")
		}
	}
	return nil
}

func (input DeleteInput) Validate() error {
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return invalidInput("function key is required")
	}
	if strings.TrimSpace(input.ResourceID) == "" {
		return invalidInput("resource id is required")
	}
	return nil
}

func (query ListQuery) Validate() error {
	if strings.TrimSpace(query.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(query.FunctionKey) == "" {
		return invalidInput("function key is required")
	}
	if query.Limit <= 0 {
		return invalidInput("limit must be greater than zero")
	}
	if query.Cursor != nil {
		if query.Cursor.CreatedAt.IsZero() {
			return invalidInput("cursor created_at is required")
		}
		if strings.TrimSpace(query.Cursor.ID) == "" {
			return invalidInput("cursor id is required")
		}
	}
	return nil
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
