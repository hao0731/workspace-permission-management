# Function Service System Resource API Design

## Background

`function-service` currently treats a Function as the external application identity for workspace resource projection and permission configuration. Product language is moving from Function to System: a System is an external application built on top of this permission management system, and the current `function_key` value is the same identity now exposed as `system_id`.

This design adds APIs for a System to define its resource types, resource tags, and resource actions. Those definitions are stored in `function-service`, and the service derives the complete resource attribute strings that downstream permission workflows can use.

Related designs:

- [Function Service Design](function-service.md): entry design for `function-service`, existing projected resources, delete workflow, runtime, and service boundaries.
- [Function Resource Permissions Design](function-resource-permissions.md): existing workspace and function scoped permission configuration APIs.
- [Resource Command and Event Domain Contracts](resource-command-event-contracts.md): existing resource lifecycle event contracts that still use `function_key` until the broader naming migration is designed.

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- HTTP payloads, response payloads, MongoDB documents, indexes, and configuration keys are explicit contracts.
- Handlers remain thin and only parse path/body input, invoke services, and render mapped responses or errors.
- Request and response DTOs belong in `internal/function-service/transport`.
- System resource definition and resource attribute domain types stay in `internal/domain/resource` and remain independent of Echo, MongoDB, and transport DTOs.
- The service owns validation orchestration, limit checks, transaction orchestration, timestamp assignment, ID generation, and resource attribute derivation.
- MongoDB access and transaction implementation details remain isolated in `internal/function-service/repositories`.
- This design document is stored under `docs/designs/` and linked from the existing `function-service` design.

## Goals

- Expose `POST /api/v1/systems/:system_id/resources`.
- Expose `GET /api/v1/systems/:system_id/resources`.
- Expose `GET /api/v1/systems/:system_id/resource-attributes`.
- Treat `system_id` as the current `function_key` identity under its future name.
- Store resource definitions in MongoDB collection `system_resources`.
- Store derived resource attributes in MongoDB collection `system_resource_attributes`.
- Support resource definition types `type`, `tag`, and `action`.
- Validate `key`, `label`, and `description` length and format rules.
- Keep type, tag, and action count limits configurable with defaults of `3`, `20`, and `5`.
- Use partial upsert semantics for `POST`: resources omitted from the request remain unchanged.
- Upsert by `{ system_id, type, key }`.
- Preserve `_id` and `created_at` for updated resources.
- Update `label`, `description`, and `updated_at` for existing resources.
- Return only the persisted resources addressed by the POST request, preserving request order.
- Recompute derived attributes from the latest persisted action, tag, and type sets after every successful write.
- Use a MongoDB transaction so resource definition updates and derived attribute updates succeed or fail together.
- Return empty arrays for successful GET requests with no stored resource definitions or attributes.

## Non-Goals

- Do not migrate existing workspace-scoped routes from `function_key` to `system_id` in this phase.
- Do not change the resource-upsert CloudEvent contract in this phase.
- Do not validate `system_id` against a system registry; no registry exists in this repository yet.
- Do not add resource definition delete APIs.
- Do not add resource definition history, audit records, or version conflict behavior.
- Do not validate existing `function_resources` projection records or permission documents against these definitions.
- Do not implement permission evaluation changes.
- Do not add frontend changes.

## System ID And Function Key Naming

`system_id` is a string identifier and is not a UUID. It is the same logical value currently named `function_key` in existing APIs, documents, and events.

The new APIs intentionally use `/systems/:system_id/...` to start the naming migration at the API boundary. Existing routes such as `/api/v1/workspaces/:workspace_id/functions/:function_key/resources` and existing CloudEvent data fields still use `function_key` until a separate migration design updates those contracts.

Validation should treat `system_id` like the existing function/app key used in NATS subjects:

- Trim surrounding whitespace.
- Reject empty values.
- Reject whitespace.
- Reject `.` because current function/app identifiers are single NATS subject tokens.

## Recommended Approach

Use a system-scoped resource definition subdomain inside `function-service`. The HTTP layer receives system resource definitions, the service layer validates the merged post-write state and derives attributes, and the repository persists all related changes in one MongoDB transaction.

This approach keeps resource definition management close to the existing function resource and permission contracts while preserving current service boundaries. It also keeps the external API small: clients can upsert definitions, list definitions, and read the derived attribute set without needing to understand the storage model.

Alternatives considered:

