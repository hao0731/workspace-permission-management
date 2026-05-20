package transport

import (
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func TestEncodeDecodeSystemGroupNextToken(t *testing.T) {
	cursor := &group.SystemGroupCursor{CreatedAt: time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC), ID: "group-1"}
	token, err := EncodeSystemGroupNextToken(cursor)
	if err != nil {
		t.Fatalf("EncodeSystemGroupNextToken error = %v, want nil", err)
	}
	out, err := DecodeSystemGroupNextToken(token)
	if err != nil {
		t.Fatalf("DecodeSystemGroupNextToken error = %v, want nil", err)
	}
	if !out.CreatedAt.Equal(cursor.CreatedAt) || out.ID != cursor.ID {
		t.Fatalf("cursor = %+v, want %+v", out, cursor)
	}
}

func TestDecodeSystemGroupNextTokenEmpty(t *testing.T) {
	cursor, err := DecodeSystemGroupNextToken("")
	if err != nil {
		t.Fatalf("DecodeSystemGroupNextToken error = %v, want nil", err)
	}
	if cursor != nil {
		t.Fatalf("cursor = %+v, want nil", cursor)
	}
}

func TestDecodeSystemGroupNextTokenInvalid(t *testing.T) {
	_, err := DecodeSystemGroupNextToken("not-base64")
	if err == nil || errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("DecodeSystemGroupNextToken error = %v, want raw pagination decode error for handler mapping", err)
	}
}
