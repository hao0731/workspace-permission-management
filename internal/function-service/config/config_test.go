package config

import (
	"testing"
	"time"
)

func TestLoadReadsRequiredEnvironment(t *testing.T) {
	t.Setenv("FUNCTION_SERVICE_ENV", "production")
	t.Setenv("FUNCTION_SERVICE_HTTP_ADDR", ":9090")
	t.Setenv("FUNCTION_SERVICE_MONGODB_URI", "mongodb://example:27017")
	t.Setenv("FUNCTION_SERVICE_MONGODB_DATABASE", "wpm")
	t.Setenv("FUNCTION_SERVICE_NATS_URL", "nats://example:4222")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_STREAM", "FUNCTION_RESOURCES")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_DURABLE", "function-service")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_SUBJECT", "app.todo.resource.upserted")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT", "25")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT", "7s")
	t.Setenv("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT", "15s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != EnvironmentProduction {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, EnvironmentProduction)
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
	if cfg.JetStream.Subject != "app.todo.resource.upserted" {
		t.Fatalf("JetStream.Subject = %q, want app.todo.resource.upserted", cfg.JetStream.Subject)
	}
	if cfg.JetStream.FetchCount != 25 {
		t.Fatalf("JetStream.FetchCount = %d, want 25", cfg.JetStream.FetchCount)
	}
	if cfg.JetStream.MaxWait != 7*time.Second {
		t.Fatalf("JetStream.MaxWait = %s, want 7s", cfg.JetStream.MaxWait)
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
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_SUBJECT", "app.todo.resource.upserted")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != EnvironmentDevelopment {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, EnvironmentDevelopment)
	}
	if cfg.JetStream.FetchCount != 20 {
		t.Fatalf("JetStream.FetchCount = %d, want 20", cfg.JetStream.FetchCount)
	}
	if cfg.JetStream.MaxWait != 5*time.Second {
		t.Fatalf("JetStream.MaxWait = %s, want 5s", cfg.JetStream.MaxWait)
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
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_SUBJECT", "app.todo.resource.upserted")

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
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_SUBJECT", "app.todo.resource.upserted")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}
