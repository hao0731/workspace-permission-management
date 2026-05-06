# Function Resource Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an idempotent `DELETE /api/v1/workspaces/:workspace_id/functions/:function_key/resources/:resource_id` API that deletes a projected function resource and publishes a resource-deleted CloudEvent after a successful document delete.

**Architecture:** Extend the existing `function-service` projection architecture without changing its dependency direction. HTTP handlers stay thin, the service owns the delete workflow and deterministic event metadata generation, MongoDB delete logic stays in the repository, transport builds the CloudEvent JSON, and the `cmd/function-service` composition root adapts `internal/shared/eventbus.Producer` to the service's domain-oriented publisher interface.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, NATS JetStream through `internal/shared/eventbus`, CloudEvents SDK for Go, `log/slog`, `viper`, standard `testing`.

---

## Source Design

Source design: [../../designs/function-service.md](../../designs/function-service.md)

Applicable policies:

- [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend work must keep handlers thin, keep domain and service logic free of Echo, MongoDB, NATS, and JetStream types, and treat API, database, and event schemas as explicit contracts.
- Plan documentation must live under `docs/plans/active/`, link to its source design, and be committed once finalized.

## Scope

Implement only the delete feature from the source design:

- New idempotent DELETE API.
- MongoDB `function_resources` delete by `_id`, `workspace_id`, and `function_key`.
- Resource-deleted CloudEvent with data fields `workspace_id`, `function_key`, and `resource_id`.
- Configurable delete event subject through `FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT`, defaulting to `app.todo.resource.deleted`.
- Synchronous delete-then-publish behavior: publish success returns `204`; publish failure after delete returns `500`.
- Missing document behavior: return `204`, publish no event, and log an idempotent no-op.

Do not add an outbox, background retry worker, workspace/function registry validation, permission evaluation, frontend changes, or new API endpoints beyond this DELETE route.

## File Structure

Modify:

- `go.mod`: make `github.com/google/uuid v1.6.0` a direct dependency when service code imports it.
- `.env.example`: add `FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT=app.todo.resource.deleted`.
- `examples/api/function_resources.http`: add DELETE examples and a note that missing targets still return `204`.
- `internal/domain/resource/resource.go`: add delete input/status/event domain types.
- `internal/function-service/config/config.go`: add delete subject config default and validation.
- `internal/function-service/config/config_test.go`: verify explicit and default delete subject config.
- `internal/function-service/repositories/mongo_resource_repository.go`: add delete filter and repository delete method.
- `internal/function-service/repositories/mongo_resource_repository_test.go`: verify delete filter construction.
- `internal/function-service/services/resource_service.go`: add delete workflow, publisher interface, options, clock, and ID generator.
- `internal/function-service/services/resource_service_test.go`: cover delete success, missing document, validation failure, repository failure, and publish failure.
- `internal/function-service/handlers/resource_handler.go`: add DELETE route and handler method.
- `internal/function-service/handlers/resource_handler_test.go`: cover DELETE success, validation error, and service failure.
- `cmd/function-service/main.go`: create JetStream producer and inject resource-deleted publisher into the resource service.
- `cmd/function-service/main_test.go`: test publisher adapter behavior.

Create:

- `internal/function-service/transport/resource_deleted_event.go`: build the resource-deleted CloudEvent JSON.
- `internal/function-service/transport/resource_deleted_event_test.go`: verify CloudEvent envelope and minimal payload.
- `cmd/function-service/resource_deleted_publisher.go`: adapt `eventbus.Producer` to the service publisher interface.

## Task 1: Domain and Config Contracts

**Files:**

- Modify: `internal/domain/resource/resource.go`
- Modify: `internal/function-service/config/config_test.go`
- Modify: `internal/function-service/config/config.go`
- Modify: `.env.example`

- [ ] **Step 1: Add failing config tests for the delete subject**

Add these assertions to `TestLoadReadsRequiredEnvironment` in `internal/function-service/config/config_test.go`:

```go
t.Setenv("FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT", "app.todo.resource.deleted")
```

```go
if cfg.ResourceDeletedSubject != "app.todo.resource.deleted" {
	t.Fatalf("ResourceDeletedSubject = %q, want app.todo.resource.deleted", cfg.ResourceDeletedSubject)
}
```

Add this assertion to `TestLoadAppliesOptionalDefaults`:

```go
if cfg.ResourceDeletedSubject != "app.todo.resource.deleted" {
	t.Fatalf("ResourceDeletedSubject = %q, want app.todo.resource.deleted", cfg.ResourceDeletedSubject)
}
```

Add this test function:

```go
func TestLoadRejectsBlankResourceDeletedSubject(t *testing.T) {
	t.Setenv("FUNCTION_SERVICE_HTTP_ADDR", ":8080")
	t.Setenv("FUNCTION_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("FUNCTION_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("FUNCTION_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_STREAM", "FUNCTION_RESOURCES")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_DURABLE", "function-service-resource-upserter")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_SUBJECT", "app.todo.resource.upserted")
	t.Setenv("FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT", " ")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}
```

- [ ] **Step 2: Run config tests and verify they fail**

Run:

```bash
go test ./internal/function-service/config -run 'TestLoad(Read|Applies|RejectsBlank)' -count=1
```

Expected: FAIL because `Config.ResourceDeletedSubject` does not exist.

- [ ] **Step 3: Add domain delete types**

Add these types to `internal/domain/resource/resource.go` after `ListQuery`:

```go
type DeleteInput struct {
	WorkspaceID string
	FunctionKey string
	ResourceID  string
}

type DeletedEvent struct {
	WorkspaceID string
	FunctionKey string
	ResourceID  string
	EventID     string
	EventTime   time.Time
}
```

Add this status type after `UpsertStatus`:

```go
type DeleteStatus string
```

Add these constants after the existing upsert status constants:

```go
const (
	DeleteStatusDeleted  DeleteStatus = "deleted"
	DeleteStatusNotFound DeleteStatus = "not_found"
)
```

- [ ] **Step 4: Add delete subject config**

Update `internal/function-service/config/config.go`:

```go
type Config struct {
	Environment            environment.Environment
	HTTPAddr               string
	MongoDB                MongoDBConfig
	NATS                   NATSConfig
	JetStream              JetStreamConfig
	ResourceDeletedSubject string
	ShutdownTimeout        time.Duration
}
```

Add the default in `Load`:

```go
v.SetDefault("FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT", "app.todo.resource.deleted")
```

Set the field in the `Config` literal:

```go
ResourceDeletedSubject: v.GetString("FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT"),
```

Add the field to the `required` map in `Validate`:

```go
"FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT": c.ResourceDeletedSubject,
```

- [ ] **Step 5: Update `.env.example`**

Add this line under the JetStream settings in `.env.example`:

```dotenv
FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT=app.todo.resource.deleted
```

- [ ] **Step 6: Run config tests and verify they pass**

Run:

```bash
go test ./internal/function-service/config -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit domain and config contract changes**

Run:

```bash
git add internal/domain/resource/resource.go internal/function-service/config/config.go internal/function-service/config/config_test.go .env.example
git commit -m "feat: add resource delete config contract"
```

## Task 2: Resource-Deleted CloudEvent Builder

**Files:**

- Create: `internal/function-service/transport/resource_deleted_event_test.go`
- Create: `internal/function-service/transport/resource_deleted_event.go`

- [ ] **Step 1: Write failing CloudEvent tests**

Create `internal/function-service/transport/resource_deleted_event_test.go`:

```go
package transport

import (
	"encoding/json"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func TestNewResourceDeletedEvent(t *testing.T) {
	eventTime := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)

	data, err := NewResourceDeletedEvent(resource.DeletedEvent{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
		EventID:     "event-1",
		EventTime:   eventTime,
	}, "app.todo.resource.deleted")
	if err != nil {
		t.Fatalf("NewResourceDeletedEvent error = %v, want nil", err)
	}

	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("unmarshal cloudevent: %v", err)
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("validate cloudevent: %v", err)
	}
	if event.SpecVersion() != "1.0" {
		t.Fatalf("SpecVersion = %q, want 1.0", event.SpecVersion())
	}
	if event.Type() != "app.todo.resource.deleted" {
		t.Fatalf("Type = %q, want app.todo.resource.deleted", event.Type())
	}
	if event.Source() != "function-service" {
		t.Fatalf("Source = %q, want function-service", event.Source())
	}
	if event.Subject() != "resource-1" {
		t.Fatalf("Subject = %q, want resource-1", event.Subject())
	}
	if event.ID() != "event-1" {
		t.Fatalf("ID = %q, want event-1", event.ID())
	}
	if !event.Time().Equal(eventTime) {
		t.Fatalf("Time = %s, want %s", event.Time(), eventTime)
	}
	if event.DataContentType() != "application/json" {
		t.Fatalf("DataContentType = %q, want application/json", event.DataContentType())
	}

	var payload map[string]string
	if err := event.DataAs(&payload); err != nil {
		t.Fatalf("DataAs error = %v, want nil", err)
	}
	if len(payload) != 3 {
		t.Fatalf("payload keys = %d, want 3", len(payload))
	}
	if payload["workspace_id"] != "workspace-1" {
		t.Fatalf("workspace_id = %q, want workspace-1", payload["workspace_id"])
	}
	if payload["function_key"] != "todo" {
		t.Fatalf("function_key = %q, want todo", payload["function_key"])
	}
	if payload["resource_id"] != "resource-1" {
		t.Fatalf("resource_id = %q, want resource-1", payload["resource_id"])
	}
}
```

- [ ] **Step 2: Run transport test and verify it fails**

Run:

```bash
go test ./internal/function-service/transport -run TestNewResourceDeletedEvent -count=1
```

Expected: FAIL because `NewResourceDeletedEvent` is not defined.

- [ ] **Step 3: Implement CloudEvent builder**

Create `internal/function-service/transport/resource_deleted_event.go`:

```go
package transport

