# Function Resource Permissions Design

## Background

The workspace permission management system uses function resources, resource tags, groups, and actions to express ABAC permission rules. `function-service` already owns the `function_resources` projection used by permission targeting. This design adds a write API that stores the complete permission configuration for one workspace/function pair.

Related context:

- [Concept Model](../concept.md)
- [Function Service Design](function-service.md)

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- The API payload, response payload, and MongoDB document are explicit contracts.
- HTTP handlers remain thin and only parse path/body input, invoke the service, and render responses or mapped errors.
- Request and response DTOs belong in `internal/function-service/transport`.
- Domain permission types and invariants stay independent of Echo and MongoDB.
- The service owns the replace workflow, `rule_id` generation, and deterministic test seams for ID generation.
- MongoDB access remains isolated in `internal/function-service/repositories`.
- This design document is stored under `docs/designs/` and linked from the existing function-service design.

## Goals

- Expose `PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions`.
- Store one complete permission configuration per `workspace_id + function_key`.
- Persist the permission configuration in MongoDB collection `function_resource_permissions`.
- Generate `rule_id` values for request extra rules that omit `rule_id`.
- Persist every extra rule with a `rule_id`.
- Persist baseline rule `enabled` values supplied by the request.
- Reject requests that contain duplicate request-provided `rule_id` values.
- Deduplicate semantically identical extra rules before persistence.
- Return `200` with the persisted permission configuration after a successful write.
- Preserve an existing MongoDB document `_id` when replacing the permission configuration for the same `workspace_id + function_key`.
- Keep validation, service workflow, persistence, and transport mapping aligned with existing `function-service` boundaries.

## Non-Goals

- Do not implement permission evaluation.
- Do not validate whether `workspace_id`, `function_key`, `group_ids`, `action_id`, or `resource_tags` reference existing records in other services.
- Do not implement partial updates, rule-level PATCH, rule deletion APIs, or rule history.
- Do not publish permission-changed events in this phase.
- Do not add frontend changes.
- Do not migrate or rewrite the existing `function_resources` projection.

## Recommended Approach

Use a single-document replace design. `workspace_id + function_key` uniquely identifies the permission configuration for a function inside a workspace. `PUT` creates the document when it does not exist and replaces the permission body when it does exist. Existing documents keep their `_id`; new documents receive a generated UUID `_id`.

This approach matches HTTP `PUT` semantics, keeps client behavior simple, and avoids introducing rule-level merge behavior before there is a clear product need for partial edits.

Alternatives considered:

- Rule-level merge: preserves unspecified rules, but conflicts with full-replacement `PUT` semantics and would need a separate delete or tombstone model.
- Versioned permission documents: enables history and audit, but requires current-version lookup, retention policy, and version conflict behavior that are outside this phase.
- Client-generated required `rule_id`: makes writes more deterministic for clients, but the requested API allows missing `rule_id`; backend generation keeps the request ergonomic.

## API Contract

Endpoint:

```http
PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions
```

Path parameters:

- `workspace_id`: workspace that owns the permission configuration.
- `function_key`: function whose resource permissions are being configured.

Request body:

```json
{
  "office_permission": {
    "baseline_rule": {
      "action_id": "view",
      "resource_tags": ["section_1"],
      "enabled": true
    },
    "extra_rules": [
      {
        "rule_id": "rule-1",
        "group_ids": ["group-1"],
        "action_id": "edit",
        "resource_tags": ["section_1"],
        "expiration_date": "2026-06-01T00:00:00Z"
      },
      {
        "group_ids": ["group-2"],
        "action_id": "delete",
        "resource_tags": ["section_2"],
        "expiration_date": "2026-07-01T00:00:00Z"
      }
    ]
  },
  "remote_permission": {
    "baseline_rule": {
      "action_id": "view",
      "resource_tags": ["remote"],
      "enabled": false
    },
    "extra_rules": []
  }
}
```

Success response:

```http
HTTP/1.1 200 OK
```

```json
{
  "permissions": {
    "office_permission": {
      "baseline_rule": {
        "action_id": "view",
        "resource_tags": ["section_1"],
        "enabled": true
      },
      "extra_rules": [
        {
          "rule_id": "rule-1",
          "group_ids": ["group-1"],
          "action_id": "edit",
          "resource_tags": ["section_1"],
          "expiration_date": "2026-06-01T00:00:00Z"
        },
        {
          "rule_id": "<GENERATED_RULE_ID>",
          "group_ids": ["group-2"],
          "action_id": "delete",
          "resource_tags": ["section_2"],
          "expiration_date": "2026-07-01T00:00:00Z"
        }
      ]
    },
    "remote_permission": {
      "baseline_rule": {
        "action_id": "view",
        "resource_tags": ["remote"],
        "enabled": false
      },
      "extra_rules": []
    }
  }
}
```

