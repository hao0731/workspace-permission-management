# Permission API Client Design

## Background

`function-service` currently registers derived system resource attributes through the shared permission client boundary. The first implementation used an in-memory client so the workflow could be integrated before a real permission API existed.

This design adds the HTTP API client implementation for that same boundary. The API client sends the complete derived resource attribute set to a permission API at `POST /api/v1/schema/write`. Local development uses a lightweight `mock-permission-api` service that logs received payloads and returns success.

Related designs:

- [Permission Client Design](permission-client-design.md)
- [In-Memory Permission Client Design](inmemory-permission-client-design.md)
- [Function Service System Resource API Design](function-service-system-resource-api-design.md)

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- The shared `permission.Client` interface remains the service-facing contract under `internal/shared/interactions/permission`.
- HTTP transport details live in `internal/shared/interactions/permission/api`, behind the shared client boundary.
- `function-service` services continue to depend only on the shared interface and domain resource types, not HTTP client details or remote API DTOs.
- `cmd/function-service/main.go` remains the composition root that wires config and concrete clients.
- The mock permission API follows the backend service layout and keeps Echo request handling in handlers.
- The remote permission API JSON field names intentionally follow the external contract (`resAttr`, `isPublic`) even though new internal APIs normally prefer `snake_case`.
- This design is stored under `docs/designs/` and cross-linked from the affected permission client design.

## Goals

- Add `internal/shared/interactions/permission/api`.
- Implement `internal/shared/interactions/permission/client.go`'s `Client` interface with an HTTP API client.
- Construct the API client with `baseURL`, `apiKey`, and the API key header name.
- Send all API requests with:
  - `Content-Type: application/json`
  - `<apiKeyHeaderKey>: <apiKey>`
- Implement `RegisterResourceAttributes` by sending `POST <baseURL>/api/v1/schema/write`.
- Put request DTOs in `internal/shared/interactions/permission/api/request.go`.
- Put API error response DTOs and client error types in `internal/shared/interactions/permission/api/errors.go`.
- Add a `mock-permission-api` service that implements `POST /api/v1/schema/write`, logs the received payload, and returns `200 OK`.
- Replace the current `function-service` in-memory permission client wiring with the API client.
- Keep `SystemResourceService` and handler behavior unchanged except for the concrete client wiring.

## Non-Goals

- Do not change the `permission.Client` method signature.
- Do not make `condition` or `isPublic` caller-configurable in this phase.
- Do not add retry, outbox, circuit breaker, or background delivery behavior.
- Do not persist received payloads in `mock-permission-api`.
- Do not validate API keys in `mock-permission-api`.
- Do not introduce frontend changes.

## Package Structure

Expected additions:

```txt
internal/shared/interactions/permission/api/
  client.go
  request.go
  errors.go

cmd/mock-permission-api/
  main.go

internal/mock-permission-api/
  config/
  handlers/
```

Responsibilities:

- `internal/shared/interactions/permission/api/client.go`: concrete HTTP client, request construction, headers, response status handling, and JSON encoding/decoding.
- `internal/shared/interactions/permission/api/request.go`: remote schema-write request DTOs.
- `internal/shared/interactions/permission/api/errors.go`: remote error response DTO and exported client error type for non-2xx responses.
- `internal/mock-permission-api/config`: environment-based HTTP address, environment, and shutdown timeout config.
- `internal/mock-permission-api/handlers`: Echo route registration, body decode/logging, and response rendering.
- `cmd/mock-permission-api/main.go`: composition root, logger setup, health routes, route registration, server start, and graceful shutdown.

No package under `internal/shared/interactions/permission/api` may import `internal/function-service` or `internal/mock-permission-api`.

## Client Contract

Package path:

```txt
internal/shared/interactions/permission/api
```

Constructor:

```go
func New(baseURL string, apiKey string, apiKeyHeaderKey string, opts ...Option) permission.Client
```

Recommended test option:

```go
func WithHTTPClient(httpClient *http.Client) Option
```

Behavior:

- Trim whitespace from constructor string arguments.
- Trim trailing `/` from `baseURL` so endpoint construction is stable.
- Use `http.DefaultClient` unless `WithHTTPClient` receives a non-nil client.
- `function-service` config validation must reject blank `baseURL`, `apiKey`, and `apiKeyHeaderKey` before constructing the client.
- Invalid or unreachable URLs surface as client errors from `RegisterResourceAttributes`.
- The client uses the caller's `context.Context` when creating HTTP requests.

