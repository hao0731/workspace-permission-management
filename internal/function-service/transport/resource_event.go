package transport

import (
	"encoding/json"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

const (
	cloudEventSpecVersion = "1.0"

	ResourceUpsertEventTypePattern = "app.*.resource.upserted"
)

type resourceUpsertData struct {
	ResourceID   string   `json:"resource_id"`
	DisplayName  string   `json:"display_name"`
	ResourceType string   `json:"resource_type"`
	ResourceTags []string `json:"resource_tags"`
	FunctionKey  string   `json:"function_key"`
	WorkspaceID  string   `json:"workspace_id"`
}

func ParseResourceUpsertEvent(data []byte) (resource.ResourceUpsertEvent, error) {
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("parse cloudevent: %w", err)
	}
	if err := event.Validate(); err != nil {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("validate cloudevent: %w", err)
	}
	if event.SpecVersion() != cloudEventSpecVersion {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("unsupported cloudevent specversion %q", event.SpecVersion())
	}
	if !matchesResourceUpsertEventTypePattern(event.Type()) {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("cloudevent type %q does not match subject pattern %q", event.Type(), ResourceUpsertEventTypePattern)
	}
	if event.DataContentType() != "application/json" {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("cloudevent datacontenttype must be application/json")
	}
	var payload resourceUpsertData
	if err := event.DataAs(&payload); err != nil {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("parse cloudevent data: %w", err)
	}
	if event.Subject() != payload.ResourceID {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("cloudevent subject must match data.resource_id")
	}
	if strings.TrimSpace(payload.ResourceID) == "" ||
		strings.TrimSpace(payload.WorkspaceID) == "" ||
		strings.TrimSpace(payload.FunctionKey) == "" ||
		strings.TrimSpace(payload.DisplayName) == "" ||
		strings.TrimSpace(payload.ResourceType) == "" {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("resource event data contains empty required field")
	}
	expectedType := resource.ResourceUpsertEvent{FunctionKey: strings.TrimSpace(payload.FunctionKey)}.Subject()
	if event.Type() != expectedType {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("cloudevent type %q does not match data.function_key subject %q", event.Type(), expectedType)
	}
	for _, tag := range payload.ResourceTags {
		if strings.TrimSpace(tag) == "" {
			return resource.ResourceUpsertEvent{}, fmt.Errorf("resource_tags must contain non-empty strings")
		}
	}
	if event.Time().IsZero() {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("cloudevent time is required")
	}
	return resource.ResourceUpsertEvent{
		ResourceID:   payload.ResourceID,
		WorkspaceID:  payload.WorkspaceID,
		FunctionKey:  payload.FunctionKey,
		DisplayName:  payload.DisplayName,
		ResourceType: payload.ResourceType,
		ResourceTags: append([]string(nil), payload.ResourceTags...),
		EventID:      event.ID(),
		EventTime:    event.Time(),
	}, nil
}

func matchesResourceUpsertEventTypePattern(eventType string) bool {
	parts := strings.Split(eventType, ".")
	if len(parts) != 4 {
		return false
	}
	return parts[0] == "app" &&
		isConcreteSubjectToken(parts[1]) &&
		parts[2] == "resource" &&
		parts[3] == "upserted"
}

func isConcreteSubjectToken(token string) bool {
	return token != "" && strings.TrimSpace(token) == token && token != "*" && token != ">"
}
