# Function Service Resource Delete Design

## Background

The existing `function-service` design focuses on ingesting resource upsert events and serving resource list queries. This design extends service responsibilities to include explicit resource deletion through HTTP API and asynchronous notification to downstream Function services.

Base design reference: [./function-service.md](./function-service.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- Handler remains transport-only (path parsing, validation, mapping response code).
- Service owns orchestration order: delete projection then publish event.
- Repository owns MongoDB deletion operation only.
- Event payload and subject are explicit transport contracts.
- This design document is stored under `docs/designs/`.

## Goals

- Add `DELETE /api/v1/workspaces/:workspace_id/functions/:function_key/resources/:resource_id`.
- Delete the target document in `function_resources` when it exists.
- Publish a JetStream CloudEvent after successful deletion.
- Make delete-event subject configurable by environment variable.
- Return HTTP `204 No Content` only after event publication succeeds.

## Non-Goals

- Do not add bulk delete endpoint.
- Do not add soft-delete or archive state.
- Do not introduce retry queue outside existing JetStream publish path.
- Do not change list API contract.
- Do not add frontend changes.

## API Contract

Endpoint:

```txt
[DELETE] /api/v1/workspaces/:workspace_id/functions/:function_key/resources/:resource_id
```

Path validation:

- `workspace_id` is required and must be non-empty.
- `function_key` is required and must be non-empty.
- `resource_id` is required and must be non-empty.

Success response:

- `204 No Content`
- Empty body.

Failure response (domain behavior):

- If target document does not exist, return `404 Not Found`.

Failure response (infrastructure behavior):

- If MongoDB delete fails unexpectedly, return `500 Internal Server Error`.
- If event publish fails after a successful delete, return `500 Internal Server Error`.

## Data Consistency and Operation Ordering

The delete workflow is:

1. Verify target by `(workspace_id, function_key, resource_id)`.
2. Delete matching `function_resources` document.
3. Publish delete event to JetStream.
4. Return `204`.

Rationale:

- The requirement states event is sent after document deletion.
- Returning `204` only after publish gives caller a stronger completion signal.

Trade-off:

- If deletion succeeds but publish fails, API returns `500` and projection is already deleted. This is accepted for initial scope; operators should rely on logs/metrics for remediation.

## Delete Event Contract

Default configurable subject/type:

```txt
app.todo.resource.deleted
```

Configuration rule:

- New required env var: `FUNCTION_SERVICE_JETSTREAM_RESOURCE_DELETED_SUBJECT`.
- Startup fails when this variable is missing or empty.

CloudEvent envelope:

```json
{
  "specversion": "1.0",
  "type": "app.todo.resource.deleted",
  "source": "function-service",
  "subject": "<RESOURCE_ID>",
  "id": "<UUID>",
  "time": "2026-05-06T12:00:00Z",
  "datacontenttype": "application/json",
  "data": {
    "resource_id": "<RESOURCE_ID>",
    "function_key": "<FUNCTION_KEY>",
    "workspace_id": "<WORKSPACE_ID>"
  }
}
```

Required data fields:

- `resource_id`
- `function_key`
- `workspace_id`

Notes:

- `subject` should equal `resource_id` for consumer routing consistency.
- `type` equals configured delete subject to match current upsert-event style.

## Service and Module Impact

Expected impacted modules:

- `internal/function-service/config`: add required delete subject config.
- `internal/function-service/handlers/resource_handler.go`: add delete route handler.
- `internal/function-service/services/resource_service.go`: add delete orchestration method.
- `internal/function-service/repositories/mongo_resource_repository.go`: add delete-by-scope method.
- `internal/function-service/transport`: add delete event DTO.
- `cmd/function-service/main.go`: wire new route and config.

## Error Mapping

Recommended mapping:

- Domain `resource not found` -> `404`.
- Validation error -> `400`.
- Repository/publisher internal error -> `500`.

## Testing Strategy

- Handler tests:
  - valid path + service success -> `204`.
  - resource not found -> `404`.
  - invalid path params -> `400`.
- Service tests:
  - delete success + publish success -> success.
  - delete success + publish failure -> error.
  - missing resource -> not-found error and no publish.
- Repository tests:
  - delete existing scoped document success.
  - scoped miss returns not-found.
- Config tests:
  - missing `FUNCTION_SERVICE_JETSTREAM_RESOURCE_DELETED_SUBJECT` returns startup error.

## Observability

Add structured logs:

- delete requested with `workspace_id`, `function_key`, `resource_id`.
- delete success before publish.
- publish success with event `id` and `type`.
- publish failure with same identifiers for replay/remediation.

Metrics recommendation (if existing metrics framework is already available in service):

- delete request count.
- delete success/failure count.
- delete event publish success/failure count.
