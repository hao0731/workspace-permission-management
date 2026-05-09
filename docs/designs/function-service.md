# Function Service Design

## Background

The workspace permission management system uses ABAC to manage access across workspaces, groups, functions, resources, and permissions. Functions are capabilities from integrated systems. Each enabled function exposes resources that can later be targeted by permission rules.

This design introduces `function-service`, a backend service that maintains a MongoDB resource projection from NATS JetStream CloudEvents, exposes a read API for listing function resources in a workspace, and supports deleting projected resources with a JetStream notification to the owning Function service.

Related concept definitions are documented in [../concept.md](../concept.md).

Related designs:

- [Function Resource Permissions Design](function-resource-permissions.md) extends `function-service` with `PUT` and `GET` APIs for storing and retrieving one permission configuration per workspace/function pair in `function_resource_permissions`.

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- Handlers remain thin and only perform transport parsing, validation, service invocation, and response or ack mapping.
- Domain and service logic stay independent of Echo, MongoDB, NATS, and JetStream types.
- Domain resource input/query invariant validation is owned by domain `Validate` methods; transport-only parsing remains in transport packages.
- MongoDB access is isolated behind repository code.
- CloudEvents, API payloads, pagination, and MongoDB schema are treated as explicit contracts.
- This design document is stored under `docs/designs/`.

## Goals

- Build `function-service` as a resource projection service.
- Receive resource upsert events from NATS JetStream through `internal/shared/eventbus`.
- Parse CloudEvents and persist resource state into MongoDB `function_resources`.
- Support idempotent event handling for JetStream at-least-once delivery.
- Expose `GET /api/v1/workspaces/:workspace_id/functions/:function_key/resources`.
- Expose `DELETE /api/v1/workspaces/:workspace_id/functions/:function_key/resources/:resource_id`.
- Delete the matching `function_resources` document by workspace, function, and resource identity.
- Publish a resource-deleted CloudEvent to JetStream after a document is actually deleted.
- Keep the resource-deleted event subject configurable.
- Treat delete requests for missing documents as idempotent success: return `204`, publish no event, and write a structured log entry.
- Support cursor-based pagination with `limit` and `next_token`.
- Keep JetStream stream, upsert subject, durable consumer name, fetch count, and wait settings configurable.
- Enable health endpoints from `internal/shared/health` in `cmd/function-service/main.go`.

## Non-Goals

- Do not implement Function registry management.
- Do not validate whether a workspace or function exists before listing or deleting resources.
- Do not implement an outbox, background retry worker, or delivery guarantee for resource-deleted events in this phase.
- Do not implement permission evaluation.
- Do not implement resource action or resource type management APIs.
- Do not introduce frontend changes.

## Recommended Approach

Use a resource projection design. `function-service` owns the `function_resources` read model. Integrated systems publish CloudEvents to NATS JetStream, and the service stores the latest accepted resource state in MongoDB.

For deletes, use a synchronous delete-and-publish workflow. The service deletes the projected document first, then publishes a resource-deleted CloudEvent to JetStream, and only returns `204` after the publish succeeds. This directly matches the desired API behavior while keeping the first delete implementation small and testable.

Alternatives considered:

- Function registry plus resource projection: more complete, but it expands scope beyond the requested resource ingestion, list API, and delete notification.
- Generic event ingestion platform: flexible, but over-abstracted for the current single projection and narrow event contracts.
- Outbox plus background retry for resource deletes: improves event delivery reliability, but adds an outbox schema, worker lifecycle, retry state, and more operational surface than this feature currently needs.
- Publish-before-delete for resource deletes: avoids a deleted document with a failed event publish, but lets downstream services observe a delete event before the projection deletion has succeeded.

## MongoDB Schema

Collection: `function_resources`

Document schema:

```ts
{
  "_id": string,
  "workspace_id": string,
  "function_key": string,
  "display_name": string,
  "resource_type": string,
  "resource_tags": string[],
  "created_at": Date,
  "updated_at": Date
}
```

Field notes:

- `_id` is the resource ID from the event `data.resource_id`.
- `workspace_id` is the workspace that owns the resource.
- `function_key` identifies the integrated function that owns the resource.
- `display_name` is the user-facing resource name.
- `resource_type` is the function-defined resource type, such as `document`.
- `resource_tags` is persisted from the CloudEvent data and used by permission rules.
- `created_at` uses CloudEvent `time` when the resource is first inserted.
- `updated_at` uses CloudEvent `time` for the latest accepted state update.