import (
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type resourceDeletedData struct {
	WorkspaceID string `json:"workspace_id"`
	FunctionKey string `json:"function_key"`
	ResourceID  string `json:"resource_id"`
}

func NewResourceDeletedEvent(input resource.DeletedEvent, eventType string) ([]byte, error) {
	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetType(eventType)
	event.SetSource("function-service")
	event.SetSubject(input.ResourceID)
	event.SetID(input.EventID)
	event.SetTime(input.EventTime)

	if err := event.SetData(cloudevents.ApplicationJSON, resourceDeletedData{
		WorkspaceID: input.WorkspaceID,
		FunctionKey: input.FunctionKey,
		ResourceID:  input.ResourceID,
	}); err != nil {
		return nil, fmt.Errorf("set resource deleted event data: %w", err)
	}

	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal resource deleted event: %w", err)
	}
	return data, nil
}
```

- [ ] **Step 4: Run transport tests and verify they pass**

Run:

```bash
go test ./internal/function-service/transport -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit CloudEvent builder**

Run:

```bash
git add internal/function-service/transport/resource_deleted_event.go internal/function-service/transport/resource_deleted_event_test.go
git commit -m "feat: build resource deleted cloudevent"
```

## Task 3: Repository Delete Operation

**Files:**

- Modify: `internal/function-service/repositories/mongo_resource_repository_test.go`
- Modify: `internal/function-service/repositories/mongo_resource_repository.go`

