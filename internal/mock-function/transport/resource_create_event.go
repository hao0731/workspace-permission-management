package transport

import (
	"encoding/json"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/mockfunction"
)

const cloudEventSpecVersion = "1.0"

type resourceCreateData struct {
	WorkspaceID  string `json:"workspace_id"`
	ResourceName string `json:"resource_name"`
	ResourceType string `json:"resource_type"`
}

func ParseResourceCreateCommandEvent(data []byte, messageSubject string, subjectAppNames map[string]string) (mockfunction.ResourceCreateCommand, error) {
	appName, ok := subjectAppNames[messageSubject]
	if !ok {
		return mockfunction.ResourceCreateCommand{}, fmt.Errorf("unknown resource create subject %q", messageSubject)
	}
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return mockfunction.ResourceCreateCommand{}, fmt.Errorf("parse cloudevent: %w", err)
	}
	if err := event.Validate(); err != nil {
		return mockfunction.ResourceCreateCommand{}, fmt.Errorf("validate cloudevent: %w", err)
	}
	if event.SpecVersion() != cloudEventSpecVersion {
		return mockfunction.ResourceCreateCommand{}, fmt.Errorf("unsupported cloudevent specversion %q", event.SpecVersion())
	}
	if event.Type() != messageSubject {
		return mockfunction.ResourceCreateCommand{}, fmt.Errorf("cloudevent type %q does not match subject %q", event.Type(), messageSubject)
	}
	if event.DataContentType() != "application/json" {
		return mockfunction.ResourceCreateCommand{}, fmt.Errorf("cloudevent datacontenttype must be application/json")
	}
	if event.Time().IsZero() {
		return mockfunction.ResourceCreateCommand{}, fmt.Errorf("cloudevent time is required")
	}
	var payload resourceCreateData
	if err := event.DataAs(&payload); err != nil {
		return mockfunction.ResourceCreateCommand{}, fmt.Errorf("parse cloudevent data: %w", err)
	}
	if event.Subject() != payload.WorkspaceID {
		return mockfunction.ResourceCreateCommand{}, fmt.Errorf("cloudevent subject must match data.workspace_id")
	}
	command := mockfunction.ResourceCreateCommand{
		WorkspaceID:  payload.WorkspaceID,
		AppName:      appName,
		ResourceName: payload.ResourceName,
		ResourceType: payload.ResourceType,
	}.Normalize()
	if strings.TrimSpace(command.WorkspaceID) == "" || strings.TrimSpace(command.ResourceName) == "" || strings.TrimSpace(command.ResourceType) == "" {
		return mockfunction.ResourceCreateCommand{}, fmt.Errorf("resource create command data contains empty required field")
	}
	return command, nil
}
