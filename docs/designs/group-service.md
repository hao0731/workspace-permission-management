# Group Service Design

## Background

The workspace permission management system uses groups as permission subjects. A group belongs to one workspace and can include members through dynamic employee attribute rules, explicitly assigned individual members, or both.

This document is the entry point for `group-service`. Endpoint-family details are split into focused design documents:

- [Group API Design](group-service-group.md): group create, read, soft delete, and grouping-rule replacement.
- [Group Individual Members API Design](group-service-individual-members.md): individual member collection schema, paginated member reads, and individual member mutations.
- [Group Expiry Command Design](group-service-group-expiry-command.md): grouping-rule expiry tasks, JetStream command contract, and idempotent command handling.
- [Group Individual Member Expiry Command Design](group-service-individual-member-expiry-command.md): individual-member expiry tasks, JetStream command contract, and idempotent command handling.
- [Group Expiry Scheduler Design](group-expiry-scheduler.md): independent scheduler service that scans expiry task collections and publishes the existing expiry command events.
- [System Group API Design](system-group-api-design.md): system-scoped group creation, update, paginated system group list, and permission relationship projection persistence.

Related context:

- [Concept Model](../concept.md)
- [Function Resource Permissions Design](function-resource-permissions.md)

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- REST payloads, MongoDB documents, indexes, soft-delete behavior, cursor tokens, and CloudEvent command payloads are explicit contracts.
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
- Persist grouping-rule expiration tasks in MongoDB collection `group_expiry_task`.
- Persist individual-member expiration tasks in MongoDB collection `individual_member_expiry_task`.
- Persist system-scoped group definitions in MongoDB collection `system_groups`.
- Persist system group permission relationship projections in MongoDB collection `system_group_relationships`.
- Share expiry task collection schema, indexes, and query helpers through `internal/shared/repositories/expiry`.
- Use soft deletion through `deleted_at` for both groups and individual members.
- Keep create and delete operations that touch both collections atomic through MongoDB transactions.
- Keep system group creation atomic across `system_groups` and `system_group_relationships` through MongoDB transactions.
- Keep system group updates atomic across `system_groups` and `system_group_relationships` through MongoDB transactions after permission API relationship writes are resolved.
- Keep individual member add, expiration update, and delete workflows scoped to an active group and atomic through MongoDB transactions.
- Keep grouping-rule creation, replacement, expiration task writes, and expiry command cleanup atomic through MongoDB transactions.
- Keep individual-member creation, expiration update, expiration task writes, and expiry command cleanup atomic through MongoDB transactions.
- Consume grouping-rule expiry commands from NATS JetStream through `internal/shared/eventbus`.
- Consume individual-member expiry commands from NATS JetStream through `internal/shared/eventbus`.
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
| `POST /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members` | [Group Individual Members API Design](group-service-individual-members.md#add-individual-members-api) | `201 Created` with added `members` | `404 Not Found` |
| `PATCH /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account` | [Group Individual Members API Design](group-service-individual-members.md#update-individual-member-expiration-api) | `204 No Content` | `404 Not Found` |
| `DELETE /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account` | [Group Individual Members API Design](group-service-individual-members.md#delete-individual-member-api) | `204 No Content` | `204 No Content` |
| `POST /api/v1/systems/:system_id/groups` | [System Group API Design](system-group-api-design.md#create-system-group) | `201 Created` with the created system group payload, or `206 Partial Content` when permission relationship tasks partially fail | Not applicable |
| `PUT /api/v1/systems/:system_id/groups/:group_id` | [System Group API Design](system-group-api-design.md#update-system-group) | `200 OK` with the saved system group payload, or `206 Partial Content` when permission relationship tasks partially fail | `404 Not Found` |
| `GET /api/v1/systems/:system_id/groups` | [System Group API Design](system-group-api-design.md#list-system-groups) | `200 OK` with `groups` and `page_info` | `200 OK` with an empty page |

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

- `cmd/group-service`: composition root. It loads configuration, creates the logger, connects to MongoDB and NATS, ensures repository indexes, registers health and group routes, starts the JetStream expiry command consumers, and handles graceful shutdown.
- `internal/group-service/handlers`: Echo handlers for path, query, and body extraction, JetStream command handlers, transport validation, service invocation, and error mapping.
- `internal/group-service/transport`: request/response DTOs, CloudEvent command DTOs, RFC3339 parsing, cursor token DTOs, and DTO/domain mapping.
- `internal/group-service/services`: use cases, domain validation orchestration, ID generation, clock usage, and transaction-oriented repository calls.
- `internal/group-service/repositories`: MongoDB documents, indexes, queries, updates, transactions, and document/domain mapping.
- `internal/domain/group`: framework-independent group models, validation, normalization, cursor models, and stable domain errors.

Alternatives considered:

- Put all group and member API details in this file: simple for the first endpoint, but the document becomes too large as group and member behavior diverge.
- Split every endpoint into its own file: precise, but too fragmented for the current service size.
- Add these endpoints to `function-service`: smaller service count, but groups are not part of the function domain and this would blur ownership.

## Common Persistence

`group-service` owns six MongoDB collections:

- `groups`: group definitions, display metadata, grouping rules, timestamps, and group soft-delete state. Details are in [Group API Design](group-service-group.md#groups-collection).
- `group_individual_members`: explicit members assigned to groups, member expiration, timestamps, and member soft-delete state. Details are in [Group Individual Members API Design](group-service-individual-members.md#group_individual_members-collection).
- `group_expiry_task`: the active outbox-like expiry task for each group that currently has dynamic grouping rules. Details are in [Group Expiry Command Design](group-service-group-expiry-command.md#group_expiry_task-collection).
- `individual_member_expiry_task`: the active outbox-like expiry task for each active individual member. Details are in [Group Individual Member Expiry Command Design](group-service-individual-member-expiry-command.md#individual_member_expiry_task-collection).
- `system_groups`: system-scoped group definitions. Details are in [System Group API Design](system-group-api-design.md#system_groups-collection).
- `system_group_relationships`: generated permission relationship projections for system groups. Details are in [System Group API Design](system-group-api-design.md#system_group_relationships-collection).

The expiry task collection schema, indexes, and task-specific query helpers are shared with `group-expiry-scheduler` through `internal/shared/repositories/expiry`. `group-service` still owns group and member write transactions and calls the shared expiry repository inside those transactions.

Soft-delete rules:

- Active documents have `deleted_at: null`.
- Soft-deleted documents set `deleted_at` and `updated_at` to the same service-generated timestamp.
- Deleting a group soft-deletes the active group document, soft-deletes all active individual member documents for that group, and deletes any group expiry tasks and individual-member expiry tasks for that group in one transaction.

Grouping-rule expiration:

- Active grouping rules have `groups.grouping_rule.expired_at: null` or no `expired_at` field on legacy documents.
- Expiry command handling sets `groups.grouping_rule.expired_at` and deletes the corresponding `group_expiry_task` in one transaction.
- Grouping-rule replacement resets `groups.grouping_rule.expired_at` to `null` and replaces the active expiry task when the replacement has dynamic rules.

Individual-member expiration:

- Active individual member records have `deleted_at: null`.
- Unexpired individual memberships have `group_individual_members.expired_at: null` or no `expired_at` field on legacy documents.
- Individual-member expiry command handling sets `group_individual_members.expired_at` and deletes the corresponding `individual_member_expiry_task` in one transaction.
- Individual-member creation writes matching expiry tasks, and individual-member expiration update resets `expired_at` to `null` while replacing the active expiry task.
- Public member responses continue to omit `expired_at`; permission evaluation should treat `expired_at != null` as not granting membership.

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
- `404 Not Found`: mutation operations whose required active group, active member, or saved system group relationship projection does not exist, except intentionally idempotent delete operations.
- `409 Conflict`: active group name already exists in the same workspace during create, a member add request contains duplicate accounts, or an active individual member already exists in the same group during member add.
- `502 Bad Gateway`: permission API request-level failure before local system group persistence.
- `500 Internal Server Error`: unexpected repository, transaction, or infrastructure failure.

`DELETE /groups/:group_id` and `DELETE /groups/:group_id/individual-members/:nt_account` are intentionally idempotent and return `204` even when the active target is already missing.

## Configuration

`group-service` uses environment-based configuration through `viper`, matching existing backend service conventions.

Required configuration:

- `GROUP_SERVICE_HTTP_ADDR`
- `GROUP_SERVICE_MONGODB_URI`
- `GROUP_SERVICE_MONGODB_DATABASE`
- `GROUP_SERVICE_NATS_URL`
- `GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM`
- `GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE`
- `GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT`
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM`
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE`
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT`

Optional configuration with defaults:

- `GROUP_SERVICE_ENV`: default `development`
- `GROUP_SERVICE_SHUTDOWN_TIMEOUT`: default `10s`
- `GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS`: default `1000`
- `GROUP_SERVICE_MAX_GROUPING_RULES`: default `10`
- `GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT`: default `20`
- `GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT`: default `5s`
- `GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE`: default `UTC`
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT`: default `20`
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT`: default `5s`
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE`: default `UTC`

Validation rules:

- Environment must be a known value from `internal/shared/environment`.
- Required string values must be non-empty after trimming.
- Shutdown timeout must be positive.
- `GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS` must be positive.
- `GROUP_SERVICE_MAX_GROUPING_RULES` must be positive.
- Group expiry command fetch count must be positive.
- Group expiry command max wait must be positive.
- Group expiry bucket timezone must be `UTC` or a supported fixed offset such as `UTC+8`.
- Individual-member expiry command fetch count must be positive.
- Individual-member expiry command max wait must be positive.
- Individual-member expiry bucket timezone must be `UTC` or a supported fixed offset such as `UTC+8`.
- Missing `.env` files must not fail startup.

Member list pagination uses `internal/shared/pagination` defaults unless implementation later adds explicit group-service pagination configuration:

- default `limit`: `20`
- maximum `limit`: `50`
- cursor query parameter: `next_token`

## Health and Shutdown

`cmd/group-service/main.go` should reuse `internal/shared/health` and `internal/shared/eventbus`.

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

At startup, `cmd/group-service/main.go` should connect to NATS, bind the configured JetStream streams and durable consumers, validate that each configured subject is included in its durable consumer filter, and run the expiry command consumers alongside the HTTP server. Startup should fail fast when a configured stream or durable consumer is missing.

## REST Client Examples

The implementation should keep `examples/api/groups.http` aligned with all group-service APIs. It should include:

- Successful group creation.
- Duplicate group name conflict.
- Validation errors for empty membership source, invalid rules, duplicate individual `nt_account`, invalid `limit`, and invalid `next_token`.
- Successful group read and group-not-found read.
- Idempotent group delete.
- Successful grouping-rule replacement.
- Grouping-rule replacement validation where empty `rules` is rejected because no unexpired active individual members remain.
- Paginated individual member list with `limit` and `next_token`.
- Successful individual member add, expiration update, and idempotent delete.
- Individual member add conflict and invalid expiration validation.

The implementation should add `examples/api/system-groups.http` for system-scoped group APIs. It should include successful create, empty-rule create, paginated list, empty list, unsupported `not_eq`, duplicate `job_type`, invalid `limit`, and invalid `next_token` examples.
It should also include successful system group update, partial permission-write update, and update target-not-found examples.

## Testing Strategy

Detailed test expectations live with each split design:

- [Group API Design](group-service-group.md#testing-strategy)
- [Group Individual Members API Design](group-service-individual-members.md#testing-strategy)
- [Group Expiry Command Design](group-service-group-expiry-command.md#testing-strategy)
- [Group Individual Member Expiry Command Design](group-service-individual-member-expiry-command.md#testing-strategy)
- [System Group API Design](system-group-api-design.md#testing-strategy)

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
- Member mutation status split: add and expiration update return `404` when their required active group or member target is missing, while member delete is idempotent `204` for retry safety.

## Implementation Plan Notes

Any implementation plan should be created under `docs/plans/active/` and link back to this entry document plus the relevant split design documents.

The plan should follow test-driven sequencing:

1. Domain model and validation tests.
2. Transport request, response, CloudEvent command, and cursor mapping tests.
3. Service workflow tests.
4. Repository transaction, soft-delete, read, update, pagination, and expiry-task tests.
5. Handler route, command handler, and error mapping tests.
6. Config, main wiring, health route, JetStream consumers, and REST Client example updates.
