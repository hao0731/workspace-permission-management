package config

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

func validConfig() Config {
	return Config{
		Environment: environment.Development,
		HTTPAddr:    ":8083",
		MongoDB: MongoDBConfig{
			URI:      "mongodb://localhost:27017",
			Database: "workspace_permission_management",
		},
		NATS: NATSConfig{URL: "nats://localhost:4222"},
		HR:   HRConfig{BaseURL: "http://localhost:8082"},
		ResourceMappings: ResourceMappings{
			Documents: ResourceMapping{AppName: "documents", ResourceType: "document"},
			Tasks:     ResourceMapping{AppName: "tasks", ResourceType: "task"},
			Drive:     ResourceMapping{AppName: "drive", ResourceType: "file"},
		},
		CommandPublishTimeout: time.Second,
		ShutdownTimeout:       time.Second,
	}
}

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestConfigValidateRequiresHTTPAddr(t *testing.T) {
	cfg := validConfig()
	cfg.HTTPAddr = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestConfigValidateRejectsInvalidHRBaseURL(t *testing.T) {
	cfg := validConfig()
	cfg.HR.BaseURL = "://bad"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestConfigValidateRejectsInvalidAppName(t *testing.T) {
	cfg := validConfig()
	cfg.ResourceMappings.Documents.AppName = "bad.name"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestConfigValidateRequiresPositiveTimeouts(t *testing.T) {
	cfg := validConfig()
	cfg.CommandPublishTimeout = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() command timeout error = nil, want error")
	}
	cfg = validConfig()
	cfg.ShutdownTimeout = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() shutdown timeout error = nil, want error")
	}
}
