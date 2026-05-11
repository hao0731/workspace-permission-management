package transport

import (
	"encoding/json"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

const cloudEventSpecVersion = "1.0"

type groupExpiryCommandData struct {
	TaskID           string `json:"task_id"`
	WorkspaceID      string `json:"workspace_id"`
	GroupID          string `json:"group_id"`
	ExpirationBucket string `json:"expiration_bucket"`
}

func ParseGroupExpiryCommandEvent(data []byte, expectedType string) (group.ExpireGroupingRuleCommand, error) {
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("parse cloudevent: %w", err)
	}
	if err := event.Validate(); err != nil {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("validate cloudevent: %w", err)
	}
	if event.SpecVersion() != cloudEventSpecVersion {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("unsupported cloudevent specversion %q", event.SpecVersion())
	}
	if event.Type() != expectedType {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("cloudevent type %q does not match expected %q", event.Type(), expectedType)
	}
	if event.DataContentType() != "application/json" {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("cloudevent datacontenttype must be application/json")
	}
	if event.Time().IsZero() {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("cloudevent time is required")
	}

	var payload groupExpiryCommandData
	if err := event.DataAs(&payload); err != nil {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("parse cloudevent data: %w", err)
	}
	if event.Subject() != payload.TaskID {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("cloudevent subject must match data.task_id")
	}
	if strings.TrimSpace(payload.TaskID) == "" ||
		strings.TrimSpace(payload.WorkspaceID) == "" ||
		strings.TrimSpace(payload.GroupID) == "" ||
		strings.TrimSpace(payload.ExpirationBucket) == "" {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("group expiry command data contains empty required field")
	}
	command := group.ExpireGroupingRuleCommand{
		TaskID:           payload.TaskID,
		WorkspaceID:      payload.WorkspaceID,
		GroupID:          payload.GroupID,
		ExpirationBucket: payload.ExpirationBucket,
	}.Normalize()
	if !group.IsValidExpirationBucket(command.ExpirationBucket) {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("expiration_bucket must use yyyy-MM-dd")
	}
	return command, nil
}