Field contract:

- Public JSON field names use `snake_case`.
- `expiration_date` must be accepted as an RFC3339 timestamp string and stored as a MongoDB Date.
- The response returns timestamps as RFC3339 strings through Go's default JSON encoding for `time.Time`.
- Request and response baseline rules include `enabled`.
- Baseline rule `enabled` is client-controlled and must be persisted exactly as supplied.
- Response extra rules always include `rule_id`.
- Request-provided `rule_id` values must be unique across both `office_permission.extra_rules` and `remote_permission.extra_rules`.
- Semantically identical extra rules are normalized so only the first occurrence in that permission section is persisted and returned.

## Request Validation

Transport-level validation should reject malformed JSON, invalid timestamp formats, body shape errors, and missing baseline rule `enabled` values. Implementations should model request `enabled` in a way that distinguishes an omitted field from `enabled: false`.

Domain or service validation should reject:

- Empty `workspace_id`.
- Empty `function_key`.
- Missing `office_permission`.
- Missing `remote_permission`.
- Missing baseline rule for either permission section.
- Empty or whitespace-only `action_id`.
- Empty `resource_tags` arrays.
- Any empty or whitespace-only `resource_tags` entry.
- Extra rules with empty `group_ids`.
- Any empty or whitespace-only `group_ids` entry.
- Extra rules with empty or whitespace-only `action_id`.
- Extra rules with empty `resource_tags`.
- Extra rules with zero or missing `expiration_date`.
- Extra rules whose provided `rule_id` is whitespace-only.
- Duplicate request-provided `rule_id` values anywhere in `office_permission.extra_rules` or `remote_permission.extra_rules`.

The first implementation should not reject duplicate `group_ids` or `resource_tags` inside one rule. It should normalize them when comparing rules for semantic duplication.

Validation failures return the existing backend policy error shape, and handlers should construct the payload through `internal/shared/http/exception` to avoid module-local duplication:

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

HTTP status mapping:

- `400` for malformed JSON, invalid timestamp formats, path validation, and request validation failures.
- `500` for repository or unexpected service failures.
- `200` for successful create or replace.

## Persistence Semantics

Collection: `function_resource_permissions`

Identity:

- `workspace_id + function_key` is the logical unique key.
- Existing documents matching that key are replaced in place and keep their `_id`.
- New documents get a generated UUID `_id`.

Write behavior:

- Treat each request as the complete desired permission configuration.
- Replace `office_permission` and `remote_permission` atomically in one MongoDB update.
- Generate `rule_id` values before persistence for any extra rules that omit them.
- Preserve request-provided `rule_id` values after validation.
- Preserve request-provided baseline rule `enabled` values.
- Remove semantically duplicate extra rules before generating missing `rule_id` values and before persistence.
- Return the normalized persisted model from the service after MongoDB succeeds.

Recommended repository operation:

- First try to update by `{ workspace_id, function_key }` with `$set` for permission fields.
- If no document matches, insert a new document with generated `_id`.
- If insert hits a duplicate key because another request created the document concurrently, retry the update once.

This avoids replacing `_id` on existing documents and keeps the repository deterministic under common concurrent create races.

## Extra Rule Normalization

`rule_id` uniqueness is a validation rule:

- If the request contains the same non-empty `rule_id` more than once across `office_permission.extra_rules` and `remote_permission.extra_rules`, return `400`.
- Duplicate `rule_id` validation runs before semantic rule deduplication.
- Generated `rule_id` values must also be unique within the normalized permission model.

Semantic extra rule deduplication is a normalization rule:

- Deduplicate extra rules independently inside each permission section.
- Keep the first occurrence and drop later semantically identical rules.
- The kept rule preserves its request-provided `rule_id` when it has one; if the kept rule omitted `rule_id`, the service generates one after deduplication.
- Provided `rule_id` values on dropped duplicate rules are not carried forward.
- Two extra rules are semantically identical when these fields match after normalization:
  - `group_ids`
  - `action_id`
  - `resource_tags`
  - `expiration_date`
- `group_ids` and `resource_tags` should be treated as unordered sets for this comparison, so different array ordering does not create a distinct rule.
- `rule_id` is not part of the semantic deduplication key because omitted `rule_id` values are generated after deduplication, and provided duplicate `rule_id` values are already rejected.
- Deduplication does not merge across `office_permission.extra_rules` and `remote_permission.extra_rules`; office and remote permissions are separate rule sets.

