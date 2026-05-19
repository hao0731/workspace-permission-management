# In-Memory Permission Client Design

## Background

`function-service` needs a permission client dependency before the real external permission service client exists. The in-memory permission client is a placeholder implementation that satisfies the shared [Permission Client Design](permission-client-design.md) contract and allows the system resource save workflow to be wired and tested.

Related designs:

- [Permission Client Design](permission-client-design.md)
- [Function Service System Resource API Design](function-service-system-resource-api-design.md)

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- The in-memory client lives under `internal/shared/interactions/permission/inmemory` and remains service-agnostic.
- It depends only on the standard library, `internal/domain/resource`, and the shared permission client interface.
- It uses an injected `*slog.Logger` for runtime logging, with `slog.Default()` as the constructor default.
- It performs no network, database, or service-specific work.
- This design document is stored under `docs/designs/`.

## Goals

- Add `internal/shared/interactions/permission/inmemory/client.go`.
- Provide a concrete `Client` struct that implements `permission.Client`.
- Provide a simple constructor for composition-root wiring.
- Provide `WithLogger` so callers can inject the logger used by the in-memory client.
- Log `RegisterResourceAttributes` parameters at debug level through the injected logger.
- Always return `nil` from `RegisterResourceAttributes`.
- Make the implementation suitable for tests, local development, and temporary function-service wiring.

## Non-Goals

- Do not persist permission data.
- Do not perform HTTP, NATS, MongoDB, or other external calls.
- Do not validate resource attribute business rules; callers derive and validate attributes before invoking the client.
- Do not simulate permission registration failure in the production placeholder.
- Do not add configuration for this placeholder client.

## Package Contract

Package path:

```plaintext
internal/shared/interactions/permission/inmemory
```

File:

```plaintext
internal/shared/interactions/permission/inmemory/client.go
```

Shape:

```go
type Client struct {
	logger *slog.Logger
}

type Option func(*Client)

func WithLogger(logger *slog.Logger) Option

func New(opts ...Option) permission.Client

func (c *Client) RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error
```

Implementation behavior:

- `New` returns a `permission.Client`.
- `New` defaults `logger` to `slog.Default()`.
- `WithLogger` replaces the default logger only when the supplied logger is not nil.
- `RegisterResourceAttributes` logs with debug level and returns `nil`.
- The log should include at least:
  - `system_id`
  - `resource_attribute_count`
  - `resource_attributes`
- Use `c.logger.DebugContext(ctx, ...)` so request-scoped context can flow into logging where supported.
- The method must not mutate `resourceAttributes`.

Example log message:

```txt
register resource attributes with in-memory permission client
```

## Function Service Wiring

`cmd/function-service/main.go` should create this client while no real permission service client exists:

```go
permissionClient := inmemory.New(inmemory.WithLogger(logger))
systemResourceService := services.NewSystemResourceService(systemResourceRepository, limits,
	services.WithPermissionClient(permissionClient),
)
```

The exact option name can follow the implementation style chosen for `SystemResourceService`, but the dependency should be injected rather than created inside the service. This keeps the service testable and keeps the composition root responsible for concrete dependencies.

## Testing Strategy

In-memory client tests:

- Compile-time assertion that `Client` implements `permission.Client`.
- `WithLogger` directs debug logs to the injected logger.
- `New` defaults to `slog.Default()` when no logger is injected.
- `RegisterResourceAttributes` returns `nil`.
- `RegisterResourceAttributes` accepts an empty or non-empty slice without mutation.

Function service tests:

- Use a fake `permission.Client` for success and failure paths instead of relying on the in-memory implementation.
- Verify the in-memory client is only used in command wiring, not created inside service code.

Verification commands for implementation:

```bash
go test ./internal/shared/interactions/permission/...
go test ./internal/function-service/...
go test ./...
```

## Architecture Decisions

1. Use an in-memory shared interaction package rather than a function-service-local fake.
   - Rationale: The package represents the first concrete implementation of the shared permission client boundary and can be reused in tests or local composition.
   - Trade-off: It is not a real permission system and must not be mistaken for durable registration.

2. Return success unconditionally.
   - Rationale: The placeholder should not introduce operational behavior before the real permission dependency exists.
   - Trade-off: Local environments cannot exercise real registration failure without a separate fake in service tests.

3. Use debug-level structured logging.
   - Rationale: The placeholder should make calls observable during development without making normal logs noisy.
   - Trade-off: Debug logs may be disabled in some environments, so this implementation is not an audit mechanism.

4. Inject the logger with `WithLogger`.
   - Rationale: Tests and composition roots can control log output without mutating global slog state, and the placeholder follows the same dependency-injection style used by services.
   - Trade-off: The constructor has a small option surface even though the client currently has only one dependency.
