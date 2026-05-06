package main

import (
	"context"
	"log/slog"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
)

func TestLoggerNew(t *testing.T) {
	if sharedlogger.New(environment.Development) == nil {
		t.Fatal("development logger = nil, want logger")
	}
	if sharedlogger.New(environment.Production) == nil {
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

func TestLoggerNewReturnsSlogLogger(t *testing.T) {
	var logger *slog.Logger = sharedlogger.New(environment.Development)
	if logger == nil {
		t.Fatal("logger = nil, want slog logger")
	}
}