- Store resource definitions in a new system registry service: clearer long-term ownership, but there is no registry service in the current architecture and this would expand scope beyond the requested `function-service` extension.
- Make `POST` a full replacement: simpler count and attribute calculation, but it conflicts with the requested upsert behavior and the current absence of delete APIs.
- Recompute resource attributes asynchronously: reduces POST work, but creates temporary read-after-write drift and is unnecessary for the small maximum resource definition counts.

## API Contract

### Save System Resources

Endpoint:

```http
POST /api/v1/systems/:system_id/resources
Content-Type: application/json
```

Path parameters:

- `system_id`: required system identifier. This is the future name for the current `function_key` value.

Request body:

```json
{
  "resources": [
    {
      "type": "action",
      "label": "Can Edit",
      "key": "can_edit",
      "description": "Allows editing resources."
    },
    {
      "type": "tag",
      "label": "Private",
      "key": "private"
    },
    {
      "type": "type",
      "label": "Repository",
      "key": "repo",
      "description": "Repository resources."
    }
  ]
}
```

Success response:

```http
HTTP/1.1 200 OK
```

```json
{
  "resources": [
    {
      "type": "action",
      "label": "Can Edit",
      "key": "can_edit",
      "description": "Allows editing resources.",
      "created_at": "2026-05-18T10:00:00Z",
      "updated_at": "2026-05-18T10:00:00Z"
    },
    {
      "type": "tag",
      "label": "Private",
      "key": "private",
      "created_at": "2026-05-18T10:00:00Z",
      "updated_at": "2026-05-18T10:00:00Z"
    },
    {
      "type": "type",
      "label": "Repository",
      "key": "repo",
      "description": "Repository resources.",
      "created_at": "2026-05-18T10:00:00Z",
      "updated_at": "2026-05-18T10:00:00Z"
    }
  ]
}
```

Behavior contract:

- `POST` is a partial upsert.
- The request resources are full representations for their `{ type, key }` identity.
- Existing resources for the same `system_id` that are not present in the request remain unchanged.
- Existing resources with the same `{ system_id, type, key }` keep `_id` and `created_at`.
- Existing resources with the same `{ system_id, type, key }` update `label`, `description`, and `updated_at`.
- If `description` is omitted, the persisted resource has no description. On update, omission clears the existing description.
- The response contains only the persisted resources addressed by the request.
- The response order matches the request order.
- `created_at` and `updated_at` are returned as RFC3339 timestamp strings through Go's default `time.Time` JSON encoding.
- The endpoint does not check whether the system exists in another registry.

### List System Resources

Endpoint:

```http
GET /api/v1/systems/:system_id/resources
```

Success response when resources exist:

```http
HTTP/1.1 200 OK
```

```json
{
  "resources": [
    {
      "type": "action",
      "label": "Can Edit",
      "key": "can_edit",
      "description": "Allows editing resources.",
      "created_at": "2026-05-18T10:00:00Z",
      "updated_at": "2026-05-18T10:00:00Z"
    }
  ]
}
```

Success response when no resources exist:

```http
HTTP/1.1 200 OK
```

```json
{
  "resources": []
}
```

Behavior contract:

- Filter by `system_id`.
- Return all resource definitions for the system.
- No pagination is required because the default maximum total is `28` resources and remains bounded by configuration.
- If configured limits are raised materially beyond metadata-scale values, pagination should be introduced in a follow-up design.
- Return resources in deterministic order by `type ASC, key ASC`.
- The endpoint does not check whether the system exists in another registry.

### Get System Resource Attributes

Endpoint:

```http
GET /api/v1/systems/:system_id/resource-attributes
```

Success response when attributes exist:

```http
HTTP/1.1 200 OK
```

```json
{
  "resource_attributes": [
    "can_edit_private_repo",
    "can_edit_public_repo",
    "can_view_private_repo",
    "can_view_public_repo"
  ]
}
```

Success response when no attribute document exists:

```http
HTTP/1.1 200 OK
```

```json
{
  "resource_attributes": []
}
```

Behavior contract:

- Read the single `system_resource_attributes` document for `system_id`.
- Return `resource_attributes: []` when no document exists.
- The endpoint does not derive attributes on read; derivation happens during successful writes.
- The endpoint does not check whether the system exists in another registry.

## Request Validation

Transport-level validation should reject malformed JSON, body shape errors, and wrong JSON types. Domain or service validation should reject invalid business values.

Path validation:

- `system_id` is required.
- `system_id` is trimmed before validation and persistence.
- `system_id` must not contain `.`.
- `system_id` must not contain whitespace.

POST body validation:

