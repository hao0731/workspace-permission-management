---
doc_id: design.workspace-service-api
doc_type: design
title: Workspace service API design
status: implemented

tags:
  - workspace
  - api
  - hr

code_paths:
  - internal/workspace-service/**
  - internal/domain/workspace/**
  - internal/shared/interactions/hr/**
  - cmd/workspace-service/**

related:
  designs:
    - design.workspace-service
    - design.workspace-service-command
    - design.mock-hr
  adrs: []

last_updated_at: 2026-05-30

summary: >
  Read this when changing workspace create/read/favorite APIs, HR lookup
  behavior, workspace persistence, or workspace response contracts.
---

# Workspace Service API Design

## Background

`workspace-service` creates workspace records, reads a workspace by ID with owner-enriched responses, and lets the current user set or clear a workspace favorite. The service stores the stable owner NT account in MongoDB, resolves display names through the shared HR client, and stores user-specific favorites separately.

Entry point and common service concerns are documented in [Workspace Service Design](workspace-service.md). Resource-create command publishing is documented in [Workspace Service Command Design](workspace-service-command-design.md). HR APIs and the shared HR client are documented in [Mock HR Design](mock-hr.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- The REST payload, response payload, and MongoDB document are explicit contracts.
- The `X-User-Id` request header for favorite mutations is an explicit transport contract.
- HTTP handlers remain thin and only parse input, path parameters, and headers, invoke services, and render mapped responses or errors.
- Request and response DTOs belong in `internal/workspace-service/transport`.
- Workspace domain types and validation remain independent of Echo, MongoDB, NATS, and HR HTTP client details.
- The service owns ID generation, timestamp assignment, HR lookup orchestration, persistence orchestration, nullable read orchestration, and post-persistence command publishing.
- Favorite mutations are service-owned workflows that verify workspace existence before writing user-specific favorite state.
- MongoDB access remains isolated in `internal/workspace-service/repositories`.
- This design is stored under `docs/designs/` and linked from the workspace-service entry design.

## Goals

- Expose `POST /api/v1/workspaces`.
- Expose `GET /api/v1/workspaces/:workspace_id`.
- Expose `POST /api/v1/workspaces/:workspace_id/favorite`.
- Accept `name`, `description`, `owner`, and optional `documents`, `tasks`, and `drive` resource-create requests.
- Accept `X-User-Id` as the current user's NT account for favorite mutations.
- Accept favorite mutation payload `{ "favorite": true }` or `{ "favorite": false }`.
- Resolve `owner` by calling `internal/shared/interactions/hr.Client.Get`.
- Fail the create request when HR lookup fails.
- Persist a workspace document after successful HR lookup.
- Store only the owner NT account in MongoDB.
- Return `201 Created` with the created workspace and HR display name.
- Return `200 OK` with the owner-enriched workspace when a workspace ID exists.
- Return `200 OK` with `"workspace": null` when a workspace ID does not exist.
- Return `204 No Content` when setting a favorite succeeds, clearing a favorite succeeds, or clearing a favorite finds no matching favorite document.
- Return `404 Not Found` when a favorite mutation targets a missing workspace.
- Persist favorite state in `user_favorite_workspaces` with `created_at` and `updated_at`.
- Keep resource-create command publishing outside the database transaction and outside the handler.
- Keep validation and persistence aligned with existing backend boundaries.

## Non-Goals

- Do not implement workspace list, update, delete, archive, or search APIs.
- Do not implement favorite list, favorite read, or favorite count APIs.
- Do not persist owner display names.
- Do not call HR for favorite mutations.
- Do not enforce workspace name uniqueness.
- Do not validate whether optional resource target applications exist in another service.
- Do not make command publishing transactional with workspace persistence.
- Do not add frontend changes.

## Create Workspace API

Endpoint:

```http
POST /api/v1/workspaces
```

Request body:

```json
{
  "name": "Workspace Name",
  "description": "Workspace description",
  "owner": "user1",
  "documents": {
    "resource_name": "Workspace documents"
  },
  "tasks": {
    "resource_name": "Workspace tasks"
  },
  "drive": {
    "resource_name": "Workspace drive"
  }
}
```

Field contract:

- `name`: required workspace name.
- `description`: required workspace description.
- `owner`: required owner NT account.
- `documents`: optional resource-create request for the configured documents app.
- `tasks`: optional resource-create request for the configured tasks app.
- `drive`: optional resource-create request for the configured drive app.
- `*.resource_name`: required when the corresponding optional object is present.

Success response:

```http
HTTP/1.1 201 Created
```

```json
{
  "workspace": {
    "id": "workspace-1",
    "name": "Workspace Name",
    "description": "Workspace description",
    "owner": {
      "nt_account": "user1",
      "display_name": "Test User 測試員"
    }
  }
}
```

Response field contract:

- `workspace.id` is generated by `workspace-service`.
- `workspace.name` and `workspace.description` are the persisted workspace values.
- `workspace.owner.nt_account` is the persisted `owner_nt_account`.
- `workspace.owner.display_name` comes from the HR client response and is not persisted in the workspace document.
- `created_at` and `updated_at` are persistence metadata and are not included in the public response in this phase.

## Get Workspace API

Endpoint:

```http
GET /api/v1/workspaces/:workspace_id
```

Path parameters:

- `workspace_id`: required workspace ID. This maps to `workspaces._id`.

Success response when the workspace exists:

```http
HTTP/1.1 200 OK
```

```json
{
  "workspace": {
    "id": "workspace-1",
    "name": "Workspace Name",
    "description": "Workspace description",
    "owner": {
      "nt_account": "user1",
      "display_name": "Test User 測試員"
    }
  }
}
```

Success response when the workspace does not exist:

```http
HTTP/1.1 200 OK
```

```json
{
  "workspace": null
}
```

Response field contract:

- `workspace.id`, `workspace.name`, and `workspace.description` come from the persisted workspace document.
- `workspace.owner.nt_account` is the persisted `owner_nt_account`.
- `workspace.owner.display_name` comes from the HR client response and is not persisted in the workspace document.
- `workspace: null` means no workspace document exists for the supplied `workspace_id`.
- `workspace: null` does not mean related groups, function resources, permissions, or external systems were checked.

## Favorite Workspace API

Endpoint:

```http
POST /api/v1/workspaces/:workspace_id/favorite
X-User-Id: user1
Content-Type: application/json
```

Path parameters:

- `workspace_id`: required workspace ID. This maps to `workspaces._id`.

Headers:

- `X-User-Id`: required current-user NT account. The service trims surrounding whitespace and does not lowercase or otherwise normalize account casing.

Request body:

```json
{
  "favorite": true
}
```

Field contract:

- `favorite`: required boolean.
- `favorite: true` means the current user wants the workspace marked as favorite.
- `favorite: false` means the current user wants the workspace favorite cleared.

Success response for favorite set or clear:

```http
HTTP/1.1 204 No Content
```

No response body is returned.

Behavior contract:

- The service must validate `workspace_id`, `X-User-Id`, and the request body before persistence.
- The service must verify that a workspace document exists for `_id = workspace_id` before writing favorite state.
- If no workspace document exists, return `404 Not Found` with error code `workspace_not_found`.
- If `favorite` is `true`, upsert one `user_favorite_workspaces` document for the trimmed `nt_account + workspace_id` pair.
- When upserting a new favorite document, set both `created_at` and `updated_at` to the same service-generated timestamp.
- When the favorite document already exists, keep the original `created_at` and update `updated_at` to the service-generated timestamp.
- If `favorite` is `false`, delete the matching `user_favorite_workspaces` document.
- If `favorite` is `false` and no matching favorite document exists, still return `204 No Content`.
- MongoDB upsert or delete errors return `500 Internal Server Error`.
- HR lookup is not part of this endpoint; `X-User-Id` is treated as the transport-provided current-user identity.

Rationale:

- The command-style `POST` matches the requested endpoint while allowing one payload to set or clear favorite state.
- Verifying workspace existence prevents dangling user favorite documents.
- Idempotent clear lets clients send `favorite: false` without first reading current state.
- Refreshing `updated_at` on repeated favorite set records the latest successful set request while preserving the original creation time.

## Request Validation

Create request transport-level validation should reject:

- Malformed JSON.
- Missing `name`.
- Empty or whitespace-only `name`.
- Missing `description`.
- Empty or whitespace-only `description`.
- Missing `owner`.
- Empty or whitespace-only `owner`.
- Present `documents` object with empty or whitespace-only `resource_name`.
- Present `tasks` object with empty or whitespace-only `resource_name`.
- Present `drive` object with empty or whitespace-only `resource_name`.

Normalization rules:

- Trim surrounding whitespace for `name`, `description`, `owner`, and `resource_name` fields before validation and persistence or command generation.
- Preserve internal whitespace.
- Do not lowercase `owner`; NT account normalization beyond trimming is outside this phase.

Domain or service validation should reject empty identity fields after transport mapping. Resource option validation should happen before persistence so invalid resource names never create a workspace.

Get request validation should reject:

- Empty or whitespace-only `workspace_id` after trimming.

The service should trim `workspace_id` before repository lookup and should not otherwise transform it.

Favorite request transport-level validation should reject:

- Malformed JSON.
- Missing request body.
- Missing `favorite`.
- `favorite` values that are `null` or not JSON booleans.
- Missing `X-User-Id`.
- Empty or whitespace-only `X-User-Id`.
- Empty or whitespace-only `workspace_id` after trimming.

Normalization rules:

- Trim surrounding whitespace for `workspace_id` and `X-User-Id` before validation and persistence.
- Preserve internal whitespace.
- Do not lowercase `X-User-Id`; NT account normalization beyond trimming is outside this phase.
- Decode `favorite` into a nullable or pointer boolean DTO field so `false` is distinguishable from an omitted field.

## HR Lookup

`workspace-service` uses the shared HR client interface:

```go
type Client interface {
	Get(ctx context.Context, ntAccount string) (hr.User, error)
	BatchGet(ctx context.Context, ntAccounts []string) ([]hr.User, error)
}
```

For create:

- Call `Client.Get(ctx, ownerNTAccount)` before writing the workspace.
- If the call succeeds, use the returned `hr.User.DisplayName` only for the response.
- If the call returns any error, return `502 Bad Gateway`.
- On HR failure, do not insert a workspace document and do not publish any resource-create commands.

For get:

- Read the workspace document first.
- If no workspace document exists, return `200 OK` with `"workspace": null` and do not call HR.
- If a workspace document exists, call `Client.Get(ctx, workspace.OwnerNTAccount)`.
- If the call succeeds, use the returned `hr.User.DisplayName` only for the response.
- If the call returns any error, return `502 Bad Gateway`.

For favorite:

- Do not call HR.
- Use the trimmed `X-User-Id` header value as the favorite `nt_account`.
- Do not validate the current user against HR in this phase.

Rationale:

- Create and found get responses require `owner.display_name`.
- The workspace document intentionally stores only `owner_nt_account`.
- Treating HR lookup failure as an upstream dependency failure keeps owner-enriched responses complete and deterministic.
- Favorite mutation does not return user display data, so an HR lookup would add dependency failure without improving the response contract.

## workspaces Collection

Collection: `workspaces`

Document schema:

```ts
{
  "_id": string,
  "name": string,
  "description": string,
  "owner_nt_account": string,
  "created_at": Date,
  "updated_at": Date
}
```

Field notes:

- `_id` is a service-generated UUID and is returned as `workspace.id`.
- `name` is the trimmed workspace name.
- `description` is the trimmed workspace description.
- `owner_nt_account` is the trimmed owner account from the request.
- `created_at` and `updated_at` are service-generated timestamps.
- Owner display name is intentionally not stored.

Repository operations:

- `Create(ctx, workspace)` inserts a new document.
- `Get(ctx, query)` reads one document by `_id`.
- Missing reads should return an empty optional result rather than a repository failure.

Indexes:

```txt
{ owner_nt_account: 1, created_at: -1, _id: -1 }
```

Rationale:

- The first implementation creates workspaces and reads them by `_id`; MongoDB already indexes `_id`.
- Indexing by owner and creation time remains a reasonable foundation for a future owner-scoped list API.
- No unique index is defined for `name` because workspace name uniqueness is not a confirmed product rule.

## user_favorite_workspaces Collection

Collection: `user_favorite_workspaces`

Document schema:

```ts
{
  "_id": string,
  "nt_account": string,
  "workspace_id": string,
  "created_at": Date,
  "updated_at": Date
}
```

Field notes:

- `_id` is a service-generated UUID for newly inserted favorite documents.
- `nt_account` is the trimmed value from request header `X-User-Id`.
- `workspace_id` is the trimmed path parameter and references `workspaces._id`.
- `created_at` is the service-generated timestamp for the first time the favorite document is inserted.
- `updated_at` is the service-generated timestamp for the latest successful `favorite: true` upsert.
- Clearing a favorite hard-deletes the matching document; no tombstone is stored in this phase.

Repository operations:

- `UpsertFavorite(ctx, input, now)` creates or updates one document by `nt_account + workspace_id`.
- `DeleteFavorite(ctx, input)` deletes one document by `nt_account + workspace_id`.
- `DeleteFavorite` treats `DeletedCount == 0` as a successful no-op and should not surface it as an error.
- The workspace repository must provide a way for the favorite workflow to verify workspace existence by `_id`; the implementation may reuse the existing read-by-ID repository method or add an existence-specific method inside the repository boundary.

Indexes:

```txt
unique { nt_account: 1, workspace_id: 1 }
```

Rationale:

- The unique index enforces at most one favorite document per user and workspace.
- A separate collection keeps user-specific state out of the workspace document and avoids rewriting shared workspace records for per-user preferences.
- Hard delete for clear keeps the first implementation simple and matches the requested clear-document behavior.

## Service Workflow

Successful create:

1. Validate and normalize request fields.
2. Call HR client `Get(ctx, ownerNTAccount)`.
3. Generate workspace ID and timestamps through injected generators.
4. Insert the workspace document into MongoDB.
5. Build zero or more resource-create commands from present `documents`, `tasks`, and `drive` sections.
6. Publish those commands using the best-effort behavior documented in [Workspace Service Command Design](workspace-service-command-design.md).
7. Return the created workspace with owner display name from HR.

Successful get with a found workspace:

1. Validate and normalize `workspace_id`.
2. Read the workspace document by `_id`.
3. Call HR client `Get(ctx, workspace.OwnerNTAccount)`.
4. Return the persisted workspace fields with owner display name from HR.

Successful get with a missing workspace:

1. Validate and normalize `workspace_id`.
2. Read the workspace document by `_id`.
3. Return `workspace: null` when the repository reports no document.
4. Do not call HR.

Successful favorite set:

1. Validate and normalize `workspace_id`, `X-User-Id`, and request body.
2. Read the workspace document by `_id` or perform an existence check inside the repository boundary.
3. If the workspace does not exist, return a missing-workspace error.
4. Generate one timestamp through the injected clock.
5. Upsert `user_favorite_workspaces` by `nt_account + workspace_id`.
6. Set `_id`, `nt_account`, `workspace_id`, and `created_at` only on insert.
7. Set `updated_at` on every successful upsert.
8. Return no payload.

Successful favorite clear:

1. Validate and normalize `workspace_id`, `X-User-Id`, and request body.
2. Read the workspace document by `_id` or perform an existence check inside the repository boundary.
3. If the workspace does not exist, return a missing-workspace error.
4. Delete one `user_favorite_workspaces` document by `nt_account + workspace_id`.
5. Treat a zero deleted count as success.
6. Return no payload.

Failure behavior:

- Request validation failure returns `400 Bad Request`.
- Missing workspace during favorite set or clear returns `404 Not Found`.
- HR lookup failure returns `502 Bad Gateway`; create performs no persistence or publishing, and get does not render a partial workspace.
- MongoDB insert failure returns `500 Internal Server Error` and performs no publishing.
- MongoDB read failure returns `500 Internal Server Error`.
- MongoDB favorite upsert or delete failure returns `500 Internal Server Error`.
- Resource command publish failure is logged and does not alter the `201 Created` response.

## Error Mapping

Known errors should use the shared backend error response shape:

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

- `400 Bad Request`: malformed JSON, invalid request shape, or invalid field values.
- `200 OK`: successful get, including a missing workspace represented as `"workspace": null`.
- `204 No Content`: successful favorite set or clear. Favorite clear also returns `204` when no favorite document existed.
- `404 Not Found`: favorite mutation targets a missing workspace.
- `502 Bad Gateway`: HR client lookup failure for the owner of a create request or a found workspace read.
- `500 Internal Server Error`: unexpected repository, favorite repository, ID generation, clock, or infrastructure failure.

Stable error codes:

- `validation_failed`
- `workspace_not_found`
- `hr_lookup_failed`
- `internal_error`

## REST Client Examples

`examples/api/workspaces.http` should include:

```http
@baseUrl = http://localhost:8083
@workspaceId = workspace-1
@userId = user1

### Create workspace without optional resources
POST {{baseUrl}}/api/v1/workspaces
Content-Type: application/json

{
  "name": "Planning Workspace",
  "description": "Workspace for planning",
  "owner": "user1"
}

### Create workspace with all optional resources
POST {{baseUrl}}/api/v1/workspaces
Content-Type: application/json

{
  "name": "Delivery Workspace",
  "description": "Workspace for delivery",
  "owner": "user1",
  "documents": {
    "resource_name": "Delivery documents"
  },
  "tasks": {
    "resource_name": "Delivery tasks"
  },
  "drive": {
    "resource_name": "Delivery drive"
  }
}

### Missing owner returns 400
POST {{baseUrl}}/api/v1/workspaces
Content-Type: application/json

{
  "name": "Missing Owner",
  "description": "Invalid workspace"
}

### Empty optional resource name returns 400
POST {{baseUrl}}/api/v1/workspaces
Content-Type: application/json

{
  "name": "Invalid Resource",
  "description": "Invalid workspace",
  "owner": "user1",
  "documents": {
    "resource_name": ""
  }
}

### Get workspace by ID
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}

### Get missing workspace returns workspace null
GET {{baseUrl}}/api/v1/workspaces/missing-workspace

### Mark workspace as favorite
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/favorite
Content-Type: application/json
X-User-Id: {{userId}}

{
  "favorite": true
}

### Clear workspace favorite idempotently
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/favorite
Content-Type: application/json
X-User-Id: {{userId}}

{
  "favorite": false
}

### Missing favorite header returns 400
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/favorite
Content-Type: application/json

{
  "favorite": true
}

### Missing favorite field returns 400
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/favorite
Content-Type: application/json
X-User-Id: {{userId}}

{}

### Favorite missing workspace returns 404
POST {{baseUrl}}/api/v1/workspaces/missing-workspace/favorite
Content-Type: application/json
X-User-Id: {{userId}}

{
  "favorite": true
}
```

The implementation should align the default `baseUrl` with the local Docker Compose HTTP port chosen for `workspace-service`.

## Testing Strategy

Domain tests:

- Create input rejects empty `name`, `description`, and `owner`.
- Create input rejects empty optional resource names when present.
- Create input accepts omitted `documents`, `tasks`, and `drive`.
- Create input trims string fields before validation.
- Get query rejects empty `workspace_id`.
- Get query trims `workspace_id`.
- Favorite input rejects empty `workspace_id` and `nt_account`.
- Favorite input trims `workspace_id` and `nt_account`.
- Favorite input preserves `favorite: false` as an explicit clear command.

Transport tests:

- Decode valid create request with no optional resources.
- Decode valid create request with all optional resources.
- Decode malformed JSON as validation failure.
- Decode valid favorite request with `favorite: true`.
- Decode valid favorite request with `favorite: false`.
- Decode favorite request with missing `favorite` as validation failure.
- Decode favorite request with `favorite: null` or non-boolean `favorite` as validation failure.
- Map domain workspace and HR user into the expected response DTO.
- Map domain workspace and HR user into the expected get response DTO.
- Encode missing get responses as exactly `{ "workspace": null }`.

Service tests:

- Successful create calls HR before repository insert.
- Successful create persists only `owner_nt_account`, not display name.
- Successful create returns owner display name from HR.
- HR failure returns an upstream dependency error, inserts no document, and publishes no commands.
- Repository insert failure publishes no commands.
- Publish failures after insert are logged and do not fail the service result.
- Commands are attempted in deterministic order: `documents`, `tasks`, then `drive`.
- Successful get reads the repository before calling HR.
- Successful get returns owner display name from HR.
- Missing get returns an empty optional result and does not call HR.
- HR failure for a found get returns an upstream dependency error.
- Repository read failure returns an internal error.
- Favorite set validates workspace existence before writing favorite state.
- Favorite set returns missing-workspace error and does not write favorite state when the workspace is absent.
- Favorite set upserts by `nt_account + workspace_id`, sets `created_at` on insert, and updates `updated_at`.
- Favorite set does not call HR.
- Favorite clear validates workspace existence before deleting favorite state.
- Favorite clear returns success when no favorite document existed.
- Favorite repository errors return an internal error.

Repository tests:

- Insert writes `_id`, `name`, `description`, `owner_nt_account`, `created_at`, and `updated_at`.
- Insert does not write owner display name.
- Get reads by `_id`.
- Get returns an empty optional result when MongoDB reports no document.
- Ensure indexes creates the owner-created index.
- Favorite upsert inserts `_id`, `nt_account`, `workspace_id`, `created_at`, and `updated_at`.
- Favorite upsert for an existing document preserves `created_at` and updates `updated_at`.
- Favorite delete removes by `nt_account + workspace_id`.
- Favorite delete returns success when no document matches.
- Ensure indexes creates the unique favorite `nt_account + workspace_id` index.

Handler tests:

- `POST /api/v1/workspaces` returns `201` and the response contract.
- `GET /api/v1/workspaces/:workspace_id` returns `200` and the workspace response contract when found.
- `GET /api/v1/workspaces/:workspace_id` returns `200` with `workspace: null` when missing.
- `POST /api/v1/workspaces/:workspace_id/favorite` returns `204` for `favorite: true`.
- `POST /api/v1/workspaces/:workspace_id/favorite` returns `204` for `favorite: false`, including no-op deletes.
- Missing or empty `X-User-Id` returns `400` with shared error shape.
- Missing or invalid `favorite` returns `400` with shared error shape.
- Favorite mutation for a missing workspace returns `404` with `workspace_not_found`.
- Validation failures return `400` with shared error shape.
- HR failures return `502` with `hr_lookup_failed`.
- Repository failures return `500`.
