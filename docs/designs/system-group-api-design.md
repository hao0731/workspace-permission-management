# System Group API Design

## Background

`group-service` currently owns workspace-scoped groups used as permission subjects. System resource APIs now expose a system-scoped boundary through `/api/v1/systems/:system_id/...`, and system-owned groups need the same boundary so a system can define reusable group membership projections.

This design adds system group creation and list APIs to `group-service`. A system group stores its public definition in the `system_groups` collection and stores the permission API relationship projection in `system_group_relationships`.

Related designs:

- [Group Service Design](group-service.md): entry design for `group-service`.
- [Function Service System Resource API Design](function-service-system-resource-api-design.md): existing system-scoped API conventions and `system_id` validation.
- [Permission API Client Design](permission-api-client-design.md): shared permission package that defines relationship helper constructors.

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- API payloads, response payloads, MongoDB documents, indexes, pagination tokens, relationship generation rules, and checksum rules are explicit contracts.
- Handlers remain thin and only parse path, query, and body input, invoke services, and render mapped responses or errors.
- Request and response DTOs belong in `internal/group-service/transport`.
- System group validation and normalized rule models belong in `internal/domain/group` and stay independent of Echo and MongoDB.
- Relationship projection building belongs in `internal/group-service/services`; it may use `internal/shared/interactions/permission/relationship` helper constructors, but domain types must not import the permission package.
- MongoDB access and transactions remain isolated in `internal/group-service/repositories`.
- The `system_groups` collection name follows the requested persistence contract. Go constants should isolate the literal collection name so code identifiers remain idiomatic.
- This design document is stored under `docs/designs/` and linked from the existing `group-service` entry design.

## Goals

- Expose `POST /api/v1/systems/:system_id/groups`.
- Expose `GET /api/v1/systems/:system_id/groups?limit=<LIMIT>&next_token=<TOKEN>`.
- Store system group definitions in MongoDB collection `system_groups`.
- Store system group relationship projections in MongoDB collection `system_group_relationships`.
- Persist `system_id` in both collections.
- Convert accepted `grouping_rules` into permission API `Relationship` values.
- Compute one SHA256 checksum per generated relationship.
- Write generated relationships to the permission API before MongoDB persistence.
- Persist only relationships accepted by the permission API when the permission API returns per-task failures.
- Return `206 Partial Content` with the saved adjusted group and permission API error strings when some relationship write tasks fail.
- Write `system_groups` and `system_group_relationships` atomically in one MongoDB transaction during create.
- Support cursor pagination for system group list with default `limit = 20` and maximum `limit = 50`.
- Return empty pages for systems with no groups.

## Non-Goals

- Do not implement `operator: "not_eq"` behavior in this phase. Requests containing `not_eq` are rejected with `400 validation_failed`.
- Do not validate whether `system_id` references an existing system registry. No registry exists in this repository yet.
- Do not implement system group read-by-ID, update, delete, restore, hard delete, history, or relationship recalculation APIs.
- Do not publish group-created or membership-changed events.
- Do not add frontend changes.

## System ID

