package config

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

func TestConfigValidateRequiresHTTPAddr(t *testing.T) {
	cfg := Config{Environment: environment.Development, ShutdownTimeout: time.Second}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestConfigValidateRejectsInvalidEnvironment(t *testing.T) {
	cfg := Config{Environment: environment.Environment("invalid"), HTTPAddr: ":8082", ShutdownTimeout: time.Second}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestConfigValidateRequiresPositiveShutdownTimeout(t *testing.T) {
	cfg := Config{Environment: environment.Development, HTTPAddr: ":8082"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	cfg := Config{Environment: environment.Development, HTTPAddr: ":8082", ShutdownTimeout: time.Second}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}
