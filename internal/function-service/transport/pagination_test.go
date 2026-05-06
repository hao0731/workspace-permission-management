package transport

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func TestParseLimit(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "default", raw: "", want: 20},
		{name: "explicit", raw: "50", want: 50},
		{name: "too large", raw: "51", wantErr: true},
		{name: "zero", raw: "0", wantErr: true},
		{name: "not integer", raw: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLimit(tt.raw)
			if tt.wantErr && err == nil {
				t.Fatal("ParseLimit error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ParseLimit error = %v, want nil", err)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("limit = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEncodeDecodeNextToken(t *testing.T) {
	cursor := resource.Cursor{
		CreatedAt: time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
		ID:        "resource-123",
	}

	token, err := EncodeNextToken(&cursor)
	if err != nil {
		t.Fatalf("EncodeNextToken error = %v, want nil", err)
	}
	got, err := DecodeNextToken(token)
	if err != nil {
		t.Fatalf("DecodeNextToken error = %v, want nil", err)
	}
	if got.CreatedAt != cursor.CreatedAt {
		t.Fatalf("CreatedAt = %s, want %s", got.CreatedAt, cursor.CreatedAt)
	}
	if got.ID != cursor.ID {
		t.Fatalf("ID = %q, want %q", got.ID, cursor.ID)
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
