package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	schedulerconfig "github.com/hao0731/workspace-permission-management/internal/group-expiry-scheduler/config"
	"github.com/labstack/echo/v5"
)

func TestProcessIndicator(t *testing.T) {
	indicator := processIndicator{}
	if indicator.Name() != "process" {
		t.Fatalf("Name = %q, want process", indicator.Name())
	}
	if !indicator.IsHealthy(context.Background()) {
		t.Fatal("IsHealthy = false, want true")
	}
}

func TestRegisterHealthRoutes(t *testing.T) {
	e := echo.New()
	registerHealthRoutes(e)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/liveness", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestNewGocronScheduler(t *testing.T) {
	cfg := schedulerconfig.Config{
		Schedule: schedulerconfig.ScheduleConfig{
			Expression:  "* * * * *",
			WithSeconds: false,
			Location:    time.UTC,
		},
		ShutdownTimeout: time.Second,
	}

	scheduler, err := newGocronScheduler(cfg, func(context.Context) {})
	if err != nil {
		t.Fatalf("newGocronScheduler error = %v, want nil", err)
	}
	if err := scheduler.Shutdown(); err != nil {
		t.Fatalf("Shutdown error = %v", err)
	}
}