- `resources` is required.
- `resources` must be a non-empty array.
- Each resource `type` must be one of `type`, `tag`, or `action`.
- Each resource `key` is required.
- Each resource `key` must be at most `15` characters.
- Each resource `key` must match `^[a-z0-9_]+$`.
- Each resource `label` is required.
- Each resource `label` must be at most `20` characters.
- Each resource `description` is optional.
- Each resource `description` must be at most `2000` characters when present.
- The same request must not contain duplicate `{ type, key }` pairs after trimming and normalization.
- The post-write resource count for a system must not exceed configured limits:
  - `type`: default `3`
  - `action`: default `5`
  - `tag`: default `20`

Normalization:

- Trim surrounding whitespace from `system_id`, `type`, `label`, `key`, and `description`.
- Persist normalized values.
- Validate length after trimming.
- Because `key` only allows lower-case ASCII letters, digits, and underscores, byte length and character count are equivalent for `key`.
- `label` and `description` length checks should use character count, not raw UTF-8 byte length.

Count limit rules:

- Limits are checked against the merged post-write state, not only against the request payload.
- Updating an existing resource does not increase the count for that resource type.
- Creating a new `{ type, key }` identity increases the count for that resource type by one.
- Duplicate `{ type, key }` pairs in the same request are validation errors instead of last-write-wins.

## MongoDB Schema

### `system_resources`

Collection: `system_resources`

Document schema:

```ts
type SystemResourceDocument = {
  _id: string;
  system_id: string;
  type: 'type' | 'tag' | 'action';
  label: string;
  key: string;
  description?: string;
  created_at: Date;
  updated_at: Date;
}
```

Indexes:

```txt
{ system_id: 1, type: 1, key: 1 } unique
```

Rationale:

- `{ system_id, type, key }` is the logical identity.
- The unique index allows the same key to exist under different resource definition types for the same system.
- The unique index also supports reads by `system_id` with deterministic ordering by `type` and `key`.

### `system_resource_attributes`

Collection: `system_resource_attributes`

Document schema:

```ts
type SystemResourceAttributesDocument = {
  _id: string;
  system_id: string;
  resource_attributes: string[];
  created_at: Date;
  updated_at: Date;
}
```

Indexes:

```txt
{ system_id: 1 } unique
```

Rationale:

- A system has at most one derived attribute document.
- The document is updated as a whole because the attribute set is fully derived from current resource definitions.
- `_id` is generated by `function-service` when the attribute document is first created and preserved on later updates.

## Resource Attribute Derivation

Resource attributes are generated only when the latest persisted resource definitions for a system include at least one `action`, one `tag`, and one `type`.

Domain ownership:

```go
type ResourceAttribute string

func NewResourceAttribute(action, tag, resourceType string) ResourceAttribute
```

`NewResourceAttribute` belongs in `internal/domain/resource/resource_attribute.go`. It should build the canonical `<action>_<tag>_<type>` string from already-normalized resource definition keys. Validation of empty strings, key format, and length remains on the resource definition input path rather than inside the constructor.

Derivation rule:

```txt
<action_key>_<tag_key>_<type_key>
```

Example:

- Actions: `can_edit`, `can_view`
- Tags: `private`, `public`
- Types: `repo`

Derived attributes:

```json
[
  "can_edit_private_repo",
  "can_edit_public_repo",
  "can_view_private_repo",
  "can_view_public_repo"
]
```

Ordering:

- Sort actions by `key ASC`.
- Sort tags by `key ASC`.
- Sort types by `key ASC`.
- Generate combinations in action, tag, type nested-loop order.

Incomplete definition behavior:

- If any of action, tag, or type has no persisted resources for the system, do not write `system_resource_attributes`.
- `GET /api/v1/systems/:system_id/resource-attributes` returns `resource_attributes: []` when no attribute document exists.
- Because this design has no delete API, incomplete definition behavior is expected only before a system has saved at least one resource for every required category.

The derived attribute string is not designed to be parsed back into action, tag, and type keys. Clients that need the source definitions should call the resources API.

## Write Workflow

The service should use one request-scoped timestamp for all resources and the attribute document updated by a single POST request. It should inject clock and ID generators at the service boundary so tests can verify timestamps and IDs deterministically.

Save flow:

