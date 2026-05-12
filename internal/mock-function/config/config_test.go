package config

import (
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

func TestLoadReadsRequiredEnvironment(t *testing.T) {
	setRequiredMockFunctionConfig(t)
	t.Setenv("MOCK_FUNCTION_ENV", "production")
	t.Setenv("MOCK_FUNCTION_HTTP_ADDR", ":9090")
	t.Setenv("MOCK_FUNCTION_NATS_URL", "nats://example:4222")
	t.Setenv("MOCK_FUNCTION_RESOURCE_CREATE_STREAM", "RESOURCE_CREATE_COMMANDS")
	t.Setenv("MOCK_FUNCTION_RESOURCE_CREATE_DURABLE", "mock-function-resource-create")
	t.Setenv("MOCK_FUNCTION_RESOURCE_CREATE_FETCH_COUNT", "25")
	t.Setenv("MOCK_FUNCTION_RESOURCE_CREATE_MAX_WAIT", "7s")
	t.Setenv("MOCK_FUNCTION_RESOURCE_UPSERT_PUBLISH_TIMEOUT", "9s")
	t.Setenv("MOCK_FUNCTION_SHUTDOWN_TIMEOUT", "15s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Environment != environment.Production {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, environment.Production)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.NATS.URL != "nats://example:4222" {
		t.Fatalf("NATS.URL = %q, want nats://example:4222", cfg.NATS.URL)
	}
	if cfg.ResourceCreate.Stream != "RESOURCE_CREATE_COMMANDS" {
		t.Fatalf("ResourceCreate.Stream = %q, want RESOURCE_CREATE_COMMANDS", cfg.ResourceCreate.Stream)
	}
	if cfg.ResourceCreate.Durable != "mock-function-resource-create" {
		t.Fatalf("ResourceCreate.Durable = %q, want mock-function-resource-create", cfg.ResourceCreate.Durable)
	}
	if cfg.ResourceCreate.FetchCount != 25 {
		t.Fatalf("ResourceCreate.FetchCount = %d, want 25", cfg.ResourceCreate.FetchCount)
	}
	if cfg.ResourceCreate.MaxWait != 7*time.Second {
		t.Fatalf("ResourceCreate.MaxWait = %s, want 7s", cfg.ResourceCreate.MaxWait)
	}
	if cfg.AppNames.Documents != "documents" || cfg.AppNames.Tasks != "tasks" || cfg.AppNames.Drive != "drive" {
		t.Fatalf("AppNames = %+v", cfg.AppNames)
	}
	if cfg.ResourceUpsertPublishTimeout != 9*time.Second {
		t.Fatalf("ResourceUpsertPublishTimeout = %s, want 9s", cfg.ResourceUpsertPublishTimeout)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 15s", cfg.ShutdownTimeout)
	}
}

func TestLoadAppliesOptionalDefaults(t *testing.T) {
	setRequiredMockFunctionConfig(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Environment != environment.Development {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, environment.Development)
	}
	if cfg.ResourceCreate.FetchCount != 20 {
		t.Fatalf("ResourceCreate.FetchCount = %d, want 20", cfg.ResourceCreate.FetchCount)
	}
	if cfg.ResourceCreate.MaxWait != 5*time.Second {
		t.Fatalf("ResourceCreate.MaxWait = %s, want 5s", cfg.ResourceCreate.MaxWait)
	}
	if cfg.ResourceUpsertPublishTimeout != 15*time.Second {
		t.Fatalf("ResourceUpsertPublishTimeout = %s, want 15s", cfg.ResourceUpsertPublishTimeout)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 10s", cfg.ShutdownTimeout)
	}
}

func TestResourceCreateSubjectAppNames(t *testing.T) {
	cfg := Config{
		AppNames: AppNamesConfig{
			Documents: "documents",
			Tasks:     "tasks",
			Drive:     "drive",
		},
	}

	got := cfg.ResourceCreateSubjectAppNames()

	if got["cmd.app.documents.resource.create"] != "documents" {
		t.Fatalf("documents mapping = %q", got["cmd.app.documents.resource.create"])
	}
	if got["cmd.app.tasks.resource.create"] != "tasks" {
		t.Fatalf("tasks mapping = %q", got["cmd.app.tasks.resource.create"])
	}
	if got["cmd.app.drive.resource.create"] != "drive" {
		t.Fatalf("drive mapping = %q", got["cmd.app.drive.resource.create"])
	}
}

func TestResourceCreateConsumerSubject(t *testing.T) {
	if got := (Config{}).ResourceCreateConsumerSubject(); got != "cmd.app.*.resource.create" {
		t.Fatalf("ResourceCreateConsumerSubject() = %q, want cmd.app.*.resource.create", got)
	}
}

func TestLoadRejectsMissingRequiredValue(t *testing.T) {
	t.Setenv("MOCK_FUNCTION_HTTP_ADDR", ":8084")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsInvalidAppNames(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "dot", value: "documents.v1"},
		{name: "whitespace", value: "documents v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredMockFunctionConfig(t)
			t.Setenv("MOCK_FUNCTION_DOCUMENTS_APP_NAME", tt.value)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), "MOCK_FUNCTION_DOCUMENTS_APP_NAME") {
				t.Fatalf("Load() error = %v, want invalid documents app name", err)
			}
		})
	}
}

func TestLoadRejectsNonPositiveValues(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "fetch count", key: "MOCK_FUNCTION_RESOURCE_CREATE_FETCH_COUNT"},
		{name: "max wait", key: "MOCK_FUNCTION_RESOURCE_CREATE_MAX_WAIT"},
		{name: "publish timeout", key: "MOCK_FUNCTION_RESOURCE_UPSERT_PUBLISH_TIMEOUT"},
		{name: "shutdown timeout", key: "MOCK_FUNCTION_SHUTDOWN_TIMEOUT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredMockFunctionConfig(t)
			t.Setenv(tt.key, "0")

			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want error")
			}
		})
	}
}

func setRequiredMockFunctionConfig(t *testing.T) {
	t.Helper()

	t.Setenv("MOCK_FUNCTION_HTTP_ADDR", ":8084")
	t.Setenv("MOCK_FUNCTION_NATS_URL", "nats://localhost:4222")
	t.Setenv("MOCK_FUNCTION_RESOURCE_CREATE_STREAM", "RESOURCE_CREATE_COMMANDS")
	t.Setenv("MOCK_FUNCTION_RESOURCE_CREATE_DURABLE", "mock-function-resource-create")
	t.Setenv("MOCK_FUNCTION_DOCUMENTS_APP_NAME", "documents")
	t.Setenv("MOCK_FUNCTION_TASKS_APP_NAME", "tasks")
	t.Setenv("MOCK_FUNCTION_DRIVE_APP_NAME", "drive")
}
