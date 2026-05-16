package transport

import (
	"encoding/json"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const eventSource = "group-expiry-scheduler"

type GroupExpiryCommand struct {
	TaskID           string `json:"task_id"`
	WorkspaceID      string `json:"workspace_id"`
	GroupID          string `json:"group_id"`
	ExpirationBucket string `json:"expiration_bucket"`
}

type IndividualMemberExpiryCommand struct {
	TaskID           string `json:"task_id"`
	GroupID          string `json:"group_id"`
	NTAccount        string `json:"nt_account"`
	ExpirationBucket string `json:"expiration_bucket"`
}

func NewGroupExpiryCommandEvent(command GroupExpiryCommand, eventType string, eventID string, eventTime time.Time) ([]byte, error) {
	event := newCommandEvent(command.TaskID, eventType, eventID, eventTime)
	if err := event.SetData(cloudevents.ApplicationJSON, command); err != nil {
		return nil, fmt.Errorf("set group expiry command data: %w", err)
	}
	return marshalEvent(event)
}

func NewIndividualMemberExpiryCommandEvent(command IndividualMemberExpiryCommand, eventType string, eventID string, eventTime time.Time) ([]byte, error) {
	event := newCommandEvent(command.TaskID, eventType, eventID, eventTime)
	if err := event.SetData(cloudevents.ApplicationJSON, command); err != nil {
		return nil, fmt.Errorf("set individual member expiry command data: %w", err)
	}
	return marshalEvent(event)
}

func newCommandEvent(taskID string, eventType string, eventID string, eventTime time.Time) cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetSubject(taskID)
	event.SetID(eventID)
	event.SetTime(eventTime)
	return event
}

func marshalEvent(event cloudevents.Event) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal expiry command event: %w", err)
	}
	return data, nil
}
