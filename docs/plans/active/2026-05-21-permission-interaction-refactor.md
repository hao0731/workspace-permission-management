# Permission Interaction Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `internal/shared/interactions/permission` so consumer packages own the registration interface, the concrete permission API package moves from `permission/api` to `permission`, and the in-memory permission client and its design document are removed.

**Architecture:** `function-service` defines the narrow `PermissionClient` interface at the service boundary, matching the backend policy's consumer-side interface rule. The shared `internal/shared/interactions/permission` package becomes the concrete HTTP permission API client plus request/error DTOs, while helper subpackages such as `caveat`, `relationship`, `object`, `relation`, and `subject` move directly under that root. Runtime wiring injects the concrete root `permission.Client` from `cmd/function-service`; no shared client interface or in-memory fallback remains.

**Tech Stack:** Go 1.25, Echo v5, `net/http`, `log/slog`, MongoDB-backed services, repository-local Markdown design docs.

---

## Classification And Policy Summary

This is Backend + Design/Plan Docs work.

Follow:

- `docs/policies/backend-architecture-principle.md`
- `docs/policies/design-and-plan-docs-policy.md`

Backend policy points used by this plan:

- Define interfaces at the consumer side unless there is a clear shared contract.
- `internal/shared/<name>` packages must not depend on service-private packages.
- `cmd/<server>` remains the composition root for concrete dependency wiring.
- Backend verification must include `go test ./...` when practical.

Design/plan policy points used by this plan:

- Design documents changed by implementation work must be updated under `docs/designs/`.
- Implementation plans must live under `docs/plans/active/` until implementation completes.
- Related design documents should be cross-linked when helpful.

## File Structure

Modify documentation:

- Modify: `docs/designs/permission-client-design.md`
- Modify: `docs/designs/permission-api-client-design.md`
- Delete: `docs/designs/inmemory-permission-client-design.md`
- Modify: `docs/designs/function-service-system-resource-api-design.md`
- Modify: `docs/designs/system-group-api-design.md`

Move concrete permission API files:

- Move: `internal/shared/interactions/permission/api/client.go` -> `internal/shared/interactions/permission/client.go`
- Move: `internal/shared/interactions/permission/api/request.go` -> `internal/shared/interactions/permission/request.go`
- Move: `internal/shared/interactions/permission/api/errors.go` -> `internal/shared/interactions/permission/errors.go`
- Move: `internal/shared/interactions/permission/api/client_test.go` -> `internal/shared/interactions/permission/client_test.go`
- Move: `internal/shared/interactions/permission/api/caveat` -> `internal/shared/interactions/permission/caveat`
- Move: `internal/shared/interactions/permission/api/object` -> `internal/shared/interactions/permission/object`
- Move: `internal/shared/interactions/permission/api/relation` -> `internal/shared/interactions/permission/relation`
- Move: `internal/shared/interactions/permission/api/relationship` -> `internal/shared/interactions/permission/relationship`
- Move: `internal/shared/interactions/permission/api/subject` -> `internal/shared/interactions/permission/subject`

Delete obsolete shared-interface and in-memory implementation files:

- Delete: `internal/shared/interactions/permission/client.go` as the old shared interface, after replacing it with the moved concrete API client file.
- Delete: `internal/shared/interactions/permission/inmemory/client.go`
- Delete: `internal/shared/interactions/permission/inmemory/client_test.go`

Modify consumers:

- Modify: `internal/function-service/services/system_resource_service.go`
- Modify: `internal/function-service/services/system_resource_service_test.go`
- Modify: `cmd/function-service/main.go`
- Modify: `cmd/function-service/main_test.go`
- Modify: `internal/mock-permission-api/handlers/schema_handler.go`
- Modify: `internal/group-service/services/system_group_relationship_builder.go`
- Modify moved files under `internal/shared/interactions/permission/relationship` and `internal/shared/interactions/permission/subject` to import the new helper package paths.

## Task 1: Update Design Documents

**Files:**

- Modify: `docs/designs/permission-client-design.md`
- Modify: `docs/designs/permission-api-client-design.md`
- Delete: `docs/designs/inmemory-permission-client-design.md`
- Modify: `docs/designs/function-service-system-resource-api-design.md`
- Modify: `docs/designs/system-group-api-design.md`

