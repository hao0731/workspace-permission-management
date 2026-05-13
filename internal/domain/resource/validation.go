package resource

import (
	"fmt"
	"strings"
)

func (c ResourceCreateCommand) Normalize() ResourceCreateCommand {
	c.WorkspaceID = strings.TrimSpace(c.WorkspaceID)
	c.AppName = strings.TrimSpace(c.AppName)
	c.ResourceName = strings.TrimSpace(c.ResourceName)
	c.ResourceType = strings.TrimSpace(c.ResourceType)
	c.EventID = strings.TrimSpace(c.EventID)
	return c
}

func (c ResourceCreateCommand) Validate() error {
	normalized := c.Normalize()
	if normalized.WorkspaceID == "" {
		return invalidInput("workspace id is required")
	}
	if normalized.AppName == "" {
		return invalidInput("app name is required")
	}
	if normalized.ResourceName == "" {
		return invalidInput("resource name is required")
	}
	if normalized.ResourceType == "" {
		return invalidInput("resource type is required")
	}
	if normalized.EventID == "" {
		return invalidInput("event id is required")
	}
	if normalized.EventTime.IsZero() {
		return invalidInput("event time is required")
	}
	return nil
}

func (e ResourceUpsertEvent) Normalize() ResourceUpsertEvent {
	e.ResourceID = strings.TrimSpace(e.ResourceID)
	e.DisplayName = strings.TrimSpace(e.DisplayName)
	e.ResourceType = strings.TrimSpace(e.ResourceType)
	e.FunctionKey = strings.TrimSpace(e.FunctionKey)
	e.WorkspaceID = strings.TrimSpace(e.WorkspaceID)
	e.EventID = strings.TrimSpace(e.EventID)
	return e
}

func (e ResourceUpsertEvent) Validate() error {
	normalized := e.Normalize()
	if normalized.ResourceID == "" {
		return invalidInput("resource id is required")
	}
	if normalized.DisplayName == "" {
		return invalidInput("display name is required")
	}
	if normalized.ResourceType == "" {
		return invalidInput("resource type is required")
	}
	if normalized.FunctionKey == "" {
		return invalidInput("function key is required")
	}
	if normalized.WorkspaceID == "" {
		return invalidInput("workspace id is required")
	}
	if normalized.EventID == "" {
		return invalidInput("event id is required")
	}
	if normalized.EventTime.IsZero() {
		return invalidInput("event time is required")
	}
	for _, tag := range normalized.ResourceTags {
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
