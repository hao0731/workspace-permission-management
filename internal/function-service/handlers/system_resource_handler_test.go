package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/labstack/echo/v5"
)

type fakeHTTPSystemResourceService struct {
	saveInput  resource.ResourceDefinitionSaveInput
	saveResult []resource.ResourceDefinition
	saveErr    error
	listQuery  resource.ResourceDefinitionsQuery
	listResult []resource.ResourceDefinition
	listErr    error
	attrQuery  resource.ResourceAttributesQuery
	attrs      []resource.ResourceAttribute
	attrErr    error
}

func (f *fakeHTTPSystemResourceService) SaveSystemResources(ctx context.Context, input resource.ResourceDefinitionSaveInput) ([]resource.ResourceDefinition, error) {
	f.saveInput = input
	if f.saveErr != nil {
		return nil, f.saveErr
	}
	return f.saveResult, nil
}

func (f *fakeHTTPSystemResourceService) ListSystemResources(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error) {
	f.listQuery = query
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}

func (f *fakeHTTPSystemResourceService) GetSystemResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) ([]resource.ResourceAttribute, error) {
	f.attrQuery = query
	if f.attrErr != nil {
		return nil, f.attrErr
	}
	return f.attrs, nil
}

func TestSystemResourceHandlerSaveSystemResources(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	service := &fakeHTTPSystemResourceService{
		saveResult: []resource.ResourceDefinition{{SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit", CreatedAt: now, UpdatedAt: now}},
	}
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(service, newTestLogger()))
	body := bytes.NewBufferString(`{"resources":[{"type":"action","label":"Can Edit","key":"can_edit"}]}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/systems/todo/resources", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.saveInput.SystemID != "todo" || service.saveInput.Resources[0].Key != "can_edit" {
		t.Fatalf("input = %+v, want todo/can_edit", service.saveInput)
	}
}

func TestSystemResourceHandlerRejectsInvalidJSON(t *testing.T) {
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(&fakeHTTPSystemResourceService{}, newTestLogger()))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/systems/todo/resources", bytes.NewBufferString(`{"resources":`))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSystemResourceHandlerListSystemResources(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	service := &fakeHTTPSystemResourceService{
		listResult: []resource.ResourceDefinition{{SystemID: "todo", Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private", CreatedAt: now, UpdatedAt: now}},
	}
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(service, newTestLogger()))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/systems/todo/resources", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.listQuery.SystemID != "todo" {
		t.Fatalf("query = %+v, want todo", service.listQuery)
	}
}

func TestSystemResourceHandlerGetAttributesEmpty(t *testing.T) {
	service := &fakeHTTPSystemResourceService{}
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(service, newTestLogger()))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/systems/todo/resource-attributes", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response map[string][]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response["resource_attributes"]) != 0 {
		t.Fatalf("attributes = %#v, want empty", response["resource_attributes"])
	}
}

func TestSystemResourceHandlerServiceFailure(t *testing.T) {
	service := &fakeHTTPSystemResourceService{listErr: errors.New("database unavailable")}
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(service, newTestLogger()))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/systems/todo/resources", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
