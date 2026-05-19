package config

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

func TestLoadReadsRequiredEnvironment(t *testing.T) {
	t.Setenv("FUNCTION_SERVICE_ENV", "production")
	t.Setenv("FUNCTION_SERVICE_HTTP_ADDR", ":9090")
	t.Setenv("FUNCTION_SERVICE_MONGODB_URI", "mongodb://example:27017")
	t.Setenv("FUNCTION_SERVICE_MONGODB_DATABASE", "wpm")
	t.Setenv("FUNCTION_SERVICE_NATS_URL", "nats://example:4222")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_STREAM", "FUNCTION_RESOURCES")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_DURABLE", "function-service")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT", "25")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT", "7s")
	t.Setenv("FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT", "app.todo.resource.deleted")
	t.Setenv("FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT", "4")
	t.Setenv("FUNCTION_SERVICE_SYSTEM_RESOURCE_ACTION_LIMIT", "6")
	t.Setenv("FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT", "21")
	t.Setenv("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT", "15s")

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
	if cfg.NATS.URL != "nats://example:4222" {
		t.Fatalf("NATS.URL = %q, want nats://example:4222", cfg.NATS.URL)
	}
	if cfg.JetStream.Stream != "FUNCTION_RESOURCES" {
		t.Fatalf("JetStream.Stream = %q, want FUNCTION_RESOURCES", cfg.JetStream.Stream)
	}
	if cfg.JetStream.Durable != "function-service" {
		t.Fatalf("JetStream.Durable = %q, want function-service", cfg.JetStream.Durable)
	}
	if cfg.JetStream.FetchCount != 25 {
		t.Fatalf("JetStream.FetchCount = %d, want 25", cfg.JetStream.FetchCount)
	}
	if cfg.JetStream.MaxWait != 7*time.Second {
		t.Fatalf("JetStream.MaxWait = %s, want 7s", cfg.JetStream.MaxWait)
	}
	if cfg.ResourceDeletedSubject != "app.todo.resource.deleted" {
		t.Fatalf("ResourceDeletedSubject = %q, want app.todo.resource.deleted", cfg.ResourceDeletedSubject)
	}
	if cfg.SystemResourceLimits.Type != 4 {
		t.Fatalf("SystemResourceLimits.Type = %d, want 4", cfg.SystemResourceLimits.Type)
	}
	if cfg.SystemResourceLimits.Action != 6 {
		t.Fatalf("SystemResourceLimits.Action = %d, want 6", cfg.SystemResourceLimits.Action)
	}
	if cfg.SystemResourceLimits.Tag != 21 {
		t.Fatalf("SystemResourceLimits.Tag = %d, want 21", cfg.SystemResourceLimits.Tag)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 15s", cfg.ShutdownTimeout)
	}
}

func TestLoadAppliesOptionalDefaults(t *testing.T) {
	t.Setenv("FUNCTION_SERVICE_HTTP_ADDR", ":8080")
	t.Setenv("FUNCTION_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("FUNCTION_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("FUNCTION_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_STREAM", "FUNCTION_RESOURCES")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_DURABLE", "function-service-resource-upserter")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Development {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, environment.Development)
	}
	if cfg.JetStream.FetchCount != 20 {
		t.Fatalf("JetStream.FetchCount = %d, want 20", cfg.JetStream.FetchCount)
	}
	if cfg.JetStream.MaxWait != 5*time.Second {
		t.Fatalf("JetStream.MaxWait = %s, want 5s", cfg.JetStream.MaxWait)
	}
	if cfg.ResourceDeletedSubject != "app.todo.resource.deleted" {
		t.Fatalf("ResourceDeletedSubject = %q, want app.todo.resource.deleted", cfg.ResourceDeletedSubject)
	}
	if cfg.SystemResourceLimits.Type != 3 {
		t.Fatalf("SystemResourceLimits.Type = %d, want 3", cfg.SystemResourceLimits.Type)
	}
	if cfg.SystemResourceLimits.Action != 5 {
		t.Fatalf("SystemResourceLimits.Action = %d, want 5", cfg.SystemResourceLimits.Action)
	}
	if cfg.SystemResourceLimits.Tag != 20 {
		t.Fatalf("SystemResourceLimits.Tag = %d, want 20", cfg.SystemResourceLimits.Tag)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 10s", cfg.ShutdownTimeout)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("FUNCTION_SERVICE_ENV", "staging")
	t.Setenv("FUNCTION_SERVICE_HTTP_ADDR", ":8080")
	t.Setenv("FUNCTION_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("FUNCTION_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("FUNCTION_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_STREAM", "FUNCTION_RESOURCES")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_DURABLE", "function-service-resource-upserter")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsMissingRequiredValue(t *testing.T) {
	t.Setenv("FUNCTION_SERVICE_HTTP_ADDR", ":8080")
	t.Setenv("FUNCTION_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("FUNCTION_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_STREAM", "FUNCTION_RESOURCES")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_DURABLE", "function-service-resource-upserter")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsBlankResourceDeletedSubject(t *testing.T) {
	t.Setenv("FUNCTION_SERVICE_HTTP_ADDR", ":8080")
	t.Setenv("FUNCTION_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("FUNCTION_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("FUNCTION_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_STREAM", "FUNCTION_RESOURCES")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_DURABLE", "function-service-resource-upserter")
	t.Setenv("FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT", " ")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsInvalidSystemResourceLimits(t *testing.T) {
	t.Setenv("FUNCTION_SERVICE_HTTP_ADDR", ":8080")
	t.Setenv("FUNCTION_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("FUNCTION_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("FUNCTION_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_STREAM", "FUNCTION_RESOURCES")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_DURABLE", "function-service-resource-upserter")
	t.Setenv("FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT", "0")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}
