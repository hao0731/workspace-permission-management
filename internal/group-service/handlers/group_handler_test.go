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

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/labstack/echo/v5"
)

type fakeHTTPGroupService struct {
	input group.CreateInput
	model group.Group
	err   error
	calls int
}

func (f *fakeHTTPGroupService) CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return group.Group{}, f.err
	}
	return f.model, nil
}

func validGroupRequestBody() string {
	return `{
		"name": "Design Reviewers",
		"description": "Employees who can review design documents.",
		"grouping_rule": {
			"rules": [
				{
					"attribute_key": "department",
					"operator": "eq",
					"multi": false,
					"value": "ABCD-123"
				}
			],
			"expiration_date": "2026-06-01T00:00:00Z"
		},
		"individual_members": [
			{
				"nt_account": "user1",
				"expiration_date": "2026-06-01T00:00:00Z"
			}
		]
	}`
}

func groupModel() group.Group {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return group.Group{
		ID:             "group-1",
		WorkspaceID:    "workspace-1",
		Name:           "Design Reviewers",
		NormalizedName: "Design Reviewers",
		Description:    "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: "department",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: expiration,
		},
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-1",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: expiration,
		}},
	}
}

func TestGroupHandlerCreateGroup(t *testing.T) {
	service := &fakeHTTPGroupService{model: groupModel()}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if service.calls != 1 {
		t.Fatalf("service calls = %d, want 1", service.calls)
	}
	if service.input.WorkspaceID != "workspace-1" {
		t.Fatalf("WorkspaceID = %q, want workspace-1", service.input.WorkspaceID)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["group"]; !ok {
		t.Fatal("response missing group")
	}
}

func TestGroupHandlerRejectsMalformedJSON(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(`{"name":`))
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

func TestGroupHandlerValidationError(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrInvalidInput}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGroupHandlerDuplicateName(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrDuplicateName}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestGroupHandlerServiceFailure(t *testing.T) {
	service := &fakeHTTPGroupService{err: errors.New("database unavailable")}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