- [ ] **Step 1: Write failing delete filter test**

Add this test to `internal/function-service/repositories/mongo_resource_repository_test.go`:

```go
func TestBuildDeleteFilter(t *testing.T) {
	got := buildDeleteFilter(resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	})
	want := bson.M{
		"_id":          "resource-1",
		"workspace_id": "workspace-1",
		"function_key": "todo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}
```

- [ ] **Step 2: Run repository test and verify it fails**

Run:

```bash
go test ./internal/function-service/repositories -run TestBuildDeleteFilter -count=1
```

Expected: FAIL because `buildDeleteFilter` is not defined.

- [ ] **Step 3: Implement repository delete**

Add this method to `internal/function-service/repositories/mongo_resource_repository.go` after `List`:

```go
func (r *MongoResourceRepository) Delete(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error) {
	result, err := r.collection.DeleteOne(ctx, buildDeleteFilter(input))
	if err != nil {
		return "", fmt.Errorf("delete resource: %w", err)
	}
	if result.DeletedCount == 0 {
		return resource.DeleteStatusNotFound, nil
	}
	return resource.DeleteStatusDeleted, nil
}
```

Add this helper after `buildListFilter`:

```go
func buildDeleteFilter(input resource.DeleteInput) bson.M {
	return bson.M{
		"_id":          input.ResourceID,
		"workspace_id": input.WorkspaceID,
		"function_key": input.FunctionKey,
	}
}
```

