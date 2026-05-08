package transport

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/shared/pagination"
)

type nextTokenPayload struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func EncodeNextToken(cursor *resource.Cursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	payload := nextTokenPayload{
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        cursor.ID,
	}
	return pagination.EncodeNextToken(payload)
}

func DecodeNextToken(token string) (*resource.Cursor, error) {
	payload, err := pagination.DecodeNextToken[nextTokenPayload](token)
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
	return &resource.Cursor{CreatedAt: createdAt, ID: payload.ID}, nil
}
