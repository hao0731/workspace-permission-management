# Group Service Design

## Background

The workspace permission management system uses groups as permission subjects. A group belongs to one workspace and can include members through dynamic employee attribute rules, explicitly assigned individual members, or both.

This design introduces `group-service`, a backend service that creates workspace-scoped groups and persists both the group definition and individual member overrides atomically in MongoDB.

Related context:

- [Concept Model](../concept.md)
- [Function Resource Permissions Design](function-resource-permissions.md)

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- The API payload, response payload, MongoDB documents, indexes, and transaction behavior are explicit contracts.
- HTTP handlers remain thin and only parse path/body input, invoke the service, and render responses or mapped errors.
- Request and response DTOs belong in `internal/group-service/transport`.
- Group domain types and invariants stay independent of Echo and MongoDB.
- The service owns the create workflow, UUID generation, timestamp assignment, and transaction boundary.
- MongoDB access remains isolated in `internal/group-service/repositories`.
- `workspace_id` is a path scope and persistence field; it is intentionally not returned in the create response.
- This design document is stored under `docs/designs/`.

## Goals

- Build `group-service` as an independent backend service.
- Expose `POST /api/v1/workspaces/:workspace_id/groups`.
- Return `201 Created` with the created group payload.
- Persist group definitions in MongoDB collection `groups`.
- Persist individual member overrides in MongoDB collection `group_individual_members`.
- Use a MongoDB transaction so `groups` and `group_individual_members` writes commit or fail together.
- Store `workspace_id` on group documents.
- Enforce active group name uniqueness within one workspace.
- Allow the same group name in different workspaces.
- Trim group names before validation, persistence, and uniqueness checks.
- Require at least one membership source: dynamic grouping rules or individual members.
- Treat grouping rules as an `AND` relationship.
- Require all expiration dates to be later than request processing time.
- Reject duplicate `individual_members[].nt_account` values in one request.
- Enforce at most one active `group_individual_members` document for the same `group_id + nt_account`.
- Store `deleted_at` as `null` when records are active.
- Reuse `internal/shared/health` for liveness probing.
- Keep validation, service workflow, persistence, and transport mapping aligned with existing repository boundaries.

## Non-Goals

- Do not validate whether `workspace_id` references an existing workspace.
- Do not materialize or evaluate group membership from employee attributes.
- Do not define an employee attribute catalog or type system.
- Do not implement group list, get, update, delete, or soft-delete APIs.
- Do not publish group-created or membership-changed events.
- Do not implement NATS or JetStream integration for `group-service` in this phase.
- Do not add frontend changes.
- Do not change existing `function-service` behavior.

## Recommended Approach

Create an independent `group-service` following the existing backend layout:

```plaintext
cmd/group-service
internal/group-service/config
internal/group-service/handlers
internal/group-service/repositories
internal/group-service/services
internal/group-service/transport
internal/domain/group
```

`cmd/group-service` is the composition root. It loads configuration, creates the logger, connects to MongoDB, ensures repository indexes, registers health routes, registers group API routes, and handles graceful shutdown.

`internal/group-service/services` owns the create group use case. The service validates business invariants through domain types, generates IDs and timestamps through injectable functions, and calls a repository method that executes the MongoDB transaction.

`internal/group-service/repositories` owns MongoDB documents, indexes, and transaction execution. Repository code maps between MongoDB documents and domain models before returning data to services.

Alternatives considered:

- Add the group API to `function-service`: smaller initial surface, but groups are not part of the function domain and this would blur service ownership.
- Persist only the group document first and add individual members later: simpler, but it violates the requested atomic write behavior and would make the create API incomplete.
- Put transaction handling in the HTTP handler: direct, but it would mix transport and persistence concerns and violate the backend policy.

## API Contract

Endpoint:

```http
POST /api/v1/workspaces/:workspace_id/groups
```

Path parameters:

- `workspace_id`: workspace that owns the group. It must be non-empty. This API does not validate that the workspace exists.

Request body:

```json
{
  "name": "Design Reviewers",
  "description": "Employees who can review design documents.",
  "grouping_rule": {
    "rules": [
      {
        "attribute_key": "department",
        "operator": "eq",
        "multi": false,
        "value": "ABCD-123"
      },
      {
        "attribute_key": "level",
        "operator": "gte",
        "multi": false,
        "value": 5
      }
    ],
    "expiration_date": "2026-06-01T00:00:00Z"
  },
  "individual_members": [
    {
      "nt_account": "user1",
      "expiration_date": "2026-06-01T00:00:00Z"
    }
  ]
}
```

