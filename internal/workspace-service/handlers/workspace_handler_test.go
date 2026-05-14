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
	result        services.CreateWorkspaceResult
	err           error
	input         workspace.CreateInput
	getResult     services.GetWorkspaceResult
	getErr        error
	getInput      workspace.GetQuery
	favoriteInput workspace.FavoriteInput
	favoriteErr   error
	favoriteCalls int
}

func (f *fakeHTTPWorkspaceService) CreateWorkspace(_ context.Context, input workspace.CreateInput) (services.CreateWorkspaceResult, error) {
	f.input = input
	if f.err != nil {
		return services.CreateWorkspaceResult{}, f.err
	}
	return f.result, nil
}

func (f *fakeHTTPWorkspaceService) GetWorkspace(_ context.Context, input workspace.GetQuery) (services.GetWorkspaceResult, error) {
	f.getInput = input
	if f.getErr != nil {
		return services.GetWorkspaceResult{}, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeHTTPWorkspaceService) SetWorkspaceFavorite(_ context.Context, input workspace.FavoriteInput) error {
	f.favoriteCalls++
	f.favoriteInput = input
	if f.favoriteErr != nil {
		return f.favoriteErr
	}
	return nil
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

func TestWorkspaceHandlerGetWorkspace(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{
		getResult: services.GetWorkspaceResult{
			Found: true,
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"workspace-1"`) || !strings.Contains(rec.Body.String(), `"display_name":"Test User 測試員"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if service.getInput.ID != "workspace-1" {
		t.Fatalf("service get input = %+v", service.getInput)
	}
}

func TestWorkspaceHandlerGetMissingWorkspace(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{
		getResult: services.GetWorkspaceResult{Found: false},
	}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/missing-workspace", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != `{"workspace":null}` {
		t.Fatalf("body = %s, want workspace null", rec.Body.String())
	}
}

func TestWorkspaceHandlerGetWorkspaceMapsInvalidInput(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{getErr: workspace.ErrInvalidInput}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/bad-workspace", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandlerGetWorkspaceMapsHRLookupFailure(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{getErr: services.ErrHRLookupFailed}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), `"code":"hr_lookup_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandlerGetWorkspaceMapsUnexpectedError(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{getErr: errors.New("database down")}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandlerSetWorkspaceFavorite(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := workspaceFavoriteRequest(`{"favorite":true}`)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if service.favoriteCalls != 1 {
		t.Fatalf("favorite calls = %d, want 1", service.favoriteCalls)
	}
	if service.favoriteInput.WorkspaceID != "workspace-1" || service.favoriteInput.NTAccount != "user1" || !service.favoriteInput.Favorite {
		t.Fatalf("favorite input = %+v, want workspace/user true", service.favoriteInput)
	}
}

func TestWorkspaceHandlerClearWorkspaceFavorite(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := workspaceFavoriteRequest(`{"favorite":false}`)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if service.favoriteInput.Favorite {
		t.Fatalf("favorite input = %+v, want favorite false", service.favoriteInput)
	}
}

func TestWorkspaceHandlerSetWorkspaceFavoriteRejectsMissingHeader(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/favorite", strings.NewReader(`{"favorite":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if service.favoriteCalls != 0 {
		t.Fatalf("favorite calls = %d, want 0", service.favoriteCalls)
	}
}

func TestWorkspaceHandlerSetWorkspaceFavoriteRejectsMissingFavorite(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := workspaceFavoriteRequest(`{}`)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if service.favoriteCalls != 0 {
		t.Fatalf("favorite calls = %d, want 0", service.favoriteCalls)
	}
}

func TestWorkspaceHandlerSetWorkspaceFavoriteMapsMissingWorkspace(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{favoriteErr: workspace.ErrNotFound}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := workspaceFavoriteRequest(`{"favorite":true}`)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"workspace_not_found"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandlerSetWorkspaceFavoriteMapsUnexpectedError(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{favoriteErr: errors.New("database down")}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := workspaceFavoriteRequest(`{"favorite":true}`)
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

func workspaceFavoriteRequest(body string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/favorite", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "user1")
	return req
}
