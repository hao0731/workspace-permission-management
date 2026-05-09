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
	t.Setenv("GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS", "250")
	t.Setenv("GROUP_SERVICE_MAX_GROUPING_RULES", "5")

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
	if cfg.Validation.MaxIndividualMembers != 250 {
		t.Fatalf("MaxIndividualMembers = %d, want 250", cfg.Validation.MaxIndividualMembers)
	}
	if cfg.Validation.MaxGroupingRules != 5 {
		t.Fatalf("MaxGroupingRules = %d, want 5", cfg.Validation.MaxGroupingRules)
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
	if cfg.Validation.MaxIndividualMembers != 1000 {
		t.Fatalf("MaxIndividualMembers = %d, want 1000", cfg.Validation.MaxIndividualMembers)
	}
	if cfg.Validation.MaxGroupingRules != 10 {
		t.Fatalf("MaxGroupingRules = %d, want 10", cfg.Validation.MaxGroupingRules)
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

func TestLoadRejectsInvalidValidationLimits(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			name: "max individual members",
			key:  "GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS",
		},
		{
			name: "max grouping rules",
			key:  "GROUP_SERVICE_MAX_GROUPING_RULES",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
			t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
			t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
			t.Setenv(tt.key, "0")

			if _, err := Load(); err == nil {
				t.Fatal("Load error = nil, want error")
			}
		})
	}
}
