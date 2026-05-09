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
	payload := individualMemberNextTokenPayload{
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        cursor.ID,
	}
	return pagination.EncodeNextToken(payload)
}

func DecodeIndividualMemberNextToken(token string) (*group.IndividualMemberCursor, error) {
	payload, err := pagination.DecodeNextToken[individualMemberNextTokenPayload](token)
	if err != nil {
		if errors.Is(err, pagination.ErrEmptyToken) {
			return nil, nil
		}
		return nil, err
	}
	if strings.TrimSpace(payload.ID) == "" {
		return nil, fmt.Errorf("next_token.id is required")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("next_token.created_at must be RFC3339 timestamp")
	}
	return &group.IndividualMemberCursor{CreatedAt: createdAt, ID: strings.TrimSpace(payload.ID)}, nil
}
