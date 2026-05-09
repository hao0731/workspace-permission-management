# Group API Design

## Background

This document defines group-level APIs for `group-service`. It covers creating groups, reading one group, soft-deleting one group, and replacing the group's dynamic grouping rules.

Entry point and shared service concerns are documented in [Group Service Design](group-service.md). Individual member reads and the member collection are documented in [Group Individual Members API Design](group-service-individual-members.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- Group API payloads, response payloads, MongoDB documents, indexes, and transaction behavior are explicit contracts.
- Handlers parse transport input and map errors; services own use-case behavior; repositories own MongoDB details.
- Request and response DTOs belong in `internal/group-service/transport`.
- Domain validation stays in `internal/domain/group` without Echo or MongoDB dependencies.

## Goals

- Expose `POST /api/v1/workspaces/:workspace_id/groups`.
- Expose `GET /api/v1/workspaces/:workspace_id/groups/:group_id`.
- Expose `DELETE /api/v1/workspaces/:workspace_id/groups/:group_id`.
- Expose `PUT /api/v1/workspaces/:workspace_id/groups/:group_id/grouping-rules`.
- Persist group definitions in MongoDB collection `groups`.
- Enforce active group name uniqueness within one workspace.
- Allow the same group name in different workspaces.
- Trim group names before validation, persistence, uniqueness checks, and response rendering.
- Treat grouping rules as an `AND` relationship.
- Require all grouping-rule expiration dates to be later than request processing time.
- Preserve the create invariant that a group must have at least one membership source: dynamic grouping rules or active individual members.
- Soft-delete groups and their active individual members using `deleted_at`.

## Non-Goals

- Do not validate whether `workspace_id` references an existing workspace.
- Do not materialize or evaluate group membership from employee attributes.
- Do not define an employee attribute catalog or type system.
- Do not implement group list, group name update, description update, hard delete, restore, or history APIs.
- Do not implement individual member add, replace, or delete APIs in this phase.
- Do not publish group-created, group-updated, group-deleted, or membership-changed events.
- Do not implement NATS or JetStream integration for `group-service` in this phase.
- Do not add frontend changes.

## Create Group API

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

Create field contract:

- `workspace_id` is stored in MongoDB and is not returned in the response.
- `name` is trimmed before validation, persistence, uniqueness checks, and response rendering.
- `description` is persisted as provided. Empty descriptions are allowed.
- `grouping_rule.rules` and `individual_members` may each be empty, but not both.
- `grouping_rule.rules` is limited by `GROUP_SERVICE_MAX_GROUPING_RULES`, defaulting to 10 items.
- `individual_members` is limited by `GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS`, defaulting to 1000 items.
- `created_at`, `updated_at`, `deleted_at`, and `normalized_name` are persistence metadata and are not included in the public response contract.

## Get Group API

Endpoint:

```http
GET /api/v1/workspaces/:workspace_id/groups/:group_id
```

Path parameters:

- `workspace_id`: workspace scope for the group.
- `group_id`: group identifier.

Behavior:

- Read only active group documents where `deleted_at` is `null`.
- Filter by both `workspace_id` and `group_id`.
- Do not validate whether the workspace exists.
- Do not include individual members in this response. Clients should use the paginated individual members endpoint for member details.

Success response when the active group exists:

```http
HTTP/1.1 200 OK
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
    }
  }
}
```

Success response when no active group exists for `workspace_id + group_id`:

```http
HTTP/1.1 200 OK
```

```json
{
  "group": null
}
```

GET field contract:

- `group: null` means no active group document exists for the supplied path identity.
- `group: null` does not mean the workspace was checked and found missing.
- Soft-deleted groups are treated as missing.

## Delete Group API

Endpoint:

```http
DELETE /api/v1/workspaces/:workspace_id/groups/:group_id
```

Path parameters:

- `workspace_id`: workspace scope for the group.
- `group_id`: group identifier.

Behavior:

- Delete is idempotent.
- If an active group exists for `workspace_id + group_id`, soft-delete the group and all active individual members for the group in one MongoDB transaction.
- If no active group exists, return `204 No Content` without modifying documents.
- Soft-delete updates set `deleted_at` and `updated_at` to the same service-generated `now` timestamp.
- Do not publish events in this phase.

Success response:

```http
HTTP/1.1 204 No Content
```

Failure behavior:

- `400 Bad Request`: invalid or empty path identity.
- `500 Internal Server Error`: unexpected repository, transaction, or infrastructure failure.

## Replace Grouping Rules API

Endpoint:

```http
PUT /api/v1/workspaces/:workspace_id/groups/:group_id/grouping-rules
```

Path parameters:

- `workspace_id`: workspace scope for the group.
- `group_id`: group identifier.

Request body:

```json
{
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
}
```

Success response:

```http
HTTP/1.1 204 No Content
```

Behavior:

- Replace the active group's entire `grouping_rule` value.
- Set the group document `updated_at` to the service-generated `now`.
- Return `404 Not Found` when no active group exists for `workspace_id + group_id`.
- `rules` may be empty only when the group still has at least one active individual member. This preserves the create invariant that a group has at least one membership source.
- When `rules` is empty and the group has no active individual members, return `400 Bad Request` with `validation_failed`.
- Do not return the updated group body. Clients can call GET group after a successful update if they need the current representation.

## Rule Contract

Rules model employee attribute predicates. All grouping rules in one group are interpreted as an `AND` relationship.

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
- Empty `group_id` for GET, DELETE, and PUT.
- Empty or whitespace-only `name` after trimming during create.
- Missing `grouping_rule` during create.
- Missing grouping-rule `expiration_date`.
- Grouping-rule `expiration_date` that is not later than the service's request processing time.
- Create requests where both `grouping_rule.rules` and `individual_members` are empty.
- PUT grouping-rules requests where `rules` is empty and the group has no active individual members.
- Requests where `grouping_rule.rules` or PUT `rules` exceeds the configured `GROUP_SERVICE_MAX_GROUPING_RULES` limit.
- Invalid rule attributes, operators, or values.
- Create requests where `individual_members` exceeds the configured `GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS` limit.
- Empty or whitespace-only `individual_members[].nt_account` after trimming during create.
- Duplicate `individual_members[].nt_account` values in one create request after trimming.
- `individual_members[].expiration_date` values that are not later than the service's request processing time.

The implementation should inject the clock and validation limit options at the service boundary so expiration-date and maximum-count validation are deterministic in tests.

## Error Handling

Status mapping:

- `400 Bad Request`: malformed JSON, invalid request shape, invalid path identity, invalid field values, configured limit violations, duplicate request `nt_account` values, expiration dates that are not in the future, or empty PUT `rules` when no active individual members remain.
- `404 Not Found`: PUT grouping-rules target group does not exist as an active group.
- `409 Conflict`: active group name already exists in the same workspace during create.
- `500 Internal Server Error`: unexpected repository, transaction, or infrastructure failure.

The handler should log unexpected errors with structured keys such as `err`, `workspace_id`, and `group_id`, while keeping validation, not-found, and conflict responses safe for clients.

Recommended error codes:

- `validation_failed`
- `not_found`
- `conflict`
- `internal_error`

## Domain Model

The domain package should model group use cases without Echo or MongoDB dependencies.

Primary types:

- `Group`: persisted group aggregate.
- `CreateInput`: create input containing `workspace_id`, `name`, `description`, grouping rule, and individual members.
- `GetQuery`: read identity containing `workspace_id` and `group_id`.
- `DeleteInput`: delete identity containing `workspace_id` and `group_id`.
- `UpdateGroupingRuleInput`: replacement input containing `workspace_id`, `group_id`, `rules`, and `expiration_date`.
- `GroupingRule`: `rules` plus expiration date.
- `Rule`: employee attribute predicate.
- `IndividualMember`: explicit user membership with expiration date.

Validation methods should support:

- create input validation with `WithMaxIndividualMembers` and `WithMaxGroupingRules`.
- grouping-rule replacement validation with `WithMaxGroupingRules`.
- path identity validation for get, delete, and update use cases.

The domain should expose stable errors such as:

- `ErrInvalidInput`
- `ErrDuplicateName`
- `ErrNotFound`

The service may map repository duplicate-key errors into `ErrDuplicateName` and missing active group updates into `ErrNotFound`.

## Groups Collection

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
- `updated_at` changes on grouping-rule replacement and group soft delete.
- `deleted_at` is `null` for active groups.

Indexes:

```txt
partial unique { workspace_id: 1, normalized_name: 1 } where deleted_at == null
{ workspace_id: 1, created_at: -1, _id: -1 }
```

Rationale:

- The partial unique index enforces one active group with the same trimmed name per workspace while allowing soft-deleted historical records.
- The support index anticipates a future workspace group list API without changing current workflows.
- Direct group reads and deletes include `_id`, so MongoDB's `_id` index supports lookup while the additional `workspace_id` filter protects workspace scope.

## Service Workflows

### Create

1. Handler decodes the request and maps it to `group.CreateInput`.
2. Service validates domain invariants, including configured grouping-rule and individual-member count limits.
3. Service generates `group_id`, individual member IDs, and one `now` timestamp.
4. Service builds the domain `Group` model and member models.
5. Repository starts a MongoDB session and executes the write callback through `session.WithTransaction`.
6. Repository inserts the `groups` document.
7. Repository inserts all `group_individual_members` documents when the request includes individual members.
8. MongoDB driver commits the transaction and retries transient transaction errors according to `WithTransaction` behavior.
9. Service returns the created domain `Group`.
10. Handler renders `201 Created`.

Failure behavior:

- If any insert fails, the transaction is aborted and neither collection should contain partial create data.
- If the `groups` partial unique index rejects the name, repository maps the duplicate key to a service/domain duplicate-name error.

### Get

1. Handler extracts `workspace_id` and `group_id`.
2. Service validates path identity.
3. Repository reads one active group by `_id`, `workspace_id`, and `deleted_at: null`.
4. Service returns an optional group result.
5. Handler renders `200 OK` with either a group object or `group: null`.

### Delete

1. Handler extracts `workspace_id` and `group_id`.
2. Service validates path identity and generates one `now` timestamp.
3. Repository starts a MongoDB transaction.
4. Repository soft-deletes the active group matching `_id`, `workspace_id`, and `deleted_at: null`.
5. If no active group matched, repository makes no member changes and reports idempotent success.
6. If the group matched, repository soft-deletes active `group_individual_members` documents for the group.
7. Handler renders `204 No Content`.

### Replace Grouping Rules

1. Handler extracts `workspace_id` and `group_id`, decodes the request, and maps it to `group.UpdateGroupingRuleInput`.
2. Service validates path identity, timestamp, rule shape, and configured maximum rule count.
3. Repository starts a MongoDB transaction because validation may depend on current active individual member state.
4. Repository confirms the active group exists for `_id`, `workspace_id`, and `deleted_at: null`.
5. If the group is missing, service returns `ErrNotFound` and the handler renders `404 Not Found`.
6. If the replacement `rules` array is empty, repository counts active individual members for the group.
7. If the replacement `rules` array is empty and active member count is zero, service returns `ErrInvalidInput` and the handler renders `400 Bad Request`.
8. Repository updates the group document's `grouping_rule` and `updated_at`.
9. Handler renders `204 No Content`.

## REST Client Examples

`examples/api/groups.http` should include:

- Successful create with grouping rules and individual members.
- Duplicate active group name returning `409`.
- Missing membership source returning `400`.
- Duplicate individual `nt_account` returning `400`.
- Invalid multi-value rule returning `400`.
- Successful GET group.
- GET group returning `group: null`.
- Idempotent DELETE group.
- Successful PUT grouping-rules.
- PUT grouping-rules returning `404` for a missing active group.
- PUT grouping-rules returning `400` when `rules` is empty and the group has no active individual members.

## Testing Strategy

Domain tests:

- Trimmed group name is required.
- At least one membership source is required during create.
- Grouping-rule expiration must be in the future.
- Grouping-rule count limits are enforced for create and PUT.
- Individual member count limits are enforced for create.
- Rule operator validation rejects unsupported operators.
- `multi: false` rejects `null` and array values.
- `multi: true` rejects non-array, empty array, and arrays containing `null`.
- Duplicate individual `nt_account` values are rejected after trimming.
- PUT grouping-rules rejects empty `rules` when no active individual members remain.

Transport tests:

- Malformed JSON returns a decode error.
- RFC3339 timestamps parse correctly.
- Invalid timestamp strings are rejected.
- Rule JSON value shape is preserved and mapped to the domain.
- GET, DELETE, and PUT path parameters map to domain identity inputs.

Service tests:

- Successful create injects deterministic IDs and timestamps.
- Duplicate group name is surfaced as a conflict error.
- Repository failures are wrapped with context.
- Validation failures do not call the repository.
- GET returns a present group and an empty optional result.
- DELETE treats missing groups as successful idempotent deletes.
- DELETE passes one timestamp to group and member soft-delete persistence.
- PUT grouping-rules returns not found when the active group is missing.
- PUT grouping-rules validates empty rules against active member count.

Repository tests:

- `EnsureIndexes` creates the required partial unique and support indexes.
- Successful create writes `groups` and `group_individual_members` in one transaction.
- Insert failure rolls back both collections.
- Duplicate active group name in the same workspace fails.
- Same group name in different workspaces succeeds.
- GET filters by `_id`, `workspace_id`, and `deleted_at: null`.
- DELETE soft-deletes the group and active individual members in one transaction.
- DELETE missing group makes no member changes and succeeds.
- PUT grouping-rules updates only an active group in the requested workspace.

Handler tests:

- Successful create returns `201` and the documented response body.
- Successful GET returns `200` with the documented group response.
- GET missing group returns `200` with `group: null`.
- Successful DELETE returns `204`.
- DELETE missing group returns `204`.
- Successful PUT grouping-rules returns `204`.
- PUT missing group returns `404`.
- Invalid requests return `400`.
- Duplicate active group name returns `409`.
- Unexpected service failure returns `500`.