1. Handler extracts `system_id` and decodes the request body.
2. Transport maps the request to a domain input.
3. Service validates path and body invariants.
4. Service starts a repository transaction through a consumer-side transaction interface that does not expose MongoDB driver types.
5. Inside the transaction, repository reads existing resources for `system_id`.
6. Service merges existing resources with the normalized request resources and validates post-write count limits.
7. Repository upserts each request resource by `{ system_id, type, key }`.
8. For new resources, repository sets `_id`, `created_at`, and `updated_at`.
9. For existing resources, repository preserves `_id` and `created_at`, and sets `label`, `description`, and `updated_at`.
10. Repository reads the latest persisted resources for `system_id` inside the same transaction.
11. Service derives attributes from the latest persisted resources.
12. If all three categories are present, repository upserts the single `system_resource_attributes` document for `system_id`.
13. If any category is missing, repository does not write `system_resource_attributes`.
14. Transaction commits.
15. Handler renders the persisted resources corresponding to the request, preserving request order.

Transaction boundary:

- The service owns the transaction boundary as a use-case concern.
- The concrete MongoDB repository implements the transaction runner and session binding.
- Service and domain packages must not import MongoDB driver types.
- MongoDB transaction failure returns an internal service error and no partial successful response.

Concurrency note:

- Limit validation must use the latest state read in the transaction.
- If strict concurrent enforcement for multiple service instances becomes necessary, the implementation plan should add a per-system write serialization mechanism rather than relying on handler-level state.

## Service Structure

Expected additions:

```txt
internal/domain/resource/
  resource_definition.go
  resource_attribute.go

internal/function-service/repositories/
  mongo_system_resource_repository.go

internal/function-service/services/
  system_resource_service.go

internal/function-service/handlers/
  system_resource_handler.go

internal/function-service/transport/
  system_resource_request.go
  system_resource_response.go
```

Responsibilities:

- `internal/domain/resource`: framework-independent resource model, resource command/event contracts, resource definition models, save/list/get-attributes inputs, resource definition type enum, limit config model, normalization, validation, `ResourceAttribute`, `NewResourceAttribute`, and domain errors.
- `internal/function-service/transport`: HTTP request and response DTOs, JSON decoding, DTO-to-domain mapping, and response construction.
- `internal/function-service/handlers`: Echo route registration, path/body parsing, service invocation, and error mapping.
- `internal/function-service/services`: save/list/get-attributes workflows, post-write count validation, transaction orchestration, deterministic ID/time seams, and attribute derivation through `resource.NewResourceAttribute`.
- `internal/function-service/repositories`: MongoDB documents, index initialization, transaction runner, resource upserts, list reads, attribute document reads, and attribute upserts.
- `cmd/function-service/main.go`: config wiring, repository construction, index initialization, service construction, and route registration.

## Configuration

Configuration must come from environment variables, with `.env` support allowed for local development.

Optional settings:

- `FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT`, default `3`.
- `FUNCTION_SERVICE_SYSTEM_RESOURCE_ACTION_LIMIT`, default `5`.
- `FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT`, default `20`.

Config validation:

- Each limit must be a positive integer.
- The defaults are used only when the corresponding environment variable is absent.
- Invalid values fail startup.

These settings should be added to `.env.example` during implementation.

## Error Handling

HTTP APIs use the backend policy error response shape through `internal/shared/http/exception`:

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

HTTP error mapping:

- Invalid JSON returns `400 validation_failed`.
- Path or body validation failures return `400 validation_failed`.
- Post-write limit violations return `400 validation_failed`.
- MongoDB read, write, index, or transaction failures return `500 internal_error`.
- GET resources with no documents returns `200` with `resources: []`.
- GET resource attributes with no document returns `200` with `resource_attributes: []`.

Runtime logging:

- Use `log/slog`.
- Use structured keys such as `err`, `system_id`, `resource_type`, `resource_key`, and `resource_count`.
- Do not log full request bodies because `description` can be large.

## Testing Strategy

Domain tests:

- `system_id` validation rejects empty, whitespace, `.` and whitespace-containing values.
- Resource type validation accepts only `type`, `tag`, and `action`.
- Resource key validation rejects empty, over-15-character, uppercase, hyphenated, dotted, and whitespace values.
- Resource label validation rejects empty and over-20-character values.
- Resource description validation accepts omitted descriptions and rejects over-2000-character values.
- Duplicate request `{ type, key }` values are rejected after normalization.
- Post-write limit validation counts merged existing and requested resources.
- `NewResourceAttribute("can_edit", "private", "repo")` returns `ResourceAttribute("can_edit_private_repo")`.

Service tests:

- Save performs partial upsert and preserves existing resources omitted from the request.
- Save preserves request order in the response.
- Save preserves `created_at` and `_id` when updating an existing resource.
- Save updates `label`, `description`, and `updated_at` when updating an existing resource.
- Omitted update description clears the existing description.
- Save rejects requests that exceed type, tag, or action limits after merging.
- Save does not write attributes when any of action, tag, or type is missing.
- Save writes deterministic attributes when all three categories exist.
- Save recomputes attributes from all latest persisted resources, not just the request payload.
- Transaction failure returns an error and does not render a partial success.

Repository tests:

- Ensure indexes creates unique `{ system_id, type, key }` and unique `{ system_id }` indexes.
- Upsert inserts new resource documents with generated IDs and timestamps.
- Upsert updates existing resource documents while preserving `_id` and `created_at`.
- List reads all resources for a system and excludes other systems.
- Get attributes returns a not-found signal when no attribute document exists.
- Upsert attributes creates the document when missing and preserves `_id` and `created_at` when updating.
- Transaction runner commits on success and aborts on error.

Handler and transport tests:

- POST success returns `200` and the persisted resources in request order.
- POST invalid JSON returns the shared validation error shape.
- POST validation failures return `400 validation_failed`.
- GET resources returns `200` with resources when present.
- GET resources returns `200` with an empty array when absent.
- GET resource attributes returns `200` with attributes when present.
- GET resource attributes returns `200` with an empty array when absent.
- Unexpected service failures return `500 internal_error`.

Manual API examples:

- Add `examples/api/system_resources.http` during implementation.
- Include a primary POST success request.
- Include a GET resources request.
- Include a GET resource attributes request.
- Include validation examples for invalid key, over-limit resources, and duplicate request keys.

Verification commands for implementation:

```bash
go test ./...
```

Additional verification may include `go vet ./...` if the implementation touches transaction wiring or startup code in ways that benefit from vet checks.

## Rollout And Compatibility Notes

- Existing `function_resources` and `function_resource_permissions` documents continue to use `function_key`.
- Existing workspace-scoped routes continue to use `/functions/:function_key`.
- New system resource definition routes use `/systems/:system_id`.
- `system_id` and `function_key` must be treated as the same value by clients during the migration period.
- Because no registry exists, an empty GET response means no stored definitions or attributes were found; it does not prove the system itself is missing.
- The `system_resource_attributes` document can be rebuilt from `system_resources` for any system that has at least one action, tag, and type.

## Architecture Decisions

1. Use `system_id` in the new route while leaving existing `function_key` contracts unchanged.
   - Rationale: This starts the naming migration without breaking current APIs, events, or stored permission documents.
   - Trade-off: The codebase temporarily contains both names for the same concept.

2. Use partial upsert semantics for `POST`.
   - Rationale: The requested behavior updates matching keys and creates missing keys, and there is no delete API in this phase.
   - Trade-off: Clients need a future delete API to remove obsolete definitions.

3. Use `{ system_id, type, key }` as the unique identity.
   - Rationale: Resource type, tag, and action are separate namespaces. A system can reuse the same key under different definition categories.
   - Trade-off: Clients must include `type` when addressing or updating a definition.

4. Store and return `description`.
   - Rationale: The request contract includes `description`, and clients need read-after-write visibility for all editable fields.
   - Trade-off: Responses are slightly larger, but the maximum resource count is small and `description` is capped.

5. Derive attributes during writes, not reads.
   - Rationale: Write-time derivation gives `GET /resource-attributes` a simple read path and keeps the derived document consistent with the resource definition write.
   - Trade-off: POST does extra work, but the maximum combination size is bounded by configuration.

6. Skip attribute writes when any category is missing.
   - Rationale: The attribute format requires action, tag, and type keys. Incomplete combinations should not create malformed attributes.
   - Trade-off: The missing-document case must be treated as an empty attribute set by the GET endpoint.

7. Put resource definition and resource attribute behavior in `internal/domain/resource`.
   - Rationale: Resource definitions, projected resources, resource lifecycle events, and derived resource attributes are all part of the same resource domain vocabulary. Keeping them in one package avoids a second resource-like domain package and follows the existing `resource.ResourceUpsertEvent` ownership.
   - Trade-off: `internal/domain/resource` grows beyond projected resources, so implementation should keep the new definitions in focused files such as `resource_definition.go` and `resource_attribute.go`.

## Implementation Plan Notes

The follow-up implementation plan should be created under `docs/plans/active/` and link back to this design document and [function-service.md](function-service.md).

The plan should implement tests before production code where practical, especially for validation rules, post-write limit checks, transaction behavior, attribute derivation, upsert timestamp preservation, empty GET responses, and API examples.