- [ ] **Step 1: Rewrite the shared client design around consumer-side interfaces**

Replace `docs/designs/permission-client-design.md` with a design that states:

```markdown
# Permission Registration Consumer Interface Design

## Background

`function-service` registers derived system resource attributes with the permission API after local system resource persistence commits. The previous design placed a shared `Client` interface in `internal/shared/interactions/permission/client.go`. The refactor removes that shared interface and applies the backend architecture policy rule that interfaces should be defined at the consumer side unless there is a clear shared contract.

The HTTP permission API implementation is documented in [Permission API Client Design](permission-api-client-design.md). The caller integration from the system resource API is documented in [Function Service System Resource API Design](function-service-system-resource-api-design.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- `internal/function-service/services` owns the `PermissionClient` interface because `SystemResourceService` is the consumer of permission registration.
- `internal/shared/interactions/permission` owns the concrete HTTP permission API client and DTOs, not a shared interface.
- The shared permission package must not depend on `internal/function-service`, `internal/group-service`, or `internal/mock-permission-api`.
- `cmd/function-service/main.go` remains the composition root that constructs the concrete permission API client and injects it into `SystemResourceService`.
- This design document is stored under `docs/designs/` and cross-linked from related system resource and permission API designs.

## Goals

- Remove `internal/shared/interactions/permission/client.go` as a shared interface contract.
- Define the narrow permission registration interface in `internal/function-service/services`.
- Keep the method shape `RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error`.
- Keep `context.Context` propagation and the existing post-commit registration behavior.
- Let `cmd/function-service/main.go` inject the concrete HTTP permission client from `internal/shared/interactions/permission`.
- Remove the in-memory permission client from source and design documentation.

## Non-Goals

- Do not change the permission API schema-write payload in this refactor.
- Do not make permission registration asynchronous.
- Do not add retry, outbox, circuit breaker, or background delivery behavior.
- Do not move resource attribute derivation out of `function-service`.
- Do not introduce frontend changes.

## Consumer Interface Contract

Package:

```plaintext
internal/function-service/services
```

Interface:

```go
type PermissionClient interface {
	RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error
}
```

Input contract:

- `systemID` is the normalized system identifier used by `/api/v1/systems/:system_id`.
- `resourceAttributes` is the complete derived attribute set for the latest persisted resource definitions of that system.
- `SystemResourceService` should not invoke the client when the derived attribute set is empty.
- The concrete client must not mutate the supplied slice.

Error contract:

- Return `nil` when registration is accepted or completed by the concrete implementation.
- Return an error when registration cannot be completed.
- `SystemResourceService` maps any returned error to `ErrPermissionRegistrationFailed`.

## Function Service Usage

`function-service` uses this client after local persistence succeeds:

1. Save resource definitions and derived attributes in MongoDB inside the existing repository transaction.
2. Commit the MongoDB transaction.
3. If the derived attribute set is non-empty, call `RegisterResourceAttributes(ctx, systemID, attributes)`.
4. If registration fails, log the failure with `slog.ErrorContext` and return an upstream dependency error to the HTTP caller.

This is not a cross-system atomic transaction. A permission registration failure does not roll back already committed local MongoDB changes.

Runtime wiring should use the API client from `internal/shared/interactions/permission`:

```go
permissionClient := permission.New(
	cfg.PermissionAPI.BaseURL,
	cfg.PermissionAPI.APIKey,
	cfg.PermissionAPI.APIKeyHeader,
)
```

`cmd/function-service/main.go` is responsible for constructing the concrete client from configuration and injecting it into `SystemResourceService`.

## Testing Strategy

Function service tests:

- `SystemResourceService` accepts a consumer-side fake that implements `services.PermissionClient`.
- Save calls `RegisterResourceAttributes` after a successful local transaction when derived attributes exist.
- Save does not call the client when derived attributes are empty.
- Client failure is logged and returned as an upstream dependency failure.
- Client failure does not undo the local repository calls already completed before registration.
- Function-service config validates the required permission API settings.
- Function-service composition wiring uses the concrete API client from `internal/shared/interactions/permission`.

Permission API client tests are owned by [Permission API Client Design](permission-api-client-design.md).

Verification commands for implementation:

```bash
go test ./internal/function-service/services ./cmd/function-service ./internal/shared/interactions/permission/...
go test ./...
```

## Architecture Decisions

1. Move the interface to `internal/function-service/services`.
   - Rationale: `SystemResourceService` is the consumer and needs only one method, so a shared interface creates an unnecessary cross-package contract.
   - Trade-off: Additional consumers must define their own narrow interface if they need permission registration.

2. Keep the concrete API client in `internal/shared/interactions/permission`.
   - Rationale: The HTTP client and DTOs are reusable internal integration code, but they are implementation details rather than the service-facing abstraction.
   - Trade-off: The package name `permission` now refers to the concrete API client package, so imports near `internal/domain/permission` should use aliases when both are present.

3. Remove the in-memory client.
   - Rationale: Runtime wiring now uses the HTTP API client, and service tests already use local fakes for success and failure paths.
   - Trade-off: Local development that wants a fake upstream must run `mock-permission-api` instead of swapping in an in-memory implementation.

4. Keep registration synchronous for this refactor.
   - Rationale: The refactor changes package boundaries only and preserves current HTTP response semantics.
   - Trade-off: A permission API outage can still make the function-service POST return `502` after local MongoDB writes have committed.
```

