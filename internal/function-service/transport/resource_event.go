package transport

import (
	"encoding/json"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

const cloudEventSpecVersion = "1.0"

type resourceUpsertData struct {
	ResourceID   string   `json:"resource_id"`
	DisplayName  string   `json:"display_name"`
	ResourceType string   `json:"resource_type"`
	ResourceTags []string `json:"resource_tags"`
	FunctionKey  string   `json:"function_key"`
	WorkspaceID  string   `json:"workspace_id"`
}

func ParseResourceUpsertEvent(data []byte, expectedType string) (resource.UpsertInput, error) {
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return resource.UpsertInput{}, fmt.Errorf("parse cloudevent: %w", err)
	}
	if err := event.Validate(); err != nil {
		return resource.UpsertInput{}, fmt.Errorf("validate cloudevent: %w", err)
	}
	if event.SpecVersion() != cloudEventSpecVersion {
		return resource.UpsertInput{}, fmt.Errorf("unsupported cloudevent specversion %q", event.SpecVersion())
	}
	if event.Type() != expectedType {
		return resource.UpsertInput{}, fmt.Errorf("cloudevent type %q does not match expected %q", event.Type(), expectedType)
	}
	if event.DataContentType() != "application/json" {
		return resource.UpsertInput{}, fmt.Errorf("cloudevent datacontenttype must be application/json")
	}
	var payload resourceUpsertData
	if err := event.DataAs(&payload); err != nil {
		return resource.UpsertInput{}, fmt.Errorf("parse cloudevent data: %w", err)
	}
	if event.Subject() != payload.ResourceID {
		return resource.UpsertInput{}, fmt.Errorf("cloudevent subject must match data.resource_id")
	}
	if strings.TrimSpace(payload.ResourceID) == "" ||
		strings.TrimSpace(payload.WorkspaceID) == "" ||
		strings.TrimSpace(payload.FunctionKey) == "" ||
		strings.TrimSpace(payload.DisplayName) == "" ||
		strings.TrimSpace(payload.ResourceType) == "" {
		return resource.UpsertInput{}, fmt.Errorf("resource event data contains empty required field")
	}
	for _, tag := range payload.ResourceTags {
		if strings.TrimSpace(tag) == "" {
			return resource.UpsertInput{}, fmt.Errorf("resource_tags must contain non-empty strings")
		}
	}
	if event.Time().IsZero() {
		return resource.UpsertInput{}, fmt.Errorf("cloudevent time is required")
	}
	return resource.UpsertInput{
		ID:          payload.ResourceID,
		WorkspaceID: payload.WorkspaceID,
		FunctionKey: payload.FunctionKey,
		DisplayName: payload.DisplayName,
		Type:        payload.ResourceType,
		Tags:        append([]string(nil), payload.ResourceTags...),
		EventTime:   event.Time(),
	}, nil
}