Indexes:

```txt
{ workspace_id: 1, function_key: 1, created_at: -1, _id: -1 }
```

Rationale:

- The compound index supports the list API filter and sort.
- `_id` is included as a stable tie-breaker when multiple resources share the same `created_at`.
- The delete API filters by `_id`, `workspace_id`, and `function_key`. MongoDB's `_id` index supports the lookup, while the additional fields protect against deleting a resource under the wrong workspace or function if an incorrect path is supplied.

## Resource Upsert Event Contract

NATS subject and CloudEvent type:

```txt
app.todo.resource.upserted
```

The subject remains configurable, but the default deployment contract is `app.todo.resource.upserted`.

Implementation should parse the CloudEvent envelope with the CloudEvents Go SDK identified by the backend policy instead of hand-rolling envelope parsing. This is an intentional dependency because CloudEvents are part of the repository's backend technology stack and are the public event contract for brokered events.

CloudEvent envelope:

```json
{
  "specversion": "1.0",
  "type": "app.todo.resource.upserted",
  "source": "<NODE_ID>",
  "subject": "<RESOURCE_ID>",
  "id": "<UUID>",
  "time": "2026-05-05T07:31:00Z",
  "datacontenttype": "application/json",
  "data": {
    "resource_id": "<RESOURCE_ID>",
    "display_name": "TEST",
    "resource_type": "document",
    "resource_tags": ["section_1"],
    "function_key": "<FUNCTION_KEY>",
    "workspace_id": "<WORKSPACE_ID>"
  }
}
```

Required envelope fields:

- `specversion`
- `type`
- `source`
- `subject`
- `id`
- `time`
- `datacontenttype`
- `data`

Required data fields:

- `resource_id`
- `display_name`
- `resource_type`
- `resource_tags`
- `function_key`
- `workspace_id`

Validation rules:

- `specversion` must be `1.0`.
- `type` must match the configured subject, whose default is `app.todo.resource.upserted`.
- `datacontenttype` must be `application/json`.
- `time` must be a valid timestamp.
- `data.resource_id`, `workspace_id`, `function_key`, `display_name`, and `resource_type` must be non-empty strings.
- `data.resource_tags` must be a JSON array of strings.
- `subject` should match `data.resource_id`; mismatch is treated as a poison message.

## Resource Upsert Event Handling Semantics

Resource events are processed as upserts keyed by `resource_id`.

Insert behavior:

- If no document exists for `_id = resource_id`, create a new document.
- Set both `created_at` and `updated_at` to CloudEvent `time`.

Update behavior:

- If a document exists and CloudEvent `time` is greater than or equal to the current `updated_at`, update:
  - `workspace_id`
  - `function_key`
  - `display_name`
  - `resource_type`
  - `resource_tags`
  - `updated_at`
- Preserve the original `created_at`.

Older event behavior:

- If a document exists and CloudEvent `time` is older than the current `updated_at`, ignore the event and acknowledge it.
- This prevents delayed or replayed events from moving the projection backward.

Idempotency:

- Duplicate delivery of the same event is safe because writes are keyed by `resource_id`.
- Replaying an event with the same CloudEvent `time` is accepted as the same current state update and leaves the resource in the same logical state.

## Resource Deleted Event Contract

NATS subject and CloudEvent type:

```txt
app.todo.resource.deleted
```

The subject remains configurable through `FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT`. The default deployment contract is `app.todo.resource.deleted`.

The resource-deleted event is published only after a `function_resources` document is actually deleted. Delete requests that find no matching document return `204` and publish no event.

CloudEvent envelope:

```json
{
  "specversion": "1.0",
  "type": "app.todo.resource.deleted",
  "source": "function-service",
  "subject": "<RESOURCE_ID>",
  "id": "<UUID>",
  "time": "2026-05-06T10:00:00Z",
  "datacontenttype": "application/json",
  "data": {
    "workspace_id": "<WORKSPACE_ID>",
    "function_key": "<FUNCTION_KEY>",
    "resource_id": "<RESOURCE_ID>"
  }
}
```

Required data fields:

- `workspace_id`
- `function_key`
- `resource_id`

Generation rules:

- `type` must equal the configured resource-deleted subject.
- `subject` must equal `data.resource_id`.
- `source` is `function-service`.
- `datacontenttype` is `application/json`.
- `id` is generated by the service.
- `time` is generated by the service at publish time.
- The service should inject an ID generator and clock at the service boundary so tests can verify the event deterministically.