Example request fragment:

```json
{
  "extra_rules": [
    {
      "group_ids": ["group-1", "group-2"],
      "action_id": "edit",
      "resource_tags": ["section_1"],
      "expiration_date": "2026-06-01T00:00:00Z"
    },
    {
      "group_ids": ["group-2", "group-1"],
      "action_id": "edit",
      "resource_tags": ["section_1"],
      "expiration_date": "2026-06-01T00:00:00Z"
    }
  ]
}
```

Only the first rule should be persisted and returned.

## MongoDB Schema

Document schema:

```json
{
  "_id": "<UUID>",
  "workspace_id": "<WORKSPACE_ID>",
  "function_key": "<FUNCTION_KEY>",
  "office_permission": {
    "baseline_rule": {
      "action_id": "<ACTION_ID>",
      "resource_tags": ["<RESOURCE_TAG>"],
      "enabled": true
    },
    "extra_rules": [
      {
        "rule_id": "<RULE_ID>",
        "group_ids": ["<GROUP_ID>"],
        "action_id": "<ACTION_ID>",
        "resource_tags": ["<RESOURCE_TAG>"],
        "expiration_date": "2026-06-01T00:00:00Z"
      }
    ]
  },
  "remote_permission": {
    "baseline_rule": {
      "action_id": "<ACTION_ID>",
      "resource_tags": ["<RESOURCE_TAG>"],
      "enabled": false
    },
    "extra_rules": []
  }
}
```

Field notes:

- `_id` is a backend-generated UUID for the permission document.
- `workspace_id` and `function_key` identify the owner and function.
- `office_permission` stores the rules for office access.
- `remote_permission` stores the rules for remote access.
- `baseline_rule.enabled` is persisted from the request and may be `true` or `false`.
- `extra_rules.rule_id` is always present in MongoDB.
- Semantically duplicate extra rules are not stored.
- `extra_rules.expiration_date` is stored as a MongoDB Date, represented as an RFC3339 string in JSON examples.

Indexes:

```txt
{ workspace_id: 1, function_key: 1 } unique
```

Rationale:

- The unique index enforces the one-document-per-workspace-function contract.
- Querying and replacing by workspace/function does not need a separate read path in this phase.

## Service Structure

Expected additions:

```txt
internal/domain/permission/
  permission.go
  validation.go
  errors.go

internal/function-service/repositories/
  mongo_permission_repository.go

internal/function-service/services/
  permission_service.go

internal/function-service/handlers/
  permission_handler.go

internal/function-service/transport/
  permission_request.go
  permission_response.go
```

Responsibilities:

- `internal/domain/permission`: framework-independent permission models, baseline rule and extra rule models, normalized save input, validation, and domain errors.
- `internal/function-service/transport`: HTTP request/response DTOs, timestamp parsing through `time.Time`, JSON field names, and DTO/domain mapping.
- `internal/function-service/services`: save permission workflow, validation invocation, generated document/rule IDs, and repository interface definition at the consumer side.
- `internal/function-service/repositories`: MongoDB document mapping, unique index initialization, update-or-insert behavior, and duplicate-key retry.
- `internal/function-service/handlers`: Echo route registration, path/body parsing, service invocation, and HTTP error mapping.
- `cmd/function-service/main.go`: repository construction, index initialization, service construction, and route registration.

The permission domain should be separate from `internal/domain/resource` because the permission aggregate has different invariants and persistence shape. It can still use resource tag strings as plain values without depending on the resource projection model.

## Runtime and Configuration

No new environment variables are required for the first implementation.

Startup should initialize indexes for both:

- existing `function_resources`
- new `function_resource_permissions`

If index initialization fails, startup should fail fast, consistent with the current function-service startup behavior.

## Manual API Examples

Add or extend an executable REST Client example under `examples/api/`.

Recommended file:

```txt
examples/api/function_resource_permissions.http
```

It should include:

- A successful PUT request with one request-provided `rule_id`.
- A successful PUT request with one omitted `rule_id`.
- A successful PUT request that sets one baseline rule to `enabled: false`.
- A successful PUT request that sends duplicate semantic extra rules and receives only one in the response.
- A validation error example for an invalid or missing field.
- A validation error example for duplicate `rule_id` values.
- A note that success returns `200` for both create and replace.

## Testing Strategy

Domain tests:

- Accept a valid permission save input.
- Reject empty workspace/function identity values.
- Reject missing office or remote baseline rules.
- Reject empty action IDs.
- Reject empty resource tags.
- Reject extra rules with empty group IDs.
- Reject zero expiration dates.
- Reject whitespace-only request-provided `rule_id`.
- Reject duplicate request-provided `rule_id` values.

Service tests:

- Save generates document ID and missing rule IDs on create.
- Save preserves request-provided rule IDs.
- Save preserves request-provided baseline `enabled` values in the returned model.
- Save removes semantically duplicate extra rules before ID generation and persistence.
- Save keeps generated rule IDs in the returned model.
- Save returns validation errors before repository calls.
- Save wraps repository failures.

Repository tests:

- Ensure the unique `{ workspace_id, function_key }` index.
- Insert a new permission document with generated `_id`.
- Replace an existing permission document while preserving `_id`.
- Persist request-provided baseline `enabled` values, including `false`.
- Persist every extra rule with `rule_id`.
- Persist only normalized, deduplicated extra rules.
- Retry update once after duplicate-key insert race.

Handler tests:

- Register the `PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions` route.
- Decode a valid request and return `200`.
- Return generated `rule_id` values in the response.
- Return only one extra rule when the request contains duplicate semantic extra rules.
- Return `400` when the request contains duplicate `rule_id` values.
- Return `400` for malformed JSON.
- Return `400` for validation failures.
- Return `500` for service or repository failures.

Transport tests:

- Decode request baseline `enabled` values without treating `false` as missing.
- Reject requests that omit baseline `enabled`.
- Preserve request order before service-level semantic deduplication so the service can keep the first duplicate rule.
- Decode RFC3339 `expiration_date` values into `time.Time`.
- Encode response baseline `enabled`.
- Map request DTOs to domain inputs without leaking Echo or MongoDB types.
- Map domain permission models to the response shape exactly.

Verification commands for implementation:

```bash
go test ./...
```

Additional verification may include `go vet ./...` if implementation touches startup wiring or repository concurrency logic in a non-trivial way.

## Rollout and Compatibility Notes

- This is a new API and a new MongoDB collection, so it does not require migrating existing resource projection documents.
- Deployments must ensure the service can create or verify the unique index on `function_resource_permissions`.
- Because `PUT` replaces the full permission body, clients must send the complete desired configuration every time.
- A retry of the same request that omitted `rule_id` can generate new `rule_id` values if the first response was lost and the client retries with the same omitted-rule payload. Clients that need stable rule identity across retries should send `rule_id`.
- The first implementation does not expose a read endpoint for permissions. Clients receive the persisted model from the PUT response.

## Architecture Decisions

1. Use `workspace_id + function_key` as the logical unique key.
   - Rationale: A workspace/function pair has one current permission configuration.
   - Trade-off: Historical versions and audit are deferred.

2. Implement `PUT` as full replace.
   - Rationale: The request payload represents the complete desired permission state.
   - Trade-off: Clients must send unchanged rules again when editing one rule.

3. Preserve existing MongoDB `_id` on replace.
   - Rationale: The document identity remains stable while the permission body changes.
   - Trade-off: Repository logic needs update-then-insert behavior instead of a simple replacement with a new document.

4. Generate missing `rule_id` values in the service.
   - Rationale: ID generation is business workflow logic and can be tested deterministically with an injected generator.
   - Trade-off: Requests that omit IDs are not naturally idempotent if the response is lost and the client retries.

5. Persist request-provided `baseline_rule.enabled`.
   - Rationale: Clients need to enable or disable baseline rules as part of the complete permission configuration.
   - Trade-off: Transport decoding must distinguish omitted `enabled` from `enabled: false` so validation remains precise.

6. Reject duplicate `rule_id` values but deduplicate semantic extra-rule duplicates.
   - Rationale: `rule_id` is an identity field and must be unambiguous, while duplicate semantic rules are redundant configuration that can be safely normalized.
   - Trade-off: The service needs a canonical comparison key for `group_ids` and `resource_tags`.

7. Keep permission domain types separate from resource domain types.
   - Rationale: Permission rules and resource projections have different invariants and persistence lifecycles.
   - Trade-off: Resource tag strings are duplicated as value fields rather than centralized in a shared type.

## Implementation Plan Notes

The follow-up implementation plan should be created under `docs/plans/active/` and link back to this design document.

The plan should use test-driven steps for domain validation, transport mapping, service ID generation, repository upsert/replace behavior, handler error mapping, route registration, API examples, and startup index initialization.
