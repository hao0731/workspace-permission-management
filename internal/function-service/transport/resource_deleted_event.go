package transport

import (
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type resourceDeletedData struct {
	WorkspaceID string `json:"workspace_id"`
	FunctionKey string `json:"function_key"`
	ResourceID  string `json:"resource_id"`
}

func NewResourceDeletedEvent(input resource.DeletedEvent, eventType string) ([]byte, error) {
	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetType(eventType)
	event.SetSource("function-service")
	event.SetSubject(input.ResourceID)
	event.SetID(input.EventID)
	event.SetTime(input.EventTime)

	if err := event.SetData(cloudevents.ApplicationJSON, resourceDeletedData{
		WorkspaceID: input.WorkspaceID,
		FunctionKey: input.FunctionKey,
		ResourceID:  input.ResourceID,
	}); err != nil {
		return nil, fmt.Errorf("set resource deleted event data: %w", err)
	}

	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal resource deleted event: %w", err)
	}
	return data, nil
}