- [ ] **Step 4: Run repository tests and verify they pass**

Run:

```bash
go test ./internal/function-service/repositories -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit repository delete operation**

Run:

```bash
git add internal/function-service/repositories/mongo_resource_repository.go internal/function-service/repositories/mongo_resource_repository_test.go
git commit -m "feat: delete function resource projection"
```

## Task 4: Service Delete Workflow

**Files:**

- Modify: `go.mod`
- Modify: `internal/function-service/services/resource_service_test.go`
- Modify: `internal/function-service/services/resource_service.go`

- [ ] **Step 1: Write failing service tests**

Update `fakeResourceRepository` in `internal/function-service/services/resource_service_test.go`:

```go
type fakeResourceRepository struct {
	upsertStatus resource.UpsertStatus
	upsertInput  resource.UpsertInput
	upsertErr    error
	listQuery    resource.ListQuery
	listPage     resource.Page
	listErr      error
	deleteStatus resource.DeleteStatus
	deleteInput  resource.DeleteInput
	deleteErr    error
}
```

Add this method to the fake:

```go
func (f *fakeResourceRepository) Delete(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error) {
	f.deleteInput = input
	if f.deleteErr != nil {
		return "", f.deleteErr
	}
	return f.deleteStatus, nil
}
```

Add this fake publisher:

```go
type fakeResourceDeletedPublisher struct {
	event resource.DeletedEvent
	calls int
	err   error
}

func (f *fakeResourceDeletedPublisher) PublishResourceDeleted(ctx context.Context, event resource.DeletedEvent) error {
	f.calls++
	f.event = event
	return f.err
}
```

Add these tests:

```go
func TestResourceServiceDeleteResourcePublishesAfterDelete(t *testing.T) {
	eventTime := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := &fakeResourceRepository{deleteStatus: resource.DeleteStatusDeleted}
	publisher := &fakeResourceDeletedPublisher{}
	service := NewResourceService(repo,
		WithResourceDeletedPublisher(publisher),
		WithClock(func() time.Time { return eventTime }),
		WithIDGenerator(func() string { return "event-1" }),
	)

	status, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	})
	if err != nil {
		t.Fatalf("DeleteResource error = %v, want nil", err)
	}
	if status != resource.DeleteStatusDeleted {
		t.Fatalf("status = %q, want %q", status, resource.DeleteStatusDeleted)
	}
	if repo.deleteInput.ResourceID != "resource-1" {
		t.Fatalf("repo delete input = %+v, want resource-1", repo.deleteInput)
	}
	if publisher.calls != 1 {
		t.Fatalf("publisher calls = %d, want 1", publisher.calls)
	}
	if publisher.event.EventID != "event-1" {
		t.Fatalf("event id = %q, want event-1", publisher.event.EventID)
	}
	if !publisher.event.EventTime.Equal(eventTime) {
		t.Fatalf("event time = %s, want %s", publisher.event.EventTime, eventTime)
	}
	if publisher.event.WorkspaceID != "workspace-1" || publisher.event.FunctionKey != "todo" || publisher.event.ResourceID != "resource-1" {
		t.Fatalf("event = %+v, want workspace-1/todo/resource-1", publisher.event)
	}
}

func TestResourceServiceDeleteResourceMissingDoesNotPublish(t *testing.T) {
	repo := &fakeResourceRepository{deleteStatus: resource.DeleteStatusNotFound}
	publisher := &fakeResourceDeletedPublisher{}
	service := NewResourceService(repo, WithResourceDeletedPublisher(publisher))

	status, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	})
	if err != nil {
		t.Fatalf("DeleteResource error = %v, want nil", err)
	}
	if status != resource.DeleteStatusNotFound {
		t.Fatalf("status = %q, want %q", status, resource.DeleteStatusNotFound)
	}
	if publisher.calls != 0 {
		t.Fatalf("publisher calls = %d, want 0", publisher.calls)
	}
}

