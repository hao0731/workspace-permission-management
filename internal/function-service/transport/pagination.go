package transport

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

const (
	DefaultLimit = 20
	MaxLimit     = 50
)

type nextTokenPayload struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func ParseLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return DefaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("limit must be an integer")
	}
	if limit < 1 {
		return 0, fmt.Errorf("limit must be greater than zero")
	}
	if limit > MaxLimit {
		return 0, fmt.Errorf("limit must be less than or equal to %d", MaxLimit)
	}
	return limit, nil
}

func EncodeNextToken(cursor *resource.Cursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	payload := nextTokenPayload{
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        cursor.ID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal next token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func DecodeNextToken(token string) (*resource.Cursor, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("next_token must be base64url encoded JSON")
	}
	var payload nextTokenPayload
	if unmarshalErr := json.Unmarshal(data, &payload); unmarshalErr != nil {
		return nil, fmt.Errorf("next_token must be JSON")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("next_token.created_at must be RFC3339 timestamp")
	}
	if strings.TrimSpace(payload.ID) == "" {
		return nil, fmt.Errorf("next_token.id is required")
	}
	return &resource.Cursor{CreatedAt: createdAt, ID: payload.ID}, nil
}
