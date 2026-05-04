# Backend architecture principle

## Tech stack

- Language: [Golang](https://go.dev)
  - Minimum supported version: Go 1.25. Use of modern idioms (e.g., generics, `min` / `max` built-ins, `cmp` package) is encouraged.
- Web framework: [Echo](https://echo.labstack.com), version: `v5`.
- Logging: `log/slog`.
- Configuration: [viper](https://github.com/spf13/viper)
  - local development env file: `.env`
- Database:
  - [MongoDB](https://www.mongodb.com/)
- Message broker:
  - [NATS JetStream](https://docs.nats.io/nats-concepts/jetstream)
- Event envelope: [Cloudevents](https://github.com/cloudevents/sdk-go)

## Core principle

- Keep transport concerns separate from business logic.
- Keep domain logic independent from infrastructure and framework details.
- Keep handlers thin.
- Prefer explicit, readable code over clever abstractions.
- Follow existing repository structure, dependency direction, and naming conventions unless intentionally refactoring.
- Treat API, database, and event schemas as explicit contracts. Changes to those contracts require tests and migration or compatibility notes.
- Keep external side effects behind infrastructure boundaries so domain and service behavior can be tested deterministically.

---

## Directory rules

### Expected Layout

```plaintext
.
├── cmd
│   └── <server>
│       └── main.go
├── internal
│   ├── <server>
│   │   ├── config
│   │   ├── handlers
│   │   ├── services
│   │   ├── repositories
│   │   └── transport
│   ├── domain
│   │   └── <domain-name>
│   ├── shared
│   │   └── <shared-module>
│   └── utils
└── pkg
```

### Responsibilities

- `cmd/<server>`
  - application entrypoint
  - dependency wiring
  - bootstrap
  - startup and shutdown
- `internal/<server>/config`
  - service-specific configuration loading and validation
- `internal/<server>/handlers`
  - Echo handlers
  - NATS / NATS JetStream handlers
  - request parsing
  - response rendering
  - Error mapping
- `internal/<server>/services`
  - use cases
  - transaction boundaries where applicable
- `internal/<server>/repositories`
  - persistence access
  - database operations
  - storage implementations
- `internal/<server>/transport`
  - request/response DTOs
  - protocol-specific validation schemas
  - route registration or protocol glue code
  - transport-to-domain model mappers
- `internal/domain/<domain-name>`
  - domain models
  - business rules
  - invariants
  - domain errors
- `internal/shared/<shared-module>`
  - reusable internal packages shared across services.
- `internal/utils`
  - low-level helpers
  - small utility functions
- `pkg`
  - only for packages intentionally designed for external reuse
  - never place service-private code here

---
 
## Import Boundaries

### Allowed

All packages may import:
  - Go standard library packages

### Rules

- `cmd/<server>` is the composition root and may import what is needed to bootstrap the service.
- `internal/utils` must NOT depend on any `internal/*` package.
- `internal/domain/<name>` must NOT depend on:
  - Echo
  - database drivers
  - ORM libraries
  - HTTP clients
  - message broker clients
  - `internal/<server>/*` package
- `internal/shared/<name>` must NOT depend on any `internal/<server>/*` package.
- `internal/<server>/*` must NOT import another `internal/<other-server>/*` package directly.
- Define interfaces at the consumer side unless there is a clear shared contract.
- Do not introduce interfaces prematurely.
- Handlers may depend on services, transport DTOs/mappers, and domain errors; handlers must not call repositories directly.
- Transport packages may depend on domain packages for DTO-to-domain mapping, but must not depend on repositories or infrastructure clients.
- Services may depend on domain packages and consumer-side repository or publisher interfaces; services must not depend on Echo, NATS, NATS JetStream, MongoDB drivers, or transport DTOs.
- Repositories may depend on database drivers and domain packages, but must not depend on handlers, transport packages, or services.
- Event publishers and subscribers should expose domain-oriented methods to services and keep broker-specific details in infrastructure packages.

---

## Coding rules

### Style

- Follow the Uber Go Style Guide as the baseline.
- Group imports in this order:
  1. standard library
  2. third-party packages
  3. local packages
 
### Naming

- Use `PascalCase` for exported identifiers.
- Use `camelCase` for unexported identifiers.
- Prefer descriptive names over unclear abbreviations.

### Pointers and Values

- Use pointers for mutable state, large structs, or types that should avoid copying.
- Use values for small immutable structs, slices, maps, and channels.
- Never use pointers to interfaces.

### Error Handling

- Check errors immediately.
- Add context when returning errors.
- Prefer wrapping errors with `%w`.
- Do not silently ignore meaningful errors.
- Handle deferred cleanup errors intentionally.

### Constructors

- Prefer explicit config structs when configuration is naturally grouped.
- Use Functional Options only when there are many optional parameters or backward-compatible extensibility is needed.

### Context

- Always pass `context.Context` explicitly across:
  - handler -> service
  - service -> repository
- Never store `context.Context` in a struct.
- Do not create background contexts inside request paths unless explicitly required.

---

## Configuration Rules

- Configuration MUST come from environment variables.
- `.env` support is allowed for local development only.
- Missing `.env` files must not fail application startup.
- Required configuration keys must be validated explicitly.
- Invalid configuration formats must fail fast.
- Defaults may be used only for explicitly optional configuration keys.

---

## Logging rules

- Use `log/slog` exclusively for runtime logging.
- Do NOT use:
  - `fmt.Println`
  - `log.Println`
  - ad-hoc debug prints in committed code
 
### Environment Output

- development: `slog.NewTextHandler`
- production: `slog.NewJSONHandler`

### Structured Logging

Use consistent structured keys such as:

- `err`
- `request_id`
- `trace_id`

Prefer request-scoped logging with context propagation.
Domain packages should remain logger-agnostic unless logging is abstracted behind an interface.

---

## Handler rules

Handlers must remain thin.
Handlers are responsible for:
- request/event parsing
- path/query/header/body extraction
- triggering validation
- invoking service methods
- mapping results to response DTOs
- mapping service/domain errors to HTTP responses

Handlers must NOT contain heavy business logic.

### API contract rules

- Public JSON APIs should use `snake_case` field names by default. Existing endpoints may keep their established style for backward compatibility.
- Request/response DTOs must be defined in transport packages, not in handlers or domain packages.
- Request validation belongs at the transport boundary for shape, type, and basic format checks.
- Business invariant validation belongs in services or domain packages.
- API responses should not expose database models, driver types, or internal-only fields.
- API contract changes must include updates to frontend schemas, documentation, and tests when applicable.
- Every REST API endpoint or endpoint family must include a REST Client `.http` example file under `examples/api/`.
- REST Client example files should be split by API or endpoint family, not grouped into one repository-wide file.
- Each `.http` file should be executable against local development defaults and include:
  - variables such as `baseUrl` and representative path/query/body values
  - the primary success request
  - pagination, cursor, authentication, or required-header examples when the API uses them
  - at least one validation or error example when the API has request validation
- REST API contract changes must update the matching `examples/api/*.http` file in the same change.
- `.http` examples are developer-facing API documentation as well as manual test fixtures, so they must stay aligned with route paths, DTO field names, required parameters, and error shapes.

### DTO boundary rules

- Keep transport DTOs separate from domain models.
- Do not expose domain entities directly as HTTP JSON responses unless explicitly intended.

### Error mapping

Use a centralized and consistent strategy for:
- HTTP status codes
- response body shape
- logging behavior

Avoid ad-hoc error response formats per handler.

New HTTP APIs should use this error response shape unless an existing endpoint family already defines a compatible alternative:

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

- `code` must be stable enough for clients to branch on.
- `message` should be safe to display or log, and must not leak secrets or internal implementation details.
- `details` is optional and should contain field-level validation errors or structured diagnostic context.
- `request_id` should be included when available.

### Pagination and filtering

- List endpoints must document pagination, sorting, and filtering behavior explicitly.
- Prefer cursor pagination for large or append-heavy datasets.
- Offset pagination is acceptable only when the dataset is small or the trade-off is documented.
- Validate maximum page sizes at the transport boundary.

### Shutdown

Services must support graceful shutdown with timeout-bound context.

---

## Service and Domain Rules

### Services

Services should contain:
- business workflows
- orchestration logic
- transaction boundaries
- integration coordination

Services must NOT depend on Echo / NATS / NATS JetStream types.

### Domain

Domain packages should contain:
- core business rules
- invariants
- domain errors
- domain models and behavior

Domain code must remain framework-agnostic.

### Validation

- transport-level validation is for request shape and basic input checks
- domain/service-level validation is for business invariants

Do not rely only on handler-level validation for business correctness.

### Time and identifiers

- Inject time and ID generators when behavior depends on them and tests need determinism.
- Do not call wall-clock time, random generators, or UUID generators deep inside domain logic unless they are passed in or abstracted at the service boundary.

---

## Repository and data rules

- Map database documents into domain models before returning values to services.
- Do not leak paging state, or driver errors outside repository boundaries without mapping them to domain/service-level concepts.
- Schema changes must include migration or rollout notes, compatibility considerations, and repository tests or integration tests where practical.

---

## Event and messaging rules

### NATS JetStream

- Keep NATS and JetStream types out of domain and service logic.
- Subscribers must classify errors as retryable, non-retryable, or poison-message cases.
- Ack/nack/term behavior must be intentional and covered by tests where practical.
- Event handlers must be idempotent or explicitly document why duplicate delivery is safe.
- Durable consumer names, subjects, stream names, and retry policies must be treated as deployment contracts.

### CloudEvents

- Use CloudEvents as the event envelope for cross-service or brokered events.
- Event type names must be stable and namespaced, for example `com.example.resource.changed`.
- Events must include a stable `id`, `source`, `type`, `time` when available, and a documented data schema.
- Event payloads should use versioned schemas or backward-compatible evolution rules.
- Convert CloudEvents payloads into domain models before invoking service logic.

---

## Testing Rules

- Use the standard `testing` package by default.
- Prefer table-driven tests for business logic.
- Use `t.Run(...)` for named scenarios.
- Keep tests deterministic and behavior-focused.

### HTTP handler tests

- Use `net/http/httptest`

### Mocking

- Use `mockery` for interface-based mocks when mocks add value.
- Mock external systems and side effects when isolation is needed, including:
  - databases
  - brokers
  - external HTTP services
  - time providers
  - UUID generators
- Do not force mocks for pure logic when real values are simpler and clearer.

### Test Types

- Unit tests:
  - fast
  - isolated
  - deterministic
- Integration tests:
  - verify repository behavior
  - verify external integrations
  - may use real dependency containers where appropriate

### Required coverage by change type

- Domain rule changes must include unit tests for invariants and edge cases.
- Service workflow changes must include tests for success, validation failure, dependency failure, and relevant partial-failure behavior.
- Handler/API changes must include tests for request validation, status codes, response DTOs, and error mapping, plus matching REST Client examples under `examples/api/` for REST APIs.
- Repository changes must include tests that cover query behavior and mapping. Use integration tests when driver behavior or MongoDB query constraints matter.
- Event handler changes must include tests for parsing, idempotency, ack/nack decision behavior, and error classification where practical.
- Bug fixes must include a failing test or a documented reason why the bug cannot be reproduced in an automated test.

## Verification rules

- Prefer repository-provided commands such as Make targets or task runner commands when available.
- Backend changes should run `go test ./...` at minimum when Go code exists.
- Run `go vet ./...`, race tests, linting, or integration tests when the changed area or repository tooling makes them relevant.
- Do not mark backend work complete until relevant checks have either passed or been explicitly reported as skipped with reason and risk.
