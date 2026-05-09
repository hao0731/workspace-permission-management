package transport

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func TestEncodeDecodeIndividualMemberNextToken(t *testing.T) {
	cursor := &group.IndividualMemberCursor{
		CreatedAt: time.Date(2026, 5, 9, 7, 31, 0, 0, time.UTC),
		ID:        "member-123",
	}

	token, err := EncodeIndividualMemberNextToken(cursor)
	if err != nil {
		t.Fatalf("Encode error = %v, want nil", err)
	}
	got, err := DecodeIndividualMemberNextToken(token)
	if err != nil {
		t.Fatalf("Decode error = %v, want nil", err)
	}
	if got.ID != cursor.ID || !got.CreatedAt.Equal(cursor.CreatedAt) {
		t.Fatalf("cursor = %+v, want %+v", got, cursor)
	}
}

func TestDecodeIndividualMemberNextTokenEmpty(t *testing.T) {
	cursor, err := DecodeIndividualMemberNextToken("")
	if err != nil {
		t.Fatalf("Decode error = %v, want nil", err)
	}
	if cursor != nil {
		t.Fatalf("cursor = %+v, want nil", cursor)
	}
}

func TestDecodeIndividualMemberNextTokenRejectsInvalidToken(t *testing.T) {
	_, err := DecodeIndividualMemberNextToken("not-base64")
	if err == nil {
		t.Fatal("Decode error = nil, want error")
	}
}
