# Function Resource Permissions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions` in `function-service` with full-replacement semantics, MongoDB persistence in `function_resource_permissions`, UUID handling for `extra_rules.rule_id`, and canonical persisted response payload.

**Architecture:** Keep handlers transport-thin, keep service/domain logic framework-independent, isolate MongoDB details in repository, and keep API/DB contracts explicit and versionable.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, `log/slog`, `viper`, standard `testing`.

---

## Source Design

Source designs:

- [../../designs/function-resource-permissions.md](../../designs/function-resource-permissions.md)
- [../../designs/function-service.md](../../designs/function-service.md)

Applicable policies:

- [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend implementation keeps dependency direction clean: handler → service → repository.
- Domain/service layers must not depend on Echo/MongoDB driver DTOs.
- Design/plan artifacts must live under `docs/plans/active/` before implementation and be committed.

## Scope

Implement:

- `PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions`.
- Request validation and error mapping using existing shared error envelope.
- Upsert persistence into `function_resource_permissions` by `(workspace_id, function_key)`.
- UUID v4 generation for missing `extra_rules.rule_id`.
- UUID validation for provided `extra_rules.rule_id`.
- Canonical response from persisted data.
- Required tests for handler/service/repository/transport mapping.

Out of scope:

- Permission evaluation engine.
- Additional permissions read endpoints.
- Frontend changes.
- Background jobs/outbox/retry pipeline.

## File Structure

Create (if missing):

- `internal/function-service/transport/permission_request.go`
- `internal/function-service/transport/permission_response.go`
- `internal/function-service/transport/permission_request_test.go`
- `internal/function-service/repositories/mongo_permission_repository.go`
- `internal/function-service/repositories/mongo_permission_repository_test.go`
- `internal/function-service/services/permission_service.go`
- `internal/function-service/services/permission_service_test.go`
- `internal/function-service/handlers/permission_handler.go`
- `internal/function-service/handlers/permission_handler_test.go`
- `examples/api/function_permissions.http`

Modify (expected):

- `internal/domain/resource/resource.go` (or equivalent domain package for permission inputs/entities)
- `internal/domain/resource/errors.go`
- `cmd/function-service/main.go` (wire route + dependencies)
- `.env.example` (if new config keys are introduced)

## Task 1: Domain Contract and Validation

- [ ] Define domain input/output types for baseline/extra rules and workspace-function scoped permissions.
- [ ] Add domain-level `Validate()` for invariant checks:
  - non-empty `workspace_id`, `function_key`
  - non-empty baseline `action_id`
  - non-empty baseline `resource_tags`
  - non-empty `group_ids`
  - non-empty extra-rule `resource_tags`
  - `expiration_date` must be future time
  - provided `rule_id` must be valid UUID
- [ ] Ensure validation errors wrap existing `resource.ErrInvalidInput` (or equivalent) for stable error mapping.
- [ ] Add/extend domain tests to lock these invariants.

## Task 2: Transport DTOs and Request Parsing

- [ ] Add PUT request DTOs with `baseline_rule.enabled` required in request.
- [ ] Add response DTOs matching canonical persisted format.
- [ ] Implement transport-to-domain mapping.
- [ ] Handler must map transport validation/domain validation errors to `400` using shared error response envelope.
- [ ] Add handler/transport tests for:
  - success
  - missing fields
  - non-UUID provided `rule_id`
  - past `expiration_date`

## Task 3: Service Workflow (Full Replacement + Rule ID Strategy)

- [ ] Add permission service interface and implementation.
- [ ] Implement full-replacement behavior for target `(workspace_id, function_key)` document.
- [ ] For each extra rule:
  - keep provided UUID `rule_id`
  - generate UUID v4 when missing
- [ ] Return canonical persisted value from repository result mapping.
- [ ] Add service tests for generate/preserve/replace behavior and repository failure path.

## Task 4: MongoDB Repository and Indexes

- [ ] Add repository contract and MongoDB implementation for collection `function_resource_permissions`.
- [ ] Implement upsert filter `{workspace_id, function_key}` and replacement update for permission blocks.
- [ ] Ensure one-document invariant via unique index `{workspace_id: 1, function_key: 1}`.
- [ ] Implement read-back (or `FindOneAndUpdate` return-document) to support canonical response.
- [ ] Add repository tests for upsert semantics, index creation, and mapping.

## Task 5: Route Wiring and Runtime Integration

- [ ] Register route: `PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions`.
- [ ] Wire handler/service/repository in `cmd/function-service/main.go`.
- [ ] Keep composition root responsible for infrastructure dependency construction only.
- [ ] Add/update `.http` example for success and validation failures.

## Task 6: Verification and Completion

- [ ] Run targeted tests first:

```bash
go test ./internal/function-service/transport ./internal/function-service/services ./internal/function-service/repositories ./internal/function-service/handlers -count=1
```

- [ ] Run full backend verification:

```bash
go test ./... -count=1
```

- [ ] Optional static check when touching runtime wiring/concurrency:

```bash
go vet ./...
```

- [ ] If any command cannot run, record exact command + reason + residual risk in implementation notes.

## Risks and Mitigations

1. **Clock skew vs future `expiration_date` check**
   - Mitigation: compare using server UTC now; clearly document behavior.

2. **Client/server schema drift on `baseline_rule.enabled`**
   - Mitigation: strict DTO validation + examples + design link references.

3. **Concurrent PUT race on same workspace/function**
   - Mitigation: atomic upsert per filter and deterministic last-write-wins behavior.

## Completion Criteria

- Endpoint implemented with `200` success and canonical persisted response.
- Validation/error mapping matches existing error envelope.
- DB document shape follows design and always contains `rule_id` for extra rules.
- `(workspace_id, function_key)` uniqueness enforced.
- Relevant tests pass.
- Plan remains in `docs/plans/active/` until implementation is completed; then move to `docs/plans/completed/` in a separate completion commit.
