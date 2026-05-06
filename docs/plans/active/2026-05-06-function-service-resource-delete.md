# Function Service Resource Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement resource deletion in `function-service` with `DELETE` API, scoped MongoDB deletion, post-delete JetStream event publishing, and `204` response on full success.

**Architecture:** Keep transport thin in Echo handler, orchestrate delete-then-publish in service layer, isolate persistence in repository, and define delete event payload in transport DTO. Follow existing `function-service` layering and error mapping so domain not-found maps to `404`, validation to `400`, and infrastructure failures to `500`.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, NATS JetStream via `internal/shared/eventbus`, CloudEvents SDK for Go, `testing`.

---

## Source Design

- [../../designs/function-service-resource-delete.md](../../designs/function-service-resource-delete.md)
- [../../designs/function-service.md](../../designs/function-service.md)

Applicable policies:

- [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

## Scope

Implement only:

- `DELETE /api/v1/workspaces/:workspace_id/functions/:function_key/resources/:resource_id`
- Scoped deletion in `function_resources`
- Required config `FUNCTION_SERVICE_JETSTREAM_RESOURCE_DELETED_SUBJECT`
- Publish delete event **after** delete succeeds
- Return `204` when delete + publish both succeed
- Return `404` when target resource does not exist

Out of scope:

- Soft delete / archive
- Retry queue or compensating transaction
- Bulk deletion API
- Frontend changes

## File Structure

Modify:

- `.env.example`
- `cmd/function-service/main.go`
- `internal/function-service/config/config.go`
- `internal/function-service/config/config_test.go`
- `internal/function-service/handlers/resource_handler.go`
- `internal/function-service/handlers/resource_handler_test.go`
- `internal/function-service/services/resource_service.go`
- `internal/function-service/services/resource_service_test.go`
- `internal/function-service/repositories/mongo_resource_repository.go`
- `internal/function-service/repositories/mongo_resource_repository_test.go`

Create:

- `internal/function-service/transport/resource_deleted_event.go`
- `internal/function-service/transport/resource_deleted_event_test.go`
- `examples/api/function_resource_delete.http`

---

### Task 1: Add required delete-event subject configuration

**Files:**
- Modify: `.env.example`
- Modify: `internal/function-service/config/config.go`
- Modify: `internal/function-service/config/config_test.go`

- [ ] **Step 1: Add failing config test for missing delete subject**
  - Extend `TestLoadRejectsMissingRequiredValue` coverage to assert `Load()` returns error when `FUNCTION_SERVICE_JETSTREAM_RESOURCE_DELETED_SUBJECT` is absent.

- [ ] **Step 2: Add failing config test for successful load with delete subject**
  - Extend `TestLoadReadsRequiredEnvironment` to include:
    - `t.Setenv("FUNCTION_SERVICE_JETSTREAM_RESOURCE_DELETED_SUBJECT", "app.todo.resource.deleted")`
    - assertion that config contains this value.

- [ ] **Step 3: Run config tests and verify they fail**
  - Run: `go test ./internal/function-service/config -run TestLoad -v`
  - Expected: FAIL because config field/validation not implemented yet.

- [ ] **Step 4: Implement config field and required validation**
  - Add config field under JetStream section (for example `ResourceDeletedSubject string`).
  - Bind env var `FUNCTION_SERVICE_JETSTREAM_RESOURCE_DELETED_SUBJECT`.
  - Make empty/missing value return validation error from `Load()`.

- [ ] **Step 5: Update `.env.example`**
  - Add:
    - `FUNCTION_SERVICE_JETSTREAM_RESOURCE_DELETED_SUBJECT=app.todo.resource.deleted`

- [ ] **Step 6: Re-run config tests and verify pass**
  - Run: `go test ./internal/function-service/config -run TestLoad -v`
  - Expected: PASS.

- [ ] **Step 7: Commit Task 1**
  - `git add .env.example internal/function-service/config/config.go internal/function-service/config/config_test.go`
  - `git commit -m "feat(function-service): require resource deleted event subject config"`

---

### Task 2: Add delete event transport DTO and tests

**Files:**
- Create: `internal/function-service/transport/resource_deleted_event.go`
- Create: `internal/function-service/transport/resource_deleted_event_test.go`

- [ ] **Step 1: Write failing tests for delete event payload mapping**
  - Test fields include `resource_id`, `function_key`, `workspace_id` and reject empty required values.

- [ ] **Step 2: Run transport tests and verify fail**
  - Run: `go test ./internal/function-service/transport -run ResourceDeleted -v`

- [ ] **Step 3: Implement DTO + validation helper**
  - Add typed struct and constructor/validator used by service before publishing CloudEvent.

- [ ] **Step 4: Re-run transport tests and verify pass**
  - Run: `go test ./internal/function-service/transport -run ResourceDeleted -v`

- [ ] **Step 5: Commit Task 2**
  - `git add internal/function-service/transport/resource_deleted_event.go internal/function-service/transport/resource_deleted_event_test.go`
  - `git commit -m "feat(function-service): add resource deleted event transport model"`

---

### Task 3: Add repository scoped delete with not-found semantics

**Files:**
- Modify: `internal/function-service/repositories/mongo_resource_repository.go`
- Modify: `internal/function-service/repositories/mongo_resource_repository_test.go`

- [ ] **Step 1: Write failing repository tests**
  - Add test: delete existing `(workspace_id, function_key, resource_id)` succeeds.
  - Add test: no matched document returns domain not-found error.

- [ ] **Step 2: Run repository tests and verify fail**
  - Run: `go test ./internal/function-service/repositories -run Delete -v`

- [ ] **Step 3: Implement repository delete method**
  - Add method such as `DeleteByWorkspaceFunctionResourceID(ctx, workspaceID, functionKey, resourceID string) error`.
  - Filter must match all 3 fields.
  - Map `DeletedCount == 0` to domain not-found error.

- [ ] **Step 4: Re-run repository tests and verify pass**
  - Run: `go test ./internal/function-service/repositories -run Delete -v`

- [ ] **Step 5: Commit Task 3**
  - `git add internal/function-service/repositories/mongo_resource_repository.go internal/function-service/repositories/mongo_resource_repository_test.go`
  - `git commit -m "feat(function-service): add scoped resource deletion repository"`

---

### Task 4: Implement service delete orchestration (delete then publish)

**Files:**
- Modify: `internal/function-service/services/resource_service.go`
- Modify: `internal/function-service/services/resource_service_test.go`

- [ ] **Step 1: Write failing service tests**
  - Success path: repository delete success -> publish success -> nil error.
  - Not-found path: repository not-found -> return domain not-found and do not publish.
  - Publish failure path: repository success + publish failure -> return internal error.

- [ ] **Step 2: Run service tests and verify fail**
  - Run: `go test ./internal/function-service/services -run Delete -v`

- [ ] **Step 3: Implement service method**
  - Add method such as `DeleteResource(ctx context.Context, workspaceID, functionKey, resourceID string) error`.
  - Ordering must be:
    1. call repository delete
    2. build delete event payload
    3. publish event with configured subject
    4. return success

- [ ] **Step 4: Add structured logging in service**
  - Log requested delete identifiers, publish success/failure, and event metadata.

- [ ] **Step 5: Re-run service tests and verify pass**
  - Run: `go test ./internal/function-service/services -run Delete -v`

- [ ] **Step 6: Commit Task 4**
  - `git add internal/function-service/services/resource_service.go internal/function-service/services/resource_service_test.go`
  - `git commit -m "feat(function-service): orchestrate delete and post-delete event publish"`

---

### Task 5: Add HTTP DELETE handler and route wiring

**Files:**
- Modify: `internal/function-service/handlers/resource_handler.go`
- Modify: `internal/function-service/handlers/resource_handler_test.go`
- Modify: `cmd/function-service/main.go`

- [ ] **Step 1: Write failing handler tests**
  - Valid path + service success -> `204`.
  - Service not-found -> `404`.
  - Missing/invalid path params -> `400`.

- [ ] **Step 2: Run handler tests and verify fail**
  - Run: `go test ./internal/function-service/handlers -run Delete -v`

- [ ] **Step 3: Implement handler method**
  - Parse `workspace_id`, `function_key`, `resource_id` from path.
  - Validate non-empty.
  - Call service delete method.
  - Map errors to `400/404/500`.
  - Return `204` with empty body on success.

- [ ] **Step 4: Wire route in main**
  - Register DELETE route under existing `/api/v1` group with current middleware stack.

- [ ] **Step 5: Re-run handler tests and verify pass**
  - Run: `go test ./internal/function-service/handlers -run Delete -v`

- [ ] **Step 6: Commit Task 5**
  - `git add internal/function-service/handlers/resource_handler.go internal/function-service/handlers/resource_handler_test.go cmd/function-service/main.go`
  - `git commit -m "feat(function-service): add delete resource API endpoint"`

---

### Task 6: Add API example and run full verification

**Files:**
- Create: `examples/api/function_resource_delete.http`

- [ ] **Step 1: Add API example file**
  - Include sample DELETE request path and expected `204` + `404` scenarios.

- [ ] **Step 2: Run focused package tests**
  - `go test ./internal/function-service/config ./internal/function-service/transport ./internal/function-service/repositories ./internal/function-service/services ./internal/function-service/handlers -v`

- [ ] **Step 3: Run full backend verification**
  - `go test ./...`

- [ ] **Step 4: Commit Task 6**
  - `git add examples/api/function_resource_delete.http`
  - `git commit -m "docs(function-service): add delete resource API example"`

---

## Definition of Done

- Required env var `FUNCTION_SERVICE_JETSTREAM_RESOURCE_DELETED_SUBJECT` is enforced at startup.
- DELETE API returns:
  - `204` only when delete and publish both succeed.
  - `404` when resource does not exist.
  - `400` for invalid path params.
  - `500` for repository/publish failures.
- Delete event uses configured subject (default in `.env.example`: `app.todo.resource.deleted`).
- All related tests pass and `go test ./...` passes.
