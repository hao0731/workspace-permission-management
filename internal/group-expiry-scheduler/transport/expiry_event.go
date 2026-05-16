package transport

import (
	"encoding/json"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

const eventSource = "group-expiry-scheduler"

type groupExpiryCommandData struct {
	TaskID           string `json:"task_id"`
	WorkspaceID      string `json:"workspace_id"`
	GroupID          string `json:"group_id"`
	ExpirationBucket string `json:"expiration_bucket"`
}

type individualMemberExpiryCommandData struct {
	TaskID           string `json:"task_id"`
	GroupID          string `json:"group_id"`
	NTAccount        string `json:"nt_account"`
	ExpirationBucket string `json:"expiration_bucket"`
}

func NewGroupExpiryCommandEvent(task expiry.GroupTask, eventType string, eventID string, eventTime time.Time) ([]byte, error) {
	event := newCommandEvent(task.ID, eventType, eventID, eventTime)
	if err := event.SetData(cloudevents.ApplicationJSON, groupExpiryCommandData{
		TaskID:           task.ID,
		WorkspaceID:      task.WorkspaceID,
		GroupID:          task.GroupID,
		ExpirationBucket: task.ExpirationBucket,
	}); err != nil {
		return nil, fmt.Errorf("set group expiry command data: %w", err)
	}
	return marshalEvent(event)
}

func NewIndividualMemberExpiryCommandEvent(task expiry.IndividualMemberTask, eventType string, eventID string, eventTime time.Time) ([]byte, error) {
	event := newCommandEvent(task.ID, eventType, eventID, eventTime)
	if err := event.SetData(cloudevents.ApplicationJSON, individualMemberExpiryCommandData{
		TaskID:           task.ID,
		GroupID:          task.GroupID,
		NTAccount:        task.NTAccount,
		ExpirationBucket: task.ExpirationBucket,
	}); err != nil {
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
