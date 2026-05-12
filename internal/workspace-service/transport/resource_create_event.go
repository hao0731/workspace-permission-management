package transport

import (
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
)

type resourceCreateData struct {
	WorkspaceID  string `json:"workspace_id"`
	ResourceName string `json:"resource_name"`
	ResourceType string `json:"resource_type"`
}

func NewResourceCreateEvent(command workspace.ResourceCreateCommand) ([]byte, error) {
	command = command.Normalize()
	if err := command.Validate(); err != nil {
		return nil, err
	}
	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetType(command.Subject())
	event.SetSource("workspace-service")
	event.SetSubject(command.WorkspaceID)
	event.SetID(command.EventID)
	event.SetTime(command.EventTime)
	if err := event.SetData(cloudevents.ApplicationJSON, resourceCreateData{
		WorkspaceID:  command.WorkspaceID,
		ResourceName: command.ResourceName,
		ResourceType: command.ResourceType,
	}); err != nil {
		return nil, fmt.Errorf("set resource create event data: %w", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal resource create event: %w", err)
	}
	return data, nil
}
