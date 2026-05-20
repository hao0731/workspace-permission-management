package transport

import (
	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func EncodeSystemGroupNextToken(cursor *group.SystemGroupCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	return encodeCreatedAtIDNextToken(cursor.CreatedAt, cursor.ID)
}

func DecodeSystemGroupNextToken(token string) (*group.SystemGroupCursor, error) {
	createdAt, id, err := decodeCreatedAtIDNextToken(token)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, nil
	}
	return &group.SystemGroupCursor{CreatedAt: createdAt, ID: id}, nil
}
