package config

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

func TestLoadReadsRequiredEnvironment(t *testing.T) {
	t.Setenv("GROUP_SERVICE_ENV", "production")
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":9090")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://example:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "wpm")
	t.Setenv("GROUP_SERVICE_SHUTDOWN_TIMEOUT", "15s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Production {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, environment.Production)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.MongoDB.URI != "mongodb://example:27017" {
		t.Fatalf("MongoDB.URI = %q, want mongodb://example:27017", cfg.MongoDB.URI)
	}
	if cfg.MongoDB.Database != "wpm" {
		t.Fatalf("MongoDB.Database = %q, want wpm", cfg.MongoDB.Database)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 15s", cfg.ShutdownTimeout)
	}
}

func TestLoadAppliesOptionalDefaults(t *testing.T) {
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Development {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, environment.Development)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 10s", cfg.ShutdownTimeout)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("GROUP_SERVICE_ENV", "staging")
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsMissingRequiredValue(t *testing.T) {
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsInvalidShutdownTimeout(t *testing.T) {
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("GROUP_SERVICE_SHUTDOWN_TIMEOUT", "0s")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}
