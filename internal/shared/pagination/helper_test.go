package pagination

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func newContext(t *testing.T, rawURL string) *echo.Context {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c
}

func TestNewDefaults(t *testing.T) {
	helper := New()
	c := newContext(t, "/")
	limit, err := helper.ParseLimit(c)
	if err != nil {
		t.Fatalf("ParseLimit error = %v, want nil", err)
	}
	if limit != 20 {
		t.Fatalf("limit = %d, want 20", limit)
	}
}

func TestParseLimitWithOptions(t *testing.T) {
	helper := New(WithDefaultLimit(10), WithMaxLimit(30))
	c := newContext(t, "/")
	limit, err := helper.ParseLimit(c)
	if err != nil {
		t.Fatalf("ParseLimit error = %v, want nil", err)
	}
	if limit != 10 {
		t.Fatalf("limit = %d, want 10", limit)
	}
	c = newContext(t, "/?limit=31")
	if _, err := helper.ParseLimit(c); err == nil {
		t.Fatal("ParseLimit error = nil, want error")
	}
}

func TestParseToken(t *testing.T) {
	helper := New()
	c := newContext(t, "/")
	if _, err := helper.ParseToken(c); err == nil {
		t.Fatal("ParseToken error = nil, want error")
	}

	c = newContext(t, "/?next_token=abc")
	token, err := helper.ParseToken(c)
	if err != nil {
		t.Fatalf("ParseToken error = %v, want nil", err)
	}
	if token != "abc" {
		t.Fatalf("token = %q, want abc", token)
	}
}
