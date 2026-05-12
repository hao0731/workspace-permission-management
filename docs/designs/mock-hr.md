# Mock HR Design

## Background

`mock-hr` is a local integration service that provides deterministic user profile responses for services that need HR data. `workspace-service` uses it through a shared HR client to resolve owner display names during workspace creation.

Related designs:

- [Workspace Service Design](workspace-service.md)
- [Workspace Service API Design](workspace-service-api-design.md)

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- HR API payloads and shared client interfaces are explicit contracts.
- HTTP handlers remain thin and only parse path/body input, invoke service behavior, and render responses or mapped errors.
- HR domain types stay independent of Echo and HTTP client details.
- The shared interaction package exposes an interface; the concrete mock HR HTTP client lives under `internal/shared/interactions/hr/poc`.
- This design is stored under `docs/designs/`.

## Goals

- Build `mock-hr` as an independent backend service.
- Expose `GET /api/v1/users/:nt_account`.
- Expose `POST /api/v1/user-list`.
- Return deterministic user data for any non-empty NT account.
- Use display name `Test User 測試員`.
- Add `internal/domain/hr` with the shared `User` model.
- Add `internal/shared/interactions/hr/client.go` with the shared `Client` interface.
- Add `internal/shared/interactions/hr/poc/mock_hr_client.go` as the concrete HTTP client for `mock-hr`.
- Configure the mock HR API host through the caller service config.
- Use `internal/shared/health` for liveness.

## Non-Goals

- Do not persist users.
- Do not model departments, titles, managers, or employee attributes.
- Do not implement authentication or authorization.
- Do not support user search.
- Do not simulate user-not-found behavior in the first version.
- Do not add frontend changes.

## Domain Model

Package:

```plaintext
internal/domain/hr
```

Model:

```go
type User struct {
	NTAccount   string
	DisplayName string
}
```

Validation:

- `NTAccount` must be non-empty after trimming.
- `DisplayName` must be non-empty after trimming.

The domain package must not depend on Echo, HTTP clients, config packages, or mock service packages.

## Mock HR API

### Get User

Endpoint:

```http
GET /api/v1/users/:nt_account
```

Success response:

```http
HTTP/1.1 200 OK
```

```json
{
  "user": {
    "nt_account": "user1",
    "display_name": "Test User 測試員"
  }
}
```

Behavior:

- Trim `:nt_account` before validation and response rendering.
- Return `200 OK` for any non-empty account.
- Always return `display_name` as `Test User 測試員`.
- Return `400 Bad Request` for empty or whitespace-only account values.

### Batch Get Users

Endpoint:

```http
POST /api/v1/user-list
```

Request body:

```json
{
  "nt_accounts": ["user1", "user2"]
}
```

Success response:

```http
HTTP/1.1 200 OK
```

```json
{
  "users": [
    {
      "nt_account": "user1",
      "display_name": "Test User 測試員"
    },
    {
      "nt_account": "user2",
      "display_name": "Test User 測試員"
    }
  ]
}
```

Behavior:

- `nt_accounts` is required.
- `nt_accounts` must be a non-empty array.
- Each account is trimmed before validation and response rendering.
- Every account must be non-empty after trimming.
- Response order matches request order.
- Duplicate accounts are allowed and return duplicate response entries.
- Return `400 Bad Request` for malformed JSON, missing `nt_accounts`, empty array, or empty account values.

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

- `400 Bad Request`: malformed JSON, missing account values, empty account values, or empty batch input.
- `500 Internal Server Error`: unexpected service failure.

Stable error codes:

- `validation_failed`
- `internal_error`

## Shared HR Client Interface

Package:

```plaintext
internal/shared/interactions/hr
```

Interface:

```go
type Client interface {
	Get(ctx context.Context, ntAccount string) (domainhr.User, error)
	BatchGet(ctx context.Context, ntAccounts []string) ([]domainhr.User, error)
}
```

Implementation note:

- The source file can import `internal/domain/hr` with an alias such as `domainhr` because the shared interaction package is also named `hr`.
- The interface belongs in the shared interaction package so services can depend on a stable boundary without importing the concrete mock client.
- Services should define behavior-specific errors or map client errors at their own boundary. `workspace-service` maps any owner lookup error to `502 Bad Gateway`.

Client validation:

- `Get` should reject empty `ntAccount` before making an HTTP request.
- `BatchGet` should reject empty arrays and empty account values before making an HTTP request.

## POC Mock HR Client

Package path:

```plaintext
internal/shared/interactions/hr/poc
```

File:

```plaintext
internal/shared/interactions/hr/poc/mock_hr_client.go
```

Constructor:

```go
func New(baseURL string) hr.Client
```

The implementation may add functional options later if tests need a custom `*http.Client`, but the first public constructor should keep caller wiring simple.

Behavior:

- `baseURL` comes from caller service config, such as `WORKSPACE_SERVICE_HR_BASE_URL`.
- `Get` calls `GET <baseURL>/api/v1/users/{url-escaped-nt-account}`.
- `BatchGet` calls `POST <baseURL>/api/v1/user-list`.
- Decode the `user` and `users` response envelopes into `internal/domain/hr.User`.
- Non-2xx responses return an error.
- Malformed responses return an error.
- HTTP request failures return an error.
- The client should use request contexts supplied by callers.

Rationale:

- Keeping the concrete client under `poc` makes it clear this implementation is for the mock HR service and local proof-of-concept wiring.
- The shared `Client` interface remains stable if a future production HR client replaces the POC client.

## Mock HR Service Architecture

Expected layout:

```plaintext
cmd/mock-hr
internal/mock-hr/config
internal/mock-hr/handlers
internal/mock-hr/services
internal/mock-hr/transport
internal/domain/hr
internal/shared/interactions/hr
internal/shared/interactions/hr/poc
```

Responsibilities:

- `cmd/mock-hr`: composition root. It loads config, creates the logger, registers health and HR routes, starts Echo, and handles graceful shutdown.
- `internal/mock-hr/config`: environment-based config for HTTP address, environment, and shutdown timeout.
- `internal/mock-hr/handlers`: Echo handlers for path/body parsing, service invocation, and error mapping.
- `internal/mock-hr/services`: deterministic user generation.
- `internal/mock-hr/transport`: request and response DTOs.
- `internal/domain/hr`: shared user model.
- `internal/shared/interactions/hr`: client interface.
- `internal/shared/interactions/hr/poc`: HTTP client for the mock HR API.

## Configuration

`mock-hr` required settings:

- `MOCK_HR_HTTP_ADDR`

`mock-hr` optional settings:

- `MOCK_HR_ENV`: default `development`
- `MOCK_HR_SHUTDOWN_TIMEOUT`: default `10s`

Caller service setting:

- `WORKSPACE_SERVICE_HR_BASE_URL`

Validation rules:

- Required string values must be non-empty after trimming.
- Environment must be a known value from `internal/shared/environment`.
- Shutdown timeout must be positive.
- Caller-provided HR base URL must be non-empty and parseable as an absolute HTTP or HTTPS URL.

## Health and Shutdown

`cmd/mock-hr/main.go` should use `internal/shared/health`.

Liveness endpoint:

```http
GET /health/liveness
```

The first implementation should use a `process` indicator. The service must support graceful shutdown with a timeout-bound context.

## REST Client Examples

The implementation should add `examples/api/mock_hr.http` with:

- Successful `GET /api/v1/users/:nt_account`.
- Successful `POST /api/v1/user-list`.
- Validation error for empty batch list.
- Validation error for empty account in batch list.

## Testing Strategy

Domain tests:

- `hr.User` validation rejects empty `NTAccount`.
- `hr.User` validation rejects empty `DisplayName`.

Transport tests:

- Decode valid batch request.
- Reject malformed JSON.
- Reject missing `nt_accounts`.
- Reject empty `nt_accounts`.
- Reject empty account values.
- Render get response with a top-level `user` object.
- Render batch response with a top-level `users` array.

Service tests:

- Get returns requested NT account and fixed display name.
- BatchGet preserves request order.
- BatchGet preserves duplicates.

Handler tests:

- `GET /api/v1/users/:nt_account` returns `200`.
- Empty account returns `400`.
- `POST /api/v1/user-list` returns `200`.
- Invalid batch input returns `400`.

Client tests:

- `Get` calls the expected URL path with URL escaping.
- `Get` decodes the user envelope.
- `BatchGet` sends the expected JSON body.
- `BatchGet` decodes the users envelope.
- Non-2xx responses return an error.
- Malformed responses return an error.
- Empty inputs are rejected before HTTP calls.
