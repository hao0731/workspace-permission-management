# Workspace Service Design

## Background

The workspace permission management system uses workspaces as the top-level scope for groups, function resources, and permissions. This design introduces `workspace-service` as the service that creates workspace records and optionally asks configured application functions to create initial resources for the workspace.

This document is the entry point for `workspace-service`. Focused endpoint and command details are split into:

- [Workspace Service API Design](workspace-service-api-design.md): `POST /api/v1/workspaces`, workspace persistence, HR lookup, response contract, and error mapping.
- [Workspace Service Command Design](workspace-service-command-design.md): config-driven resource-create command publishing for `documents`, `tasks`, and `drive`.

Related designs:

- [Mock Function Design](mock-function.md): consumes workspace resource-create commands and publishes function resource upsert events.
- [Mock HR Design](mock-hr.md): provides mock HR APIs and the shared HR interaction client used by `workspace-service`.
- [Function Service Design](function-service.md): consumes function resource upsert events produced by `mock-function`.

Related concept definitions are documented in [../concept.md](../concept.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- HTTP payloads, MongoDB documents, and CloudEvent command payloads are explicit contracts.
- Handlers stay thin and only parse request bodies, invoke services, and render responses or mapped errors.
- Request and response DTOs belong in `internal/workspace-service/transport`.
- Workspace domain types and invariants stay independent of Echo, MongoDB, NATS, JetStream, and HR HTTP client details.
- MongoDB access is isolated in `internal/workspace-service/repositories`.
- JetStream publishing is exposed to services through a consumer-side interface; NATS and JetStream types stay out of service and domain logic.
- HR lookups use `internal/shared/interactions/hr.Client`; the concrete mock HR HTTP client lives under `internal/shared/interactions/hr/poc`.
- These design documents are stored under `docs/designs/`.

## Service Goals

- Build `workspace-service` as an independent backend service.
- Expose `POST /api/v1/workspaces`.
- Use `internal/shared/health` for `GET /health/liveness`.
- Resolve the request `owner` through the shared HR client before creating a workspace.
- Persist workspace records in MongoDB collection `workspaces`.
- Store only `owner_nt_account` in the workspace document; do not persist HR display names.
- Return `201 Created` with the created workspace and owner display name from HR.
- Optionally publish resource-create commands for `documents`, `tasks`, and `drive` when those sections are present in the request.
- Make resource-create command subjects and payload values config-driven by application mapping.
- Treat resource-create command publishing as a best-effort side effect after workspace persistence.
- Log command publish failures without changing the successful workspace create response.
- Keep API, HR lookup, persistence, and JetStream publishing behavior testable through explicit boundaries.

## Non-Goals

- Do not implement workspace read, update, delete, list, or search APIs in this phase.
- Do not implement workspace name uniqueness rules.
- Do not persist the owner display name in `workspaces`.
- Do not implement an outbox, retry worker, or guaranteed command delivery for resource-create commands in this phase.
- Do not validate whether configured application functions exist in another service.
- Do not add frontend changes.

## API Surface

| Endpoint | Design | Success | External dependency behavior |
| --- | --- | --- | --- |
| `POST /api/v1/workspaces` | [Workspace Service API Design](workspace-service-api-design.md) | `201 Created` with `workspace` object | HR lookup failure returns `502`; resource-command publish failures are logged and do not affect the response |
| `GET /health/liveness` | This entry design | `200 OK` when the process indicator is healthy | Does not check MongoDB, NATS, or HR availability |

## Recommended Architecture

`workspace-service` should follow the existing backend layout:

```plaintext
cmd/workspace-service
internal/workspace-service/config
internal/workspace-service/handlers
internal/workspace-service/repositories
internal/workspace-service/services
internal/workspace-service/transport
internal/domain/workspace
```

Responsibilities:

- `cmd/workspace-service`: composition root. It loads configuration, creates the logger, connects to MongoDB and NATS, creates the HR client, ensures repository indexes, registers health and workspace routes, and handles graceful shutdown.
- `internal/workspace-service/config`: environment-based configuration and validation for HTTP, MongoDB, NATS, HR base URL, resource mappings, publish timeout, and shutdown timeout.
- `internal/workspace-service/handlers`: Echo handlers for request decoding, service invocation, response rendering, and error mapping.
- `internal/workspace-service/transport`: request DTOs, response DTOs, CloudEvent command builders, and DTO/domain mappers.
- `internal/workspace-service/services`: create-workspace workflow, owner HR lookup orchestration, ID and clock usage, workspace repository calls, and best-effort command publishing orchestration.
- `internal/workspace-service/repositories`: MongoDB documents, indexes, insert behavior, and document/domain mapping.
- `internal/domain/workspace`: workspace model, create input, command request model, validation, and stable domain errors.

The service also depends on shared packages:

- `internal/shared/health` for liveness.
- `internal/shared/eventbus` for JetStream producer integration.
- `internal/shared/interactions/hr` for the HR client interface.
- `internal/shared/http/exception` for the standard error response shape.

## Common Workflow

Create workspace:

1. Handler decodes `POST /api/v1/workspaces` into a transport request.
2. Transport maps request fields into a domain/service create input.
3. Service validates required fields and resource options.
4. Service calls HR client `Get(ctx, ownerNTAccount)` before writing the workspace.
5. If HR lookup fails, service returns an upstream dependency error and no workspace document is written.
6. Service generates `workspace_id`, `created_at`, and `updated_at`.
7. Repository inserts the workspace document with `owner_nt_account` only.
8. Service builds resource-create commands for any present `documents`, `tasks`, and `drive` request sections.
9. Service publishes commands one by one through a publisher interface.
10. Publish failures are logged with structured fields and do not change the returned `201`.
11. Handler renders the created workspace response using persisted workspace fields and the HR user display name.

Rationale:

- HR lookup is required before persistence because the public create response includes `owner.display_name`.
- Persisting only `owner_nt_account` keeps workspace ownership data stable and avoids storing copied HR attributes.
- Best-effort command publishing keeps the first implementation small and matches the requested behavior.

## Configuration

`workspace-service` uses environment-based configuration through `viper`, matching existing backend service conventions.

Required configuration:

- `WORKSPACE_SERVICE_HTTP_ADDR`
- `WORKSPACE_SERVICE_MONGODB_URI`
- `WORKSPACE_SERVICE_MONGODB_DATABASE`
- `WORKSPACE_SERVICE_NATS_URL`
- `WORKSPACE_SERVICE_HR_BASE_URL`
- `WORKSPACE_SERVICE_DOCUMENTS_APP_NAME`
- `WORKSPACE_SERVICE_DOCUMENTS_RESOURCE_TYPE`
- `WORKSPACE_SERVICE_TASKS_APP_NAME`
- `WORKSPACE_SERVICE_TASKS_RESOURCE_TYPE`
- `WORKSPACE_SERVICE_DRIVE_APP_NAME`
- `WORKSPACE_SERVICE_DRIVE_RESOURCE_TYPE`

Optional configuration with defaults:

- `WORKSPACE_SERVICE_ENV`: default `development`
- `WORKSPACE_SERVICE_SHUTDOWN_TIMEOUT`: default `10s`
- `WORKSPACE_SERVICE_COMMAND_PUBLISH_TIMEOUT`: default `15s`

Validation rules:

- Environment must be a known value from `internal/shared/environment`.
- Required string values must be non-empty after trimming.
- Shutdown timeout and command publish timeout must be positive.
- Application names must be valid subject tokens for `cmd.app.<APP_NAME>.resource.create`.
- Resource types must be non-empty strings after trimming.
- Missing `.env` files must not fail startup.

## Health and Shutdown

`cmd/workspace-service/main.go` should use `internal/shared/health`.

Liveness endpoint:

```http
GET /health/liveness
```

The first implementation should use the same lightweight `process` indicator pattern as `function-service` and `group-service`.

Rationale:

- Liveness should answer whether the process can serve at all, not whether MongoDB, NATS, or HR is temporarily reachable.
- A future readiness endpoint can include dependency checks if deployment needs traffic gating.
- Driver-specific checks must not be placed in domain or service packages.

The service must support graceful shutdown with a timeout-bound context and disconnect MongoDB during shutdown. NATS connections should be closed during shutdown.

## REST Client Examples

The implementation should add `examples/api/workspaces.http` with:

- Successful workspace create without optional resources.
- Successful workspace create with `documents`, `tasks`, and `drive`.
- Validation error for missing or empty `name`.
- Validation error for missing or empty `owner`.
- Validation error for an optional resource object with empty `resource_name`.

## Testing Strategy

Detailed test expectations live in:

- [Workspace Service API Design](workspace-service-api-design.md#testing-strategy)
- [Workspace Service Command Design](workspace-service-command-design.md#testing-strategy)

Repository-wide verification for implementation:

```bash
go test ./...
```

Additional repository integration tests may require local MongoDB from Docker Compose.

## Architecture Decisions and Trade-Offs

- Independent service boundary: keeps workspace creation ownership separate from group and function projections, at the cost of adding a service entrypoint and configuration surface.
- Split design documents: keeps this entry document readable while preserving traceability from one service-level entry point.
- HR lookup before persistence: ensures the create response can include `owner.display_name`, but means temporary HR outage blocks workspace creation.
- Persist only `owner_nt_account`: avoids stale copied HR attributes, but future read APIs must call HR to enrich owner display names.
- Best-effort command publishing: matches the requested simple flow and avoids outbox complexity, but command delivery is not guaranteed after workspace persistence.
- No workspace name uniqueness: avoids defining product rules early, but clients cannot rely on names being unique.

## Implementation Plan Notes

Any implementation plan should be created under `docs/plans/active/` and link back to this entry document plus:

- [Workspace Service API Design](workspace-service-api-design.md)
- [Workspace Service Command Design](workspace-service-command-design.md)
- [Mock HR Design](mock-hr.md)
- [Mock Function Design](mock-function.md)

The plan should follow test-driven sequencing:

1. HR domain and shared client interface tests or compile checks.
2. Workspace domain model and validation tests.
3. Workspace transport request, response, and command event builder tests.
4. Workspace service workflow tests with fake HR client, repository, publisher, clock, and ID generator.
5. Workspace repository insert and index tests.
6. Workspace handler route and error mapping tests.
7. Config, main wiring, health route, Docker Compose, and REST Client example updates.
