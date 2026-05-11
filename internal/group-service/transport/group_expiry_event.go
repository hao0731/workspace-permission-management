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
	return parseExpiryCommandEvent[groupExpiryCommandData, group.ExpireGroupingRuleCommand](
		data,
		expectedType,
		"group expiry command data contains empty required field",
	)
}

func (payload groupExpiryCommandData) taskID() string {
	return payload.TaskID
}

func (payload groupExpiryCommandData) hasEmptyRequiredField() bool {
	return strings.TrimSpace(payload.TaskID) == "" ||
		strings.TrimSpace(payload.WorkspaceID) == "" ||
		strings.TrimSpace(payload.GroupID) == "" ||
		strings.TrimSpace(payload.ExpirationBucket) == ""
}

func (payload groupExpiryCommandData) expirationBucket() string {
	return payload.ExpirationBucket
}

func (payload groupExpiryCommandData) toCommand() group.ExpireGroupingRuleCommand {
	return group.ExpireGroupingRuleCommand{
		TaskID:           payload.TaskID,
		WorkspaceID:      payload.WorkspaceID,
		GroupID:          payload.GroupID,
		ExpirationBucket: payload.ExpirationBucket,
	}.Normalize()
}

type expiryCommandEventPayload[C any] interface {
	taskID() string
	hasEmptyRequiredField() bool
	expirationBucket() string
	toCommand() C
}

func parseExpiryCommandEvent[P expiryCommandEventPayload[C], C any](data []byte, expectedType string, emptyFieldMessage string) (C, error) {
	var zero C

	event, err := parseCommandCloudEvent(data, expectedType)
	if err != nil {
		return zero, err
	}

	var payload P
	if err := event.DataAs(&payload); err != nil {
		return zero, fmt.Errorf("parse cloudevent data: %w", err)
	}
	if event.Subject() != payload.taskID() {
		return zero, fmt.Errorf("cloudevent subject must match data.task_id")
	}
	if payload.hasEmptyRequiredField() {
		return zero, fmt.Errorf("%s", emptyFieldMessage)
	}
	if !group.IsValidExpirationBucket(payload.expirationBucket()) {
		return zero, fmt.Errorf("expiration_bucket must use yyyy-MM-dd")
	}
	return payload.toCommand(), nil
}

func parseCommandCloudEvent(data []byte, expectedType string) (cloudevents.Event, error) {
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return cloudevents.Event{}, fmt.Errorf("parse cloudevent: %w", err)
	}
	if err := event.Validate(); err != nil {
		return cloudevents.Event{}, fmt.Errorf("validate cloudevent: %w", err)
	}
	if event.SpecVersion() != cloudEventSpecVersion {
		return cloudevents.Event{}, fmt.Errorf("unsupported cloudevent specversion %q", event.SpecVersion())
	}
	if event.Type() != expectedType {
		return cloudevents.Event{}, fmt.Errorf("cloudevent type %q does not match expected %q", event.Type(), expectedType)
	}
	if event.DataContentType() != "application/json" {
		return cloudevents.Event{}, fmt.Errorf("cloudevent datacontenttype must be application/json")
	}
	if event.Time().IsZero() {
		return cloudevents.Event{}, fmt.Errorf("cloudevent time is required")
	}
	return event, nil
}
