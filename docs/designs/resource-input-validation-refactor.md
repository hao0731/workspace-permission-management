---
doc_id: design.resource-input-validation-refactor
doc_type: design
title: Resource input validation refactor design
status: implemented

tags:
  - resource
  - validation
  - refactor

code_paths:
  - internal/domain/resource/**
  - internal/function-service/**
  - internal/workspace-service/**
  - internal/mock-function/**

related:
  designs:
    - design.resource-command-event-contracts
    - design.function-service
    - design.shared-pagination-helper-refactor
  adrs: []

last_updated_at: 2026-05-30

summary: >
  Read this when changing resource domain validation, transport-to-domain
  parsing, or shared resource input invariants.
---

# Resource Input Validation Refactor Design

## Status

Partially superseded for the resource upsert path by [resource-command-event-contracts.md](resource-command-event-contracts.md). Future upsert work should use `ResourceUpsertEvent.Validate()` directly instead of reintroducing `UpsertInput`. The `DeleteInput` and `ListQuery` validation decisions in this document remain current.

## Background

`function-service` currently validates resource workflow inputs through private helper functions in `internal/function-service/services/resource_service.go`:

- `validateUpsertInput`
- `validateDeleteInput`
- `validateListQuery`

The input and query types themselves live in `internal/domain/resource`. Moving these checks to `UpsertInput.Validate()`, `DeleteInput.Validate()`, and `ListQuery.Validate()` keeps the business invariant rules close to the framework-independent types that own those invariants.

The existing source design, [function-service.md](function-service.md), describes the service structure and validation behavior but does not name these helper functions. The implementation plans under `docs/plans/` contain historical code snippets that mention the helpers; those plans should be superseded by this design for future validation placement.

The later shared command/event ownership decision is documented in [resource-command-event-contracts.md](resource-command-event-contracts.md). That design extends `internal/domain/resource` with `ResourceCreateCommand` and `ResourceUpsertEvent`; it supersedes `UpsertInput` for the resource upsert workflow. This document remains current for `DeleteInput` and `ListQuery`, while its `UpsertInput` sections are historical context for the validation behavior that should move to `ResourceUpsertEvent.Validate()`.

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- Domain validation stays independent of Echo, MongoDB, NATS, JetStream, and transport DTOs.
- Handlers continue to parse transport input and map validation errors to HTTP responses.
- Services continue to own workflows, but they delegate input invariant checks to the domain input/query types.
- No API, event, database, or repository contract changes are introduced by this refactor.
- This design document is stored under `docs/designs/`.

## Goals

- Replace service-private validation helpers with methods on domain input/query types:
  - `func (input UpsertInput) Validate() error`
  - `func (input DeleteInput) Validate() error`
  - `func (query ListQuery) Validate() error`
- Preserve existing validation behavior and error messages.
- Preserve `errors.Is(err, resource.ErrInvalidInput)` behavior for all invalid input cases.
- Keep service methods responsible for workflow sequencing, repository calls, publish behavior, and repository/publisher error wrapping.
- Keep transport-only validation in transport packages, except shared pagination parsing/encode/decode moved to `internal/shared/pagination` per [shared-pagination-helper-refactor.md](shared-pagination-helper-refactor.md).

## Non-Goals

- Do not change HTTP routes, request fields, response fields, status codes, or error response shape.
- Do not change CloudEvent contracts, MongoDB schema, repository interfaces, or pagination token encoding.
- Do not add external dependencies.
- Do not add workspace or function existence validation.
- Do not move transport-only parsing concerns into the domain package.

## Recommended Approach

Add validation methods in `internal/domain/resource`, preferably in a focused `validation.go` file so `resource.go` can remain a compact type declaration file. The methods should use value receivers because they do not mutate state and the input/query structs are small enough to copy safely.

`ResourceService` should call the methods directly:

```go
if err := input.Validate(); err != nil {
	return "", err
}
```

```go
if err := query.Validate(); err != nil {
	return resource.Page{}, err
}
```

After the service methods delegate to the domain validation methods, remove `validateUpsertInput`, `validateDeleteInput`, and `validateListQuery` from `resource_service.go`. This also removes the service package's need for `strings` solely for validation.

Alternatives considered:

- Keep private service helpers. This minimizes code churn, but keeps invariant rules away from the types they validate and makes reuse by future service or repository-adjacent code less discoverable.
- Move all validation into transport. This would leave event-driven and non-HTTP service paths without a domain-level guard and would mix transport parsing rules with business input invariants.
- Create shared unexported validator functions in the domain package. This centralizes the logic, but method-based validation is easier to discover from the type API and matches the proposed call sites.

## Validation Ownership

Transport packages remain responsible for transport-specific shape and parsing checks:

- HTTP `limit` parsing, defaulting, and maximum limit enforcement via shared pagination helper (`internal/shared/pagination`).
- HTTP `next_token` base64url JSON decoding via shared pagination package (`internal/shared/pagination`).
- CloudEvent envelope validation and event data mapping.
- DTO-to-domain mapping.

Domain input/query validation methods are responsible for service workflow invariants:

- Required identity fields are non-empty after trimming whitespace.
- Upsert payloads include required display/type fields, non-empty tags, and a non-zero event time.
- List queries include a positive limit.
- List cursors include a non-zero `created_at` and non-empty ID when a cursor is present.

Services are responsible for:

- Calling `Validate()` before invoking repositories or publishers.
- Returning validation errors unchanged so handlers can keep using `errors.Is(err, resource.ErrInvalidInput)`.
- Wrapping repository and publisher errors with workflow context.

Repositories may continue to assume they receive already-validated inputs from services.

## Method Contracts

`UpsertInput.Validate()` must return `nil` only when:

- `ID` is non-empty after `strings.TrimSpace`.
- `WorkspaceID` is non-empty after `strings.TrimSpace`.
- `FunctionKey` is non-empty after `strings.TrimSpace`.
- `DisplayName` is non-empty after `strings.TrimSpace`.
- `Type` is non-empty after `strings.TrimSpace`.
- `EventTime` is not zero.
- Every tag in `Tags` is non-empty after `strings.TrimSpace`.

Invalid upsert inputs must keep the existing error messages:

- `resource id is required`
- `workspace id is required`
- `function key is required`
- `display name is required`
- `resource type is required`
- `event time is required`
- `resource tags must be non-empty strings`

`DeleteInput.Validate()` must return `nil` only when:

- `WorkspaceID` is non-empty after `strings.TrimSpace`.
- `FunctionKey` is non-empty after `strings.TrimSpace`.
- `ResourceID` is non-empty after `strings.TrimSpace`.

Invalid delete inputs must keep the existing error messages:

- `workspace id is required`
- `function key is required`
- `resource id is required`

`ListQuery.Validate()` must return `nil` only when:

- `WorkspaceID` is non-empty after `strings.TrimSpace`.
- `FunctionKey` is non-empty after `strings.TrimSpace`.
- `Limit` is greater than zero.
- If `Cursor` is present, `Cursor.CreatedAt` is not zero.
- If `Cursor` is present, `Cursor.ID` is non-empty after `strings.TrimSpace`.

Invalid list queries must keep the existing error messages:

- `workspace id is required`
- `function key is required`
- `limit must be greater than zero`
- `cursor created_at is required`
- `cursor id is required`

Every validation error must wrap `resource.ErrInvalidInput` with `%w`.

## Testing Strategy

Add domain validation tests in `internal/domain/resource/validation_test.go`:

- Valid `UpsertInput`, `DeleteInput`, and `ListQuery` return `nil`.
- Each invalid required field returns an error where `errors.Is(err, ErrInvalidInput)` is true.
- Blank strings containing only whitespace are invalid.
- Blank resource tags are invalid.
- Zero upsert event time is invalid.
- `ListQuery` rejects `Limit <= 0`.
- `ListQuery` rejects cursors with zero `CreatedAt` or blank `ID`.

Keep or adjust existing service tests so they continue proving:

- Invalid input returns `ErrInvalidInput` through the service boundary.
- Invalid input does not invoke repository or publisher behavior.
- Valid input still reaches repositories with unchanged values.
- Delete publish behavior remains unchanged after validation is moved.

Verification command:

```bash
go test ./...
```

## Rollout and Compatibility Notes

This is an internal refactor. It does not change the public API contract, CloudEvent contract, MongoDB schema, repository interface, or error mapping behavior.

The main compatibility requirement is preserving the existing `ErrInvalidInput` wrapping and human-readable validation messages so handlers, tests, and API clients observe the same validation failures.

## Architecture Decisions

1. Put validation methods on `resource.UpsertInput`, `resource.DeleteInput`, and `resource.ListQuery`.
   - Rationale: The validation rules describe domain input/query invariants and should be discoverable from the types that carry those values.
   - Trade-off: The domain package imports `fmt` and `strings` for validation helpers, but remains independent of framework, broker, and database dependencies.

2. Keep HTTP parsing and cursor-token decoding in transport.
   - Rationale: `limit` defaulting, maximum HTTP limit, and token encoding are transport concerns rather than core domain invariants.
   - Trade-off: Validation remains intentionally split between transport parsing and domain input invariants.

3. Preserve validation error text and sentinel wrapping.
   - Rationale: Existing handlers and tests rely on `errors.Is(err, resource.ErrInvalidInput)`, and stable messages avoid accidental API behavior changes.
   - Trade-off: The refactor keeps current message wording even where field-specific error codes could be more structured in a future API revision.

## Implementation Plan Notes

The follow-up implementation plan should update `docs/plans/active/` and link back to this design document and [function-service.md](function-service.md).

The implementation should be test-first: add domain validation tests that fail because the methods do not exist, implement the methods, update services to call them, remove the old helpers, and run `go test ./...`.
