# Permission Client System Resource Registration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Register non-empty system resource attributes with the shared permission client after `function-service` successfully persists system resource definitions and derived attributes.

**Architecture:** Add a shared `internal/shared/interactions/permission.Client` boundary and an `inmemory` implementation that logs debug details through an injected logger and returns success. Inject the client into `SystemResourceService`; after the MongoDB transaction commits, call the client with the complete derived attribute set, return a `502 permission_registration_failed` response when registration fails, and keep already committed local data intact.

**Tech Stack:** Go 1.25, `context`, `log/slog`, Echo v5, MongoDB repository boundary, standard `testing`, existing `internal/domain/resource` models.

---

## Source Designs And Policies

- Source design: [../../designs/function-service-system-resource-api-design.md](../../designs/function-service-system-resource-api-design.md)
- Source design: [../../designs/permission-client-design.md](../../designs/permission-client-design.md)
- Source design: [../../designs/inmemory-permission-client-design.md](../../designs/inmemory-permission-client-design.md)
- Related design: [../../designs/function-service.md](../../designs/function-service.md)
- Policy: [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- Policy: [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- This is backend implementation plus design-plan documentation work.
- Domain packages must remain independent of Echo, MongoDB, HTTP clients, NATS, and service-specific packages.
- `internal/shared/interactions/permission` must not depend on `internal/function-service` or any other service-specific package.
- `function-service` services may depend on domain types and shared interaction interfaces, but not Echo, MongoDB driver types, or transport DTOs.
- Handlers stay thin and map service/domain errors to HTTP responses.
- This plan lives under `docs/plans/active/` and links to its source design documents.

## Scope

Implement:

- `internal/shared/interactions/permission/client.go`.
- `internal/shared/interactions/permission/inmemory/client.go`.
- Unit tests for the in-memory client.
- Post-commit permission registration in `SystemResourceService.SaveSystemResources`.
- `ErrPermissionRegistrationFailed` sentinel in `internal/function-service/services`.
- `slog.Error` logging when permission registration fails.
- `502 Bad Gateway` handler mapping with error code `permission_registration_failed`.
- `cmd/function-service/main.go` wiring with `inmemory.New(inmemory.WithLogger(logger))`.

Do not implement:

- A real external permission service client.
- Permission service HTTP contracts, authentication, retry policy, or timeout configuration.
- Outbox storage or a background worker.
- Any frontend changes.

## File Structure And Responsibilities

- Create: `internal/shared/interactions/permission/client.go`
  - Shared client interface for permission registration.
- Create: `internal/shared/interactions/permission/inmemory/client.go`
  - Temporary debug-logging client that implements the shared interface, accepts `WithLogger`, and returns `nil`.
- Create: `internal/shared/interactions/permission/inmemory/client_test.go`
  - Compile-time interface assertion, success behavior, and no-mutation check.
- Modify: `internal/function-service/services/system_resource_service.go`
  - Inject permission client and logger, expose `ErrPermissionRegistrationFailed`, call the client after local transaction success.
- Modify: `internal/function-service/services/system_resource_service_test.go`
  - Add fake permission client, verify post-commit call, skip behavior for empty attributes, and failure logging/error behavior.
- Modify: `internal/function-service/handlers/system_resource_handler.go`
  - Map `ErrPermissionRegistrationFailed` to `502 permission_registration_failed`.
- Modify: `internal/function-service/handlers/system_resource_handler_test.go`
  - Add handler test for permission registration failure response.
- Modify: `cmd/function-service/main.go`
  - Wire `internal/shared/interactions/permission/inmemory.New(inmemory.WithLogger(logger))` into `SystemResourceService`.

---

### Task 1: Shared Permission Client And In-Memory Implementation

**Files:**

- Create: `internal/shared/interactions/permission/client.go`
- Create: `internal/shared/interactions/permission/inmemory/client.go`
- Create: `internal/shared/interactions/permission/inmemory/client_test.go`

- [ ] **Step 1: Write the failing in-memory client tests**

Create `internal/shared/interactions/permission/inmemory/client_test.go`:

```go
package inmemory

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)

var _ clientpermission.Client = (*Client)(nil)

func TestClientRegisterResourceAttributesReturnsNilAndLogs(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	attrs := []resource.ResourceAttribute{resource.ResourceAttribute("can_edit_private_repo")}
	client := New(WithLogger(logger))

	err := client.RegisterResourceAttributes(context.Background(), "todo", attrs)
	if err != nil {
		t.Fatalf("RegisterResourceAttributes error = %v, want nil", err)
	}
	if attrs[0] != resource.ResourceAttribute("can_edit_private_repo") {
		t.Fatalf("attrs mutated to %#v", attrs)
	}

	output := logBuffer.String()
	if !strings.Contains(output, "register resource attributes with in-memory permission client") {
		t.Fatalf("log output = %q, want registration message", output)
	}
	if !strings.Contains(output, "system_id=todo") {
		t.Fatalf("log output = %q, want system_id", output)
	}
	if !strings.Contains(output, "resource_attribute_count=1") {
		t.Fatalf("log output = %q, want resource_attribute_count", output)
	}
}

func TestNewReturnsPermissionClient(t *testing.T) {
	client := New()
	if client == nil {
		t.Fatal("New() = nil, want permission client")
	}

	if err := client.RegisterResourceAttributes(context.Background(), "todo", nil); err != nil {
		t.Fatalf("RegisterResourceAttributes error = %v, want nil", err)
	}
}
```

- [ ] **Step 2: Run the targeted shared client test and confirm failure**

Run:

```bash
go test ./internal/shared/interactions/permission/... -run 'TestClient|TestNew' -v
```

Expected: FAIL because `internal/shared/interactions/permission` does not exist and `Client` / `New` are undefined.

- [ ] **Step 3: Add the shared permission client interface**

Create `internal/shared/interactions/permission/client.go`:

```go
package permission

import (
	"context"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type Client interface {
	RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error
}
```

- [ ] **Step 4: Add the in-memory permission client**

Create `internal/shared/interactions/permission/inmemory/client.go`:

```go
package inmemory

import (
	"context"
	"log/slog"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)

type Client struct {
	logger *slog.Logger
}

type Option func(*Client)

func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) {
		if logger != nil {
			c.logger = logger
		}
	}
}

func New(opts ...Option) clientpermission.Client {
	client := &Client{logger: slog.Default()}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client
}

func (c *Client) RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error {
	c.logger.DebugContext(ctx, "register resource attributes with in-memory permission client",
		"system_id", systemID,
		"resource_attribute_count", len(resourceAttributes),
		"resource_attributes", resourceAttributes,
	)
	return nil
}
```

- [ ] **Step 5: Run the shared client tests and confirm success**

Run:

```bash
go test ./internal/shared/interactions/permission/... -v
```

Expected: PASS for `permission` and `permission/inmemory`.

- [ ] **Step 6: Commit shared client changes**

Run:

```bash
git add internal/shared/interactions/permission
git commit -m "feat: add permission client interface"
```

Expected: commit succeeds and includes only the shared permission client files.

---

### Task 2: System Resource Service Permission Registration

**Files:**

- Modify: `internal/function-service/services/system_resource_service.go`
- Modify: `internal/function-service/services/system_resource_service_test.go`

- [ ] **Step 1: Add fake permission client and transaction state to service tests**

In `internal/function-service/services/system_resource_service_test.go`, update the import block to include `bytes`, `log/slog`, and `strings`:

```go
import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)
```

Update `fakeSystemResourceRepository` with transaction state fields:

```go
type fakeSystemResourceRepository struct {
	transactionCalls     int
	inTransaction        bool
	transactionCommitted bool
	existing             []resource.ResourceDefinition
	latest               []resource.ResourceDefinition
	saved                []resource.ResourceDefinition
	attributes           resource.ResourceAttributes
	attributesFound      bool
	attributesSaved      resource.ResourceAttributes
	err                  error
}
```

Replace `RunInTransaction` with:

```go
func (f *fakeSystemResourceRepository) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	f.transactionCalls++
	f.inTransaction = true
	err := fn(ctx)
	f.inTransaction = false
	if err != nil {
		return err
	}
	f.transactionCommitted = true
	return nil
}
```

Add this fake permission client below the fake repository methods:

```go
type fakePermissionClient struct {
	repo                       *fakeSystemResourceRepository
	calls                      int
	systemID                   string
	attributes                 []resource.ResourceAttribute
	calledDuringTransaction    bool
	transactionCommittedAtCall bool
	err                        error
}

func (f *fakePermissionClient) RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error {
	f.calls++
	f.systemID = systemID
	f.attributes = append([]resource.ResourceAttribute(nil), resourceAttributes...)
	if f.repo != nil {
		f.calledDuringTransaction = f.repo.inTransaction
		f.transactionCommittedAtCall = f.repo.transactionCommitted
	}
	return f.err
}
```

- [ ] **Step 2: Update existing service tests to pass a permission client**

In each existing `NewSystemResourceService` call, pass a permission client as the third argument.

For `TestSystemResourceServiceSaveSystemResources`, create the client next to the repository:

```go
permissionClient := &fakePermissionClient{repo: repo}
service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20}, permissionClient,
	WithSystemResourceClock(func() time.Time { return now }),
	WithSystemResourceIDGenerator(sequenceIDs("new-action", "new-type", "attributes-1")),
)
```

After the existing attribute assertion in `TestSystemResourceServiceSaveSystemResources`, add:

```go
if permissionClient.calls != 1 {
	t.Fatalf("permission client calls = %d, want 1", permissionClient.calls)
}
if permissionClient.systemID != "todo" {
	t.Fatalf("permission systemID = %q, want todo", permissionClient.systemID)
}
if !reflect.DeepEqual(permissionClient.attributes, wantAttributes) {
	t.Fatalf("permission attributes = %#v, want %#v", permissionClient.attributes, wantAttributes)
}
if permissionClient.calledDuringTransaction {
	t.Fatal("permission client called during transaction, want post-commit call")
}
if !permissionClient.transactionCommittedAtCall {
	t.Fatal("permission client called before transaction commit")
}
```

For tests that do not need registration assertions, pass `&fakePermissionClient{}` as the third argument:

```go
service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20}, &fakePermissionClient{},
	WithSystemResourceClock(func() time.Time { return now }),
	WithSystemResourceIDGenerator(sequenceIDs("definition-1", "attributes-1")),
)
```

```go
service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 1, Actions: 5, Tags: 20}, &fakePermissionClient{})
```

```go
service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20}, &fakePermissionClient{})
```

- [ ] **Step 3: Add failing tests for skip and failure behavior**

In `TestSystemResourceServiceDoesNotWriteAttributesWhenIncomplete`, replace the inline fake client with a variable:

```go
permissionClient := &fakePermissionClient{}
service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20}, permissionClient,
	WithSystemResourceClock(func() time.Time { return now }),
	WithSystemResourceIDGenerator(sequenceIDs("definition-1", "attributes-1")),
)
```

After the existing `attributesSaved` assertion, add:

```go
if permissionClient.calls != 0 {
	t.Fatalf("permission client calls = %d, want 0", permissionClient.calls)
}
```

Append this new test:

```go
func TestSystemResourceServiceReturnsPermissionRegistrationFailureAfterLocalCommit(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeSystemResourceRepository{
		latest: []resource.ResourceDefinition{
			{ID: "new-action", SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit", CreatedAt: now, UpdatedAt: now},
			{ID: "new-tag", SystemID: "todo", Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private", CreatedAt: now, UpdatedAt: now},
			{ID: "new-type", SystemID: "todo", Type: resource.ResourceDefinitionKindType, Label: "Repository", Key: "repo", CreatedAt: now, UpdatedAt: now},
		},
	}
	permissionClient := &fakePermissionClient{repo: repo, err: errors.New("permission unavailable")}
	var logBuffer bytes.Buffer
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20}, permissionClient,
		WithSystemResourceClock(func() time.Time { return now }),
		WithSystemResourceIDGenerator(sequenceIDs("new-action", "new-tag", "new-type", "attributes-1")),
		WithSystemResourceLogger(slog.New(slog.NewTextHandler(&logBuffer, nil))),
	)

	_, err := service.SaveSystemResources(context.Background(), resource.ResourceDefinitionSaveInput{
		SystemID: "todo",
		Resources: []resource.ResourceDefinitionInput{
			{Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit"},
			{Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private"},
			{Type: resource.ResourceDefinitionKindType, Label: "Repository", Key: "repo"},
		},
	})
	if !errors.Is(err, ErrPermissionRegistrationFailed) {
		t.Fatalf("error = %v, want ErrPermissionRegistrationFailed", err)
	}
	if !repo.transactionCommitted {
		t.Fatal("transactionCommitted = false, want true")
	}
	if len(repo.attributesSaved.Values) != 1 {
		t.Fatalf("saved attributes = %#v, want committed derived attribute", repo.attributesSaved.Values)
	}
	if permissionClient.calls != 1 {
		t.Fatalf("permission client calls = %d, want 1", permissionClient.calls)
	}
	output := logBuffer.String()
	if !strings.Contains(output, "failed to register resource attributes") {
		t.Fatalf("log output = %q, want permission failure message", output)
	}
	if !strings.Contains(output, "system_id=todo") {
		t.Fatalf("log output = %q, want system_id", output)
	}
	if !strings.Contains(output, "resource_attribute_count=1") {
		t.Fatalf("log output = %q, want resource_attribute_count", output)
	}
}
```

- [ ] **Step 4: Run service tests and confirm failure**

Run:

```bash
go test ./internal/function-service/services -run 'SystemResourceService' -v
```

Expected: FAIL because `NewSystemResourceService` does not accept the permission client argument, `ErrPermissionRegistrationFailed` is undefined, and `WithSystemResourceLogger` is undefined.

- [ ] **Step 5: Implement service dependency injection and registration**

In `internal/function-service/services/system_resource_service.go`, update imports to:

```go
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)
```

Add the sentinel after imports:

```go
var ErrPermissionRegistrationFailed = errors.New("permission registration failed")
```

Add this option:

```go
func WithSystemResourceLogger(logger *slog.Logger) SystemResourceOption {
	return func(s *SystemResourceService) {
		if logger != nil {
			s.logger = logger
		}
	}
}
```

Update `SystemResourceService`:

```go
type SystemResourceService struct {
	repository       SystemResourceRepository
	limits           resource.ResourceDefinitionLimits
	permissionClient clientpermission.Client
	clock            func() time.Time
	idGenerator      func() string
	logger           *slog.Logger
}
```

Replace the constructor with:

```go
func NewSystemResourceService(repository SystemResourceRepository, limits resource.ResourceDefinitionLimits, permissionClient clientpermission.Client, opts ...SystemResourceOption) *SystemResourceService {
	service := &SystemResourceService{
		repository:       repository,
		limits:           limits,
		permissionClient: permissionClient,
		clock:            func() time.Time { return time.Now().UTC() },
		idGenerator:      uuid.NewString,
		logger:           slog.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}
```

In `SaveSystemResources`, replace the existing `var saved []resource.ResourceDefinition` declaration before the transaction with:

```go
	var saved []resource.ResourceDefinition
	var derivedAttributes []resource.ResourceAttribute
```

Inside the transaction, immediately after `attributes := deriveResourceAttributes(latest)` and the empty check, copy the attributes before the repository upsert:

```go
		attributes := deriveResourceAttributes(latest)
		if len(attributes) == 0 {
			return nil
		}
		derivedAttributes = append([]resource.ResourceAttribute(nil), attributes...)
		if _, err := s.repository.UpsertResourceAttributes(tx, resource.ResourceAttributes{
			ID:        attributeID,
			SystemID:  normalized.SystemID,
			Values:    attributes,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("upsert system resource attributes: %w", err)
		}
```

After the transaction block succeeds and before `return saved, nil`, add:

```go
	if len(derivedAttributes) > 0 {
		if err := s.registerResourceAttributes(ctx, normalized.SystemID, derivedAttributes); err != nil {
			return nil, err
		}
	}
	return saved, nil
```

Add this helper method below `SaveSystemResources`:

```go
func (s *SystemResourceService) registerResourceAttributes(ctx context.Context, systemID string, attributes []resource.ResourceAttribute) error {
	if s.permissionClient == nil {
		err := errors.New("permission client is not configured")
		s.logger.ErrorContext(ctx, "failed to register resource attributes",
			"err", err,
			"system_id", systemID,
			"resource_attribute_count", len(attributes),
		)
		return fmt.Errorf("%w: %w", ErrPermissionRegistrationFailed, err)
	}
	if err := s.permissionClient.RegisterResourceAttributes(ctx, systemID, attributes); err != nil {
		s.logger.ErrorContext(ctx, "failed to register resource attributes",
			"err", err,
			"system_id", systemID,
			"resource_attribute_count", len(attributes),
		)
		return fmt.Errorf("%w: %w", ErrPermissionRegistrationFailed, err)
	}
	return nil
}
```

- [ ] **Step 6: Run service tests and confirm success**

Run:

```bash
go test ./internal/function-service/services -run 'SystemResourceService' -v
```

Expected: PASS.

- [ ] **Step 7: Commit service changes**

Run:

```bash
git add internal/function-service/services/system_resource_service.go internal/function-service/services/system_resource_service_test.go
git commit -m "feat: register system resource attributes"
```

Expected: commit succeeds and includes only service files.

---

### Task 3: Handler Mapping For Permission Registration Failure

**Files:**

- Modify: `internal/function-service/handlers/system_resource_handler.go`
- Modify: `internal/function-service/handlers/system_resource_handler_test.go`

- [ ] **Step 1: Write the failing handler test**

In `internal/function-service/handlers/system_resource_handler_test.go`, add this import:

```go
	"github.com/hao0731/workspace-permission-management/internal/function-service/services"
```

Append this test:

```go
func TestSystemResourceHandlerPermissionRegistrationFailure(t *testing.T) {
	service := &fakeHTTPSystemResourceService{saveErr: services.ErrPermissionRegistrationFailed}
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(service, newTestLogger()))
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/v1/systems/todo/resources",
		bytes.NewBufferString(`{"resources":[{"type":"action","label":"Can Edit","key":"can_edit"}]}`),
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error.Code != "permission_registration_failed" {
		t.Fatalf("code = %q, want permission_registration_failed", response.Error.Code)
	}
}
```

- [ ] **Step 2: Run handler tests and confirm failure**

Run:

```bash
go test ./internal/function-service/handlers -run 'SystemResourceHandler' -v
```

Expected: FAIL because the handler still maps the permission registration failure to `500`.

- [ ] **Step 3: Implement handler mapping**

In `internal/function-service/handlers/system_resource_handler.go`, add the services import:

```go
	"github.com/hao0731/workspace-permission-management/internal/function-service/services"
```

In `SaveSystemResources`, add this branch after the `resource.ErrInvalidInput` branch and before the generic failure branch:

```go
		if errors.Is(err, services.ErrPermissionRegistrationFailed) {
			return c.JSON(http.StatusBadGateway, exception.WrapResponse(exception.New("permission_registration_failed", "Failed to register resource attributes")))
		}
```

- [ ] **Step 4: Run handler tests and confirm success**

Run:

```bash
go test ./internal/function-service/handlers -run 'SystemResourceHandler' -v
```

Expected: PASS.

- [ ] **Step 5: Commit handler changes**

Run:

```bash
git add internal/function-service/handlers/system_resource_handler.go internal/function-service/handlers/system_resource_handler_test.go
git commit -m "feat: map permission registration failures"
```

Expected: commit succeeds and includes only handler files.

---

### Task 4: Function Service Wiring

**Files:**

- Modify: `cmd/function-service/main.go`

- [ ] **Step 1: Update command wiring**

In `cmd/function-service/main.go`, add this import:

```go
	permissioninmemory "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/inmemory"
```

Replace the `systemResourceService` construction with:

```go
	systemResourceService := services.NewSystemResourceService(systemResourceRepository, resource.ResourceDefinitionLimits{
		Types:   cfg.SystemResourceLimits.Type,
		Actions: cfg.SystemResourceLimits.Action,
		Tags:    cfg.SystemResourceLimits.Tag,
	}, permissioninmemory.New(permissioninmemory.WithLogger(logger)))
```

- [ ] **Step 2: Run focused compile tests**

Run:

```bash
go test ./cmd/function-service ./internal/function-service/... ./internal/shared/interactions/permission/...
```

Expected: PASS.

- [ ] **Step 3: Commit command wiring**

Run:

```bash
git add cmd/function-service/main.go
git commit -m "chore: wire in-memory permission client"
```

Expected: commit succeeds and includes only `cmd/function-service/main.go`.

---

### Task 5: Final Verification

**Files:**

- Verify the full repository.

- [ ] **Step 1: Run formatting**

Run:

```bash
gofmt -w internal/shared/interactions/permission/client.go internal/shared/interactions/permission/inmemory/client.go internal/shared/interactions/permission/inmemory/client_test.go internal/function-service/services/system_resource_service.go internal/function-service/services/system_resource_service_test.go internal/function-service/handlers/system_resource_handler.go internal/function-service/handlers/system_resource_handler_test.go cmd/function-service/main.go
```

Expected: command exits `0`.

- [ ] **Step 2: Run full tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Run repository diff check**

Run:

```bash
git diff --check
```

Expected: no output and exit code `0`.

- [ ] **Step 4: Inspect changed files**

Run:

```bash
git status --short
git diff --stat
```

Expected: only intended files are changed, or no files are changed if every task was committed.

- [ ] **Step 5: Commit formatting or test cleanup if needed**

If `gofmt` changed files after the earlier commits, run:

```bash
git add internal/shared/interactions/permission internal/function-service/services internal/function-service/handlers cmd/function-service/main.go
git commit -m "chore: format permission registration changes"
```

Expected: commit succeeds if there were formatting changes; skip this step when `git status --short` shows no formatting changes.

---

## Implementation Notes

- Do not call the permission client inside `repository.RunInTransaction`; external side effects must stay outside the MongoDB transaction.
- A permission registration failure returns an API error after local persistence has already committed.
- Use `slog.ErrorContext` in the service failure path with `err`, `system_id`, and `resource_attribute_count`.
- Use the fake permission client in service tests instead of the in-memory implementation.
- Keep the in-memory implementation service-agnostic; it must not import `internal/function-service`.
- Inject the in-memory client's logger with `WithLogger` instead of mutating or relying directly on global slog state in tests or command wiring.

## Verification Checklist

- [ ] `go test ./internal/shared/interactions/permission/... -v`
- [ ] `go test ./internal/function-service/services -run 'SystemResourceService' -v`
- [ ] `go test ./internal/function-service/handlers -run 'SystemResourceHandler' -v`
- [ ] `go test ./cmd/function-service ./internal/function-service/... ./internal/shared/interactions/permission/...`
- [ ] `go test ./...`
- [ ] `git diff --check`

## Policy Compliance Checklist

- [ ] Plan is stored under `docs/plans/active/`.
- [ ] Plan links to the source design documents.
- [ ] Shared permission packages do not depend on service-specific packages.
- [ ] Function service service code depends on the shared permission interface, not a concrete external transport.
- [ ] Handler code maps the service sentinel error and remains transport-focused.
- [ ] No new dependency, public API, or cross-layer import is introduced without rationale.
