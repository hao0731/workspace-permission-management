# Function Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `function-service` to ingest resource upsert CloudEvents from NATS JetStream into MongoDB and expose a cursor-paginated resource list API.

**Architecture:** The service is a resource projection service. Echo and eventbus handlers stay thin, services own workflows, repositories own MongoDB access, and `internal/domain/resource` remains independent of framework, broker, and database types.

**Tech Stack:** Go 1.25, Echo v5, viper, MongoDB Go Driver v2, NATS JetStream via `internal/shared/eventbus`, CloudEvents SDK for Go, `log/slog`, standard `testing`.

---

## Source Design

Source design: [../../designs/function-service.md](../../designs/function-service.md)

Applicable policies:

- [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

## Scope

Build the first version of `function-service`:

- Resource projection collection: `function_resources`.
- CloudEvent subject and type default: `app.todo.resource.upserted`.
- Upsert semantics keyed by `resource_id`.
- Older event ignore semantics.
- HTTP list endpoint with cursor pagination.
- Health route from `internal/shared/health`.
- Runtime config through env vars and `.env` for local development.

Do not add workspace/function registry validation, delete events, permission evaluation, frontend code, or new public APIs beyond the resource list endpoint.

## File Structure

Create:

- `.env.example`
- `cmd/function-service/main.go`
- `cmd/function-service/main_test.go`
- `examples/api/function_resources.http`
- `internal/domain/resource/errors.go`
- `internal/domain/resource/resource.go`
- `internal/function-service/config/config.go`
- `internal/function-service/config/config_test.go`
- `internal/function-service/handlers/resource_event_handler.go`
- `internal/function-service/handlers/resource_event_handler_test.go`
- `internal/function-service/handlers/resource_handler.go`
- `internal/function-service/handlers/resource_handler_test.go`
- `internal/function-service/repositories/mongo_resource_repository.go`
- `internal/function-service/repositories/mongo_resource_repository_test.go`
- `internal/function-service/services/resource_service.go`
- `internal/function-service/services/resource_service_test.go`
- `internal/function-service/transport/pagination.go`
- `internal/function-service/transport/pagination_test.go`
- `internal/function-service/transport/resource_event.go`
- `internal/function-service/transport/resource_event_test.go`
- `internal/function-service/transport/resource_response.go`

Modify:

- `.gitignore`
- `go.mod`
- `go.sum`

## Task 1: Environment Contract and Config Loader

**Files:**

- Create: `.env.example`
- Modify: `.gitignore`
- Create: `internal/function-service/config/config_test.go`
- Create: `internal/function-service/config/config.go`

- [ ] **Step 1: Ensure `.env` is ignored**

Confirm `.gitignore` contains this line under local environment entries:

```gitignore
.env
```

If it is missing, add it.

- [ ] **Step 2: Create `.env.example`**

Create `.env.example` with exactly this content:

```dotenv
# Function service
FUNCTION_SERVICE_ENV=development
FUNCTION_SERVICE_HTTP_ADDR=:8080

# MongoDB
FUNCTION_SERVICE_MONGODB_URI=mongodb://localhost:27017
FUNCTION_SERVICE_MONGODB_DATABASE=workspace_permission_management

# NATS JetStream
FUNCTION_SERVICE_NATS_URL=nats://localhost:4222
FUNCTION_SERVICE_JETSTREAM_STREAM=FUNCTION_RESOURCES
FUNCTION_SERVICE_JETSTREAM_DURABLE=function-service-resource-upserter
FUNCTION_SERVICE_JETSTREAM_SUBJECT=app.todo.resource.upserted
FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT=20
FUNCTION_SERVICE_JETSTREAM_MAX_WAIT=5s

# Shutdown
FUNCTION_SERVICE_SHUTDOWN_TIMEOUT=10s
```

- [ ] **Step 3: Write failing config tests**

Create `internal/function-service/config/config_test.go`:

```go
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
	if cfg.Environment != environment.Development {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, environment.Development)
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
```

- [ ] **Step 4: Run config tests to verify failure**

Run:

```bash
go test ./internal/function-service/config
```

Expected: FAIL because package `internal/function-service/config` or `Load` is not defined.

- [ ] **Step 5: Implement config loader**

Create `internal/function-service/config/config.go`:

```go
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Environment string

const (
	Development Environment = "development"
	Production  Environment = "production"
)

type Config struct {
	Environment     Environment
	HTTPAddr        string
	MongoDB         MongoDBConfig
	NATS            NATSConfig
	JetStream       JetStreamConfig
	ShutdownTimeout time.Duration
}

type MongoDBConfig struct {
	URI      string
	Database string
}

type NATSConfig struct {
	URL string
}

type JetStreamConfig struct {
	Stream     string
	Durable    string
	Subject    string
	FetchCount int
	MaxWait    time.Duration
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("FUNCTION_SERVICE_ENV", string(environment.Development))
	v.SetDefault("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT", 20)
	v.SetDefault("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT", "5s")
	v.SetDefault("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT", "10s")

	cfg := Config{
		Environment: Environment(v.GetString("FUNCTION_SERVICE_ENV")),
		HTTPAddr:    v.GetString("FUNCTION_SERVICE_HTTP_ADDR"),
		MongoDB: MongoDBConfig{
			URI:      v.GetString("FUNCTION_SERVICE_MONGODB_URI"),
			Database: v.GetString("FUNCTION_SERVICE_MONGODB_DATABASE"),
		},
		NATS: NATSConfig{
			URL: v.GetString("FUNCTION_SERVICE_NATS_URL"),
		},
		JetStream: JetStreamConfig{
			Stream:     v.GetString("FUNCTION_SERVICE_JETSTREAM_STREAM"),
			Durable:    v.GetString("FUNCTION_SERVICE_JETSTREAM_DURABLE"),
			Subject:    v.GetString("FUNCTION_SERVICE_JETSTREAM_SUBJECT"),
			FetchCount: v.GetInt("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT"),
			MaxWait:    v.GetDuration("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT"),
		},
		ShutdownTimeout: v.GetDuration("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT"),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.Environment != environment.Development && c.Environment != environment.Production {
		return fmt.Errorf("FUNCTION_SERVICE_ENV must be %q or %q", environment.Development, environment.Production)
	}

	required := map[string]string{
		"FUNCTION_SERVICE_HTTP_ADDR":          c.HTTPAddr,
		"FUNCTION_SERVICE_MONGODB_URI":        c.MongoDB.URI,
		"FUNCTION_SERVICE_MONGODB_DATABASE":   c.MongoDB.Database,
		"FUNCTION_SERVICE_NATS_URL":           c.NATS.URL,
		"FUNCTION_SERVICE_JETSTREAM_STREAM":   c.JetStream.Stream,
		"FUNCTION_SERVICE_JETSTREAM_DURABLE":  c.JetStream.Durable,
		"FUNCTION_SERVICE_JETSTREAM_SUBJECT":  c.JetStream.Subject,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if c.JetStream.FetchCount <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT must be greater than zero")
	}
	if c.JetStream.MaxWait <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}
```

- [ ] **Step 6: Run config tests to verify pass**

Run:

```bash
go test ./internal/function-service/config
```

Expected: PASS.

- [ ] **Step 7: Commit config work**

Run:

```bash
git add .gitignore .env.example internal/function-service/config/config.go internal/function-service/config/config_test.go
git commit -m "feat: add function service config"
```

Expected: commit succeeds.

## Task 2: Domain Models and Resource Service

**Files:**

- Create: `internal/domain/resource/errors.go`
- Create: `internal/domain/resource/resource.go`
- Create: `internal/function-service/services/resource_service_test.go`
- Create: `internal/function-service/services/resource_service.go`

- [ ] **Step 1: Write domain model files**

Create `internal/domain/resource/errors.go`:

```go
package resource

import "errors"

var ErrInvalidInput = errors.New("invalid resource input")
```

Create `internal/domain/resource/resource.go`:

```go
package resource

import "time"

type Resource struct {
	ID           string
	WorkspaceID  string
	FunctionKey  string
	DisplayName  string
	Type         string
	Tags         []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UpsertInput struct {
	ID          string
	WorkspaceID string
	FunctionKey string
	DisplayName string
	Type        string
	Tags        []string
	EventTime   time.Time
}

type Cursor struct {
	CreatedAt time.Time
	ID        string
}

type ListQuery struct {
	WorkspaceID string
	FunctionKey string
	Limit       int
	Cursor      *Cursor
}

type Page struct {
	Resources   []Resource
	HasNextPage bool
	NextCursor  *Cursor
}

type UpsertStatus string

const (
	UpsertStatusInserted UpsertStatus = "inserted"
	UpsertStatusUpdated  UpsertStatus = "updated"
	UpsertStatusIgnored  UpsertStatus = "ignored"
)
```

- [ ] **Step 2: Write failing service tests**

Create `internal/function-service/services/resource_service_test.go`:

```go
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type fakeResourceRepository struct {
	upsertStatus resource.UpsertStatus
	upsertInput  resource.UpsertInput
	upsertErr    error
	listQuery    resource.ListQuery
	listPage     resource.Page
	listErr      error
}

func (f *fakeResourceRepository) Upsert(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	f.upsertInput = input
	if f.upsertErr != nil {
		return "", f.upsertErr
	}
	return f.upsertStatus, nil
}

func (f *fakeResourceRepository) List(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	f.listQuery = query
	if f.listErr != nil {
		return resource.Page{}, f.listErr
	}
	return f.listPage, nil
}

func TestResourceServiceUpsertResource(t *testing.T) {
	eventTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	repo := &fakeResourceRepository{upsertStatus: resource.UpsertStatusInserted}
	service := NewResourceService(repo)

	got, err := service.UpsertResource(context.Background(), resource.UpsertInput{
		ID:          "resource-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		DisplayName: "Spec",
		Type:        "document",
		Tags:        []string{"section_1"},
		EventTime:   eventTime,
	})
	if err != nil {
		t.Fatalf("UpsertResource error = %v, want nil", err)
	}
	if got != resource.UpsertStatusInserted {
		t.Fatalf("status = %q, want %q", got, resource.UpsertStatusInserted)
	}
	if repo.upsertInput.ID != "resource-1" {
		t.Fatalf("repo input ID = %q, want resource-1", repo.upsertInput.ID)
	}
	if repo.upsertInput.EventTime != eventTime {
		t.Fatalf("repo input EventTime = %s, want %s", repo.upsertInput.EventTime, eventTime)
	}
}

func TestResourceServiceRejectsInvalidUpsertInput(t *testing.T) {
	service := NewResourceService(&fakeResourceRepository{})

	_, err := service.UpsertResource(context.Background(), resource.UpsertInput{
		ID:          "",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		DisplayName: "Spec",
		Type:        "document",
		Tags:        []string{"section_1"},
		EventTime:   time.Now(),
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestResourceServiceListResources(t *testing.T) {
	cursorTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	repo := &fakeResourceRepository{
		listPage: resource.Page{
			Resources: []resource.Resource{{ID: "resource-1"}},
			HasNextPage: true,
			NextCursor: &resource.Cursor{CreatedAt: cursorTime, ID: "resource-1"},
		},
	}
	service := NewResourceService(repo)

	page, err := service.ListResources(context.Background(), resource.ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       20,
	})
	if err != nil {
		t.Fatalf("ListResources error = %v, want nil", err)
	}
	if len(page.Resources) != 1 {
		t.Fatalf("resources len = %d, want 1", len(page.Resources))
	}
	if !page.HasNextPage {
		t.Fatal("HasNextPage = false, want true")
	}
	if repo.listQuery.WorkspaceID != "workspace-1" || repo.listQuery.FunctionKey != "todo" {
		t.Fatalf("repo query = %+v, want workspace-1/todo", repo.listQuery)
	}
}

func TestResourceServiceRejectsInvalidListQuery(t *testing.T) {
	service := NewResourceService(&fakeResourceRepository{})

	_, err := service.ListResources(context.Background(), resource.ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       0,
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}
```

- [ ] **Step 3: Run service tests to verify failure**

Run:

```bash
go test ./internal/function-service/services
```

Expected: FAIL because `NewResourceService` is not defined.

- [ ] **Step 4: Implement resource service**

Create `internal/function-service/services/resource_service.go`:

```go
package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type ResourceRepository interface {
	Upsert(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error)
	List(ctx context.Context, query resource.ListQuery) (resource.Page, error)
}

type ResourceService struct {
	repository ResourceRepository
}

func NewResourceService(repository ResourceRepository) *ResourceService {
	return &ResourceService{repository: repository}
}

func (s *ResourceService) UpsertResource(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	if err := validateUpsertInput(input); err != nil {
		return "", err
	}
	status, err := s.repository.Upsert(ctx, input)
	if err != nil {
		return "", fmt.Errorf("upsert resource: %w", err)
	}
	return status, nil
}

func (s *ResourceService) ListResources(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	if err := validateListQuery(query); err != nil {
		return resource.Page{}, err
	}
	page, err := s.repository.List(ctx, query)
	if err != nil {
		return resource.Page{}, fmt.Errorf("list resources: %w", err)
	}
	return page, nil
}

func validateUpsertInput(input resource.UpsertInput) error {
	if strings.TrimSpace(input.ID) == "" {
		return fmt.Errorf("%w: resource id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return fmt.Errorf("%w: function key is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		return fmt.Errorf("%w: display name is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.Type) == "" {
		return fmt.Errorf("%w: resource type is required", resource.ErrInvalidInput)
	}
	if input.EventTime.IsZero() {
		return fmt.Errorf("%w: event time is required", resource.ErrInvalidInput)
	}
	for _, tag := range input.Tags {
		if strings.TrimSpace(tag) == "" {
			return fmt.Errorf("%w: resource tags must be non-empty strings", resource.ErrInvalidInput)
		}
	}
	return nil
}

func validateListQuery(query resource.ListQuery) error {
	if strings.TrimSpace(query.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(query.FunctionKey) == "" {
		return fmt.Errorf("%w: function key is required", resource.ErrInvalidInput)
	}
	if query.Limit <= 0 {
		return fmt.Errorf("%w: limit must be greater than zero", resource.ErrInvalidInput)
	}
	if query.Cursor != nil {
		if query.Cursor.CreatedAt.IsZero() {
			return fmt.Errorf("%w: cursor created_at is required", resource.ErrInvalidInput)
		}
		if strings.TrimSpace(query.Cursor.ID) == "" {
			return fmt.Errorf("%w: cursor id is required", resource.ErrInvalidInput)
		}
	}
	return nil
}
```

- [ ] **Step 5: Run service tests to verify pass**

Run:

```bash
go test ./internal/domain/resource ./internal/function-service/services
```

Expected: PASS.

- [ ] **Step 6: Commit domain and service work**

Run:

```bash
git add internal/domain/resource internal/function-service/services
git commit -m "feat: add resource domain service"
```

Expected: commit succeeds.

## Task 3: Cursor Pagination and HTTP Response Transport

**Files:**

- Create: `internal/function-service/transport/pagination_test.go`
- Create: `internal/function-service/transport/pagination.go`
- Create: `internal/function-service/transport/resource_response.go`

- [ ] **Step 1: Write failing pagination tests**

Create `internal/function-service/transport/pagination_test.go`:

```go
package transport

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func TestParseLimit(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "default", raw: "", want: 20},
		{name: "explicit", raw: "50", want: 50},
		{name: "too large", raw: "51", wantErr: true},
		{name: "zero", raw: "0", wantErr: true},
		{name: "not integer", raw: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLimit(tt.raw)
			if tt.wantErr && err == nil {
				t.Fatal("ParseLimit error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ParseLimit error = %v, want nil", err)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("limit = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEncodeDecodeNextToken(t *testing.T) {
	cursor := resource.Cursor{
		CreatedAt: time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
		ID:        "resource-123",
	}

	token, err := EncodeNextToken(&cursor)
	if err != nil {
		t.Fatalf("EncodeNextToken error = %v, want nil", err)
	}
	got, err := DecodeNextToken(token)
	if err != nil {
		t.Fatalf("DecodeNextToken error = %v, want nil", err)
	}
	if got.CreatedAt != cursor.CreatedAt {
		t.Fatalf("CreatedAt = %s, want %s", got.CreatedAt, cursor.CreatedAt)
	}
	if got.ID != cursor.ID {
		t.Fatalf("ID = %q, want %q", got.ID, cursor.ID)
	}
}

func TestEncodeNextTokenEmptyCursor(t *testing.T) {
	token, err := EncodeNextToken(nil)
	if err != nil {
		t.Fatalf("EncodeNextToken error = %v, want nil", err)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty", token)
	}
}

func TestDecodeNextTokenRejectsInvalidToken(t *testing.T) {
	if _, err := DecodeNextToken("not-base64"); err == nil {
		t.Fatal("DecodeNextToken error = nil, want error")
	}
}
```

- [ ] **Step 2: Run pagination tests to verify failure**

Run:

```bash
go test ./internal/function-service/transport
```

Expected: FAIL because `ParseLimit`, `EncodeNextToken`, and `DecodeNextToken` are not defined.

- [ ] **Step 3: Implement pagination helpers**

Create `internal/function-service/transport/pagination.go`:

```go
package transport

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

const (
	DefaultLimit = 20
	MaxLimit     = 50
)

type nextTokenPayload struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func ParseLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return DefaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("limit must be an integer")
	}
	if limit < 1 {
		return 0, fmt.Errorf("limit must be greater than zero")
	}
	if limit > MaxLimit {
		return 0, fmt.Errorf("limit must be less than or equal to %d", MaxLimit)
	}
	return limit, nil
}

func EncodeNextToken(cursor *resource.Cursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	payload := nextTokenPayload{
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        cursor.ID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal next token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func DecodeNextToken(token string) (*resource.Cursor, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("next_token must be base64url encoded JSON")
	}
	var payload nextTokenPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("next_token must be JSON")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("next_token.created_at must be RFC3339 timestamp")
	}
	if strings.TrimSpace(payload.ID) == "" {
		return nil, fmt.Errorf("next_token.id is required")
	}
	return &resource.Cursor{CreatedAt: createdAt, ID: payload.ID}, nil
}
```

- [ ] **Step 4: Implement HTTP response DTOs**

Create `internal/function-service/transport/resource_response.go`:

```go
package transport

import "github.com/hao0731/workspace-permission-management/internal/domain/resource"

type ResourceListResponse struct {
	Resources []ResourceResponse `json:"resources"`
	PageInfo  PageInfoResponse   `json:"page_info"`
}

type ResourceResponse struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"display_name"`
	Type         string   `json:"type"`
	ResourceTags []string `json:"resource_tags"`
}

