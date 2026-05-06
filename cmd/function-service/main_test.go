package main

import (
	"context"
	"log/slog"
	"testing"

	serviceconfig "github.com/hao0731/workspace-permission-management/internal/function-service/config"
)

func TestNewLogger(t *testing.T) {
	if newLogger(serviceconfig.EnvironmentDevelopment) == nil {
		t.Fatal("development logger = nil, want logger")
	}
	if newLogger(serviceconfig.EnvironmentProduction) == nil {
		t.Fatal("production logger = nil, want logger")
	}
}

func TestProcessIndicator(t *testing.T) {
	indicator := processIndicator{}
	if indicator.Name() != "process" {
		t.Fatalf("Name = %q, want process", indicator.Name())
	}
	if !indicator.IsHealthy(context.Background()) {
		t.Fatal("IsHealthy = false, want true")
	}
}

func TestNewLoggerReturnsSlogLogger(t *testing.T) {
	var logger *slog.Logger = newLogger(serviceconfig.EnvironmentDevelopment)
	if logger == nil {
		t.Fatal("logger = nil, want slog logger")
	}
}