Payload rationale:

- The event intentionally includes only stable identity fields.
- Downstream Function services should use those identifiers to perform their own resource cleanup.
- The event does not include deleted document snapshots such as `display_name`, `resource_type`, or `resource_tags`, because those are projection details and are not needed for the deletion notification.

## Resource Delete Semantics

The delete API is idempotent from the HTTP client's perspective.

Delete behavior:

- Match documents using `_id = resource_id`, `workspace_id = :workspace_id`, and `function_key = :function_key`.
- If `DeletedCount = 0`, return `204`, publish no event, and log the no-op with `workspace_id`, `function_key`, and `resource_id`.
- If `DeletedCount = 1`, publish the resource-deleted CloudEvent and return `204` after the publish succeeds.

Publish failure behavior:

- If MongoDB delete succeeds but JetStream publish fails, return `500`.
- Log the failure with `err`, `workspace_id`, `function_key`, `resource_id`, and the configured delete subject.
- This phase does not guarantee event delivery after a successful delete. A client retry after this failure may find no document, return `204`, and publish no event.
- If stronger delivery is required later, add an outbox design instead of hiding retry state inside the handler or service.

## Service Structure

Expected files:

```txt
cmd/function-service/main.go

internal/domain/resource/
  resource.go
  errors.go

internal/function-service/config/
  config.go

internal/function-service/repositories/
  mongo_resource_repository.go

internal/function-service/services/
  resource_service.go

internal/function-service/handlers/
  resource_event_handler.go
  resource_handler.go

internal/function-service/transport/
  resource_event.go
  resource_deleted_event.go
  resource_response.go
```

See shared pagination refactor design: [shared-pagination-helper-refactor.md](shared-pagination-helper-refactor.md).

Responsibilities:

- `cmd/function-service/main.go`: composition root, config loading, MongoDB and NATS setup, JetStream consumer and producer setup, Echo setup, health route registration, resource route registration, eventbus consumer startup, goroutine lifecycle, and graceful shutdown.
- `internal/domain/resource`: framework-independent resource model, resource input/query validation methods, and domain errors.
- `internal/function-service/config`: environment and `.env` backed config loading through viper, including validation and defaults for optional settings.
- `internal/shared/environment`: shared runtime environment contract (`Development`, `Production`), `IsValidEnvironment`, and `ErrInvalidEnv` for validation consistency across services.
- `internal/shared/logger`: shared `logger.New(environment, ...options)` factory; supports environment-aware handler selection and optional `WithLevel` log level override.
- `internal/function-service/repositories`: MongoDB document mapping, index initialization, upsert query, delete query, and list query.
- `internal/function-service/services`: resource upsert, list, and delete workflows. Services call domain input/query `Validate` methods before repositories or publishers, define consumer-side repository and publisher interfaces, and do not depend on Echo, MongoDB, NATS, JetStream, or transport DTOs.
- `internal/function-service/handlers`: Echo HTTP handler, route registration, and eventbus handler. Handlers parse transport input, call services, and map errors to HTTP responses or eventbus handle results.
- `internal/function-service/transport`: CloudEvent data DTOs, HTTP response DTOs, resource-deleted event DTO construction, and DTO/domain mapping. Pagination query parsing and cursor token encode/decode are provided by the shared pagination package (see [shared-pagination-helper-refactor.md](shared-pagination-helper-refactor.md)).

## Resource Input Validation Boundary

Resource input and query invariant validation is defined in [resource-input-validation-refactor.md](resource-input-validation-refactor.md).

The domain package owns:

- `UpsertInput.Validate()`
- `DeleteInput.Validate()`
- `ListQuery.Validate()`

These methods validate framework-independent service workflow invariants such as non-empty resource identity fields, required upsert fields, non-empty resource tags, non-zero event time, positive list limit, and valid cursor fields. They must return errors wrapping `resource.ErrInvalidInput` so HTTP and event handlers can keep their existing error mapping.

Transport packages continue to own transport-specific parsing and validation, including CloudEvent envelope validation and DTO/domain mapping. Pagination query parsing and cursor token encode/decode are migrated to the shared helper package as documented in [shared-pagination-helper-refactor.md](shared-pagination-helper-refactor.md).

Services should call the domain `Validate` methods before invoking repositories or publishers. Services should not keep separate private helper functions that duplicate those domain input/query invariant checks.

## Configuration

Configuration must come from environment variables, with `.env` support allowed for local development.

