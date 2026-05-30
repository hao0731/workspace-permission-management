---
doc_id: design.shared-pagination-helper-refactor
doc_type: design
title: Shared pagination helper refactor
status: implemented

tags:
  - pagination
  - shared
  - refactor

code_paths:
  - internal/shared/pagination/**
  - internal/*/handlers/**

related:
  designs:
    - design.function-service
  adrs: []

last_updated_at: 2026-05-30

summary: >
  Read this when changing shared cursor pagination parsing, token handling, or
  handler integration with the pagination helper.
---

# Shared Pagination Helper Refactor

## Background and Goals

`internal/function-service/transport/pagination.go` currently mixes two concerns:

1. HTTP query parameter parsing (`limit`, `next_token`) tied to Echo handlers.
2. Cursor token encode/decode utilities that are logically reusable.

This design introduces a shared pagination package so handlers can inject a helper for transport parsing while using package-level cursor utilities for token serialization/deserialization.

Goals:

- Add `internal/shared/pagination` as the canonical pagination utility package.
- Introduce a configurable `PaginationHelper` for handler-side query parsing.
- Keep existing limit/token validation behavior compatible with current `pagination.go` behavior.
- Remove `internal/function-service/transport/pagination.go` and migrate callers.

## Scope and Boundaries

In scope:

- New package: `internal/shared/pagination`.
- New helper struct: `PaginationHelper`.
- Constructor + options:
  - `New(...Option) *PaginationHelper`
  - `WithDefaultLimit(int)` default value: `20`
  - `WithMaxLimit(int)` default value: `50`
- Helper methods:
  - `ParseLimit(*echo.Context) (int, error)`
  - `ParseToken(*echo.Context) (string, error)`
- Package functions:
  - `EncodeNextToken[T any](input T) (string, error)`
  - `DecodeNextToken[T any](raw string) (T, error)`
- Handler wiring through dependency injection of `PaginationHelper`.

Out of scope:

- Changing API contract field names (`limit`, `next_token`).
- Changing cursor token format or error shape semantics beyond relocation.
- Introducing new external dependencies.

## Design Decisions and Rationale

### 1) Shared package placement

Decision: place pagination utilities under `internal/shared/pagination`.

Rationale:

- Pagination behavior is cross-cutting and not unique to `function-service` transport.
- Keeps handler logic thin while preventing transport package duplication across services.

Trade-off:

- Shared package becomes a central dependency and needs stable interfaces.

### 2) Helper + options pattern

Decision: use `PaginationHelper` with option-based constructor.

Rationale:

- Gives composition root explicit control over default and max limit values.
- Keeps defaults safe when no option is provided.

Defaults:

- `defaultLimit = 20`
- `maxLimit = 50`

Trade-off:

- Slightly more setup in handlers versus pure package-level parsing function.

### 3) ParseLimit behavior compatibility

Decision: `ParseLimit` will read `(*ctx).QueryParam("limit")` and preserve current limit legality rules from existing implementation.

Rationale:

- Refactor should preserve externally observable behavior where possible.
- Avoids API regressions in list endpoints.

### 4) ParseToken strict empty check

Decision: `ParseToken` reads `(*ctx).QueryParam("next_token")`; if the result is empty string (`""`), it returns an error.

Rationale:

- This is explicitly required by current refactor request.
- Establishes a clear contract for downstream decode flow.

Trade-off:

- Handlers must decide whether endpoint usage requires token optionality before calling `ParseToken`.

### 5) Generic encode/decode API

Decision:

- `EncodeNextToken[T any]` accepts generic input for encoding.
- `DecodeNextToken[T any]` accepts raw string and decodes into generic output type.

Rationale:

- Enables reuse for different cursor DTO shapes without interface{} casts.
- Keeps type safety at call sites.

Trade-off:

- Generic API may surface type-mismatch errors at instantiation time; call sites must provide concrete token DTO types.

## Proposed Package API (Draft)

```go
package pagination

type Option func(*PaginationHelper)

type PaginationHelper struct {
	defaultLimit int
	maxLimit     int
}

func New(opts ...Option) *PaginationHelper
func WithDefaultLimit(limit int) Option
func WithMaxLimit(limit int) Option

func (h *PaginationHelper) ParseLimit(ctx *echo.Context) (int, error)
func (h *PaginationHelper) ParseToken(ctx *echo.Context) (string, error)

func EncodeNextToken[T any](input T) (string, error)
func DecodeNextToken[T any](raw string) (T, error)
```

## Integration Impact

- Remove `internal/function-service/transport/pagination.go`.
- Update handlers to receive `*pagination.PaginationHelper` from composition root.
- Keep transport/domain/service boundaries consistent with the function-service design policy.

Related existing design document:

- Function service architecture and transport boundary notes: [function-service.md](function-service.md)

## Risks and Mitigations

- Risk: behavior drift during migration.
  - Mitigation: copy and adapt existing pagination tests to new shared package and update handler integration tests.
- Risk: misunderstanding of strict empty-token behavior.
  - Mitigation: explicitly document `ParseToken` semantics and add test cases for empty `next_token`.

## Expected Follow-up

After this design is approved, create an implementation plan under `docs/plans/active/` that references this document and breaks migration into test-first tasks.
