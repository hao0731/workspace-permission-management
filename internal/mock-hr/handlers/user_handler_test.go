package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/mock-hr/services"
	"github.com/labstack/echo/v5"
)

func TestGetUserReturnsFixedDisplayName(t *testing.T) {
	e := echo.New()
	handler := NewUserHandler(services.NewUserService(), slog.Default())
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/user1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"display_name":"Test User 測試員"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestGetUserRejectsEmptyAccount(t *testing.T) {
	e := echo.New()
	handler := NewUserHandler(services.NewUserService(), slog.Default())
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/%20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestBatchGetUsers(t *testing.T) {
	e := echo.New()
	handler := NewUserHandler(services.NewUserService(), slog.Default())
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/user-list", strings.NewReader(`{"nt_accounts":["user1","user2"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"users"`) || !strings.Contains(rec.Body.String(), `"nt_account":"user2"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestBatchGetUsersRejectsInvalidInput(t *testing.T) {
	e := echo.New()
	handler := NewUserHandler(services.NewUserService(), slog.Default())
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/user-list", strings.NewReader(`{"nt_accounts":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}