JetStream stream and durable consumer provisioning is outside the first version of `function-service`. The service binds to the configured stream and durable consumer through `internal/shared/eventbus` during startup and fails fast when they do not exist or when the consumer filter does not include the configured subject.

Required settings:

- `FUNCTION_SERVICE_HTTP_ADDR`
- `FUNCTION_SERVICE_MONGODB_URI`
- `FUNCTION_SERVICE_MONGODB_DATABASE`
- `FUNCTION_SERVICE_NATS_URL`
- `FUNCTION_SERVICE_JETSTREAM_STREAM`
- `FUNCTION_SERVICE_JETSTREAM_DURABLE`
- `FUNCTION_SERVICE_JETSTREAM_SUBJECT`

Optional settings:

- `FUNCTION_SERVICE_ENV`, default `development`. Allowed values are `development` and `production`.
- `FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT`, default `20`.
- `FUNCTION_SERVICE_JETSTREAM_MAX_WAIT`, default `5s`.
- `FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT`, default `app.todo.resource.deleted`.
- `FUNCTION_SERVICE_SHUTDOWN_TIMEOUT`, default `10s`.

Environment behavior:

- `FUNCTION_SERVICE_ENV=development` uses `slog.NewTextHandler`.
- `FUNCTION_SERVICE_ENV=production` uses `slog.NewJSONHandler`.

Repository environment files:

- Commit `.env.example` with local development defaults and all function-service config keys.
- Keep `.env` ignored in `.gitignore` so local secrets and machine-specific settings are not committed.

Deployment defaults:

- `FUNCTION_SERVICE_JETSTREAM_SUBJECT=app.todo.resource.upserted`
- `FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT=app.todo.resource.deleted`

Config validation:

- `FUNCTION_SERVICE_ENV` must be either `development` or `production`.
- Required string settings must be non-empty.
- `FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT` must be non-empty after applying its default.
- Fetch count must be greater than zero.
- Durations must be valid and positive.
- Invalid config fails startup.

## Runtime and Health

`cmd/function-service/main.go` starts the service runtime.

Startup responsibilities:

1. Load and validate config.
2. Initialize `slog`.
3. Connect to MongoDB.
4. Initialize MongoDB indexes for `function_resources`.
5. Connect to NATS.
6. Create an `internal/shared/eventbus` JetStream producer for resource-deleted events.
7. Create an `internal/shared/eventbus` JetStream consumer using configured stream, durable, subject, fetch count, and wait duration.
8. Create Echo and register:
   - `/health/liveness` from `internal/shared/health`.
   - resource API routes.
9. Run the HTTP server in a goroutine.
10. Run the JetStream consumer in a goroutine.
11. Wait for OS signal or runtime error.
12. Shut down HTTP server, event consumer, NATS, and MongoDB with a timeout context.

Initial health behavior:

- Register a basic liveness indicator so `/health/liveness` confirms the process can serve requests.
- MongoDB and NATS health indicators may be added in the composition root if implementation exposes deterministic ping checks.
- Driver-specific health checks must not be placed in domain or service packages.

## API Contract

List endpoint:

```http
GET /api/v1/workspaces/:workspace_id/functions/:function_key/resources
```

Query string:

- `limit`: optional integer. Default `20`, maximum `50`.
- `next_token`: optional opaque cursor token.

Behavior:

- Filter by `workspace_id` and `function_key`.
- Do not validate whether the workspace or function exists.
- Return `200` with an empty resource list when no matching resources exist.
- Sort by `created_at DESC, _id DESC`.
- Fetch `limit + 1` records to determine `has_next_page`.

Success response:

```json
{
  "resources": [
    {
      "id": "resource-123",
      "display_name": "TEST",
      "type": "document",
      "resource_tags": ["section_1"]
    }
  ],
  "page_info": {
    "has_next_page": true,
    "next_token": "<opaque-token>"
  }
}
```

Empty response:

```json
{
  "resources": [],
  "page_info": {
    "has_next_page": false,
    "next_token": ""
  }
}
```

Delete endpoint:

```http
DELETE /api/v1/workspaces/:workspace_id/functions/:function_key/resources/:resource_id
```

Path parameters:

- `workspace_id`: workspace that owns the projected resource.
- `function_key`: function that owns the projected resource.
- `resource_id`: projected resource ID.

Behavior:

- Validate all path parameters as non-empty strings.
- Delete only when `_id`, `workspace_id`, and `function_key` all match.
- Return `204 No Content` when a document is deleted and the resource-deleted event is published.
- Return `204 No Content` when no document matches, publish no event, and write a structured log entry.
- Return `500` when MongoDB delete fails.
- Return `500` when JetStream publish fails after a successful MongoDB delete.
- Never return `404` for a missing delete target in this phase.

Success response:

```http
HTTP/1.1 204 No Content
```

## Cursor Pagination

Cursor token content before encoding:

```json
{
  "created_at": "2026-05-05T07:31:00Z",
  "id": "resource-123"
}
```

Encoding:

- Base64url encoded JSON.
- Treated as opaque by clients.

First page query:

```txt
workspace_id = :workspace_id
AND function_key = :function_key
ORDER BY created_at DESC, _id DESC
LIMIT limit + 1
```

Next page query:

```txt
workspace_id = :workspace_id
AND function_key = :function_key
AND (
  created_at < token.created_at
  OR (created_at = token.created_at AND _id < token.id)
)
ORDER BY created_at DESC, _id DESC
LIMIT limit + 1
```

Validation errors:

- `limit` is not an integer.
- `limit` is less than `1`.
- `limit` is greater than `50`.
- `next_token` is not valid base64url JSON.
- `next_token.created_at` is missing or invalid.
- `next_token.id` is missing or empty.

## Error Handling

HTTP APIs use the backend policy error response shape, constructed via `internal/shared/http/exception` (`Exception`, `New`, `WithDetails`, and `WrapResponse`) to keep handler error payload creation consistent across modules:

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

- Validation failures return `400`.
- Repository or unexpected service failures return `500`.
- Empty result sets return `200`, not `404`.
- Delete requests with no matching document return `204`, not `404`.
- Delete publish failures return `500` after logging the failed event publish.

Event handling result mapping:

- Ack:
  - Valid event was inserted or updated.
  - Valid event was older than the stored projection and safely ignored.
- Retry:
  - MongoDB transient failure.
  - Context deadline or cancellation while processing a valid message where retry may succeed.
- Terminate:
  - CloudEvent envelope cannot be parsed.
  - Required envelope fields are missing or invalid.
  - Required data fields are missing or invalid.
  - Event subject and `data.resource_id` mismatch.
  - `resource_tags` is not an array of strings.

Runtime logging:

- Use `log/slog`.
- Use structured keys such as `err`, `event_id`, `event_type`, `event_subject`, `resource_id`, `workspace_id`, and `function_key`.
- Do not log secrets or full connection strings.
- Missing delete targets should be logged as an idempotent no-op, not as an application error.

## Testing Strategy

Domain and service tests:

- Insert new resource from event data.
- Update existing resource when event time is newer.
- Update existing resource when event time equals current `updated_at`.
- Ignore older event and return a successful outcome.
- List resources with default limit.
- List resources with cursor and page info.
- Delete existing resource publishes a resource-deleted event and returns success.
- Delete missing resource returns success and does not publish an event.
- Delete repository failure returns an error.
- Delete event publish failure returns an error after the repository delete succeeds.

Repository tests:

- Map MongoDB document fields to domain resource values.
- Upsert inserts a new document with `created_at` and `updated_at`.
- Upsert updates mutable fields while preserving `created_at`.
- Upsert ignores older events.
- List query filters by `workspace_id` and `function_key`.
- List query sorts by `created_at DESC, _id DESC`.
- List query applies cursor boundary correctly.
- Index initialization creates the compound query index.
- Delete query filters by `_id`, `workspace_id`, and `function_key`.
- Delete returns a result that distinguishes deleted and not-found/no-op cases.

Event handler tests:

- Valid CloudEvent returns Ack.
- Invalid JSON returns Terminate.
- Missing required CloudEvent field returns Terminate.
- Invalid event data returns Terminate.
- Service transient error returns Retry.
- Older valid event returns Ack.

HTTP handler tests:

- No `limit` uses default `20`.
- `limit=50` is accepted.
- `limit=51` returns validation error.
- Non-integer `limit` returns validation error.
- Invalid `next_token` returns validation error.
- Empty repository result returns `200` with empty resources.
- Success response uses `id`, `display_name`, `type`, `resource_tags`, and `page_info`.
- DELETE success returns `204` with an empty body.
- DELETE missing document behavior still returns `204`.
- DELETE validation failure returns the shared validation error shape.
- DELETE unexpected service or publish failure returns `500`.

Transport tests:

