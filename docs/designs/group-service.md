# Group Service Design

## Background

The workspace permission management system uses groups as permission subjects. A group belongs to one workspace and can include members through dynamic employee attribute rules, explicitly assigned individual members, or both.

This document is the entry point for `group-service`. Endpoint-family details are split into focused design documents:

- [Group API Design](group-service-group.md): group create, read, soft delete, and grouping-rule replacement.
- [Group Individual Members API Design](group-service-individual-members.md): individual member collection schema and paginated member reads.

Related context:

- [Concept Model](../concept.md)
- [Function Resource Permissions Design](function-resource-permissions.md)

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- REST payloads, MongoDB documents, indexes, soft-delete behavior, and cursor tokens are explicit contracts.
- HTTP handlers remain thin and only parse path, query, and body input, invoke services, and render responses or mapped errors.
- Request and response DTOs belong in `internal/group-service/transport`.
- Group domain types and invariants stay independent of Echo and MongoDB.
- MongoDB access remains isolated in `internal/group-service/repositories`.
- This entry design and its split documents are stored under `docs/designs/`.

## Service Goals

- Build `group-service` as an independent backend service.
- Keep `workspace_id` as the path scope for every group endpoint.
- Persist group definitions in MongoDB collection `groups`.
- Persist explicit individual members in MongoDB collection `group_individual_members`.
- Use soft deletion through `deleted_at` for both groups and individual members.
- Keep create and delete operations that touch both collections atomic through MongoDB transactions.
- Keep validation, service workflows, persistence, and transport mapping aligned with repository boundaries.
- Use `internal/shared/health` for liveness probing.
- Use `internal/shared/pagination` for cursor-based member list parsing and token handling.

## API Surface

