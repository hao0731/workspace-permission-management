# Permission Client Design

## Background

System resource definitions managed by `function-service` produce derived resource attributes. Those attributes must be registered with an external permission system after `function-service` has persisted the local resource definition and attribute state.

This design defines the shared permission interaction boundary. The first concrete implementation is intentionally in-memory and is documented in [In-Memory Permission Client Design](inmemory-permission-client-design.md). The caller integration from the system resource API is documented in [Function Service System Resource API Design](function-service-system-resource-api-design.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- The client interface is an explicit shared interaction contract under `internal/shared/interactions/permission`.
- The shared interaction package must not depend on any service-specific package.
- The interface may depend on `internal/domain/resource` because resource attributes are part of the shared resource domain vocabulary.
- Concrete external transport concerns must stay behind this client boundary and out of `function-service` services, handlers, transport DTOs, and domain packages.
- This design document is stored under `docs/designs/` and cross-linked from related system resource designs.

## Goals

- Add `internal/shared/interactions/permission/client.go`.
- Define a shared `Client` interface for registering system resource attributes.
- Use `context.Context` for cancellation, deadlines, and request-scoped logging propagation.
- Accept `systemID` as the system identity currently equivalent to `function_key`.
- Accept derived `[]resource.ResourceAttribute` values from `internal/domain/resource`.
- Let callers treat any returned error as an upstream permission registration failure.
- Keep the interface stable so `function-service` can start with an in-memory implementation and later switch to a real permission service client from the composition root.

## Non-Goals

- Do not implement the real external permission service client in this phase.
- Do not define HTTP routes, payloads, authentication, retries, or timeouts for the future external permission service in this phase.
- Do not add asynchronous delivery, background workers, or outbox storage in this phase.
- Do not move resource attribute derivation out of `function-service` in this phase.
- Do not introduce frontend changes.

## Package Contract

Package:

```plaintext
internal/shared/interactions/permission
```

File:

```plaintext
internal/shared/interactions/permission/client.go
```

Interface:

```go
type Client interface {
	RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error
}
```

Import note:

- `client.go` imports `context`.
- `client.go` imports `github.com/hao0731/workspace-permission-management/internal/domain/resource`.
- Callers may alias this package as `clientpermission` or `permissionclient` when that improves readability near `internal/domain/permission` imports.

Input contract:

- `systemID` is the normalized system identifier used by `/api/v1/systems/:system_id`.
- `resourceAttributes` is the complete derived attribute set for the latest persisted resource definitions of that system.
- Callers should not invoke the client when the derived attribute set is empty because there is nothing meaningful to register.
- The client must not mutate the supplied slice.

Error contract:

- Return `nil` when registration is accepted or completed by the concrete implementation.
- Return an error when registration cannot be completed.
- The interface does not prescribe a concrete error type yet. Callers map errors at their own boundary.

## Function Service Usage

`function-service` uses this client after local persistence succeeds:

1. Save resource definitions and derived attributes in MongoDB inside the existing repository transaction.
2. Commit the MongoDB transaction.
3. If the derived attribute set is non-empty, call `RegisterResourceAttributes(ctx, systemID, attributes)`.
4. If registration fails, log the failure with `slog.Error` and return an upstream dependency error to the HTTP caller.

This is not a cross-system atomic transaction. A permission registration failure does not roll back already committed local MongoDB changes.

The first wiring uses `internal/shared/interactions/permission/inmemory.New(inmemory.WithLogger(logger))` in `cmd/function-service/main.go`. A future production client should replace only the concrete wiring and any required configuration, not the `function-service` service workflow contract.

## Future Async Option

If permission registration later needs retries, stronger delivery guarantees, or decoupling from POST latency, use an outbox strategy as a follow-up design. The likely shape is:

- Write a permission-registration outbox record in the same MongoDB transaction as the resource attribute document.
- Return success after the local transaction commits.
- Process outbox records with a background worker that calls the real permission client.
- Track retry count, last error, and delivery status for operational visibility.

That strategy is intentionally deferred because the current requirement is synchronous post-commit registration with an in-memory placeholder client.

## Testing Strategy

Interface package checks:

- `internal/shared/interactions/permission` compiles with only standard library and domain imports.
- Concrete clients assert they implement `permission.Client`.

Function service tests:

- Save calls `RegisterResourceAttributes` after a successful local transaction when derived attributes exist.
- Save does not call the client when derived attributes are empty.
- Client failure is logged and returned as an upstream dependency failure.
- Client failure does not undo the local repository calls already completed before registration.

Verification commands for implementation:

```bash
go test ./internal/shared/interactions/permission/... ./internal/function-service/...
go test ./...
```

## Architecture Decisions

1. Put the interface under `internal/shared/interactions/permission`.
   - Rationale: Permission registration is an external interaction used by service workflows and should be behind a shared internal boundary.
   - Trade-off: This creates a shared package that must remain stable and service-agnostic.

2. Pass `[]resource.ResourceAttribute` instead of transport DTOs or raw strings.
   - Rationale: Resource attributes are already a domain concept and this keeps callers away from external permission transport shapes.
   - Trade-off: A future real client may need a mapper from domain attributes to its remote API payload.

3. Keep registration synchronous for the first integration.
   - Rationale: The caller gets immediate feedback when permission registration fails, matching the requested API behavior.
   - Trade-off: The POST response can fail after local persistence has committed, so clients may need to retry registration through a future operation if a repeated POST is not appropriate.