Success response:

```http
HTTP/1.1 201 Created
```

```json
{
  "group": {
    "id": "0d5c4f7e-7675-4c90-b495-93655c2d3c40",
    "name": "Design Reviewers",
    "description": "Employees who can review design documents.",
    "grouping_rule": {
      "rules": [
        {
          "attribute_key": "department",
          "operator": "eq",
          "multi": false,
          "value": "ABCD-123"
        },
        {
          "attribute_key": "level",
          "operator": "gte",
          "multi": false,
          "value": 5
        }
      ],
      "expiration_date": "2026-06-01T00:00:00Z"
    },
    "individual_members": [
      {
        "nt_account": "user1",
        "expiration_date": "2026-06-01T00:00:00Z"
      }
    ]
  }
}
```

Field contract:

- Public JSON field names use `snake_case`.
- `workspace_id` is taken from the path and stored in MongoDB. It is not included in the create response.
- `name` is trimmed before validation, persistence, uniqueness checks, and response rendering.
- `description` is persisted as provided. Empty descriptions are allowed.
- `grouping_rule.rules` are interpreted as an `AND` relationship.
- `grouping_rule.rules` is limited by `GROUP_SERVICE_MAX_GROUPING_RULES`, defaulting to 10 items.
- `grouping_rule.expiration_date` and `individual_members[].expiration_date` are accepted as RFC3339 timestamp strings and stored as MongoDB Date values.
- `individual_members` is limited by `GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS`, defaulting to 1000 items.
- Responses return timestamp fields as RFC3339 strings through Go's JSON encoding for `time.Time`.
- `created_at`, `updated_at`, `deleted_at`, and `normalized_name` are persistence metadata and are not included in the public response contract.

## Rule Contract

Rules model employee attribute predicates.

Single-value rule:

```json
{
  "attribute_key": "department",
  "operator": "eq",
  "multi": false,
  "value": "ABCD-123"
}
```

Multi-value rule:

```json
{
  "attribute_key": "department",
  "operator": "eq",
  "multi": true,
  "value": ["ABCD-123", "WXYZ-789"]
}
```

Allowed operators:

- `eq`
- `not_eq`
- `gt`
- `gte`
- `lt`
- `lte`

Validation rules:

- `attribute_key` must be non-empty after trimming.
- `operator` must be one of the allowed operators.
- `multi` is required because it determines the expected `value` shape.
- When `multi` is `false`, `value` must not be `null` and must not be an array.
- When `multi` is `true`, `value` must be a non-empty array and no array item may be `null`.
- `value` is otherwise stored as a JSON value without imposing an employee attribute type system in this phase.

## Request Validation

Transport-level validation should reject malformed JSON, invalid timestamp formats, missing required fields, and rule value shape errors that depend on JSON structure.

Domain or service validation should reject:

- Empty `workspace_id`.
- Empty or whitespace-only `name` after trimming.
- Missing `grouping_rule`.
- Missing `grouping_rule.expiration_date`.
- `grouping_rule.expiration_date` that is not later than the service's request processing time.
- Requests where both `grouping_rule.rules` and `individual_members` are empty.
- Requests where `grouping_rule.rules` exceeds the configured `GROUP_SERVICE_MAX_GROUPING_RULES` limit. The initial default limit is 10.
- Invalid rule attributes, operators, or values.
- Requests where `individual_members` exceeds the configured `GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS` limit. The initial default limit is 1000.
- Empty or whitespace-only `individual_members[].nt_account` after trimming.
- Duplicate `individual_members[].nt_account` values in one request after trimming.
- `individual_members[].expiration_date` values that are not later than the service's request processing time.

The implementation should inject the clock and validation limit options at the service boundary so expiration-date and maximum-count validation are deterministic in tests. Limit values come from service configuration, but validation itself remains in the domain layer so non-HTTP callers cannot bypass the same invariants.

## Error Handling

New HTTP APIs should use the shared backend error response shape:

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

Status mapping:

