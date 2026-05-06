package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/labstack/echo/v5"
)

type fakeHTTPResourceService struct {
	query        resource.ListQuery
	page         resource.Page
	err          error
	deleteInput  resource.DeleteInput
	deleteStatus resource.DeleteStatus
	deleteErr    error
}

func (f *fakeHTTPResourceService) ListResources(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	f.query = query
	if f.err != nil {
		return resource.Page{}, f.err
	}
	return f.page, nil
}

func (f *fakeHTTPResourceService) DeleteResource(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error) {
	f.deleteInput = input
	if f.deleteErr != nil {
		return "", f.deleteErr
	}
	return f.deleteStatus, nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewResourceHandlerUsesProvidedLogger(t *testing.T) {
	logger := newTestLogger()
	handler := NewResourceHandler(&fakeHTTPResourceService{}, logger)

	if handler.logger != logger {
		t.Fatal("handler logger did not use provided logger")
	}
}

func TestResourceHandlerListResources(t *testing.T) {
	service := &fakeHTTPResourceService{
		page: resource.Page{
			Resources: []resource.Resource{{
				ID:          "resource-1",
				DisplayName: "Spec",
				Type:        "document",
				Tags:        []string{"section_1"},
				CreatedAt:   time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
			}},
		},
	}
	e := echo.New()
	handler := NewResourceHandler(service, newTestLogger())
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/functions/todo/resources", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if service.query.WorkspaceID != "workspace-1" || service.query.FunctionKey != "todo" {
		t.Fatalf("query = %+v, want workspace-1/todo", service.query)
	}
	if service.query.Limit != 20 {
		t.Fatalf("limit = %d, want 20", service.query.Limit)
	}
}

func TestResourceHandlerRejectsInvalidLimit(t *testing.T) {
	e := echo.New()
	handler := NewResourceHandler(&fakeHTTPResourceService{}, newTestLogger())
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/functions/todo/resources?limit=51", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestResourceHandlerDeleteResource(t *testing.T) {
	service := &fakeHTTPResourceService{deleteStatus: resource.DeleteStatusDeleted}
	e := echo.New()
	handler := NewResourceHandler(service, newTestLogger())
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/functions/todo/resources/resource-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body len = %d, want 0", rec.Body.Len())
	}
	if service.deleteInput.WorkspaceID != "workspace-1" || service.deleteInput.FunctionKey != "todo" || service.deleteInput.ResourceID != "resource-1" {
		t.Fatalf("delete input = %+v, want workspace-1/todo/resource-1", service.deleteInput)
	}
}

func TestResourceHandlerDeleteMissingResourceStillReturnsNoContent(t *testing.T) {
	service := &fakeHTTPResourceService{deleteStatus: resource.DeleteStatusNotFound}
	e := echo.New()
	handler := NewResourceHandler(service, newTestLogger())
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/functions/todo/resources/resource-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestResourceHandlerDeleteValidationError(t *testing.T) {
	service := &fakeHTTPResourceService{deleteErr: resource.ErrInvalidInput}
	e := echo.New()
	handler := NewResourceHandler(service, newTestLogger())
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/functions/todo/resources/resource-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestResourceHandlerDeleteServiceFailure(t *testing.T) {
	service := &fakeHTTPResourceService{deleteErr: errors.New("publish failed")}
	e := echo.New()
	handler := NewResourceHandler(service, newTestLogger())
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/functions/todo/resources/resource-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
