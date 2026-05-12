package transport

import (
	"strings"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type individualMemberExpiryCommandData struct {
	TaskID           string `json:"task_id"`
	GroupID          string `json:"group_id"`
	NTAccount        string `json:"nt_account"`
	ExpirationBucket string `json:"expiration_bucket"`
}

func ParseIndividualMemberExpiryCommandEvent(data []byte, expectedType string) (group.ExpireIndividualMemberCommand, error) {
	return parseExpiryCommandEvent[individualMemberExpiryCommandData, group.ExpireIndividualMemberCommand](
		data,
		expectedType,
		"individual member expiry command data contains empty required field",
	)
}

func (payload individualMemberExpiryCommandData) taskID() string {
	return payload.TaskID
}

func (payload individualMemberExpiryCommandData) hasEmptyRequiredField() bool {
	return strings.TrimSpace(payload.TaskID) == "" ||
		strings.TrimSpace(payload.GroupID) == "" ||
		strings.TrimSpace(payload.NTAccount) == "" ||
		strings.TrimSpace(payload.ExpirationBucket) == ""
}

func (payload individualMemberExpiryCommandData) expirationBucket() string {
	return payload.ExpirationBucket
}

func (payload individualMemberExpiryCommandData) toCommand() group.ExpireIndividualMemberCommand {
	return group.ExpireIndividualMemberCommand{
		TaskID:           payload.TaskID,
		GroupID:          payload.GroupID,
		NTAccount:        payload.NTAccount,
		ExpirationBucket: payload.ExpirationBucket,
	}.Normalize()
}
