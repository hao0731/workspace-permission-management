package pagination

import (
	"strings"
	"testing"
	"time"
)

type tokenPayload struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func TestEncodeDecodeNextToken(t *testing.T) {
	in := tokenPayload{CreatedAt: time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC).Format(time.RFC3339Nano), ID: "resource-123"}
	token, err := EncodeNextToken(in)
	if err != nil {
		t.Fatalf("EncodeNextToken error = %v, want nil", err)
	}
	if strings.TrimSpace(token) == "" {
		t.Fatal("token empty, want non-empty")
	}
	out, err := DecodeNextToken[tokenPayload](token)
	if err != nil {
		t.Fatalf("DecodeNextToken error = %v, want nil", err)
	}
	if out != in {
		t.Fatalf("decode = %+v, want %+v", out, in)
	}
}

func TestDecodeNextTokenInvalid(t *testing.T) {
	if _, err := DecodeNextToken[tokenPayload]("not-base64"); err == nil {
		t.Fatal("DecodeNextToken error = nil, want error")
	}
}