func TestResourceServiceDeleteResourceRejectsInvalidInput(t *testing.T) {
	service := NewResourceService(&fakeResourceRepository{})

	_, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "",
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestResourceServiceDeleteResourceReturnsRepositoryError(t *testing.T) {
	repo := &fakeResourceRepository{deleteErr: errors.New("database unavailable")}
	service := NewResourceService(repo, WithResourceDeletedPublisher(&fakeResourceDeletedPublisher{}))

	_, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	})
	if err == nil {
		t.Fatal("DeleteResource error = nil, want error")
	}
}

func TestResourceServiceDeleteResourceReturnsPublishError(t *testing.T) {
	repo := &fakeResourceRepository{deleteStatus: resource.DeleteStatusDeleted}
	publisher := &fakeResourceDeletedPublisher{err: errors.New("publish failed")}
	service := NewResourceService(repo, WithResourceDeletedPublisher(publisher))

	_, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	})
	if err == nil {
		t.Fatal("DeleteResource error = nil, want error")
	}
	if publisher.calls != 1 {
		t.Fatalf("publisher calls = %d, want 1", publisher.calls)
	}
}
```

- [ ] **Step 2: Run service tests and verify they fail**

Run:

```bash
go test ./internal/function-service/services -run TestResourceServiceDeleteResource -count=1
```

Expected: FAIL because `ResourceRepository.Delete`, `WithResourceDeletedPublisher`, `WithClock`, `WithIDGenerator`, and `DeleteResource` are not defined.

- [ ] **Step 3: Make uuid a direct dependency**

Run:

```bash
go get github.com/google/uuid@v1.6.0
```

Expected: `go.mod` lists `github.com/google/uuid v1.6.0` in the direct `require` block.

- [ ] **Step 4: Implement service options and delete workflow**

Update `internal/function-service/services/resource_service.go` imports:

```go
import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)
```

Update the repository interface:

```go
type ResourceRepository interface {
	Upsert(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error)
	List(ctx context.Context, query resource.ListQuery) (resource.Page, error)
	Delete(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error)
}
```

Add publisher and option types after `ResourceRepository`:

```go
type ResourceDeletedPublisher interface {
	PublishResourceDeleted(ctx context.Context, event resource.DeletedEvent) error
}

type Option func(*ResourceService)

func WithResourceDeletedPublisher(publisher ResourceDeletedPublisher) Option {
	return func(s *ResourceService) {
		s.deletedPublisher = publisher
	}
}

