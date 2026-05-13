package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
	"github.com/hao0731/workspace-permission-management/internal/workspace-service/services"
	"github.com/labstack/echo/v5"
)

type fakeHTTPWorkspaceService struct {
	result services.CreateWorkspaceResult
	err    error
	input  workspace.CreateInput
}

func (f *fakeHTTPWorkspaceService) CreateWorkspace(_ context.Context, input workspace.CreateInput) (services.CreateWorkspaceResult, error) {
	f.input = input
	if f.err != nil {
		return services.CreateWorkspaceResult{}, f.err
	}
	return f.result, nil
}

func TestWorkspaceHandlerCreateWorkspace(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{
		result: services.CreateWorkspaceResult{
			Workspace: workspace.Workspace{
				ID:             "workspace-1",
				Name:           "Planning",
				Description:    "Planning workspace",
				OwnerNTAccount: "user1",
			},
			Owner: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"},
		},
	}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces", strings.NewReader(`{
		"name":"Planning",
		"description":"Planning workspace",
		"owner":"user1"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"display_name":"Test User 測試員"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if service.input.OwnerNTAccount != "user1" {
		t.Fatalf("service input = %+v", service.input)
	}
}

func TestWorkspaceHandlerRejectsMalformedJSON(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, NewWorkspaceHandler(&fakeHTTPWorkspaceService{}, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces", strings.NewReader(`{"name":`))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandlerMapsInvalidInput(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, NewWorkspaceHandler(&fakeHTTPWorkspaceService{}, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces", strings.NewReader(`{
		"name":"Planning",
		"description":"Planning workspace"
	}`))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandlerMapsHRLookupFailure(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{err: services.ErrHRLookupFailed}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := validWorkspaceRequest()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), `"code":"hr_lookup_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandlerMapsUnexpectedError(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{err: errors.New("database down")}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := validWorkspaceRequest()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func validWorkspaceRequest() *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces", strings.NewReader(`{
		"name":"Planning",
		"description":"Planning workspace",
		"owner":"user1"
	}`))
	req.Header.Set("Content-Type", "application/json")
	return req
}