- Resource-deleted CloudEvent uses the configured subject as `type`.
- Resource-deleted CloudEvent `subject` equals `resource_id`.
- Resource-deleted CloudEvent data contains only `workspace_id`, `function_key`, and `resource_id`.
- Resource-deleted CloudEvent generation uses injected ID and time values.

Manual API example:

- Add `examples/api/function_resources.http`.
- Include primary success request.
- Include pagination request with `next_token`.
- Include invalid `limit` example.
- Include invalid `next_token` example.
- Include DELETE request for the delete API.
- Document that DELETE returns `204` both when a document is deleted and when it is already absent.

Verification commands for implementation:

```bash
go test ./...
```

Additional verification may include `go vet ./...` if the implementation plan touches startup, concurrency, or repository logic in a way that benefits from vet checks.

## Rollout and Compatibility Notes

- The `function_resources` collection is a projection and can be rebuilt by replaying JetStream messages if the stream retains enough history.
- The first release requires producers to publish `resource_tags` in CloudEvent `data`.
- Event schema changes must be backward compatible or versioned in a future design.
- The service assumes JetStream delivers messages at least once; idempotent upsert behavior is required for correctness.
- The service does not validate workspace or function existence in this phase, so clients should treat an empty list as "no projected resources found" rather than definitive non-existence.
- The delete API is idempotent for missing projection documents. Clients should not infer workspace, function, or resource existence from a `204` delete response.
- Downstream Function services must subscribe to the configured resource-deleted subject to perform resource cleanup.
- This phase does not include an outbox. If a delete succeeds and publish fails, the API returns `500`, but the deleted document will not be available for a later retry to republish the event.

## Architecture Decisions

1. Use resource projection as the first version.
   - Rationale: It directly satisfies event ingestion and list API requirements.
   - Trade-off: Workspace and function existence validation is deferred.

2. Use CloudEvent `time` for `created_at` and `updated_at`.
   - Rationale: Resource timestamps represent source event time, not ingestion time.
   - Trade-off: Producers must provide trustworthy event time.

3. Ignore older events.
   - Rationale: Prevent delayed or replayed events from overwriting newer projected state.
   - Trade-off: Late corrections with old timestamps will not update the projection.

4. Use `created_at DESC, _id DESC` cursor pagination.
   - Rationale: Newest resources appear first and `_id` gives a stable tie-breaker.
   - Trade-off: Cursor order follows creation time, not display name.

5. Return `200` empty lists for missing workspace/function resource projections.
   - Rationale: This service only owns the resource projection and does not query a registry.
   - Trade-off: Clients cannot distinguish "workspace/function missing" from "no resources projected" through this endpoint.

6. Bind to pre-provisioned JetStream stream and durable consumer.
   - Rationale: The existing `internal/shared/eventbus` consumer binds and validates configured stream, durable, and subject contracts at startup.
   - Trade-off: Local and deployment environments must provision JetStream resources before starting `function-service`.

7. Return `204` for missing delete targets and publish no event.
   - Rationale: DELETE remains idempotent and tolerant of repeated requests or already-pruned projections.
   - Trade-off: Clients cannot distinguish "deleted now" from "was already absent" through the HTTP response.

8. Publish resource-deleted events synchronously after MongoDB deletion.
   - Rationale: This directly follows the required delete sequence and keeps the design small.
   - Trade-off: Without an outbox, a publish failure after deletion can leave downstream services uninformed.

9. Keep delete event payload minimal.
   - Rationale: Downstream Function services only need workspace, function, and resource identity to react to deletion.
   - Trade-off: Consumers that need display metadata must own or fetch it elsewhere; the delete event will not provide a deleted document snapshot.

10. Use the configured delete subject as the CloudEvent type.
   - Rationale: This follows the existing upsert event style and keeps subject/type configuration simple.
   - Trade-off: Changing the configured subject also changes the CloudEvent type contract and must be coordinated with consumers.

11. Put resource input/query invariant validation on domain `Validate` methods.
   - Rationale: `UpsertInput`, `DeleteInput`, and `ListQuery` carry framework-independent service workflow values, so their invariant checks should be discoverable from those types and reusable across service entry points.
   - Trade-off: The domain package owns basic string/time validation helpers, while transport packages still own HTTP and CloudEvent parsing concerns.

## Implementation Plan Notes

The follow-up implementation plan should be created or updated under `docs/plans/active/` and link back to this design document.

The plan should implement the service with tests before production code where practical, especially for cursor token validation, event parsing, upsert semantics, delete semantics, resource-deleted event generation, synchronous publish behavior, and ack/retry/terminate mapping.