func WithClock(clock func() time.Time) Option {
	return func(s *ResourceService) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func WithIDGenerator(generator func() string) Option {
	return func(s *ResourceService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}
```

Update `ResourceService` and constructor:

```go
type ResourceService struct {
	repository       ResourceRepository
	deletedPublisher ResourceDeletedPublisher
	clock            func() time.Time
	idGenerator      func() string
}

func NewResourceService(repository ResourceRepository, opts ...Option) *ResourceService {
	service := &ResourceService{
		repository:  repository,
		clock:       func() time.Time { return time.Now().UTC() },
		idGenerator: uuid.NewString,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}
```

Add this method after `ListResources`:

```go
func (s *ResourceService) DeleteResource(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error) {
	if err := validateDeleteInput(input); err != nil {
		return "", err
	}
	status, err := s.repository.Delete(ctx, input)
	if err != nil {
		return "", fmt.Errorf("delete resource: %w", err)
	}
	if status == resource.DeleteStatusNotFound {
		return status, nil
	}
	if s.deletedPublisher == nil {
		return "", fmt.Errorf("publish resource deleted event: publisher is not configured")
	}
	if err := s.deletedPublisher.PublishResourceDeleted(ctx, resource.DeletedEvent{
		WorkspaceID: input.WorkspaceID,
		FunctionKey: input.FunctionKey,
		ResourceID:  input.ResourceID,
		EventID:     s.idGenerator(),
		EventTime:   s.clock(),
	}); err != nil {
		return "", fmt.Errorf("publish resource deleted event: %w", err)
	}
	return status, nil
}
```

Add this validator after `validateListQuery`:

```go
func validateDeleteInput(input resource.DeleteInput) error {
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return fmt.Errorf("%w: function key is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.ResourceID) == "" {
		return fmt.Errorf("%w: resource id is required", resource.ErrInvalidInput)
	}
	return nil
}
```

- [ ] **Step 5: Run service tests and verify they pass**

Run:

```bash
go test ./internal/function-service/services -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit service workflow**

Run:

```bash
git add go.mod go.sum internal/function-service/services/resource_service.go internal/function-service/services/resource_service_test.go
git commit -m "feat: add resource delete workflow"
```

## Task 5: HTTP DELETE Route

**Files:**

- Modify: `internal/function-service/handlers/resource_handler_test.go`
- Modify: `internal/function-service/handlers/resource_handler.go`

- [ ] **Step 1: Write failing handler tests**

Update `fakeHTTPResourceService` in `internal/function-service/handlers/resource_handler_test.go`:

```go
type fakeHTTPResourceService struct {
	query        resource.ListQuery
	page         resource.Page
	err          error
	deleteInput  resource.DeleteInput
	deleteStatus resource.DeleteStatus
	deleteErr    error
}
```

Add this fake method:

```go
func (f *fakeHTTPResourceService) DeleteResource(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error) {
	f.deleteInput = input
	if f.deleteErr != nil {
		return "", f.deleteErr
	}
	return f.deleteStatus, nil
}
```

Add `errors` to the test imports:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/labstack/echo/v5"
)
```

Add these tests:

```go
func TestResourceHandlerDeleteResource(t *testing.T) {
	service := &fakeHTTPResourceService{deleteStatus: resource.DeleteStatusDeleted}
	e := echo.New()
	handler := NewResourceHandler(service)
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/functions/todo/resources/resource-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body len = %d, want 0", rec.Body.Len())
	}
	if service.deleteInput.WorkspaceID != "workspace-1" || service.deleteInput.FunctionKey != "todo" || service.deleteInput.ResourceID != "resource-1" {
		t.Fatalf("delete input = %+v, want workspace-1/todo/resource-1", service.deleteInput)
	}
}

func TestResourceHandlerDeleteMissingResourceStillReturnsNoContent(t *testing.T) {
	service := &fakeHTTPResourceService{deleteStatus: resource.DeleteStatusNotFound}
	e := echo.New()
	handler := NewResourceHandler(service)
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/functions/todo/resources/resource-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestResourceHandlerDeleteValidationError(t *testing.T) {
	service := &fakeHTTPResourceService{deleteErr: resource.ErrInvalidInput}
	e := echo.New()
	handler := NewResourceHandler(service)
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/functions/todo/resources/resource-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestResourceHandlerDeleteServiceFailure(t *testing.T) {
	service := &fakeHTTPResourceService{deleteErr: errors.New("publish failed")}
	e := echo.New()
	handler := NewResourceHandler(service)
	RegisterRoutes(e, handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/functions/todo/resources/resource-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
```

- [ ] **Step 2: Run handler tests and verify they fail**

Run:

```bash
go test ./internal/function-service/handlers -run TestResourceHandlerDelete -count=1
```

Expected: FAIL because `HTTPResourceService.DeleteResource` and `ResourceHandler.DeleteResource` are not defined and the DELETE route is not registered.

- [ ] **Step 3: Implement DELETE route and handler**

Update `internal/function-service/handlers/resource_handler.go` imports:

```go
import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/labstack/echo/v5"
)
```

Update `HTTPResourceService` in `internal/function-service/handlers/resource_handler.go`:

```go
type HTTPResourceService interface {
	ListResources(ctx context.Context, query resource.ListQuery) (resource.Page, error)
	DeleteResource(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error)
}
```

Update `ResourceHandler` and constructor:

```go
type ResourceHandler struct {
	service HTTPResourceService
	logger  *slog.Logger
}

func NewResourceHandler(service HTTPResourceService) *ResourceHandler {
	return &ResourceHandler{service: service, logger: slog.Default()}
}
```

Update `RegisterRoutes`:

```go
func RegisterRoutes(e *echo.Echo, handler *ResourceHandler) {
	e.GET("/api/v1/workspaces/:workspace_id/functions/:function_key/resources", handler.ListResources)
	e.DELETE("/api/v1/workspaces/:workspace_id/functions/:function_key/resources/:resource_id", handler.DeleteResource)
}
```

Add this method after `ListResources`:

```go
func (h *ResourceHandler) DeleteResource(c *echo.Context) error {
	status, err := h.service.DeleteResource(c.Request().Context(), resource.DeleteInput{
		WorkspaceID: c.Param("workspace_id"),
		FunctionKey: c.Param("function_key"),
		ResourceID:  c.Param("resource_id"),
	})
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to delete resource",
			"err", err,
			"workspace_id", c.Param("workspace_id"),
			"function_key", c.Param("function_key"),
			"resource_id", c.Param("resource_id"),
		)
		return c.JSON(http.StatusInternalServerError, transport.ErrorResponse{
			Error: transport.ErrorBody{
				Code:    "internal_error",
				Message: "Internal server error",
			},
		})
	}
	if status == resource.DeleteStatusNotFound {
		h.logger.Info("resource delete target already absent",
			"workspace_id", c.Param("workspace_id"),
			"function_key", c.Param("function_key"),
			"resource_id", c.Param("resource_id"),
		)
		return c.NoContent(http.StatusNoContent)
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 4: Run handler tests and verify they pass**

Run:

```bash
go test ./internal/function-service/handlers -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit HTTP DELETE route**

Run:

```bash
git add internal/function-service/handlers/resource_handler.go internal/function-service/handlers/resource_handler_test.go
git commit -m "feat: expose resource delete api"
```

## Task 6: Runtime Publisher Wiring and Manual API Examples

**Files:**

- Create: `cmd/function-service/resource_deleted_publisher.go`
- Modify: `cmd/function-service/main_test.go`
- Modify: `cmd/function-service/main.go`
- Modify: `examples/api/function_resources.http`

- [ ] **Step 1: Write failing publisher adapter test**

Add these imports to `cmd/function-service/main_test.go`:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
)
```

Add this fake publisher:

```go
type fakeMessagePublisher struct {
	subject string
	data    []byte
	err     error
}

func (f *fakeMessagePublisher) Publish(ctx context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error {
	f.subject = subject
	f.data = append([]byte(nil), data...)
	return f.err
}
```

Add these tests:

```go
func TestResourceDeletedPublisherPublishesConfiguredSubject(t *testing.T) {
	messagePublisher := &fakeMessagePublisher{}
	publisher := newResourceDeletedPublisher(messagePublisher, "app.todo.resource.deleted")
	eventTime := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)

	err := publisher.PublishResourceDeleted(context.Background(), resource.DeletedEvent{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
		EventID:     "event-1",
		EventTime:   eventTime,
	})
	if err != nil {
		t.Fatalf("PublishResourceDeleted error = %v, want nil", err)
	}
	if messagePublisher.subject != "app.todo.resource.deleted" {
		t.Fatalf("subject = %q, want app.todo.resource.deleted", messagePublisher.subject)
	}

	var event cloudevents.Event
	if err := json.Unmarshal(messagePublisher.data, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event.Type() != "app.todo.resource.deleted" {
		t.Fatalf("event type = %q, want app.todo.resource.deleted", event.Type())
	}
	if event.Subject() != "resource-1" {
		t.Fatalf("event subject = %q, want resource-1", event.Subject())
	}
}

func TestResourceDeletedPublisherReturnsPublishError(t *testing.T) {
	messagePublisher := &fakeMessagePublisher{err: errors.New("publish failed")}
	publisher := newResourceDeletedPublisher(messagePublisher, "app.todo.resource.deleted")

	err := publisher.PublishResourceDeleted(context.Background(), resource.DeletedEvent{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
		EventID:     "event-1",
		EventTime:   time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("PublishResourceDeleted error = nil, want error")
	}
}
```

- [ ] **Step 2: Run cmd tests and verify they fail**

Run:

```bash
go test ./cmd/function-service -run TestResourceDeletedPublisher -count=1
```

Expected: FAIL because `newResourceDeletedPublisher` is not defined.

- [ ] **Step 3: Implement publisher adapter**

Create `cmd/function-service/resource_deleted_publisher.go`:

```go
package main

import (
	"context"
	"fmt"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type messagePublisher interface {
	Publish(ctx context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error
}

type resourceDeletedPublisher struct {
	publisher messagePublisher
	subject   string
}

func newResourceDeletedPublisher(publisher messagePublisher, subject string) resourceDeletedPublisher {
	return resourceDeletedPublisher{
		publisher: publisher,
		subject:   subject,
	}
}

func (p resourceDeletedPublisher) PublishResourceDeleted(ctx context.Context, event resource.DeletedEvent) error {
	data, err := transport.NewResourceDeletedEvent(event, p.subject)
	if err != nil {
		return fmt.Errorf("build resource deleted event: %w", err)
	}
	if err := p.publisher.Publish(ctx, p.subject, data); err != nil {
		return fmt.Errorf("publish resource deleted event: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Wire producer in `main.go`**

Update `cmd/function-service/main.go` imports:

```go
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
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
	"github.com/labstack/echo/v5"
	"github.com/nats-io/nats.go"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)
```

Replace the service construction and NATS setup block with this sequence:

```go
repository := repositories.NewMongoResourceRepository(mongoClient.Database(cfg.MongoDB.Database))
if ensureIndexErr := repository.EnsureIndexes(ctx); ensureIndexErr != nil {
	return ensureIndexErr
}

nc, err := nats.Connect(cfg.NATS.URL)
if err != nil {
	return err
}
defer nc.Close()

producer, err := eventbus.NewJetStreamProducer(ctx, nc, logger)
if err != nil {
	return err
}

resourceService := services.NewResourceService(repository,
	services.WithResourceDeletedPublisher(newResourceDeletedPublisher(producer, cfg.ResourceDeletedSubject)),
)

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
```

- [ ] **Step 5: Update REST Client examples**

Append these examples to `examples/api/function_resources.http`:

```http
### Delete resource
DELETE {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/resources/resource-123

### Delete resource again returns 204 and publishes no event when already absent
DELETE {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/resources/resource-123
```

- [ ] **Step 6: Run cmd tests and verify they pass**

Run:

```bash
go test ./cmd/function-service -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit runtime wiring**

Run:

```bash
git add cmd/function-service/main.go cmd/function-service/main_test.go cmd/function-service/resource_deleted_publisher.go examples/api/function_resources.http
git commit -m "feat: publish resource delete events"
```

## Task 7: Final Verification

**Files:**

- Verify all changed backend and documentation files.

- [ ] **Step 1: Run formatting**

Run:

```bash
gofmt -w cmd/function-service internal/domain/resource internal/function-service
```

Expected: command exits with status 0.

- [ ] **Step 2: Run all backend tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Run vet**

Run:

```bash
go vet ./...
```

Expected: PASS.

- [ ] **Step 4: Check working tree**

Run:

```bash
git status --short
```

Expected: only intentional changes are present. The pre-existing unstaged `lefthook.yml` change may still be present and must not be included unless the user explicitly asks for it.

- [ ] **Step 5: Commit final formatting or dependency cleanup if needed**

If `gofmt`, `go test`, or `go vet` changed files or required dependency cleanup, run:

```bash
git add go.mod go.sum cmd/function-service internal/domain/resource internal/function-service examples/api/function_resources.http
git commit -m "chore: finalize resource delete implementation"
```

Expected: commit succeeds only if there are intentional implementation changes to record.

## Implementation Notes

- Missing delete targets intentionally return `204` and do not publish events.
- The service returns `500` if publish fails after MongoDB deletion. This is an accepted phase-one risk from the source design because there is no outbox.
- The delete event payload must stay minimal: `workspace_id`, `function_key`, and `resource_id`.
- Keep `eventbus.Producer`, NATS, and JetStream types out of `internal/function-service/services`.
- Keep MongoDB driver types out of `internal/function-service/services` and `internal/domain/resource`.
- Do not modify the unrelated unstaged `lefthook.yml` change.