- `400 Bad Request`: malformed JSON, invalid request shape, invalid field values, duplicate request `nt_account` values, configured limit violations, or expiration dates that are not in the future.
- `409 Conflict`: active group name already exists in the same workspace.
- `500 Internal Server Error`: unexpected repository, transaction, or infrastructure failure.

The handler should log unexpected errors with structured keys such as `err` and `workspace_id`, while keeping validation and conflict responses safe for clients.

## Domain Model

The domain package should model the create use case without Echo or MongoDB dependencies.

Primary types:

- `Group`: persisted group aggregate returned after creation.
- `CreateInput`: service input containing `workspace_id`, `name`, `description`, grouping rule, and individual members.
- `GroupingRule`: `rules` plus expiration date.
- `Rule`: employee attribute predicate.
- `IndividualMember`: explicit user membership with expiration date.
- `CreateInput.Validate(now, options...)`: validates the create input with default count limits, optionally overridden through `WithMaxIndividualMembers` and `WithMaxGroupingRules`.

The domain should expose stable errors such as:

- `ErrInvalidInput`
- `ErrDuplicateName`

The service may map repository duplicate-key errors into `ErrDuplicateName` so handlers can return `409 Conflict`.

## MongoDB Schema

Collection: `groups`

Document schema:

```ts
{
  "_id": string,
  "workspace_id": string,
  "name": string,
  "normalized_name": string,
  "description": string,
  "grouping_rule": {
    "rules": [
      {
        "attribute_key": string,
        "operator": "eq" | "not_eq" | "gt" | "gte" | "lt" | "lte",
        "multi": boolean,
        "value": unknown | unknown[]
      }
    ],
    "expiration_date": Date
  },
  "created_at": Date,
  "updated_at": Date,
  "deleted_at": Date | null
}
```

Field notes:

- `_id` is a service-generated UUID.
- `workspace_id` scopes the group to one workspace.
- `name` is the trimmed display name returned to clients.
- `normalized_name` is the trimmed name used for active uniqueness in this phase.
- `description` is client-provided text.
- `grouping_rule.rules` are dynamic membership predicates interpreted as `AND`.
- `created_at` and `updated_at` are set to the same service-generated `now` during creation.
- `deleted_at` is `null` for active groups.

Indexes:

```txt
partial unique { workspace_id: 1, normalized_name: 1 } where deleted_at == null
{ workspace_id: 1, created_at: -1, _id: -1 }
```

Rationale:

- The partial unique index enforces one active group with the same trimmed name per workspace while allowing soft-deleted historical records.
- The support index anticipates a future workspace group list API without changing the create workflow.

Collection: `group_individual_members`

Document schema:

```ts
{
  "_id": string,
  "group_id": string,
  "nt_account": string,
  "expiration_date": Date,
  "created_at": Date,
  "updated_at": Date,
  "deleted_at": Date | null
}
```

Field notes:

- `_id` is a service-generated UUID.
- `group_id` references `groups._id`.
- `nt_account` is trimmed before validation and persistence.
- `expiration_date` is the explicit membership expiration date.
- `created_at` and `updated_at` are set to the same service-generated `now` during creation.
- `deleted_at` is `null` for active individual members.

Indexes:

```txt
partial unique { group_id: 1, nt_account: 1 } where deleted_at == null
{ group_id: 1 }
```

Rationale:

- The partial unique index prevents multiple active member rows for the same `group_id + nt_account`.
- The support index keeps future group membership reads efficient.

## Transaction Workflow

The service should expose a create workflow equivalent to:

1. Receive `CreateInput` from the handler through transport mapping.
2. Validate domain invariants, including configured `grouping_rule.rules` and `individual_members` count limits.
3. Generate `group_id`, individual member IDs, and one `now` timestamp.
4. Build the domain `Group` model and member models.
5. Call the repository create method.
6. Repository starts a MongoDB session and transaction.
7. Repository inserts the `groups` document.
8. Repository inserts all `group_individual_members` documents when the request includes individual members.
9. Repository commits the transaction.
10. Service returns the created domain `Group`.
11. Handler renders `201 Created`.

Failure behavior:

- If any insert fails, the transaction aborts and neither collection should contain partial create data.
- If the `groups` partial unique index rejects the name, repository maps the duplicate key to a service/domain duplicate-name error.
- If the transaction commit fails, handler returns `500` unless the failure is mapped to a known conflict.

