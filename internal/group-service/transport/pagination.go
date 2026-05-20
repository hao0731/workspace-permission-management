package transport

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/pagination"
)

type individualMemberNextTokenPayload struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func EncodeIndividualMemberNextToken(cursor *group.IndividualMemberCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	return encodeCreatedAtIDNextToken(cursor.CreatedAt, cursor.ID)
}

func DecodeIndividualMemberNextToken(token string) (*group.IndividualMemberCursor, error) {
	createdAt, id, err := decodeCreatedAtIDNextToken(token)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, nil
	}
	return &group.IndividualMemberCursor{CreatedAt: createdAt, ID: id}, nil
}

func encodeCreatedAtIDNextToken(createdAt time.Time, id string) (string, error) {
	payload := individualMemberNextTokenPayload{
		CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
		ID:        id,
	}
	return pagination.EncodeNextToken(payload)
}

func decodeCreatedAtIDNextToken(token string) (time.Time, string, error) {
	payload, err := pagination.DecodeNextToken[individualMemberNextTokenPayload](token)
	if err != nil {
		if errors.Is(err, pagination.ErrEmptyToken) {
			return time.Time{}, "", nil
		}
		return time.Time{}, "", err
	}
	if strings.TrimSpace(payload.ID) == "" {
		return time.Time{}, "", fmt.Errorf("next_token.id is required")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("next_token.created_at must be RFC3339 timestamp")
	}
	return createdAt, strings.TrimSpace(payload.ID), nil
}