- [ ] **Step 2: Update the permission API client design paths and constructor contract**

In `docs/designs/permission-api-client-design.md`, make these exact semantic changes:

- Remove every reference to `[In-Memory Permission Client Design](inmemory-permission-client-design.md)`.
- Replace `internal/shared/interactions/permission/api` with `internal/shared/interactions/permission`.
- Replace `permission/api` with `permission` where describing the concrete HTTP API package.
- Replace `permission.Client` as the shared interface with `services.PermissionClient` when describing the consumer-side contract.
- Replace constructor text:

```go
func New(baseURL string, apiKey string, apiKeyHeaderKey string, opts ...Option) permission.Client
```

with:

```go
func New(baseURL string, apiKey string, apiKeyHeaderKey string, opts ...Option) *Client
```

- Replace the package structure block with:

```txt
internal/shared/interactions/permission/
  client.go
  request.go
  errors.go
  caveat/
  object/
  relation/
  relationship/
  subject/

cmd/mock-permission-api/
  main.go

internal/mock-permission-api/
  config/
  handlers/
```

- Update testing bullets so they assert `*permission.Client` satisfies the `services.PermissionClient` method shape through function-service wiring, not through a shared package interface.
- Remove the architecture decision titled "Keep the shared `permission.Client` interface unchanged" and replace it with:

```markdown
1. Return the concrete `*Client` from the permission API package constructor.
   - Rationale: The service-facing interface is now owned by `internal/function-service/services`, and the shared package should expose its concrete implementation directly.
   - Trade-off: Consumers that want an abstraction must define their own narrow interface.
```

- [ ] **Step 3: Remove the in-memory design document**

Run:

```bash
git rm docs/designs/inmemory-permission-client-design.md
```

Expected: `docs/designs/inmemory-permission-client-design.md` is staged for deletion.

- [ ] **Step 4: Update the system resource design**

In `docs/designs/function-service-system-resource-api-design.md`, make these exact semantic changes:

- Remove the related design bullet for `In-Memory Permission Client Design`.
- Replace `internal/shared/interactions/permission.Client` with `internal/function-service/services.PermissionClient`.
- Replace "shared permission client" with "consumer-side permission client interface" when referring to the service dependency.
- Replace `internal/shared/interactions/permission/api` with `internal/shared/interactions/permission`.
- Remove all references to `internal/shared/interactions/permission/inmemory`.
- In the package ownership section, use:

```markdown
- The consumer-side interface belongs in `internal/function-service/services`.
- The runtime HTTP implementation belongs in `internal/shared/interactions/permission`.
- `cmd/function-service/main.go` wires the concrete API client into `SystemResourceService` from function-service permission API configuration.
- `SystemResourceService` receives the client through dependency injection and should not construct the concrete client itself.
```