## Request Contract

File:

```txt
internal/shared/interactions/permission/api/request.go
```

Types:

```go
type RegisterResourceAttributesRelationRequest struct {
	ResourceAttribute resource.ResourceAttribute `json:"resAttr"`
	Condition         string                     `json:"condition"`
	IsPublic          bool                       `json:"isPublic"`
}

type RegisterResourceAttributesRequest struct {
	Definition string                                      `json:"definition"`
	Relations  []RegisterResourceAttributesRelationRequest `json:"relations"`
}
```

Mapping from the shared client method:

```go
RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error
```

Request mapping rules:

- `definition` is the normalized `systemID`.
- `relations` contains one item per supplied `resource.ResourceAttribute`.
- `relations[*].resAttr` is the resource attribute value.
- `relations[*].condition` is always `enable_dynamic_context`.
- `relations[*].isPublic` is always `false`.
- Preserve the supplied `resourceAttributes` order.
- Do not mutate the supplied slice.

Example payload:

```json
{
  "definition": "todo",
  "relations": [
    {
      "resAttr": "can_edit_private_repo",
      "condition": "enable_dynamic_context",
      "isPublic": false
    }
  ]
}
```

## HTTP Behavior

Endpoint:

```http
POST /api/v1/schema/write
```

The concrete client sends:

```http
POST <baseURL>/api/v1/schema/write
Content-Type: application/json
<apiKeyHeaderKey>: <apiKey>
```

Success behavior:

- Any `2xx` response means registration succeeded.
- The response body is ignored for success.

Failure behavior:

- Network, request construction, JSON marshal, and response body close errors return wrapped errors.
- Non-2xx responses return an API client error.
- When the response body matches the permission API error shape, include that decoded response in the client error.
- When the response body cannot be decoded as the permission API error shape, return an error that includes the HTTP status code and decode failure.

## Error Contract

File:

```txt
internal/shared/interactions/permission/api/errors.go
```

Remote error response:

```go
type ErrorResponse struct {
	Code    int    `json:"code"`
	Error   string `json:"error"`
	Message string `json:"message"`
}
```

Recommended client error:

```go
type Error struct {
	StatusCode int
	Response   ErrorResponse
}

func (e *Error) Error() string
```

`SystemResourceService` does not need to branch on this concrete error type. It continues to wrap any client error with `ErrPermissionRegistrationFailed`, and the function-service handler continues mapping that sentinel to `502 Bad Gateway`.

The permission API error shape is intentionally separate from `internal/shared/http/exception` because it is the remote permission API contract, not this repository's public HTTP error response contract.

## Function Service Configuration And Wiring

Add required config values:

- `FUNCTION_SERVICE_PERMISSION_API_BASE_URL`
- `FUNCTION_SERVICE_PERMISSION_API_KEY`
- `FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER`

Validation:

- Each value must be non-empty after trimming.
- `FUNCTION_SERVICE_PERMISSION_API_BASE_URL` should be an absolute `http` or `https` URL with a host.
- `FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER` must be a valid non-empty HTTP header name.

Local example values:

```env
FUNCTION_SERVICE_PERMISSION_API_BASE_URL=http://localhost:8086
FUNCTION_SERVICE_PERMISSION_API_KEY=dev-permission-api-key
FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER=X-API-Key
```

`cmd/function-service/main.go` should replace the current in-memory wiring:

```go
permissionClient := permissionapi.New(
	cfg.PermissionAPI.BaseURL,
	cfg.PermissionAPI.APIKey,
	cfg.PermissionAPI.APIKeyHeader,
)
```

`SystemResourceService` keeps the same dependency:

```go
services.NewSystemResourceService(systemResourceRepository, limits, permissionClient, ...)
```

No handler, service method signature, or domain model change is required.

## Mock Permission API

Service name:

```txt
mock-permission-api
```

Expected layout:

```txt
cmd/mock-permission-api
internal/mock-permission-api/config
internal/mock-permission-api/handlers
```

Configuration:

Required:

- `MOCK_PERMISSION_API_HTTP_ADDR`

Optional:

- `MOCK_PERMISSION_API_ENV`, default `development`
- `MOCK_PERMISSION_API_SHUTDOWN_TIMEOUT`, default `10s`

Endpoint:

```http
POST /api/v1/schema/write
```

Behavior:

- Decode the request body into `api.RegisterResourceAttributesRequest`.
- Log the payload through `slog.InfoContext`.
- Return `200 OK`.
- Do not persist the request.
- Do not validate the API key header.
- If JSON decoding fails, return `400 Bad Request` using the permission API error shape.
- Register `GET /health/liveness` through `internal/shared/health`.

Example validation error body:

```json
{
  "code": 400,
  "error": "validation_failed",
  "message": "Invalid schema write payload"
}
```

The mock uses the same request DTOs as the client so local tests detect payload drift.

## Local Development And Docker

`.env` and `.env.example` should include the function-service permission API values and mock service config.

`docker-compose.yml` should add only a `mock-permission-api` service using `go run ./cmd/mock-permission-api`, expose host port `8086`, and set:

```yaml
MOCK_PERMISSION_API_HTTP_ADDR: :8086
```

`function-service` does not need to be added to `docker-compose.yml` for this change. For local runs, start `function-service` outside Compose with `.env` values that point to `http://localhost:8086`.

## API Example

Add a REST Client example:

```txt
examples/api/mock_permission_api.http
```

The file should include:

- `@baseUrl = http://localhost:8086`
- `@apiKey = dev-permission-api-key`
- A success request for `POST /api/v1/schema/write`
- The configured API key header
- A malformed JSON example if the mock implementation returns validation errors

## Testing Strategy

API client tests:

- Compile-time assertion that `api.Client` implements `permission.Client`.
- Constructor trims `baseURL` trailing slash.
- `RegisterResourceAttributes` sends `POST /api/v1/schema/write`.
- Request includes `Content-Type: application/json`.
- Request includes the configured API key header and value.
- Request body maps `systemID` to `definition`.
- Request body maps each resource attribute to a relation with fixed `condition: enable_dynamic_context` and `isPublic: false`.
- Success on `2xx` returns `nil`.
- Non-2xx with valid permission error body returns an API client error containing status code and decoded body.
- Non-2xx with malformed error body returns an error.
- Request failure returns an error.
- Supplied resource attribute slice is not mutated.

Function-service config and wiring tests:

- Config loads and validates the three permission API settings.
- Missing or blank permission API settings fail startup.
- `cmd/function-service/main.go` wires `permissionapi.New`, not `permissioninmemory.New`.

Mock permission API tests:

- `POST /api/v1/schema/write` decodes a valid payload, logs it, and returns `200`.
- Malformed JSON returns `400` with the permission API error shape.
- Health liveness route returns success.

Verification commands for implementation:

```bash
go test ./internal/shared/interactions/permission/... ./internal/function-service/... ./internal/mock-permission-api/... ./cmd/function-service ./cmd/mock-permission-api
go test ./...
```

## Architecture Decisions

1. Keep the shared `permission.Client` interface unchanged.
   - Rationale: `function-service` already supplies all data needed by the current remote API when `definition` maps to `systemID` and relation metadata is fixed.
   - Trade-off: If future permission schema writes need per-relation conditions or visibility, the interface will need a new input model.

2. Put the HTTP implementation in `internal/shared/interactions/permission/api`.
   - Rationale: The API client is a reusable shared interaction implementation and should stay out of service-specific packages.
   - Trade-off: The package owns remote DTOs that are not used by domain logic, so callers must continue depending on the shared interface rather than these transport types.

3. Decode remote non-2xx bodies into the permission API error shape.
   - Rationale: This preserves remote error details for logs and tests without leaking remote error decisions into `function-service` handlers.
   - Trade-off: The service still maps all registration failures to the same `502 permission_registration_failed` response.

4. Use a mock permission API instead of extending the in-memory client for local development.
   - Rationale: The mock exercises real HTTP request construction, headers, and payload shape while keeping local setup deterministic.
   - Trade-off: Local development now needs one more process when exercising the full function-service registration path.

5. Keep registration synchronous.
   - Rationale: This preserves the existing post-commit behavior and HTTP response semantics.
   - Trade-off: A permission API outage can make the function-service POST return `502` after local MongoDB writes have committed.