type PageInfoResponse struct {
	HasNextPage bool   `json:"has_next_page"`
	NextToken   string `json:"next_token"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}

func NewResourceListResponse(page resource.Page) (ResourceListResponse, error) {
	resources := make([]ResourceResponse, 0, len(page.Resources))
	for _, item := range page.Resources {
		resources = append(resources, ResourceResponse{
			ID:           item.ID,
			DisplayName:  item.DisplayName,
			Type:         item.Type,
			ResourceTags: append([]string(nil), item.Tags...),
		})
	}
	nextToken, err := EncodeNextToken(page.NextCursor)
	if err != nil {
		return ResourceListResponse{}, err
	}
	return ResourceListResponse{
		Resources: resources,
		PageInfo: PageInfoResponse{
			HasNextPage: page.HasNextPage,
			NextToken:   nextToken,
		},
	}, nil
}
```

- [ ] **Step 5: Run transport tests to verify pass**

Run:

```bash
go test ./internal/function-service/transport
```

Expected: PASS.

- [ ] **Step 6: Commit pagination transport**

Run:

```bash
git add internal/function-service/transport/pagination.go internal/function-service/transport/pagination_test.go internal/function-service/transport/resource_response.go
git commit -m "feat: add resource pagination transport"
```

Expected: commit succeeds.

## Task 4: CloudEvent Parsing and Event Handler

**Files:**

- Modify: `go.mod`
- Modify: `go.sum`
- Create: `internal/function-service/transport/resource_event_test.go`
- Create: `internal/function-service/transport/resource_event.go`
- Create: `internal/function-service/handlers/resource_event_handler_test.go`
- Create: `internal/function-service/handlers/resource_event_handler.go`

- [ ] **Step 1: Add CloudEvents SDK dependency**

Run:

```bash
go get github.com/cloudevents/sdk-go/v2
go mod tidy
```

Expected: `go.mod` and `go.sum` include `github.com/cloudevents/sdk-go/v2`.

- [ ] **Step 2: Write failing CloudEvent parser tests**

Create `internal/function-service/transport/resource_event_test.go`:

```go
package transport

import (
	"testing"
	"time"
)

func TestParseResourceUpsertEvent(t *testing.T) {
	eventJSON := []byte(`{
		"specversion":"1.0",
		"type":"app.todo.resource.upserted",
		"source":"todo-service",
		"subject":"resource-1",
		"id":"event-1",
		"time":"2026-05-05T07:31:00Z",
		"datacontenttype":"application/json",
		"data":{
			"resource_id":"resource-1",
			"display_name":"Spec",
			"resource_type":"document",
			"resource_tags":["section_1"],
			"function_key":"todo",
			"workspace_id":"workspace-1"
		}
	}`)

	got, err := ParseResourceUpsertEvent(eventJSON, "app.todo.resource.upserted")
	if err != nil {
		t.Fatalf("ParseResourceUpsertEvent error = %v, want nil", err)
	}
	if got.ID != "resource-1" || got.DisplayName != "Spec" || got.Type != "document" {
		t.Fatalf("parsed input = %+v, want resource-1/Spec/document", got)
	}
	if got.WorkspaceID != "workspace-1" || got.FunctionKey != "todo" {
		t.Fatalf("parsed scope = %s/%s, want workspace-1/todo", got.WorkspaceID, got.FunctionKey)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "section_1" {
		t.Fatalf("tags = %#v, want [section_1]", got.Tags)
	}
	wantTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	if !got.EventTime.Equal(wantTime) {
		t.Fatalf("EventTime = %s, want %s", got.EventTime, wantTime)
	}
}

func TestParseResourceUpsertEventRejectsWrongType(t *testing.T) {
	eventJSON := []byte(`{
		"specversion":"1.0",
		"type":"wrong.type",
		"source":"todo-service",
		"subject":"resource-1",
		"id":"event-1",
		"time":"2026-05-05T07:31:00Z",
		"datacontenttype":"application/json",
		"data":{
			"resource_id":"resource-1",
			"display_name":"Spec",
			"resource_type":"document",
			"resource_tags":["section_1"],
			"function_key":"todo",
			"workspace_id":"workspace-1"
		}
	}`)

	if _, err := ParseResourceUpsertEvent(eventJSON, "app.todo.resource.upserted"); err == nil {
		t.Fatal("ParseResourceUpsertEvent error = nil, want error")
	}
}

func TestParseResourceUpsertEventRejectsSubjectMismatch(t *testing.T) {
	eventJSON := []byte(`{
		"specversion":"1.0",
		"type":"app.todo.resource.upserted",
		"source":"todo-service",
		"subject":"different-resource",
		"id":"event-1",
		"time":"2026-05-05T07:31:00Z",
		"datacontenttype":"application/json",
		"data":{
			"resource_id":"resource-1",
			"display_name":"Spec",
			"resource_type":"document",
			"resource_tags":["section_1"],
			"function_key":"todo",
			"workspace_id":"workspace-1"
		}
	}`)

	if _, err := ParseResourceUpsertEvent(eventJSON, "app.todo.resource.upserted"); err == nil {
		t.Fatal("ParseResourceUpsertEvent error = nil, want error")
	}
}
```

- [ ] **Step 3: Run CloudEvent parser tests to verify failure**

Run:

```bash
go test ./internal/function-service/transport
```

Expected: FAIL because `ParseResourceUpsertEvent` is not defined.

- [ ] **Step 4: Implement CloudEvent parser**

Create `internal/function-service/transport/resource_event.go`:

```go
package transport

import (
	"encoding/json"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

const cloudEventSpecVersion = "1.0"

type resourceUpsertData struct {
	ResourceID   string   `json:"resource_id"`
	DisplayName  string   `json:"display_name"`
	ResourceType string   `json:"resource_type"`
	ResourceTags []string `json:"resource_tags"`
	FunctionKey  string   `json:"function_key"`
	WorkspaceID  string   `json:"workspace_id"`
}

func ParseResourceUpsertEvent(data []byte, expectedType string) (resource.UpsertInput, error) {
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return resource.UpsertInput{}, fmt.Errorf("parse cloudevent: %w", err)
	}
	if err := event.Validate(); err != nil {
		return resource.UpsertInput{}, fmt.Errorf("validate cloudevent: %w", err)
	}
	if event.SpecVersion() != cloudEventSpecVersion {
		return resource.UpsertInput{}, fmt.Errorf("unsupported cloudevent specversion %q", event.SpecVersion())
	}
	if event.Type() != expectedType {
		return resource.UpsertInput{}, fmt.Errorf("cloudevent type %q does not match expected %q", event.Type(), expectedType)
	}
	if event.DataContentType() != "application/json" {
		return resource.UpsertInput{}, fmt.Errorf("cloudevent datacontenttype must be application/json")
	}
	var payload resourceUpsertData
	if err := event.DataAs(&payload); err != nil {
		return resource.UpsertInput{}, fmt.Errorf("parse cloudevent data: %w", err)
	}
	if event.Subject() != payload.ResourceID {
		return resource.UpsertInput{}, fmt.Errorf("cloudevent subject must match data.resource_id")
	}
	if strings.TrimSpace(payload.ResourceID) == "" ||
		strings.TrimSpace(payload.WorkspaceID) == "" ||
		strings.TrimSpace(payload.FunctionKey) == "" ||
		strings.TrimSpace(payload.DisplayName) == "" ||
		strings.TrimSpace(payload.ResourceType) == "" {
		return resource.UpsertInput{}, fmt.Errorf("resource event data contains empty required field")
	}
	for _, tag := range payload.ResourceTags {
		if strings.TrimSpace(tag) == "" {
			return resource.UpsertInput{}, fmt.Errorf("resource_tags must contain non-empty strings")
		}
	}
	if event.Time().IsZero() {
		return resource.UpsertInput{}, fmt.Errorf("cloudevent time is required")
	}
	return resource.UpsertInput{
		ID:          payload.ResourceID,
		WorkspaceID: payload.WorkspaceID,
		FunctionKey: payload.FunctionKey,
		DisplayName: payload.DisplayName,
		Type:        payload.ResourceType,
		Tags:        append([]string(nil), payload.ResourceTags...),
		EventTime:   event.Time(),
	}, nil
}
```

- [ ] **Step 5: Write failing event handler tests**

Create `internal/function-service/handlers/resource_event_handler_test.go`:

```go
package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type fakeEventResourceService struct {
	input resource.UpsertInput
	err   error
}

func (f *fakeEventResourceService) UpsertResource(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	f.input = input
	if f.err != nil {
		return "", f.err
	}
	return resource.UpsertStatusInserted, nil
}

func TestResourceEventHandlerAck(t *testing.T) {
	service := &fakeEventResourceService{}
	handler := NewResourceEventHandler(service, "app.todo.resource.upserted", nil)

	result := handler.Handle(context.Background(), eventbus.Message{
		Subject: "app.todo.resource.upserted",
		Data: []byte(`{
			"specversion":"1.0",
			"type":"app.todo.resource.upserted",
			"source":"todo-service",
			"subject":"resource-1",
			"id":"event-1",
			"time":"2026-05-05T07:31:00Z",
			"datacontenttype":"application/json",
			"data":{
				"resource_id":"resource-1",
				"display_name":"Spec",
				"resource_type":"document",
				"resource_tags":["section_1"],
				"function_key":"todo",
				"workspace_id":"workspace-1"
			}
		}`),
	})

	if result != eventbus.HandleResultAck {
		t.Fatalf("result = %q, want ack", result)
	}
	if service.input.ID != "resource-1" {
		t.Fatalf("service input ID = %q, want resource-1", service.input.ID)
	}
}

func TestResourceEventHandlerTerminatesPoisonMessage(t *testing.T) {
	handler := NewResourceEventHandler(&fakeEventResourceService{}, "app.todo.resource.upserted", nil)

	result := handler.Handle(context.Background(), eventbus.Message{
		Subject: "app.todo.resource.upserted",
		Data:    []byte(`{"bad":`),
	})

	if result != eventbus.HandleResultTerminate {
		t.Fatalf("result = %q, want terminate", result)
	}
}

func TestResourceEventHandlerRetriesTransientServiceError(t *testing.T) {
	service := &fakeEventResourceService{err: ErrRetryableEvent}
	handler := NewResourceEventHandler(service, "app.todo.resource.upserted", nil)

	result := handler.Handle(context.Background(), eventbus.Message{
		Subject: "app.todo.resource.upserted",
		Data: []byte(`{
			"specversion":"1.0",
			"type":"app.todo.resource.upserted",
			"source":"todo-service",
			"subject":"resource-1",
			"id":"event-1",
			"time":"2026-05-05T07:31:00Z",
			"datacontenttype":"application/json",
			"data":{
				"resource_id":"resource-1",
				"display_name":"Spec",
				"resource_type":"document",
				"resource_tags":["section_1"],
				"function_key":"todo",
				"workspace_id":"workspace-1"
			}
		}`),
	})

	if result != eventbus.HandleResultRetry {
		t.Fatalf("result = %q, want retry", result)
	}
}

func TestResourceEventHandlerTerminatesInvalidServiceInput(t *testing.T) {
	service := &fakeEventResourceService{err: resource.ErrInvalidInput}
	handler := NewResourceEventHandler(service, "app.todo.resource.upserted", nil)

	result := handler.Handle(context.Background(), eventbus.Message{
		Subject: "app.todo.resource.upserted",
		Data: []byte(`{
			"specversion":"1.0",
			"type":"app.todo.resource.upserted",
			"source":"todo-service",
			"subject":"resource-1",
			"id":"event-1",
			"time":"2026-05-05T07:31:00Z",
			"datacontenttype":"application/json",
			"data":{
				"resource_id":"resource-1",
				"display_name":"Spec",
				"resource_type":"document",
				"resource_tags":["section_1"],
				"function_key":"todo",
				"workspace_id":"workspace-1"
			}
		}`),
	})

	if result != eventbus.HandleResultTerminate {
		t.Fatalf("result = %q, want terminate", result)
	}
}

func TestIsRetryableEventError(t *testing.T) {
	if !errors.Is(ErrRetryableEvent, ErrRetryableEvent) {
		t.Fatal("ErrRetryableEvent must compare with errors.Is")
	}
}
```

- [ ] **Step 6: Run event handler tests to verify failure**

Run:

```bash
go test ./internal/function-service/handlers
```

Expected: FAIL because `NewResourceEventHandler` and `ErrRetryableEvent` are not defined.

- [ ] **Step 7: Implement event handler**

Create `internal/function-service/handlers/resource_event_handler.go`:

```go
package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

var ErrRetryableEvent = errors.New("retryable event handling error")

type EventResourceService interface {
	UpsertResource(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error)
}

type ResourceEventHandler struct {
	service      EventResourceService
	expectedType string
	logger       *slog.Logger
}

func NewResourceEventHandler(service EventResourceService, expectedType string, logger *slog.Logger) *ResourceEventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ResourceEventHandler{service: service, expectedType: expectedType, logger: logger}
}

func (h *ResourceEventHandler) Handle(ctx context.Context, msg eventbus.Message) eventbus.HandleResult {
	input, err := transport.ParseResourceUpsertEvent(msg.Data, h.expectedType)
	if err != nil {
		h.logger.Warn("terminating invalid resource event", "err", err, "subject", msg.Subject)
		return eventbus.HandleResultTerminate
	}

	status, err := h.service.UpsertResource(ctx, input)
	if err != nil {
		if errors.Is(err, ErrRetryableEvent) {
			h.logger.Warn("retrying resource event", "err", err, "resource_id", input.ID)
			return eventbus.HandleResultRetry
		}
		if errors.Is(err, resource.ErrInvalidInput) {
			h.logger.Warn("terminating invalid resource event input", "err", err, "resource_id", input.ID)
			return eventbus.HandleResultTerminate
		}
		h.logger.Warn("retrying resource event after service error", "err", err, "resource_id", input.ID)
		return eventbus.HandleResultRetry
	}

	h.logger.Info("handled resource event", "resource_id", input.ID, "status", status)
	return eventbus.HandleResultAck
}
```

- [ ] **Step 8: Run event tests to verify pass**

Run:

```bash
go test ./internal/function-service/transport ./internal/function-service/handlers
```

Expected: PASS.

- [ ] **Step 9: Commit CloudEvent and event handler work**

Run:

```bash
git add go.mod go.sum internal/function-service/transport/resource_event.go internal/function-service/transport/resource_event_test.go internal/function-service/handlers/resource_event_handler.go internal/function-service/handlers/resource_event_handler_test.go
git commit -m "feat: add resource event handler"
```

Expected: commit succeeds.

## Task 5: MongoDB Resource Repository

**Files:**

- Create: `internal/function-service/repositories/mongo_resource_repository_test.go`
- Create: `internal/function-service/repositories/mongo_resource_repository.go`

- [ ] **Step 1: Write failing repository unit tests**

Create `internal/function-service/repositories/mongo_resource_repository_test.go`:

```go
package repositories

import (
	"reflect"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestBuildListFilterWithoutCursor(t *testing.T) {
	got := buildListFilter(resource.ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       20,
	})
	want := bson.M{
		"workspace_id": "workspace-1",
		"function_key": "todo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestBuildListFilterWithCursor(t *testing.T) {
	cursorTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	got := buildListFilter(resource.ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       20,
		Cursor:      &resource.Cursor{CreatedAt: cursorTime, ID: "resource-9"},
	})
	want := bson.M{
		"workspace_id": "workspace-1",
		"function_key": "todo",
		"$or": bson.A{
			bson.M{"created_at": bson.M{"$lt": cursorTime}},
			bson.M{"created_at": cursorTime, "_id": bson.M{"$lt": "resource-9"}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestBuildPageUsesLimitPlusOne(t *testing.T) {
	items := []resource.Resource{
		{ID: "resource-1", CreatedAt: time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)},
		{ID: "resource-2", CreatedAt: time.Date(2026, 5, 5, 7, 30, 0, 0, time.UTC)},
		{ID: "resource-3", CreatedAt: time.Date(2026, 5, 5, 7, 29, 0, 0, time.UTC)},
	}

	page := buildPage(items, 2)
	if len(page.Resources) != 2 {
		t.Fatalf("resources len = %d, want 2", len(page.Resources))
	}
	if !page.HasNextPage {
		t.Fatal("HasNextPage = false, want true")
	}
	if page.NextCursor == nil || page.NextCursor.ID != "resource-2" {
		t.Fatalf("NextCursor = %+v, want resource-2", page.NextCursor)
	}
}

func TestResourceDocumentMapping(t *testing.T) {
	now := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	doc := resourceDocument{
		ID:          "resource-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		DisplayName: "Spec",
		ResourceType: "document",
		ResourceTags: []string{"section_1"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	got := doc.toDomain()
	if got.ID != "resource-1" || got.Type != "document" {
		t.Fatalf("domain resource = %+v, want resource-1/document", got)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "section_1" {
		t.Fatalf("tags = %#v, want [section_1]", got.Tags)
	}
}
```

- [ ] **Step 2: Run repository tests to verify failure**

Run:

```bash
go test ./internal/function-service/repositories
```

Expected: FAIL because repository helpers are not defined.

- [ ] **Step 3: Implement Mongo repository**

Create `internal/function-service/repositories/mongo_resource_repository.go`:

```go
package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const resourceCollectionName = "function_resources"

type MongoResourceRepository struct {
	collection *mongo.Collection
}

type resourceDocument struct {
	ID           string    `bson:"_id"`
	WorkspaceID  string    `bson:"workspace_id"`
	FunctionKey  string    `bson:"function_key"`
	DisplayName  string    `bson:"display_name"`
	ResourceType string    `bson:"resource_type"`
	ResourceTags []string  `bson:"resource_tags"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

func NewMongoResourceRepository(db *mongo.Database) *MongoResourceRepository {
	return &MongoResourceRepository{collection: db.Collection(resourceCollectionName)}
}

func (r *MongoResourceRepository) EnsureIndexes(ctx context.Context) error {
	model := mongo.IndexModel{
		Keys: bson.D{
			{Key: "workspace_id", Value: 1},
			{Key: "function_key", Value: 1},
			{Key: "created_at", Value: -1},
			{Key: "_id", Value: -1},
		},
	}
	if _, err := r.collection.Indexes().CreateOne(ctx, model); err != nil {
		return fmt.Errorf("create function_resources index: %w", err)
	}
	return nil
}

func (r *MongoResourceRepository) Upsert(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	update := bson.M{
		"$set": bson.M{
			"workspace_id":   input.WorkspaceID,
			"function_key":   input.FunctionKey,
			"display_name":   input.DisplayName,
			"resource_type":  input.Type,
			"resource_tags":  append([]string(nil), input.Tags...),
			"updated_at":     input.EventTime,
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{
		"_id":        input.ID,
		"updated_at": bson.M{"$lte": input.EventTime},
	}, update)
	if err != nil {
		return "", fmt.Errorf("update current resource: %w", err)
	}
	if result.MatchedCount > 0 {
		return resource.UpsertStatusUpdated, nil
	}

	doc := resourceDocument{
		ID:           input.ID,
		WorkspaceID:  input.WorkspaceID,
		FunctionKey:  input.FunctionKey,
		DisplayName:  input.DisplayName,
		ResourceType: input.Type,
		ResourceTags: append([]string(nil), input.Tags...),
		CreatedAt:    input.EventTime,
		UpdatedAt:    input.EventTime,
	}
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			status, retryErr := r.retryUpdateAfterDuplicate(ctx, input)
			if retryErr != nil {
				return "", retryErr
			}
			return status, nil
		}
		return "", fmt.Errorf("insert resource: %w", err)
	}
	return resource.UpsertStatusInserted, nil
}

func (r *MongoResourceRepository) retryUpdateAfterDuplicate(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	result, err := r.collection.UpdateOne(ctx, bson.M{
		"_id":        input.ID,
		"updated_at": bson.M{"$lte": input.EventTime},
	}, bson.M{
		"$set": bson.M{
			"workspace_id":   input.WorkspaceID,
			"function_key":   input.FunctionKey,
			"display_name":   input.DisplayName,
			"resource_type":  input.Type,
			"resource_tags":  append([]string(nil), input.Tags...),
			"updated_at":     input.EventTime,
		},
	})
	if err != nil {
		return "", fmt.Errorf("retry update resource: %w", err)
	}
	if result.MatchedCount == 0 {
		return resource.UpsertStatusIgnored, nil
	}
	return resource.UpsertStatusUpdated, nil
}

func (r *MongoResourceRepository) List(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(query.Limit + 1))

	cursor, err := r.collection.Find(ctx, buildListFilter(query), findOptions)
	if err != nil {
		return resource.Page{}, fmt.Errorf("find resources: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var docs []resourceDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return resource.Page{}, fmt.Errorf("decode resources: %w", err)
	}

	resources := make([]resource.Resource, 0, len(docs))
	for _, doc := range docs {
		resources = append(resources, doc.toDomain())
	}
	return buildPage(resources, query.Limit), nil
}

func buildListFilter(query resource.ListQuery) bson.M {
	filter := bson.M{
		"workspace_id": query.WorkspaceID,
		"function_key": query.FunctionKey,
	}
	if query.Cursor != nil {
		filter["$or"] = bson.A{
			bson.M{"created_at": bson.M{"$lt": query.Cursor.CreatedAt}},
			bson.M{"created_at": query.Cursor.CreatedAt, "_id": bson.M{"$lt": query.Cursor.ID}},
		}
	}
	return filter
}

func buildPage(items []resource.Resource, limit int) resource.Page {
	if len(items) <= limit {
		return resource.Page{Resources: items, HasNextPage: false}
	}
	pageItems := append([]resource.Resource(nil), items[:limit]...)
	last := pageItems[len(pageItems)-1]
	return resource.Page{
		Resources:   pageItems,
		HasNextPage: true,
		NextCursor:  &resource.Cursor{CreatedAt: last.CreatedAt, ID: last.ID},
	}
}

func (d resourceDocument) toDomain() resource.Resource {
	return resource.Resource{
		ID:          d.ID,
		WorkspaceID: d.WorkspaceID,
		FunctionKey: d.FunctionKey,
		DisplayName: d.DisplayName,
		Type:        d.ResourceType,
		Tags:        append([]string(nil), d.ResourceTags...),
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func IsTransientMongoError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}
```

- [ ] **Step 4: Run repository tests to verify pass**

Run:

```bash
go test ./internal/function-service/repositories
```

Expected: PASS.

- [ ] **Step 5: Commit repository work**

Run:

```bash
git add internal/function-service/repositories
git commit -m "feat: add mongo resource repository"
```

Expected: commit succeeds.

## Task 6: HTTP Handler, Routes, and REST Client Examples

**Files:**

- Create: `internal/function-service/handlers/resource_handler_test.go`
- Create: `internal/function-service/handlers/resource_handler.go`
- Create: `examples/api/function_resources.http`

- [ ] **Step 1: Write failing HTTP handler tests**

Create `internal/function-service/handlers/resource_handler_test.go`:

```go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/labstack/echo/v5"
)

type fakeHTTPResourceService struct {
	query resource.ListQuery
	page  resource.Page
	err   error
}

func (f *fakeHTTPResourceService) ListResources(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	f.query = query
	if f.err != nil {
		return resource.Page{}, f.err
	}
	return f.page, nil
}

func TestResourceHandlerListResources(t *testing.T) {
	service := &fakeHTTPResourceService{
		page: resource.Page{
			Resources: []resource.Resource{{
				ID:          "resource-1",
				DisplayName: "Spec",
				Type:        "document",
				Tags:        []string{"section_1"},
				CreatedAt:   time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
			}},
		},
	}
	e := echo.New()
	handler := NewResourceHandler(service)
	RegisterRoutes(e, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/workspace-1/functions/todo/resources", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if service.query.WorkspaceID != "workspace-1" || service.query.FunctionKey != "todo" {
		t.Fatalf("query = %+v, want workspace-1/todo", service.query)
	}
	if service.query.Limit != 20 {
		t.Fatalf("limit = %d, want 20", service.query.Limit)
	}
}

func TestResourceHandlerRejectsInvalidLimit(t *testing.T) {
	e := echo.New()
	handler := NewResourceHandler(&fakeHTTPResourceService{})
	RegisterRoutes(e, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/workspace-1/functions/todo/resources?limit=51", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 2: Run HTTP handler tests to verify failure**

Run:

```bash
go test ./internal/function-service/handlers
```

Expected: FAIL because `NewResourceHandler` and `RegisterRoutes` are not defined.

- [ ] **Step 3: Implement HTTP handler**

Create `internal/function-service/handlers/resource_handler.go`:

```go
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/labstack/echo/v5"
)

type HTTPResourceService interface {
	ListResources(ctx context.Context, query resource.ListQuery) (resource.Page, error)
}

type ResourceHandler struct {
	service HTTPResourceService
}

func NewResourceHandler(service HTTPResourceService) *ResourceHandler {
	return &ResourceHandler{service: service}
}

func RegisterRoutes(e *echo.Echo, handler *ResourceHandler) {
	e.GET("/api/v1/workspaces/:workspace_id/functions/:function_key/resources", handler.ListResources)
}

func (h *ResourceHandler) ListResources(c *echo.Context) error {
	limit, err := transport.ParseLimit(c.QueryParam("limit"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	cursor, err := transport.DecodeNextToken(c.QueryParam("next_token"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}

	page, err := h.service.ListResources(c.Request().Context(), resource.ListQuery{
		WorkspaceID: c.PathParam("workspace_id"),
		FunctionKey: c.PathParam("function_key"),
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusInternalServerError, transport.ErrorResponse{
			Error: transport.ErrorBody{
				Code:    "internal_error",
				Message: "Internal server error",
			},
		})
	}

	response, err := transport.NewResourceListResponse(page)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, transport.ErrorResponse{
			Error: transport.ErrorBody{
				Code:    "internal_error",
				Message: "Internal server error",
			},
		})
	}
	return c.JSON(http.StatusOK, response)
}

func validationError(message string) transport.ErrorResponse {
	return transport.ErrorResponse{
		Error: transport.ErrorBody{
			Code:    "validation_failed",
			Message: message,
			Details: map[string]any{},
		},
	}
}
```

- [ ] **Step 4: Create REST Client examples**

Create `examples/api/function_resources.http`:

```http
@baseUrl = http://localhost:8080
@workspaceId = workspace-1
@functionKey = todo
@nextToken = eyJjcmVhdGVkX2F0IjoiMjAyNi0wNS0wNVQwNzozMTowMFoiLCJpZCI6InJlc291cmNlLTEyMyJ9

### List resources
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/resources

### List resources with explicit limit
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/resources?limit=50

### List next page
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/resources?limit=20&next_token={{nextToken}}

### Invalid limit
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/resources?limit=51

### Invalid next token
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/resources?next_token=not-base64
```

- [ ] **Step 5: Run HTTP tests to verify pass**

Run:

```bash
go test ./internal/function-service/handlers ./internal/function-service/transport
```

Expected: PASS.

- [ ] **Step 6: Commit HTTP work**

Run:

```bash
git add internal/function-service/handlers/resource_handler.go internal/function-service/handlers/resource_handler_test.go examples/api/function_resources.http
git commit -m "feat: add resource list api"
```

Expected: commit succeeds.

## Task 7: Runtime Composition Root

**Files:**

- Create: `cmd/function-service/main_test.go`
- Create: `cmd/function-service/main.go`

- [ ] **Step 1: Write failing runtime tests**

Create `cmd/function-service/main_test.go`:

```go
package main

import (
	"context"
	"log/slog"
	"testing"

	serviceconfig "github.com/hao0731/workspace-permission-management/internal/function-service/config"
)

func TestNewLogger(t *testing.T) {
	if logger.New(environment.Development) == nil {
		t.Fatal("development logger = nil, want logger")
	}
	if logger.New(serviceconfig.environment.Production) == nil {
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
	var logger *slog.Logger = logger.New(serviceconfig.environment.Development)
	if logger == nil {
		t.Fatal("logger = nil, want slog logger")
	}
}
```

- [ ] **Step 2: Run runtime tests to verify failure**

Run:

```bash
go test ./cmd/function-service
```

Expected: FAIL because `logger.New` and `processIndicator` are not defined.

- [ ] **Step 3: Implement `cmd/function-service/main.go`**

Create `cmd/function-service/main.go`:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hao0731/workspace-permission-management/internal/function-service/config"
	"github.com/hao0731/workspace-permission-management/internal/function-service/handlers"
	"github.com/hao0731/workspace-permission-management/internal/function-service/repositories"
	"github.com/hao0731/workspace-permission-management/internal/function-service/services"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	"github.com/labstack/echo/v5"
	"github.com/nats-io/nats.go"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type processIndicator struct{}

func (processIndicator) Name() string {
	return "process"
}

func (processIndicator) IsHealthy(context.Context) bool {
	return true
}

func main() {
	if err := run(); err != nil {
		slog.Error("function service stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := logger.New(cfg.Environment)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoDB.URI))
	if err != nil {
		return err
	}
	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := mongoClient.Disconnect(disconnectCtx); err != nil {
			logger.Warn("failed to disconnect mongodb", "err", err)
		}
	}()

	repository := repositories.NewMongoResourceRepository(mongoClient.Database(cfg.MongoDB.Database))
	if err := repository.EnsureIndexes(ctx); err != nil {
		return err
	}
	resourceService := services.NewResourceService(repository)

	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return err
	}
	defer nc.Close()

	eventHandler := handlers.NewResourceEventHandler(resourceService, cfg.JetStream.Subject, logger)
	consumer, err := eventbus.NewJetStreamConsumer(ctx, nc, eventbus.Config{
		Stream:    cfg.JetStream.Stream,
		Subjects:  []string{cfg.JetStream.Subject},
		Durable:   cfg.JetStream.Durable,
		BatchSize: cfg.JetStream.FetchCount,
		MaxWait:   cfg.JetStream.MaxWait,
	}, eventHandler, logger)
	if err != nil {
		return err
	}

	e := echo.New()
	health.NewHealthManager(processIndicator{}).RegisterRoutes(e)
	handlers.RegisterRoutes(e, handlers.NewResourceHandler(resourceService))

	errCh := make(chan error, 2)
	go func() {
		if err := e.Start(cfg.HTTPAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	go func() {
		errCh <- consumer.Run(ctx)
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			stop()
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return nil
}

func logger.New(env config.Environment) *slog.Logger {
	if env == config.environment.Production {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
```

- [ ] **Step 4: Run runtime tests to verify pass**

Run:

```bash
go test ./cmd/function-service
```

Expected: PASS.

- [ ] **Step 5: Run focused package tests**

Run:

```bash
go test ./cmd/function-service ./internal/function-service/...
```

Expected: PASS.

- [ ] **Step 6: Commit runtime work**

Run:

```bash
git add cmd/function-service
git commit -m "feat: wire function service runtime"
```

Expected: commit succeeds.

## Task 8: Final Formatting, Verification, and Plan Finalization

**Files:**

- Modify: any Go files touched by formatting commands.
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Run gofmt**

Run:

```bash
gofmt -w cmd/function-service internal/domain/resource internal/function-service
```

Expected: command exits 0.

- [ ] **Step 2: Run go mod tidy**

Run:

```bash
go mod tidy
```

Expected: command exits 0 and `go.mod` / `go.sum` contain only required dependencies.

- [ ] **Step 3: Run full backend tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Run vet**

Run:

```bash
go vet ./...
```

Expected: PASS.

- [ ] **Step 5: Verify REST example and docs references**

Run:

```bash
test -f examples/api/function_resources.http
test -f docs/designs/function-service.md
test -f docs/plans/active/2026-05-06-function-service.md
rg -n "FUNCTION_SERVICE_LOG_FORMAT|functionresource" docs/designs internal cmd examples .env.example
```

Expected:

- The first three `test -f` commands exit 0.
- The `rg` command exits 1 because removed names are absent.

- [ ] **Step 6: Inspect final diff**

Run:

```bash
git status --short
git diff --stat
```

Expected:

- Only planned files are changed.
- No unrelated `lefthook.yml` changes are staged unless the user explicitly asked for them.

- [ ] **Step 7: Final implementation commit**

Run:

```bash
git add go.mod go.sum .gitignore .env.example cmd/function-service examples/api/function_resources.http internal/domain/resource internal/function-service
git commit -m "feat: implement function service"
```

Expected: commit succeeds if previous task commits were intentionally squashed or skipped. If each task was already committed, skip this final commit and report the commit list instead.

## Self-Review Checklist

- [ ] Plan is stored under `docs/plans/active/`.
- [ ] Plan links to `docs/designs/function-service.md`.
- [ ] Config uses `FUNCTION_SERVICE_ENV`, not `FUNCTION_SERVICE_LOG_FORMAT`.
- [ ] Domain path is `internal/domain/resource`, not `internal/domain/functionresource`.
- [ ] `.env.example` is included.
- [ ] `.env` is ignored.
- [ ] Event parsing uses CloudEvents SDK.
- [ ] Event handler maps poison messages to Terminate, retryable failures to Retry, and accepted events to Ack.
- [ ] Mongo repository preserves `created_at`, updates `updated_at`, and ignores older events.
- [ ] HTTP response uses `resources` and `page_info` with snake_case fields.
- [ ] Cursor pagination uses `created_at DESC, _id DESC`.
- [ ] `examples/api/function_resources.http` is included.
- [ ] Final verification includes `go test ./...` and `go vet ./...`.
