package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
	"github.com/labstack/echo/v5"
)

type fakeHTTPPermissionService struct {
	input permission.SaveInput
	model permission.Permission
	err   error
	calls int
}

func (f *fakeHTTPPermissionService) SavePermission(ctx context.Context, input permission.SaveInput) (permission.Permission, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return permission.Permission{}, f.err
	}
	return f.model, nil
}

func validPermissionRequestBody() string {
	return `{
		"office_permission": {
			"baseline_rule": {
				"action_id": "view",
				"resource_tags": ["section_1"],
				"enabled": true
			},
			"extra_rules": [
				{
					"group_ids": ["group-1"],
					"action_id": "edit",
					"resource_tags": ["section_1"],
					"expiration_date": "2026-06-01T00:00:00Z"
				}
			]
		},
		"remote_permission": {
			"baseline_rule": {
				"action_id": "view",
				"resource_tags": ["remote"],
				"enabled": false
			},
			"extra_rules": []
		}
	}`
}

func permissionModel() permission.Permission {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return permission.Permission{
		ID:          "permission-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		OfficePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []permission.ExtraRule{{
				RuleID:         "rule-generated-1",
				GroupIDs:       []string{"group-1"},
				ActionID:       "edit",
				ResourceTags:   []string{"section_1"},
				ExpirationDate: expiration,
			}},
		},
		RemotePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	}
}

func TestPermissionHandlerSavePermissions(t *testing.T) {
	service := &fakeHTTPPermissionService{model: permissionModel()}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/functions/todo/permissions", strings.NewReader(validPermissionRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.calls != 1 {
		t.Fatalf("service calls = %d, want 1", service.calls)
	}
	if service.input.WorkspaceID != "workspace-1" || service.input.FunctionKey != "todo" {
		t.Fatalf("input identity = %s/%s, want workspace-1/todo", service.input.WorkspaceID, service.input.FunctionKey)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["permissions"]; !ok {
		t.Fatal("response missing permissions")
	}
}

func TestPermissionHandlerRejectsMalformedJSON(t *testing.T) {
	service := &fakeHTTPPermissionService{}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/functions/todo/permissions", strings.NewReader(`{"office_permission":`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if service.calls != 0 {
		t.Fatalf("service calls = %d, want 0", service.calls)
	}
}

func TestPermissionHandlerValidationError(t *testing.T) {
	service := &fakeHTTPPermissionService{err: permission.ErrInvalidInput}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/functions/todo/permissions", strings.NewReader(validPermissionRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPermissionHandlerServiceFailure(t *testing.T) {
	service := &fakeHTTPPermissionService{err: errors.New("database unavailable")}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/functions/todo/permissions", strings.NewReader(validPermissionRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