- In the service structure block, replace:

```txt
internal/shared/interactions/permission/
  client.go

internal/shared/interactions/permission/api/
  client.go
  request.go
  errors.go

internal/shared/interactions/permission/inmemory/
  client.go
```

with:

```txt
internal/shared/interactions/permission/
  client.go
  request.go
  errors.go
  caveat/
  object/
  relation/
  relationship/
  subject/
```

- In testing strategy, remove in-memory client bullets and keep API client behavior bullets under the root `internal/shared/interactions/permission` package.
- In implementation plan notes, remove the link to `inmemory-permission-client-design.md`.

- [ ] **Step 5: Update the system group design**

In `docs/designs/system-group-api-design.md`, make these exact changes:

- Replace `internal/shared/interactions/permission/api/relationship` with `internal/shared/interactions/permission/relationship`.
- Replace `internal/shared/interactions/permission/api` with `internal/shared/interactions/permission` when referring to the shared permission DTO/helper package.
- Update the related design bullet from "Permission API Client Design: shared permission API DTO package" to "Permission API Client Design: shared permission package that defines relationship helper constructors."

- [ ] **Step 6: Verify design document references**

Run:

```bash
rg -n "inmemory-permission-client-design|permission/inmemory|internal/shared/interactions/permission/api|permission\\.Client|shared permission client" docs/designs
```

Expected: no output for `inmemory-permission-client-design`, `permission/inmemory`, or `internal/shared/interactions/permission/api`. Any remaining `permission.Client` references must refer to the concrete root package type and not a shared interface.

- [ ] **Step 7: Commit design document changes**

Run:

```bash
git add docs/designs
git commit -m "docs: update permission interaction refactor design"
```

Expected: commit succeeds with the design document updates and the deleted in-memory design file.

## Task 2: Move The Function-Service Interface To The Consumer Side

**Files:**

- Modify: `internal/function-service/services/system_resource_service_test.go`
- Modify: `internal/function-service/services/system_resource_service.go`

- [ ] **Step 1: Write the failing consumer-interface assertion**

In `internal/function-service/services/system_resource_service_test.go`, add this compile-time assertion immediately after the `fakePermissionClient` type and its methods are defined:

```go
var _ PermissionClient = (*fakePermissionClient)(nil)
```

The nearby fake should remain:

```go
type fakePermissionClient struct {
	repo       *fakeSystemResourceRepository
	calls      int
	systemID   string
	attributes []resource.ResourceAttribute
	err        error
}
```

- [ ] **Step 2: Run the service test to verify it fails**

Run:

```bash
go test ./internal/function-service/services -run TestSystemResourceService -count=1
```

Expected: FAIL with `undefined: PermissionClient`.

- [ ] **Step 3: Add the consumer-side interface and remove the shared-interface import**

In `internal/function-service/services/system_resource_service.go`, remove this import:

```go
clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
```

Add this interface below `SystemResourceRepository`:

```go
type PermissionClient interface {
	RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error
}
```

Change the service field and constructor signature from:

```go
permissionClient clientpermission.Client
```

and:

```go
func NewSystemResourceService(repository SystemResourceRepository, limits resource.ResourceDefinitionLimits, permissionClient clientpermission.Client, opts ...SystemResourceOption) *SystemResourceService {
```

to:

```go
permissionClient PermissionClient
```

and:

```go
func NewSystemResourceService(repository SystemResourceRepository, limits resource.ResourceDefinitionLimits, permissionClient PermissionClient, opts ...SystemResourceOption) *SystemResourceService {
```

- [ ] **Step 4: Run the service test to verify it passes**

Run:

```bash
go test ./internal/function-service/services -run TestSystemResourceService -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the consumer-side interface change**

Run:

```bash
git add internal/function-service/services/system_resource_service.go internal/function-service/services/system_resource_service_test.go
git commit -m "refactor: move permission client interface to function service"
```

Expected: commit succeeds.

## Task 3: Move The HTTP Permission API Client To The Root Permission Package

**Files:**

- Move: `internal/shared/interactions/permission/api/client_test.go` -> `internal/shared/interactions/permission/client_test.go`
- Move: `internal/shared/interactions/permission/api/client.go` -> `internal/shared/interactions/permission/client.go`
- Move: `internal/shared/interactions/permission/api/request.go` -> `internal/shared/interactions/permission/request.go`
- Move: `internal/shared/interactions/permission/api/errors.go` -> `internal/shared/interactions/permission/errors.go`

- [ ] **Step 1: Move and update the client test first**

Run:

```bash
mv internal/shared/interactions/permission/api/client_test.go internal/shared/interactions/permission/client_test.go
```

Edit `internal/shared/interactions/permission/client_test.go` so the top of the file is:

```go
package permission

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type resourceAttributeRegistrar interface {
	RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error
}

var _ resourceAttributeRegistrar = (*Client)(nil)
```

Keep the existing test functions unchanged below that block.

- [ ] **Step 2: Run the root permission package test to verify it fails**

Run:

```bash
go test ./internal/shared/interactions/permission -run TestClient -count=1
```

Expected: FAIL because the root package still has the old shared-interface `client.go` and does not define `New`, `Error`, `ErrorResponse`, or the request DTOs used by the moved test.

- [ ] **Step 3: Move concrete client files to the root package**

Run:

```bash
mv internal/shared/interactions/permission/api/client.go internal/shared/interactions/permission/client.go
mv internal/shared/interactions/permission/api/request.go internal/shared/interactions/permission/request.go
mv internal/shared/interactions/permission/api/errors.go internal/shared/interactions/permission/errors.go
```

This replaces the old `internal/shared/interactions/permission/client.go` shared-interface file with the concrete client implementation.

- [ ] **Step 4: Update `client.go` to the root package and concrete constructor**

Edit `internal/shared/interactions/permission/client.go` to match:

```go
package permission

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

const schemaWritePath = "/api/v1/schema/write"

