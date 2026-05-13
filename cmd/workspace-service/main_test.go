package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
	workspaceconfig "github.com/hao0731/workspace-permission-management/internal/workspace-service/config"
	"github.com/labstack/echo/v5"
)

func TestRegisterHealthRoutes(t *testing.T) {
	e := echo.New()
	registerHealthRoutes(e)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/liveness", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestNewServiceResourceMappings(t *testing.T) {
	cfg := workspaceconfig.Config{
		Environment: environment.Development,
		ResourceMappings: workspaceconfig.ResourceMappings{
			Documents: workspaceconfig.ResourceMapping{AppName: "documents", ResourceType: "document"},
			Tasks:     workspaceconfig.ResourceMapping{AppName: "tasks", ResourceType: "task"},
			Drive:     workspaceconfig.ResourceMapping{AppName: "drive", ResourceType: "file"},
		},
		CommandPublishTimeout: time.Second,
	}
	mappings := newServiceResourceMappings(cfg)
	if mappings.Documents.AppName != "documents" || mappings.Tasks.ResourceType != "task" || mappings.Drive.AppName != "drive" {
		t.Fatalf("mappings = %+v", mappings)
	}
}