`system_id` follows the system-scoped API convention from [Function Service System Resource API Design](function-service-system-resource-api-design.md#system-id-and-function-key-naming):

- Trim surrounding whitespace.
- Reject empty values.
- Reject whitespace.
- Reject `.` because current function or app identifiers are single NATS subject tokens.
- Do not check existence against another registry.

## Recommended Approach

Add a system group subdomain inside `group-service`. The transport layer decodes the system-scoped API payload, the domain layer validates the system group rule contract, the service layer builds deterministic permission relationships and checksums, and the repository writes both MongoDB documents in one transaction.

This keeps the new behavior close to existing group ownership while avoiding cross-service imports. It also separates the stored group definition from the derived relationship projection, so future phases can recalculate relationships without changing the public group response contract.

Alternatives considered:

- Store system groups in the existing `groups` collection: fewer collections, but workspace groups and system groups have different ownership, schema, and relationship side effects.
- Store only relationships and derive list responses from `system_group_relationships`: smaller write path, but the public group definition would be coupled to a projection document.
- Generate relationships in the repository: persistence code would own business translation rules and become harder to test without MongoDB.

## API Contract

### Create System Group

Endpoint:

```http
POST /api/v1/systems/:system_id/groups
Content-Type: application/json
```

Path parameters:

- `system_id`: required system identifier.

Request body:

```json
{
  "name": "System Admins",
  "grouping_rules": [
    {
      "attribute_key": "organization",
      "operator": "eq",
      "multi": true,
      "value": ["ORG-100", "ORG-200"]
    },
    {
      "attribute_key": "job_type",
      "operator": "eq",
      "multi": false,
      "value": "DL"
    },
    {
      "attribute_key": "job_level",
      "operator": "eq",
      "multi": false,
      "value": "M2"
    },
    {
      "attribute_key": "job_tag",
      "operator": "eq",
      "multi": true,
      "value": ["a4_reviewer", "_internal_secretary_"]
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
    "name": "System Admins",
    "grouping_rules": [
      {
        "attribute_key": "organization",
        "operator": "eq",
        "multi": true,
        "value": ["ORG-100", "ORG-200"]
      },
      {
        "attribute_key": "job_type",
        "operator": "eq",
        "multi": false,
        "value": "DL"
      },
      {
        "attribute_key": "job_level",
        "operator": "eq",
        "multi": false,
        "value": "M2"
      },
      {
        "attribute_key": "job_tag",
        "operator": "eq",
        "multi": true,
        "value": ["a4_reviewer", "_internal_secretary_"]
      }
    ],
    "created_at": "2026-05-20T10:00:00Z",
    "updated_at": "2026-05-20T10:00:00Z"
  }
}
```

Partial success response:

```http
HTTP/1.1 206 Partial Content
```

```json
{
  "group": {
    "id": "0d5c4f7e-7675-4c90-b495-93655c2d3c40",
    "name": "System Admins",
    "grouping_rules": [
      {
        "attribute_key": "organization",
        "operator": "eq",
        "multi": true,
        "value": ["ORG-100"]
      }
    ],
    "created_at": "2026-05-20T10:00:00Z",
    "updated_at": "2026-05-20T10:00:00Z"
  },
  "errors": [
    "permission relationship already exists"
  ]
}
```

Create field contract:

- `system_id` is taken from the path, persisted, and not returned in the group object.
- `name` is required, trimmed before validation and persistence, and returned trimmed.
- `grouping_rules` is required and must be an array. An empty array is allowed and means the generated projection falls back to all-employee HR and A4 relationships.
- The response wraps the created group in `{ "group": ... }`.
- When permission API relationship writes partially fail, the response wraps the adjusted saved group in `{ "group": ... }` and includes `errors`.
- `created_at` and `updated_at` are assigned from one service-generated UTC timestamp.
- Timestamps are returned as RFC3339 strings through Go JSON encoding for `time.Time`.
- The endpoint does not check whether the system exists in another registry.

### List System Groups

Endpoint:

```http
GET /api/v1/systems/:system_id/groups?limit=<LIMIT>&next_token=<TOKEN>
```

Path parameters:

- `system_id`: required system identifier.

Query string:

- `limit`: optional integer. Default `20`, maximum `50`.
- `next_token`: optional opaque cursor token.

Success response:

```http
HTTP/1.1 200 OK
```

```json
{
  "groups": [
    {
      "id": "0d5c4f7e-7675-4c90-b495-93655c2d3c40",
      "name": "System Admins",
      "grouping_rules": [
        {
          "attribute_key": "organization",
          "operator": "eq",
          "multi": true,
          "value": ["ORG-100", "ORG-200"]
        }
      ],
      "created_at": "2026-05-20T10:00:00Z",
      "updated_at": "2026-05-20T10:00:00Z"
    }
  ],
  "page_info": {
    "has_next_page": true,
    "next_token": "<opaque-token>"
  }
}
```

Empty response:

```http
HTTP/1.1 200 OK
```

```json
{
  "groups": [],
  "page_info": {
    "has_next_page": false,
    "next_token": ""
  }
}
```

List behavior:

- Query `system_groups` directly by `system_id`.
- Sort by `created_at DESC, _id DESC`.
- Fetch `limit + 1` documents to determine `has_next_page`.
- `next_token` is a base64url-encoded JSON cursor containing `created_at` and `id`, matching the existing shared pagination pattern.
- `page_info.next_token` is empty when `has_next_page` is `false`.
- The endpoint does not read `system_group_relationships`; relationship projection is not part of the public list response.

## Rule Contract

`grouping_rules` supports exactly four rule shapes in this phase:

```ts
type OrgRule = {
  attribute_key: "organization";
  operator: "eq";
  multi: true;
  value: string[];
}

type JobLevelRule = {
  attribute_key: "job_level";
  operator: "eq";
  multi: false;
  value: string;
}

type JobTypeRule = {
  attribute_key: "job_type";
  operator: "eq";
  multi: false;
  value: "DL" | "IDL" | "ALL";
}

type JobTagRule = {
  attribute_key: "job_tag";
  operator: "eq";
  multi: true;
  value: string[];
}

type Rule = OrgRule | JobLevelRule | JobTypeRule | JobTagRule;
```

Validation rules:

- `attribute_key` must be one of `organization`, `job_level`, `job_type`, or `job_tag`.
- `operator` must be `eq`.
- `operator: "not_eq"` returns `400 validation_failed` in this phase.
- `multi` must match the rule shape:
  - `organization`: `true`
  - `job_level`: `false`
  - `job_type`: `false`
  - `job_tag`: `true`
- Multi-value rules must use an array value.
- Single-value rules must use a string value.
- `job_type` rule values must be exactly one of `DL`, `IDL`, or `ALL`.
- String values are trimmed for validation and relationship generation.
- Empty string values are invalid.
- Empty arrays are allowed for `organization` and `job_tag` because the relationship generation rules define fallback behavior after deduplication.
- Only one `job_type` rule is allowed.
- Multiple `organization`, `job_level`, and `job_tag` rules are allowed.
- Unknown fields are ignored by JSON decoding unless the repository later standardizes strict decoding for all group-service endpoints.

The `_internal_secretary_` value is the only special sentinel. It is recognized only in `job_tag` values. When present, it is excluded from A4 role relationship generation, causes the static attributes relationship to be generated, and sets `WithIsContainSecretary(true)` on that static attributes relationship.

## Relationship Generation

The service converts accepted `grouping_rules` into `internal/shared/interactions/permission/relationship.Relationship` values before persistence.

Generation inputs:

- `groupID`: the service-generated system group ID.
- `organizationIDs`: deduped values from every `organization` rule.
- `jobType`: optional value from the single `job_type` rule.
- `jobLevels`: deduped values from every `job_level` rule.
- `jobTags`: deduped values from every `job_tag` rule.
- `containsSecretary`: `true` when any `job_tag` value equals `_internal_secretary_`.

Normalization:

- Trim all string values before deduplication.
- Reject empty strings before generation.
- Deduplicate values by exact string after trimming.
- Sort deduped values lexicographically before relationship generation so the projection is deterministic.
- Preserve the original `grouping_rules` order and values in the public group document; sorting applies only to derived relationships.

Generation order:

1. HR relationships.
2. Static attributes relationship, when applicable.
3. A4 relationships.

HR relationships:

- If `organizationIDs` is non-empty, generate one relationship per organization ID with `NewOrganizationToGroupRelationship(groupID, organizationID)`.
- If no `organization` rule exists or `organizationIDs` is empty after deduplication, generate one relationship with the existing helper `NewAllEmployeeToGroupForHRRelationship(groupID)`.
- The requirement phrase `NewAllEmployeeToGroupFromHRRelationship` maps to the existing code helper named `NewAllEmployeeToGroupForHRRelationship`.

Static attributes relationship:

- Generate `NewGroupWithStaticAttributesRelationship(groupID, options...)` when any of these is true:
  - a `job_type` rule exists,
  - `jobLevels` is non-empty,
  - `containsSecretary` is `true`.
- If a `job_type` rule exists, pass `WithAllowedTypes([]string{jobType})`.
- If `jobLevels` is non-empty, pass `WithAllowedLevels(jobLevels)`.
- If `containsSecretary` is `true`, pass `WithIsContainSecretary(true)`.
- If no job type exists, no job level exists, and no job tag value is `_internal_secretary_`, do not generate the static attributes relationship.

A4 relationships:

- Build `a4Roles` from deduped `job_tag` values after excluding `_internal_secretary_`.
- If `a4Roles` is non-empty, generate one relationship per role with `NewA4RoleToGroupRelationship(groupID, a4Role)`.
- If no `job_tag` rule exists or `a4Roles` is empty after excluding `_internal_secretary_`, generate one relationship with `NewAllEmployeeToGroupForA4Relationship(groupID)`.

Example:

```json
[
  {
    "attribute_key": "organization",
    "operator": "eq",
    "multi": true,
    "value": ["ORG-200", "ORG-100", "ORG-100"]
  },
  {
    "attribute_key": "job_level",
    "operator": "eq",
    "multi": false,
    "value": "M2"
  },
  {
    "attribute_key": "job_tag",
    "operator": "eq",
    "multi": true,
    "value": ["a4_reviewer", "_internal_secretary_"]
  }
]
```

Generates:

- `NewOrganizationToGroupRelationship(groupID, "ORG-100")`
- `NewOrganizationToGroupRelationship(groupID, "ORG-200")`
- `NewGroupWithStaticAttributesRelationship(groupID, WithAllowedLevels([]string{"M2"}), WithIsContainSecretary(true))`
- `NewA4RoleToGroupRelationship(groupID, "a4_reviewer")`

## Relationship Checksum

Each generated relationship is persisted with a checksum:

```ts
type RelationshipInfo = {
  relationship: Relationship;
  checksum: string;
}
```

Checksum contract:

- Marshal the fully built `Relationship` object to JSON using Go `encoding/json`.
- Do not add extra whitespace.
- Do not include the outer `RelationshipInfo` wrapper.
- Compute SHA256 over the JSON bytes.
- Store the checksum as a lowercase hex string.
- If marshaling fails, the create request fails before starting the MongoDB transaction.
- Failed permission API tasks are matched back to the original generated projection by recomputing this checksum over the failed relationship payload.

The relationship helper structs use deterministic struct field order. Derived value sorting keeps logically equivalent rule sets stable even when duplicate values appear in different request positions.

## Persistence

### `system_groups` Collection

Document schema:

```ts
{
  "_id": string,
  "system_id": string,
  "name": string,
  "grouping_rules": Rule[],
  "created_at": Date,
  "updated_at": Date
}
```

Field notes:

- `_id` is a service-generated UUID.
- `system_id` is copied from the path after validation.
- `name` is the trimmed display name.
- `grouping_rules` stores the accepted public rule representation.
- `created_at` and `updated_at` are set to the same service-generated UTC timestamp during creation.
- There is no soft-delete field in this phase because no delete API is part of this design.

Indexes:

```txt
{ system_id: 1, created_at: -1, _id: -1 }
```

Rationale:

- The support index matches the list endpoint filter and sort.
- No group-name uniqueness constraint is defined in this phase because the requested contract does not specify duplicate-name behavior.

### `system_group_relationships` Collection

Document schema:

```ts
{
  "system_id": string,
  "group_id": string,
  "relationship": RelationshipInfo[],
  "created_at": Date,
  "updated_at": Date
}
```

Field notes:

- `system_id` is copied from the path after validation.
- `group_id` matches `system_groups._id`.
- `relationship` stores one item per generated permission relationship.
- `relationship[].relationship` stores the permission API `Relationship` object.
- `relationship[].checksum` stores the relationship checksum.
- `created_at` and `updated_at` are set to the same service-generated UTC timestamp during creation.

Indexes:

```txt
unique { system_id: 1, group_id: 1 }
```

Rationale:

- The unique index enforces one relationship projection document per system group.
- `system_id + group_id` keeps the relationship projection scoped to the owning system even though `group_id` is globally generated.

## Service Workflow

### Create

1. Handler extracts `system_id`, decodes the request body, and maps it to a domain create input.
2. Service normalizes and validates `system_id`, `name`, and `grouping_rules`.
3. Service generates `group_id` and one UTC `now` timestamp.
4. Service builds the system group domain model.
5. Service builds the deterministic relationship projection from `grouping_rules`.
6. Service computes one checksum per relationship.
7. Service sends one `create` task per generated relationship to `WriteRelationships`.
8. If the permission API returns no failed tasks, service keeps the original group and projection unchanged.
9. If the permission API returns failed tasks, service logs a warning with `system_id`, `group_id`, failed count, and error strings.
10. If the permission API returns failed tasks, service recomputes failed relationship checksums, removes matching relationships from the original projection, and rebuilds the saved group from the remaining relationships.
11. Repository starts a MongoDB session and executes the write callback through `session.WithTransaction`.
12. Repository inserts the `system_groups` document.
13. Repository inserts the matching `system_group_relationships` document.
14. MongoDB commits the transaction.
15. Service returns the created group plus permission API error strings.
16. Handler renders `201 Created` with `{ "group": ... }` when no permission task failed.
17. Handler renders `206 Partial Content` with `{ "group": ..., "errors": [...] }` when at least one permission task failed.

Failure behavior:

- Validation errors return `400 Bad Request` with `validation_failed`.
- If relationship generation or checksum computation fails, no MongoDB transaction is started.
- If the permission API request fails as a whole, no MongoDB transaction is started and the handler returns an upstream dependency error.
- If either insert fails, the transaction is aborted and neither collection contains partial create data.
- Unexpected repository, transaction, or infrastructure failures return `500 Internal Server Error`.

### Relationship-to-Group Rebuild

Partial permission API failures require rebuilding the saved public group from the accepted relationship projection. The rebuild uses the inverse of the existing generation rules:

- Organization relationships become one `organization` multi rule containing sorted organization IDs.
- A static attributes relationship contributes:
  - one `job_type` rule when `allowed_types` contains a value,
  - one `job_level` rule per sorted `allowed_levels` value,
  - `_internal_secretary_` in the `job_tag` rule when `is_contain_secretary` is `true`.
- A4 role relationships become `job_tag` values.
- HR and A4 all-employee fallback relationships do not create public rules.

The relationship projection remains the accurate permission state when all relationships of a fallback category fail. A rebuilt group with no public rules therefore does not imply that fallback relationships were saved; callers should use the `206` errors to detect partial permission state.

### List

1. Handler extracts `system_id`, parses `limit`, and parses `next_token`.
2. Service validates `system_id`, `limit`, and cursor shape.
3. Repository queries `system_groups` by `system_id`.
4. Repository applies the cursor predicate when `next_token` is present.
5. Repository sorts by `created_at DESC, _id DESC` and fetches `limit + 1`.
6. Service returns the requested page and next cursor.
7. Handler renders `200 OK` with `groups` and `page_info`.

Cursor predicate:

```txt
created_at < cursor.created_at
OR (created_at == cursor.created_at AND _id < cursor.id)
```

## Error Handling

Known errors use the shared backend error response shape:

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

- `400 Bad Request`: malformed JSON, invalid path identity, invalid query parameter, invalid `next_token`, unsupported operator, invalid rule shape, empty `name`, empty string values, invalid `job_type` value, or more than one `job_type` rule.
- `206 Partial Content`: system group was saved after one or more permission relationship write tasks failed; response includes `errors`.
- `502 Bad Gateway`: permission API request-level failure before local persistence.
- `500 Internal Server Error`: unexpected repository, transaction, checksum, or infrastructure failure.

Recommended error codes:

- `validation_failed`
- `permission_write_failed`
- `internal_error`

The handler should log unexpected errors with structured keys such as `err`, `system_id`, and `group_id`. Validation errors should remain safe for clients and must not leak generated relationship internals.

## Domain and Service Structure

Expected additions:

```txt
internal/domain/group/
  system_group.go
  system_group_validation.go

internal/group-service/transport/
  system_group_request.go
  system_group_response.go
  system_group_pagination.go

internal/group-service/services/
  system_group_service.go
  system_group_relationship_builder.go

internal/group-service/repositories/
  mongo_system_group_repository.go
```

Responsibilities:

- `internal/domain/group`: system group model, create input, list query, cursor, rule constants, normalization, validation, and stable domain errors.
- `internal/group-service/transport`: request/response DTOs, JSON rule decoding, pagination token encode/decode, and DTO/domain mapping.
- `internal/group-service/services`: use cases, ID generation, clock usage, relationship projection building, checksum computation, and repository orchestration.
- `internal/group-service/repositories`: MongoDB documents, indexes, list query, cursor predicate, transactions, and document/domain mapping.

The existing `cmd/group-service/main.go` remains the composition root. It should construct the system group service, register the new routes, and ensure the new repository indexes during startup.

## REST Client Examples

The implementation should add `examples/api/system-groups.http` for this endpoint family. It should include:

- Successful system group creation with organization, job type, job level, and job tag rules.
- Successful creation with empty `grouping_rules`.
- Validation failure for `operator: "not_eq"`.
- Validation failure for more than one `job_type` rule.
- Validation failure for invalid `limit`.
- Validation failure for invalid `next_token`.
- Paginated system group list using `limit` and `next_token`.
- Empty system group list.

## Testing Strategy

Domain tests:

- `system_id` validation follows the system-scoped API convention.
- Trimmed `name` is required.
- `grouping_rules` is required and accepts an empty array.
- `not_eq` is rejected in this phase.
- Unknown `attribute_key` is rejected.
- `multi` must match the attribute key.
- Rule value JSON shape must match `multi`.
- Empty string values are rejected.
- `job_type` values other than `DL`, `IDL`, or `ALL` are rejected.
- Multiple `job_type` rules are rejected.
- Multiple `organization`, `job_level`, and `job_tag` rules are accepted.

Transport tests:

- Create request decodes each allowed rule shape.
- Invalid `multi` and value shape return validation errors.
- Create response renders `{ "group": ... }`.
- List query defaults `limit` to `20`.
- List query rejects `limit > 50`.
- System group next token encodes and decodes `created_at` and `id`.
- Invalid next tokens map to `400 validation_failed`.

Service tests:

- Successful create generates deterministic group ID and timestamps.
- Successful create sends generated relationships to the permission API as `create` tasks before repository persistence.
- Permission API request-level failure returns a permission write failure and does not call the repository.
- Permission API failed tasks remove matching relationships by checksum before repository persistence.
- Permission API failed tasks rebuild the saved group from the remaining relationships.
- Permission API failed tasks produce warning logs with `system_id`, `group_id`, failed count, and errors.
- Successful create builds HR organization relationships from deduped organization values.
- Create without organization values builds `NewAllEmployeeToGroupForHRRelationship`.
- Create with any job type rule, any job level rule, or a job tag value of `_internal_secretary_` builds `NewGroupWithStaticAttributesRelationship`.
- Create with a job type passes `WithAllowedTypes`.
- Create with multiple job level rules passes deduped `WithAllowedLevels`.
- Create with `_internal_secretary_` in job tags passes `WithIsContainSecretary(true)` and excludes the sentinel from A4 roles.
- Create with only non-secretary job tags does not build `NewGroupWithStaticAttributesRelationship`.
- Create with non-secretary job tags builds one `NewA4RoleToGroupRelationship` per deduped tag.
- Create without non-secretary job tags builds `NewAllEmployeeToGroupForA4Relationship`.
- Relationship generation is deterministic for duplicate values in different input order.
- Checksum computation hashes the JSON relationship object and returns lowercase hex.
- Validation failures do not call the repository.
- Repository failures are wrapped with context.
- List validates input before calling the repository.

Repository tests:

- `EnsureIndexes` creates the system group list index and relationship unique index.
- Successful create writes `system_groups` and `system_group_relationships` in one transaction.
- Insert failure for either collection rolls back both writes.
- List filters by `system_id`.
- List sorts by `created_at DESC, _id DESC`.
- List applies cursor predicates correctly.
- List returns `limit + 1` behavior as `has_next_page`.

Handler tests:

- Successful create returns `201` and the documented response body.
- Partial permission relationship write failure returns `206` with `group` and `errors`.
- Permission API request-level failure returns `502`.
- Successful list returns `200` with `groups` and `page_info`.
- Empty list returns `200` with an empty group array and empty `next_token`.
- Invalid create requests return `400`.
- Invalid list query requests return `400`.
- Unexpected service failures return `500`.

Verification commands for implementation:

```bash
go test ./internal/domain/group ./internal/group-service/... ./cmd/group-service
go test ./...
```

## Architecture Decisions

1. Keep system groups in `group-service`.
   - Rationale: Groups are permission subjects and the existing service already owns group validation, MongoDB transactions, and group HTTP boundaries.
   - Trade-off: `group-service` now has both workspace-scoped and system-scoped group APIs, so route and domain names must stay explicit.

2. Store group definitions and relationship projections separately.
   - Rationale: Public group data and permission relationship data change for different reasons.
   - Trade-off: Create requires a multi-collection transaction.

3. Generate relationships in the service layer.
   - Rationale: Relationship construction is use-case behavior and should be testable without MongoDB.
   - Trade-off: The service layer depends on the shared permission API relationship helper package.

4. Reject `not_eq` in this phase.
   - Rationale: Accepting unsupported rules would make clients believe they are enforced.
   - Trade-off: The public type includes `not_eq`, but first-phase requests using it fail until the next design implements the behavior.

5. Sort deduped values before projection generation.
   - Rationale: Stable relationship order makes persisted projections, checksums, and tests deterministic.
   - Trade-off: Relationship projection order may differ from request value order, while the public `grouping_rules` field still preserves the request representation.