MongoDB transactions require MongoDB to run as a replica set. The existing local `docker-compose.yml` already starts MongoDB with `--replSet rs0`, so the design aligns with local development infrastructure.

## Configuration

`group-service` should use environment-based configuration through `viper`, matching existing backend service conventions.

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

## Health and Shutdown

`cmd/group-service/main.go` should reuse `internal/shared/health`.

Liveness endpoint:

```http
GET /health/liveness
```

The first implementation should use the same lightweight `process` indicator pattern as `function-service`.

Rationale:

- This satisfies liveness probe requirements.
- Liveness should answer whether the process can serve at all, not whether MongoDB is temporarily reachable.
- A future readiness endpoint can include MongoDB checks if deployment needs traffic gating.

The service must support graceful shutdown with a timeout-bound context and must disconnect the MongoDB client during shutdown.

## REST Client Examples

The implementation should add:

```txt
examples/api/groups.http
```

The file should include:

- A successful create request.
- A duplicate-name example that returns `409 Conflict`.
- A validation example where both membership sources are empty.
- A validation example for duplicate individual `nt_account` values.
- Variables for `baseUrl` and `workspaceId`.

## Testing Strategy

Domain tests:

- Trimmed group name is required.
- At least one membership source is required.
- Grouping-rule expiration must be in the future.
- `grouping_rule.rules` exceeding the configured limit is rejected.
- `individual_members` exceeding the configured limit is rejected.
- Individual member expiration must be in the future.
- Rule operator validation rejects unsupported operators.
- `multi: false` rejects `null` and array values.
- `multi: true` rejects non-array, empty array, and arrays containing `null`.
- Duplicate individual `nt_account` values are rejected after trimming.

Transport tests:

- Malformed JSON returns a decode error.
- RFC3339 timestamps parse correctly.
- Invalid timestamp strings are rejected.
- Rule JSON value shape is preserved and mapped to the domain.

Service tests:

- Successful create injects deterministic IDs and timestamps.
- Duplicate group name is surfaced as a conflict error.
- Repository failures are wrapped with context.
- Validation failures do not call the repository.

Repository tests:

- `EnsureIndexes` creates the required partial unique and support indexes.
- Successful create writes `groups` and `group_individual_members` in one transaction.
- Insert failure rolls back both collections.
- Duplicate active group name in the same workspace fails.
- Same group name in different workspaces succeeds.
- Duplicate active `group_id + nt_account` fails.

Handler tests:

- Successful create returns `201` and the documented response body.
- Invalid request returns `400`.
- Duplicate active group name returns `409`.
- Unexpected service failure returns `500`.

Config and main tests:

- Missing required `GROUP_SERVICE_*` values fail validation.
- Defaults are applied for optional config values.
- Validation limit environment overrides are loaded.
- Non-positive validation limits fail configuration validation.
- Health route registration includes `/health/liveness`.

Verification commands for implementation:

```bash
go test ./...
```

Additional repository-specific integration tests may require local MongoDB from Docker Compose because transaction behavior depends on replica-set support.

## Architecture Decisions and Trade-Offs

- Independent service boundary: keeps group ownership separate from function permission ownership, at the cost of adding a new service entrypoint and config surface.
- Service-owned transaction boundary: keeps the create use case atomic and testable, while repository code owns driver-specific session mechanics.
- Trimmed-name uniqueness: prevents accidental duplicates caused by surrounding whitespace, while preserving case-sensitive names in this phase.
- `normalized_name` field: gives a stable index key for active name uniqueness and leaves room for future case-insensitive normalization, at the cost of one extra persistence field.
- No workspace existence validation: avoids coupling this service to a workspace registry before that contract exists, but means callers can create groups for any non-empty workspace ID.
- Raw JSON rule values: preserves flexibility before an employee attribute catalog exists, but delays type-specific operator validation until a later phase.
- Lightweight liveness only: avoids restarting healthy processes during temporary MongoDB outages, but does not provide readiness traffic gating yet.

## Implementation Plan Notes

The implementation plan should be created under `docs/plans/active/` and link back to this design document.

The plan should follow test-driven sequencing:

1. Domain model and validation tests.
2. Transport request and response mapping tests.
3. Service create workflow tests.
4. Repository transaction and index tests.
5. Handler route tests.
6. Config, main wiring, health route, and REST Client example updates.
