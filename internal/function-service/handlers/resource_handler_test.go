package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/labstack/echo/v5"
)

type fakeHTTPResourceService struct {
	query resource.ListQuery
	page  resource.Page
	err   error
}

func (f *fakeHTTPResourceService) ListResources(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	f.query = query
	if f.err != nil {
		return resource.Page{}, f.err
	}
	return f.page, nil
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
	handler := NewResourceHandler(service)
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
	handler := NewResourceHandler(&fakeHTTPResourceService{})
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/functions/todo/resources?limit=51", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
