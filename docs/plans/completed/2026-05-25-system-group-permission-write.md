# System Group Permission Write Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write system group relationships to the permission API before MongoDB persistence and return `206 Partial Content` for per-relationship failures.

**Architecture:** The shared permission client owns HTTP request and response DTO mapping for `/api/v1/relationships/write`. `group-service` owns the consumer-side permission writer interface, calls it from `SystemGroupService`, filters failed relationships by checksum, rebuilds the saved group from the accepted projection, and keeps handlers responsible only for status/response mapping. Runtime wiring stays in `cmd/group-service`.

**Tech Stack:** Go 1.25, Echo v5, `net/http`, `log/slog`, MongoDB repositories, repository-local Markdown design docs.

---

## Source Designs

- `docs/designs/permission-api-client-design.md`
- `docs/designs/system-group-api-design.md`
- `docs/policies/backend-architecture-principle.md`
- `docs/policies/design-and-plan-docs-policy.md`

## File Structure

- Modify `internal/shared/interactions/permission/request.go`: add relationship write request/response DTOs and result mapping types.
- Modify `internal/shared/interactions/permission/client.go`: add `WriteRelationships`.
- Modify `internal/shared/interactions/permission/client_test.go`: add client request/result/error coverage.
- Modify `internal/group-service/services/system_group_service.go`: add permission writer dependency, partial success result, warning logs, and request-level failure error.
- Modify `internal/group-service/services/system_group_relationship_builder.go`: add relationship projection filtering and relationship-to-group rebuild helpers.
- Modify `internal/group-service/services/system_group_service_test.go`: add TDD coverage for permission call ordering, failed task filtering, warning log, and request-level failure.
- Modify `internal/group-service/handlers/group_handler.go`: return `206` when service returns permission errors and `502` for request-level permission failure.
- Modify `internal/group-service/handlers/group_handler_test.go`: add `206` and `502` tests.
- Modify `internal/group-service/transport/system_group_response.go`: add partial success response DTO.
- Modify `internal/group-service/config/config.go` and `internal/group-service/config/config_test.go`: add permission API configuration.
- Modify `cmd/group-service/main.go` and `cmd/group-service/main_test.go`: wire the concrete permission client.
- Modify `internal/mock-permission-api/handlers/schema_handler.go` and tests: add local `/api/v1/relationships/write` endpoint.
- Modify `examples/api/mock_permission_api.http` and `examples/api/system-groups.http`: document the relationship write endpoint and partial success response behavior.
- Modify `.env.example`: add group-service permission API environment keys.

## Tasks

### Task 1: Shared Permission Client

- [ ] Write failing tests in `internal/shared/interactions/permission/client_test.go` proving `WriteRelationships` sends `POST /api/v1/relationships/write`, maps `operator` to `operation`, preserves relationship payloads, and maps `writes`/`deletes` success flags into `SuccessTasks` and `FailedTasks`.
- [ ] Run `go test ./internal/shared/interactions/permission -run 'TestClientWriteRelationships'` and verify the tests fail because `WriteRelationships` is missing.
- [ ] Add relationship write DTOs and `WriteRelationships` to `internal/shared/interactions/permission/request.go` and `client.go`.
- [ ] Re-run `go test ./internal/shared/interactions/permission -run 'TestClientWriteRelationships|TestClientRegisterResourceAttributes'` and verify it passes.

### Task 2: Group Service Permission Workflow

- [ ] Write failing service tests in `internal/group-service/services/system_group_service_test.go` proving create sends permission write tasks before repository persistence, request-level permission errors skip MongoDB, failed tasks filter relationships by checksum, rebuilt groups reflect accepted relationships, and failed tasks emit warning logs.
- [ ] Run `go test ./internal/group-service/services -run 'TestSystemGroupService'` and verify the new tests fail.
- [ ] Add the consumer-side system group permission writer interface, service result shape, logger option, request-level error sentinel, failed-task filtering, and relationship-to-group rebuild helpers.
- [ ] Re-run `go test ./internal/group-service/services -run 'TestSystemGroupService|TestBuildSystemGroupRelationshipProjection|TestRelationshipChecksum'` and verify it passes.

### Task 3: Handler And Transport Contract

- [ ] Write failing handler/transport tests for `206 Partial Content` with `{ "group": ..., "errors": [...] }` and `502 Bad Gateway` for request-level permission write failure.
- [ ] Run `go test ./internal/group-service/handlers ./internal/group-service/transport` and verify the new tests fail.
- [ ] Add the partial success response DTO and update `CreateSystemGroup` handler status mapping.
- [ ] Re-run `go test ./internal/group-service/handlers ./internal/group-service/transport` and verify it passes.

### Task 4: Configuration, Wiring, Mock API, And Examples

- [ ] Write failing config/main/mock tests proving group-service reads required permission API config, wires `permission.Client`, and mock-permission-api accepts `/api/v1/relationships/write`.
- [ ] Run `go test ./internal/group-service/config ./cmd/group-service ./internal/mock-permission-api/handlers` and verify the tests fail.
- [ ] Add group-service permission API config validation, command wiring, mock handler route, `.env.example`, and REST Client examples.
- [ ] Re-run `go test ./internal/group-service/config ./cmd/group-service ./internal/mock-permission-api/handlers` and verify it passes.

### Task 5: Full Verification

- [ ] Run `go test ./internal/shared/interactions/permission ./internal/group-service/... ./cmd/group-service ./internal/mock-permission-api/handlers`.
- [ ] Run `go test ./...`.
- [ ] Move this plan to `docs/plans/completed/2026-05-25-system-group-permission-write.md` after implementation passes.
