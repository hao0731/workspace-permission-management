---
doc_id: design.permission-client
doc_type: design
title: Permission registration consumer interface design
status: implemented

tags:
  - permission
  - consumer-interface
  - registration

code_paths:
  - internal/function-service/services/**
  - internal/shared/interactions/permission/**

related:
  designs:
    - design.permission-api-client
    - design.function-service-system-resource-api
  adrs: []

last_updated_at: 2026-05-30

summary: >
  Read this when changing consumer-side permission registration interfaces or
  function-service integration with shared permission interactions.
---

# Permission Registration Consumer Interface Design

## Background

`function-service` registers derived system resource attributes with the permission API after local system resource persistence commits. The previous design placed a shared `Client` interface in `internal/shared/interactions/permission/client.go`. The refactor removes that shared interface and applies the backend architecture policy rule that interfaces should be defined at the consumer side unless there is a clear shared contract.

The HTTP permission API implementation is documented in [Permission API Client Design](permission-api-client-design.md). The caller integration from the system resource API is documented in [Function Service System Resource API Design](function-service-system-resource-api-design.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- `internal/function-service/services` owns the `PermissionClient` interface because `SystemResourceService` is the consumer of permission registration.
- `internal/shared/interactions/permission` owns the concrete HTTP permission API client and DTOs, not a shared interface.
- The shared permission package must not depend on `internal/function-service`, `internal/group-service`, or `internal/mock-permission-api`.
- `cmd/function-service/main.go` remains the composition root that constructs the concrete permission API client and injects it into `SystemResourceService`.
- This design document is stored under `docs/designs/` and cross-linked from related system resource and permission API designs.

## Goals

- Remove `internal/shared/interactions/permission/client.go` as a shared interface contract.
- Define the narrow permission registration interface in `internal/function-service/services`.
- Keep the method shape `RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error`.
- Keep `context.Context` propagation and the existing post-commit registration behavior.
- Let `cmd/function-service/main.go` inject the concrete HTTP permission client from `internal/shared/interactions/permission`.
- Remove the in-memory permission client from source and design documentation.

## Non-Goals

- Do not change the permission API schema-write payload in this refactor.
- Do not make permission registration asynchronous.
- Do not add retry, outbox, circuit breaker, or background delivery behavior.
- Do not move resource attribute derivation out of `function-service`.
- Do not introduce frontend changes.

## Consumer Interface Contract

Package:

```plaintext
internal/function-service/services
```

Interface:

```go
type PermissionClient interface {
	RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error
}
```

Input contract:

- `systemID` is the normalized system identifier used by `/api/v1/systems/:system_id`.
- `resourceAttributes` is the complete derived attribute set for the latest persisted resource definitions of that system.
- `SystemResourceService` should not invoke the client when the derived attribute set is empty.
- The concrete client must not mutate the supplied slice.

Error contract:

- Return `nil` when registration is accepted or completed by the concrete implementation.
- Return an error when registration cannot be completed.
- `SystemResourceService` maps any returned error to `ErrPermissionRegistrationFailed`.

## Function Service Usage

`function-service` uses this client after local persistence succeeds:

1. Save resource definitions and derived attributes in MongoDB inside the existing repository transaction.
2. Commit the MongoDB transaction.
3. If the derived attribute set is non-empty, call `RegisterResourceAttributes(ctx, systemID, attributes)`.
4. If registration fails, log the failure with `slog.ErrorContext` and return an upstream dependency error to the HTTP caller.

This is not a cross-system atomic transaction. A permission registration failure does not roll back already committed local MongoDB changes.

Runtime wiring should use the API client from `internal/shared/interactions/permission`:

```go
permissionClient := permission.New(
	cfg.PermissionAPI.BaseURL,
	cfg.PermissionAPI.APIKey,
	cfg.PermissionAPI.APIKeyHeader,
)
```

`cmd/function-service/main.go` is responsible for constructing the concrete client from configuration and injecting it into `SystemResourceService`.

## Testing Strategy

Function service tests:

- `SystemResourceService` accepts a consumer-side fake that implements `services.PermissionClient`.
- Save calls `RegisterResourceAttributes` after a successful local transaction when derived attributes exist.
- Save does not call the client when derived attributes are empty.
- Client failure is logged and returned as an upstream dependency failure.
- Client failure does not undo the local repository calls already completed before registration.
- Function-service config validates the required permission API settings.
- Function-service composition wiring uses the concrete API client from `internal/shared/interactions/permission`.

Permission API client tests are owned by [Permission API Client Design](permission-api-client-design.md).

Verification commands for implementation:

```bash
go test ./internal/function-service/services ./cmd/function-service ./internal/shared/interactions/permission/...
go test ./...
```

## Architecture Decisions

1. Move the interface to `internal/function-service/services`.
   - Rationale: `SystemResourceService` is the consumer and needs only one method, so a shared interface creates an unnecessary cross-package contract.
   - Trade-off: Additional consumers must define their own narrow interface if they need permission registration.

2. Keep the concrete API client in `internal/shared/interactions/permission`.
   - Rationale: The HTTP client and DTOs are reusable internal integration code, but they are implementation details rather than the service-facing abstraction.
   - Trade-off: The package name `permission` now refers to the concrete API client package, so imports near `internal/domain/permission` should use aliases when both are present.

3. Remove the in-memory client.
   - Rationale: Runtime wiring now uses the HTTP API client, and service tests already use local fakes for success and failure paths.
   - Trade-off: Local development that wants a fake upstream must run `mock-permission-api` instead of swapping in an in-memory implementation.

4. Keep registration synchronous for this refactor.
   - Rationale: The refactor changes package boundaries only and preserves current HTTP response semantics.
   - Trade-off: A permission API outage can still make the function-service POST return `502` after local MongoDB writes have committed.