| Endpoint | Design | Success | Missing target behavior |
| --- | --- | --- | --- |
| `POST /api/v1/workspaces/:workspace_id/groups` | [Group API Design](group-service-group.md#create-group-api) | `201 Created` with the created group payload | Not applicable |
| `GET /api/v1/workspaces/:workspace_id/groups/:group_id` | [Group API Design](group-service-group.md#get-group-api) | `200 OK` with `group` object | `200 OK` with `"group": null` |
| `DELETE /api/v1/workspaces/:workspace_id/groups/:group_id` | [Group API Design](group-service-group.md#delete-group-api) | `204 No Content` | `204 No Content` |
| `PUT /api/v1/workspaces/:workspace_id/groups/:group_id/grouping-rules` | [Group API Design](group-service-group.md#replace-grouping-rules-api) | `204 No Content` | `404 Not Found` |
| `GET /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members` | [Group Individual Members API Design](group-service-individual-members.md#list-individual-members-api) | `200 OK` with `members` and `page_info` | `200 OK` with an empty page |

Common API conventions:

- Public JSON field names use `snake_case`.
- `workspace_id` is taken from the path and is not returned in public group responses.
- Timestamp request fields are accepted as RFC3339 strings and stored as MongoDB Date values.
- Timestamp response fields are returned as RFC3339 strings through Go JSON encoding for `time.Time`.
- New HTTP errors use the shared backend error response shape from `internal/shared/http/exception`.

## Recommended Architecture

`group-service` follows the existing backend layout:

```plaintext
cmd/group-service
internal/group-service/config
internal/group-service/handlers
internal/group-service/repositories
internal/group-service/services
internal/group-service/transport
internal/domain/group
```

Responsibilities:

- `cmd/group-service`: composition root. It loads configuration, creates the logger, connects to MongoDB, ensures repository indexes, registers health and group routes, and handles graceful shutdown.
- `internal/group-service/handlers`: Echo handlers for path, query, and body extraction, transport validation, service invocation, and error mapping.
- `internal/group-service/transport`: request/response DTOs, RFC3339 parsing, cursor token DTOs, and DTO/domain mapping.
- `internal/group-service/services`: use cases, domain validation orchestration, ID generation, clock usage, and transaction-oriented repository calls.
- `internal/group-service/repositories`: MongoDB documents, indexes, queries, updates, transactions, and document/domain mapping.
- `internal/domain/group`: framework-independent group models, validation, normalization, cursor models, and stable domain errors.

Alternatives considered:

- Put all group and member API details in this file: simple for the first endpoint, but the document becomes too large as group and member behavior diverge.
- Split every endpoint into its own file: precise, but too fragmented for the current service size.
- Add these endpoints to `function-service`: smaller service count, but groups are not part of the function domain and this would blur ownership.

## Common Persistence

`group-service` owns two MongoDB collections:

- `groups`: group definitions, display metadata, grouping rules, timestamps, and group soft-delete state. Details are in [Group API Design](group-service-group.md#groups-collection).
- `group_individual_members`: explicit members assigned to groups, member expiration, timestamps, and member soft-delete state. Details are in [Group Individual Members API Design](group-service-individual-members.md#group_individual_members-collection).

Soft-delete rules:

- Active documents have `deleted_at: null`.
- Soft-deleted documents set `deleted_at` and `updated_at` to the same service-generated timestamp.
- Deleting a group soft-deletes the active group document and all active individual member documents for that group in one transaction.

MongoDB transactions require MongoDB to run as a replica set. The existing local `docker-compose.yml` starts MongoDB with `--replSet rs0`, so the design aligns with local development infrastructure.

## Common Error Shape

Known errors should use this response shape:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Human-readable summary",
    "details": {},
    "request_id": "request-id"
  }
}
```

Common status mapping:

- `400 Bad Request`: malformed JSON, invalid query parameters, invalid request shape, invalid field values, configured limit violations, or expiration dates that are not in the future.
- `404 Not Found`: update operations whose active group target does not exist.
- `409 Conflict`: active group name already exists in the same workspace during create.
- `500 Internal Server Error`: unexpected repository, transaction, or infrastructure failure.

`DELETE /groups/:group_id` is intentionally idempotent and returns `204` even when the active group is already missing.

## Configuration

`group-service` uses environment-based configuration through `viper`, matching existing backend service conventions.

Required configuration:

- `GROUP_SERVICE_HTTP_ADDR`
- `GROUP_SERVICE_MONGODB_URI`
- `GROUP_SERVICE_MONGODB_DATABASE`

Optional configuration with defaults:

- `GROUP_SERVICE_ENV`: default `development`
- `GROUP_SERVICE_SHUTDOWN_TIMEOUT`: default `10s`
- `GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS`: default `1000`
- `GROUP_SERVICE_MAX_GROUPING_RULES`: default `10`

Validation rules:

- Environment must be a known value from `internal/shared/environment`.
- Required string values must be non-empty after trimming.
- Shutdown timeout must be positive.
- `GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS` must be positive.
- `GROUP_SERVICE_MAX_GROUPING_RULES` must be positive.
- Missing `.env` files must not fail startup.

Member list pagination uses `internal/shared/pagination` defaults unless implementation later adds explicit group-service pagination configuration:

- default `limit`: `20`
- maximum `limit`: `50`
- cursor query parameter: `next_token`

## Health and Shutdown

`cmd/group-service/main.go` should reuse `internal/shared/health`.

Liveness endpoint:

```http
GET /health/liveness
```

The first implementation should use the same lightweight `process` indicator pattern as `function-service`.

Rationale:

- Liveness should answer whether the process can serve at all, not whether MongoDB is temporarily reachable.
- A future readiness endpoint can include MongoDB checks if deployment needs traffic gating.
- Driver-specific health checks must not be placed in domain or service packages.

The service must support graceful shutdown with a timeout-bound context and must disconnect the MongoDB client during shutdown.

## REST Client Examples

The implementation should keep `examples/api/groups.http` aligned with all group-service APIs. It should include:

- Successful group creation.
- Duplicate group name conflict.
- Validation errors for empty membership source, invalid rules, duplicate individual `nt_account`, invalid `limit`, and invalid `next_token`.
- Successful group read and group-not-found read.
- Idempotent group delete.
- Successful grouping-rule replacement.
- Grouping-rule replacement validation where empty `rules` is rejected because no active individual members remain.
- Paginated individual member list with `limit` and `next_token`.

## Testing Strategy

Detailed test expectations live with each split design:

- [Group API Design](group-service-group.md#testing-strategy)
- [Group Individual Members API Design](group-service-individual-members.md#testing-strategy)

Repository-wide verification for implementation:

```bash
go test ./...
```

Additional repository integration tests may require local MongoDB from Docker Compose because transaction behavior depends on replica-set support.

## Architecture Decisions and Trade-Offs

- Independent service boundary: keeps group ownership separate from function permission ownership, at the cost of adding a service entrypoint and config surface.
- Split design documents: keeps this entry document readable while preserving traceability from one service-level entry point.
- Service-owned use cases with repository-owned MongoDB mechanics: keeps handlers thin and domain behavior testable while containing driver-specific code.
- Soft delete by `deleted_at`: preserves historical records and allows partial unique indexes for active records, at the cost of requiring every active query to include `deleted_at: null`.
- Idempotent group delete: simplifies clients and retry behavior, but clients that need to know whether a group previously existed must call GET before DELETE.
- Empty member list for missing group: avoids exposing group existence through a member-list error and matches cursor-list behavior, while GET group remains the canonical existence check.

## Implementation Plan Notes

Any implementation plan should be created under `docs/plans/active/` and link back to this entry document plus the relevant split design documents.

The plan should follow test-driven sequencing:

1. Domain model and validation tests.
2. Transport request, response, and cursor mapping tests.
3. Service workflow tests.
4. Repository transaction, soft-delete, read, update, and pagination tests.
5. Handler route and error mapping tests.
6. Config, main wiring, health route, and REST Client example updates.
