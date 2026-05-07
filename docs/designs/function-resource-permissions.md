# Function Resource Permissions API Design

## Background and Goals

`function-service` currently owns function resource projection and resource delete/list APIs. This design adds a workspace+function scoped permissions write API so clients can persist permission policy state in MongoDB and retrieve the persisted canonical value in the same response.

Goals:

- Add `PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions`.
- Persist request payload into `function_resource_permissions`.
- Use full-replacement semantics (PUT replace).
- Return the actual persisted value after write.
- Keep handler/service/repository layering aligned with backend architecture policy.

Related design:

- [Function Service Design](./function-service.md)

## Classification and Policies

This is backend + design documentation work.

Followed policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

## Scope and Boundaries

In scope:

- New PUT permissions API contract.
- Request/response DTO shape.
- Validation rules.
- MongoDB document schema and upsert behavior.

Out of scope:

- Permission evaluation runtime.
- Read API for permissions (unless added in later design).
- Frontend schema implementation details.

## API Contract

Endpoint:

```http
PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions
```

Path params:

- `workspace_id` (required, non-empty)
- `function_key` (required, non-empty)

Request body:

```ts
type Request = {
  office_permission: PermissionInput;
  remote_permission: PermissionInput;
}

type PermissionInput = {
  baseline_rule: BaselineRuleInput;
  extra_rules: ExtraRuleInput[];
}

type BaselineRuleInput = {
  action_id: string;
  resource_tags: string[];
  enabled: boolean;
}

type ExtraRuleInput = {
  rule_id?: string;
  group_ids: string[];
  action_id: string;
  resource_tags: string[];
  expiration_date: Date;
}
```

Response body (`200 OK`):

```ts
type Response = {
  permissions: Permission;
}

type Permission = {
  office_permission: PermissionBlock;
  remote_permission: PermissionBlock;
}

type PermissionBlock = {
  baseline_rule: BaselineRule;
  extra_rules: ExtraRule[];
}

type BaselineRule = {
  action_id: string;
  resource_tags: string[];
  enabled: boolean;
}

type ExtraRule = {
  rule_id: string;
  group_ids: string[];
  action_id: string;
  resource_tags: string[];
  expiration_date: Date;
}
```

Behavior:

- PUT is full replacement for the permission document identified by `(workspace_id, function_key)`.
- If a document does not exist, create it.
- If a document exists, overwrite both `office_permission` and `remote_permission` from request.
- Response returns the canonical persisted value (including generated `rule_id`s).

## Validation Rules

Path-level:

- `workspace_id` must be non-empty.
- `function_key` must be non-empty.

Body-level:

- `office_permission` and `remote_permission` are required.
- `baseline_rule.action_id` is required, non-empty.
- `baseline_rule.resource_tags` must be a non-empty string array.
- `baseline_rule.enabled` is required boolean.
- `extra_rules[*].group_ids` must be a non-empty string array.
- `extra_rules[*].action_id` is required, non-empty.
- `extra_rules[*].resource_tags` must be a non-empty string array.
- `extra_rules[*].expiration_date` must be a valid time and must be in the future at validation time.
- If `extra_rules[*].rule_id` is provided, it must be a valid UUID.

Error response format:

- Reuse existing shared HTTP error envelope:

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

- `200` write success.
- `400` validation failure.
- `500` repository/unexpected failure.

## Rule ID Handling

For every `extra_rules` element stored in DB:

- `rule_id` must always exist.
- If request includes `rule_id`, preserve it after UUID validation.
- If request omits `rule_id`, backend generates UUID v4 and stores it.

## MongoDB Schema

Collection: `function_resource_permissions`

Document:

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
        "expiration_date": "<DATE>"
      }
    ]
  },
  "remote_permission": {
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
        "expiration_date": "<DATE>"
      }
    ]
  }
}
```

Uniqueness and write strategy:

- One document per `(workspace_id, function_key)`.
- Repository upsert filter uses `{workspace_id, function_key}`.
- Recommend unique index: `{ workspace_id: 1, function_key: 1 }`.

Identity notes:

- `_id` is internal document identity (UUID).
- `rule_id` is per-extra-rule identity (UUID).

## Layering and Module Placement

- Handler: parse transport input, run transport validation, map to service input, return transport response.
- Service: enforce workflow rules (full replace, rule-id generation, canonical output), call repository.
- Repository: MongoDB upsert and mapping.
- Domain: validation invariants and domain error typing (`ErrInvalidInput` wrapping).

No layer should import higher-level concerns (e.g., repository must not depend on Echo DTOs).

## Trade-offs and Architecture Decisions

1. Use PUT full replacement.
   - Rationale: predictable idempotent client behavior and simpler conflict semantics.
   - Trade-off: clients must send full permission state each write.

2. Backend-generated `rule_id` fallback.
   - Rationale: guarantees stable identity in persistence even when client omits IDs.
   - Trade-off: generated IDs are server-owned and require client to consume response canonical value.

3. Enforce UUID validation when client supplies `rule_id`.
   - Rationale: keeps identifier format consistent for downstream operations.
   - Trade-off: legacy/non-UUID client IDs are rejected.

4. Require future `expiration_date`.
   - Rationale: prevents storing already-expired exception rules.
   - Trade-off: requires server/client clock sanity and explicit timezone handling.

## Testing Recommendations

- HTTP handler tests:
  - success `200` with generated rule IDs reflected in response.
  - invalid path/body returns `400` with shared error envelope.
  - supplied non-UUID rule_id returns `400`.
  - past expiration_date returns `400`.
- Service tests:
  - preserve provided UUID rule_id.
  - generate UUID v4 when rule_id missing.
  - full replacement semantics overwrite previous document values.
- Repository tests:
  - upsert by `(workspace_id, function_key)` keeps single-document invariant.
  - unique index creation for `(workspace_id, function_key)`.

## Implementation Plan Linkage

A follow-up implementation plan should be added under `docs/plans/active/` and link both:

- [Function Resource Permissions API Design](./function-resource-permissions.md)
- [Function Service Design](./function-service.md)