type Client struct {
	baseURL         string
	apiKey          string
	apiKeyHeaderKey string
	httpClient      *http.Client
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func New(baseURL string, apiKey string, apiKeyHeaderKey string, opts ...Option) *Client {
	client := &Client{
		baseURL:         strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:          strings.TrimSpace(apiKey),
		apiKeyHeaderKey: strings.TrimSpace(apiKeyHeaderKey),
		httpClient:      http.DefaultClient,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client
}

func (c *Client) RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error {
	payload := newRegisterResourceAttributesRequest(systemID, resourceAttributes)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal permission API schema write request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+schemaWritePath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create permission API schema write request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(c.apiKeyHeaderKey, c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send permission API request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	var errorResponse ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
		return fmt.Errorf("permission API request failed with status %d: decode permission API error response: %w", resp.StatusCode, err)
	}
	return &Error{StatusCode: resp.StatusCode, Response: errorResponse}
}
```

- [ ] **Step 5: Update `request.go` package name**

Edit `internal/shared/interactions/permission/request.go` so the first line is:

```go
package permission
```

The rest of the file remains:

```go
import "github.com/hao0731/workspace-permission-management/internal/domain/resource"

const dynamicContextCondition = "enable_dynamic_context"

type RegisterResourceAttributesRelationRequest struct {
	ResourceAttribute resource.ResourceAttribute `json:"resAttr"`
	Condition         string                     `json:"condition"`
	IsPublic          bool                       `json:"isPublic"`
}

type RegisterResourceAttributesRequest struct {
	Definition string                                      `json:"definition"`
	Relations  []RegisterResourceAttributesRelationRequest `json:"relations"`
}

func newRegisterResourceAttributesRequest(systemID string, resourceAttributes []resource.ResourceAttribute) RegisterResourceAttributesRequest {
	relations := make([]RegisterResourceAttributesRelationRequest, 0, len(resourceAttributes))
	for _, attribute := range resourceAttributes {
		relations = append(relations, RegisterResourceAttributesRelationRequest{
			ResourceAttribute: attribute,
			Condition:         dynamicContextCondition,
			IsPublic:          false,
		})
	}
	return RegisterResourceAttributesRequest{
		Definition: systemID,
		Relations:  relations,
	}
}
```

- [ ] **Step 6: Update `errors.go` package name**

Edit `internal/shared/interactions/permission/errors.go` so the first line is:

```go
package permission
```

The rest of the file remains:

```go
import "fmt"

type ErrorResponse struct {
	Code    int    `json:"code"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type Error struct {
	StatusCode int
	Response   ErrorResponse
}

func (e *Error) Error() string {
	if e.Response.Message != "" {
		return fmt.Sprintf("permission API request failed with status %d: %s", e.StatusCode, e.Response.Message)
	}
	return fmt.Sprintf("permission API request failed with status %d", e.StatusCode)
}
```

- [ ] **Step 7: Format and run root permission client tests**

Run:

```bash
gofmt -w internal/shared/interactions/permission/client.go internal/shared/interactions/permission/request.go internal/shared/interactions/permission/errors.go internal/shared/interactions/permission/client_test.go
go test ./internal/shared/interactions/permission -run TestClient -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit the root permission client move**

Run:

```bash
git add internal/shared/interactions/permission/client.go internal/shared/interactions/permission/request.go internal/shared/interactions/permission/errors.go internal/shared/interactions/permission/client_test.go internal/shared/interactions/permission/api
git commit -m "refactor: move permission API client to root package"
```

Expected: commit succeeds.

## Task 4: Move Permission Helper Subpackages Out Of `api`

**Files:**

- Move: `internal/shared/interactions/permission/api/caveat` -> `internal/shared/interactions/permission/caveat`
- Move: `internal/shared/interactions/permission/api/object` -> `internal/shared/interactions/permission/object`
- Move: `internal/shared/interactions/permission/api/relation` -> `internal/shared/interactions/permission/relation`
- Move: `internal/shared/interactions/permission/api/relationship` -> `internal/shared/interactions/permission/relationship`
- Move: `internal/shared/interactions/permission/api/subject` -> `internal/shared/interactions/permission/subject`
- Modify: `internal/group-service/services/system_group_relationship_builder.go`
- Modify moved files under `internal/shared/interactions/permission/relationship`
- Modify moved files under `internal/shared/interactions/permission/subject`

- [ ] **Step 1: Update the group-service import expectation before moving helpers**

Edit the import block in `internal/group-service/services/system_group_relationship_builder.go` from:

```go
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/caveat"
	permissionrelationship "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/relationship"
```

to:

```go
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/caveat"
	permissionrelationship "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/relationship"
```

- [ ] **Step 2: Run relationship builder tests to verify they fail**

Run:

```bash
go test ./internal/group-service/services -run 'TestBuildSystemGroupRelationshipProjection|TestRelationshipChecksum' -count=1
```

Expected: FAIL because `internal/shared/interactions/permission/caveat` and `internal/shared/interactions/permission/relationship` do not exist yet.

- [ ] **Step 3: Move helper directories**

Run:

```bash
mv internal/shared/interactions/permission/api/caveat internal/shared/interactions/permission/caveat
mv internal/shared/interactions/permission/api/object internal/shared/interactions/permission/object
mv internal/shared/interactions/permission/api/relation internal/shared/interactions/permission/relation
mv internal/shared/interactions/permission/api/relationship internal/shared/interactions/permission/relationship
mv internal/shared/interactions/permission/api/subject internal/shared/interactions/permission/subject
```

Expected: the five helper directories exist directly under `internal/shared/interactions/permission`.

- [ ] **Step 4: Update moved relationship imports**

In `internal/shared/interactions/permission/relationship/model.go`, replace the import block with:

```go
import (
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/caveat"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/object"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/relation"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/subject"
)
```

In `internal/shared/interactions/permission/relationship/helper.go`, replace the import block with:

```go
import (
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/caveat"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/object"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/relation"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/subject"
)
```

- [ ] **Step 5: Update moved subject imports**

In `internal/shared/interactions/permission/subject/model.go`, replace the import block with:

```go
import (
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/object"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/relation"
)
```

- [ ] **Step 6: Format and run relationship builder tests**

Run:

```bash
gofmt -w internal/group-service/services/system_group_relationship_builder.go internal/shared/interactions/permission/relationship/model.go internal/shared/interactions/permission/relationship/helper.go internal/shared/interactions/permission/subject/model.go
go test ./internal/group-service/services -run 'TestBuildSystemGroupRelationshipProjection|TestRelationshipChecksum' -count=1
```

Expected: PASS.

- [ ] **Step 7: Run shared permission package tests including helper packages**

Run:

```bash
go test ./internal/shared/interactions/permission/... -count=1
```

Expected: PASS for the root permission package and helper subpackages.

- [ ] **Step 8: Commit helper package moves**

Run:

```bash
git add internal/shared/interactions/permission internal/group-service/services/system_group_relationship_builder.go
git commit -m "refactor: move permission helper packages to root"
```

Expected: commit succeeds.

## Task 5: Update Composition Root And Mock Permission API Imports

**Files:**

- Modify: `cmd/function-service/main.go`
- Modify: `cmd/function-service/main_test.go`
- Modify: `internal/mock-permission-api/handlers/schema_handler.go`

- [ ] **Step 1: Update function-service test expectation**

In `cmd/function-service/main_test.go`, replace:

```go
permissionapi "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api"
```

with:

```go
permission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
```

Replace:

```go
if _, ok := client.(*permissionapi.Client); !ok {
	t.Fatalf("permission client type = %T, want *api.Client", client)
}
```

with:

```go
if _, ok := client.(*permission.Client); !ok {
	t.Fatalf("permission client type = %T, want *permission.Client", client)
}
```

- [ ] **Step 2: Run the function-service composition test to verify it fails**

Run:

```bash
go test ./cmd/function-service -run TestNewPermissionClientReturnsAPIClient -count=1
```

Expected: FAIL because `cmd/function-service/main.go` still imports the old `permission/api` package or returns the old shared interface type.

- [ ] **Step 3: Update function-service composition root imports and return type**

In `cmd/function-service/main.go`, remove:

```go
clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
permissionapi "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api"
```

Add:

```go
permission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
```

Replace:

```go
func newPermissionClient(cfg config.PermissionAPIConfig) clientpermission.Client {
	return permissionapi.New(cfg.BaseURL, cfg.APIKey, cfg.APIKeyHeader)
}
```

with:

```go
func newPermissionClient(cfg config.PermissionAPIConfig) *permission.Client {
	return permission.New(cfg.BaseURL, cfg.APIKey, cfg.APIKeyHeader)
}
```

- [ ] **Step 4: Update mock permission API handler imports**

In `internal/mock-permission-api/handlers/schema_handler.go`, replace:

```go
permissionapi "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api"
```

with:

```go
permission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
```

Replace:

```go
var request permissionapi.RegisterResourceAttributesRequest
```

with:

```go
var request permission.RegisterResourceAttributesRequest
```

Replace:

```go
return c.JSON(http.StatusBadRequest, permissionapi.ErrorResponse{
```

with:

```go
return c.JSON(http.StatusBadRequest, permission.ErrorResponse{
```

- [ ] **Step 5: Format and run focused composition and mock tests**

Run:

```bash
gofmt -w cmd/function-service/main.go cmd/function-service/main_test.go internal/mock-permission-api/handlers/schema_handler.go
go test ./cmd/function-service -run TestNewPermissionClientReturnsAPIClient -count=1
go test ./internal/mock-permission-api/handlers -run TestWriteSchema -count=1
```

Expected: PASS for both commands.

- [ ] **Step 6: Commit composition and mock import updates**

Run:

```bash
git add cmd/function-service/main.go cmd/function-service/main_test.go internal/mock-permission-api/handlers/schema_handler.go
git commit -m "refactor: wire root permission API client"
```

Expected: commit succeeds.

## Task 6: Remove The In-Memory Permission Package And Old API Directory

**Files:**

- Delete: `internal/shared/interactions/permission/inmemory/client.go`
- Delete: `internal/shared/interactions/permission/inmemory/client_test.go`
- Remove empty directory: `internal/shared/interactions/permission/api`

- [ ] **Step 1: Verify no source imports still point at in-memory or old API packages**

Run:

```bash
rg -n "internal/shared/interactions/permission/api|internal/shared/interactions/permission/inmemory|permissionapi|permissioninmemory|clientpermission" internal cmd examples
```

Expected: no output. If output appears, update the listed file to use the root `internal/shared/interactions/permission` package or the consumer-side `services.PermissionClient` interface before continuing.

- [ ] **Step 2: Delete obsolete package files**

Run:

```bash
git rm internal/shared/interactions/permission/inmemory/client.go internal/shared/interactions/permission/inmemory/client_test.go
rmdir internal/shared/interactions/permission/inmemory
rmdir internal/shared/interactions/permission/api
```

Expected: the in-memory client files are staged for deletion, and both obsolete directories are gone. If `rmdir internal/shared/interactions/permission/api` fails because files remain, run `find internal/shared/interactions/permission/api -maxdepth 2 -type f -print` and move any remaining tracked helper file to the matching root helper directory before retrying `rmdir`.

- [ ] **Step 3: Run shared permission tests**

Run:

```bash
go test ./internal/shared/interactions/permission/... -count=1
```

Expected: PASS and no `permission/inmemory` package appears in the package list.

- [ ] **Step 4: Commit package removal**

Run:

```bash
git add internal/shared/interactions/permission
git commit -m "refactor: remove in-memory permission client"
```

Expected: commit succeeds.

## Task 7: Repository-Wide Verification And Stale Reference Cleanup

**Files:**

- Verify all changed files.
- Modify any stale import or design reference reported by commands in this task.

- [ ] **Step 1: Run stale source import checks**

Run:

```bash
rg -n "internal/shared/interactions/permission/api|internal/shared/interactions/permission/inmemory|permissionapi|permissioninmemory|clientpermission" internal cmd examples
```

Expected: no output.

- [ ] **Step 2: Run stale design reference checks**

Run:

```bash
rg -n "inmemory-permission-client-design|permission/inmemory|internal/shared/interactions/permission/api|permission\\.Client|shared permission client" docs/designs
```

Expected: no output for `inmemory-permission-client-design`, `permission/inmemory`, or `internal/shared/interactions/permission/api`. Any remaining `permission.Client` reference must refer to the concrete root `permission.Client` type, not the removed shared interface.

- [ ] **Step 3: Run focused package tests**

Run:

```bash
go test ./internal/shared/interactions/permission/... ./internal/function-service/services ./cmd/function-service ./internal/mock-permission-api/handlers ./internal/group-service/services -count=1
```

Expected: PASS.

- [ ] **Step 4: Run full backend verification**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Inspect git status**

Run:

```bash
git status --short
```

Expected: only files intentionally changed by this refactor appear.

- [ ] **Step 6: Commit final cleanup if any files changed after focused commits**

Run:

```bash
git add docs/designs docs/plans/active/2026-05-21-permission-interaction-refactor.md internal cmd examples
git commit -m "chore: clean permission refactor references"
```

Expected: commit succeeds if there were cleanup changes. If there were no cleanup changes, `git status --short` remains clean except for this plan if it was intentionally left uncommitted by the implementation workflow.

## Final Acceptance Criteria

- `internal/shared/interactions/permission/client.go` contains the concrete HTTP permission API client, not a shared interface.
- `internal/shared/interactions/permission/request.go` and `internal/shared/interactions/permission/errors.go` contain the permission API DTOs.
- `internal/shared/interactions/permission/api` does not exist.
- `internal/shared/interactions/permission/inmemory` does not exist.
- `internal/function-service/services` defines `PermissionClient` locally.
- `cmd/function-service/main.go` constructs `permission.New(...)` from the root shared permission package.
- `internal/mock-permission-api/handlers` decodes `permission.RegisterResourceAttributesRequest` from the root shared package.
- `internal/group-service/services` imports permission relationship helpers from `internal/shared/interactions/permission/relationship`.
- `docs/designs/inmemory-permission-client-design.md` is deleted.
- Current design docs no longer describe the removed shared `permission.Client` interface or in-memory client as active architecture.
- `go test ./...` passes.
