package transport

import (
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type resourceUpsertData struct {
	ResourceID   string   `json:"resource_id"`
	DisplayName  string   `json:"display_name"`
	ResourceType string   `json:"resource_type"`
	ResourceTags []string `json:"resource_tags"`
	FunctionKey  string   `json:"function_key"`
	WorkspaceID  string   `json:"workspace_id"`
}

func NewResourceUpsertEvent(input resource.ResourceUpsertEvent) ([]byte, string, error) {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, "", err
	}
	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetType(input.Subject())
	event.SetSource("mock-function")
	event.SetSubject(input.ResourceID)
	event.SetID(input.EventID)
	event.SetTime(input.EventTime)
	if err := event.SetData(cloudevents.ApplicationJSON, resourceUpsertData{
		ResourceID:   input.ResourceID,
		DisplayName:  input.DisplayName,
		ResourceType: input.ResourceType,
		ResourceTags: cloneResourceTags(input.ResourceTags),
		FunctionKey:  input.FunctionKey,
		WorkspaceID:  input.WorkspaceID,
	}); err != nil {
		return nil, "", fmt.Errorf("set resource upsert event data: %w", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return nil, "", fmt.Errorf("marshal resource upsert event: %w", err)
	}
	return data, input.Subject(), nil
}

func cloneResourceTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	return append([]string(nil), tags...)
}
