package transport

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func TestEncodeDecodeNextToken(t *testing.T) {
	cursor := resource.Cursor{CreatedAt: time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC), ID: "resource-123"}
	token, err := EncodeNextToken(&cursor)
	if err != nil {
		t.Fatalf("EncodeNextToken error = %v, want nil", err)
	}
	got, err := DecodeNextToken(token)
	if err != nil {
		t.Fatalf("DecodeNextToken error = %v, want nil", err)
	}
	if got.CreatedAt != cursor.CreatedAt || got.ID != cursor.ID {
		t.Fatalf("decode = %+v, want %+v", got, &cursor)
	}
}

func TestEncodeNextTokenEmptyCursor(t *testing.T) {
	token, err := EncodeNextToken(nil)
	if err != nil {
		t.Fatalf("EncodeNextToken error = %v, want nil", err)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty", token)
	}
}

func TestDecodeNextTokenRejectsInvalidToken(t *testing.T) {
	if _, err := DecodeNextToken("not-base64"); err == nil {
		t.Fatal("DecodeNextToken error = nil, want error")
	}
}
