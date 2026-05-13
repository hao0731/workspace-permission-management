package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mockfunctionconfig "github.com/hao0731/workspace-permission-management/internal/mock-function/config"
	"github.com/labstack/echo/v5"
)

func TestProcessIndicator(t *testing.T) {
	indicator := processIndicator{}
	if indicator.Name() != "process" {
		t.Fatalf("Name() = %q, want process", indicator.Name())
	}
	if !indicator.IsHealthy(context.Background()) {
		t.Fatal("IsHealthy() = false, want true")
	}
}

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

func TestNewResourceCreateEventbusConfig(t *testing.T) {
	cfg := mockfunctionconfig.Config{
		ResourceCreate: mockfunctionconfig.ResourceCreateConfig{
			Stream:     "RESOURCE_CREATE_COMMANDS",
			Durable:    "mock-function-resource-create",
			FetchCount: 25,
			MaxWait:    7 * time.Second,
		},
	}

	got := newResourceCreateEventbusConfig(cfg)

	if got.Stream != "RESOURCE_CREATE_COMMANDS" {
		t.Fatalf("Stream = %q, want RESOURCE_CREATE_COMMANDS", got.Stream)
	}
	if got.Durable != "mock-function-resource-create" {
		t.Fatalf("Durable = %q, want mock-function-resource-create", got.Durable)
	}
	if len(got.Subjects) != 1 || got.Subjects[0] != "cmd.app.*.resource.create" {
		t.Fatalf("Subjects = %v, want [cmd.app.*.resource.create]", got.Subjects)
	}
	if got.BatchSize != 25 {
		t.Fatalf("BatchSize = %d, want 25", got.BatchSize)
	}
	if got.MaxWait != 7*time.Second {
		t.Fatalf("MaxWait = %s, want 7s", got.MaxWait)
	}
}
