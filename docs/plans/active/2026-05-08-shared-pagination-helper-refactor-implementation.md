# Shared Pagination Helper Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate pagination parsing and cursor token encode/decode from `internal/function-service/transport/pagination.go` into `internal/shared/pagination` while preserving external behavior and deleting the old transport helper.

**Architecture:** Add a shared `PaginationHelper` with option-based defaults (`WithDefaultLimit`, `WithMaxLimit`) for handler-injected query parsing, and keep cursor token serialization as package-level generic functions. Update handler wiring at composition roots to inject the helper and replace old calls. Preserve API contract (`limit`, `next_token`) and existing validation/error semantics.

**Tech Stack:** Go, Echo, existing internal error/validation utilities, `go test`.

**Design Reference:** [Shared Pagination Helper Refactor](../../designs/shared-pagination-helper-refactor.md)

---

### Task 1: Baseline behavior lock (tests first)

**Files:**
- Modify: `internal/function-service/transport/pagination_test.go` (or existing pagination-related tests under transport)
- Test: same file(s)

- [ ] **Step 1: Identify current pagination behavior under test coverage**

Run:
```bash
rg -n "ParseLimit|ParseToken|next_token|EncodeNextToken|DecodeNextToken" internal/function-service
```
Expected: Locate current helper usage and existing tests to preserve behavior.

- [ ] **Step 2: Add/extend failing behavior-focused test cases (if missing)**

Add tests that lock required behavior:
- `limit` missing => default limit (current behavior)
- `limit` invalid (non-int, <=0, >max) => error
- `next_token` empty string => error (per design)
- token encode/decode roundtrip for representative DTO

Example scaffold:
```go
func TestParseToken_EmptyString_ReturnsError(t *testing.T) {
    // Arrange context with no next_token
    // Act ParseToken
    // Assert error
}
```

- [ ] **Step 3: Run focused tests to confirm baseline expectations**

Run:
```bash
go test ./internal/function-service/transport -run Pagination -v
```
Expected: PASS if behavior already covered, or FAIL only for newly added cases exposing gaps.

- [ ] **Step 4: Commit baseline test updates**

```bash
git add internal/function-service/transport/*pagination*_test.go
git commit -m "test: lock current pagination behavior before shared migration"
```

### Task 2: Create shared pagination package with helper and options

**Files:**
- Create: `internal/shared/pagination/helper.go`
- Create: `internal/shared/pagination/token.go`
- Create: `internal/shared/pagination/errors.go` (only if needed to preserve error mapping)
- Create: `internal/shared/pagination/helper_test.go`
- Create: `internal/shared/pagination/token_test.go`

- [ ] **Step 1: Write failing tests for helper defaults/options and parse semantics**

Add tests covering:
- `New()` default values: default limit 20, max 50
- `WithDefaultLimit`, `WithMaxLimit` overrides
- `ParseLimit(*echo.Context)` compatibility checks
- `ParseToken(*echo.Context)` returns error when `QueryParam("next_token") == ""`

Example assertion pattern:
```go
helper := pagination.New()
limit, err := helper.ParseLimit(&ctx)
require.NoError(t, err)
require.Equal(t, 20, limit)
```

- [ ] **Step 2: Implement minimal helper API to satisfy tests**

Implement in `helper.go`:
```go
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
```

- [ ] **Step 3: Write failing token codec generic tests**

Add tests for:
- `EncodeNextToken[T]` produces non-empty string for valid input
- `DecodeNextToken[T]` restores expected struct values
- decode invalid token => error

- [ ] **Step 4: Implement token generic functions to pass tests**

Implement in `token.go`:
```go
func EncodeNextToken[T any](input T) (string, error)
func DecodeNextToken[T any](raw string) (T, error)
```
Reuse existing serialization format from old helper to avoid API drift.

- [ ] **Step 5: Run shared package tests**

Run:
```bash
go test ./internal/shared/pagination -v
```
Expected: PASS.

- [ ] **Step 6: Commit shared package introduction**

```bash
git add internal/shared/pagination
git commit -m "feat: add shared pagination helper and generic token codec"
```

### Task 3: Migrate function-service handlers and composition wiring

**Files:**
- Modify: function-service handler constructors and wiring files that currently reference `transport/pagination.go`
- Likely modify: `internal/function-service/.../handler*.go`, `internal/function-service/.../wire*.go` (actual paths from search)
- Test: related handler/service integration tests

- [ ] **Step 1: Locate all call sites of old pagination helper**

Run:
```bash
rg -n "transport\.ParseLimit|transport\.ParseToken|EncodeNextToken|DecodeNextToken|pagination\.go" internal/function-service
```
Expected: Complete list of migration call sites.

- [ ] **Step 2: Update constructors to accept `*pagination.PaginationHelper` dependency**

For each handler that parses pagination params:
- add field `paginationHelper *pagination.PaginationHelper`
- inject via constructor
- replace old static helper calls with `h.paginationHelper.ParseLimit(...)` / `ParseToken(...)`

- [ ] **Step 3: Replace token encode/decode imports with shared package functions**

Use:
```go
pagination.EncodeNextToken(...)
pagination.DecodeNextToken[YourTokenType](raw)
```
Ensure DTO types remain unchanged.

- [ ] **Step 4: Update composition root / DI setup**

Instantiate once (or per handler module) with defaults unless explicit config exists:
```go
paginationHelper := pagination.New()
```
Inject into handlers that need pagination parsing.

- [ ] **Step 5: Run focused function-service tests**

Run:
```bash
go test ./internal/function-service/... -v
```
Expected: PASS with no behavior regressions.

- [ ] **Step 6: Commit migration of handler wiring**

```bash
git add internal/function-service
git commit -m "refactor: inject shared pagination helper into function-service handlers"
```

### Task 4: Remove legacy transport pagination file and clean references

**Files:**
- Delete: `internal/function-service/transport/pagination.go`
- Modify: any leftover imports/tests referencing deleted file

- [ ] **Step 1: Delete old file and fix compile errors**

Run:
```bash
git rm internal/function-service/transport/pagination.go
```
Then resolve all references.

- [ ] **Step 2: Run repo-wide pagination-related checks**

Run:
```bash
rg -n "internal/function-service/transport/pagination|transport\.ParseLimit|transport\.ParseToken"
```
Expected: no matches.

- [ ] **Step 3: Run full backend verification**

Run:
```bash
go test ./...
```
Expected: PASS.

- [ ] **Step 4: Commit legacy removal**

```bash
git add -A
git commit -m "refactor: remove legacy transport pagination helper"
```

### Task 5: Documentation and plan lifecycle updates

**Files:**
- Modify (if needed): `docs/designs/shared-pagination-helper-refactor.md`
- Move on completion: `docs/plans/active/2026-05-08-shared-pagination-helper-refactor-implementation.md` -> `docs/plans/completed/...`

- [ ] **Step 1: Verify design/doc consistency after implementation**

Check that implemented signatures and semantics match design doc.
If drift exists, update design doc with rationale.

- [ ] **Step 2: Commit doc alignment changes (if any)**

```bash
git add docs/designs/shared-pagination-helper-refactor.md
git commit -m "docs: align shared pagination design with implemented behavior"
```

- [ ] **Step 3: Move plan to completed after code lands**

```bash
git mv docs/plans/active/2026-05-08-shared-pagination-helper-refactor-implementation.md docs/plans/completed/2026-05-08-shared-pagination-helper-refactor-implementation.md
git commit -m "docs: mark shared pagination implementation plan as completed"
```

## Verification Checklist (during implementation)

- [ ] `go test ./internal/shared/pagination -v`
- [ ] `go test ./internal/function-service/... -v`
- [ ] `go test ./...`
- [ ] `rg -n "transport\.ParseLimit|transport\.ParseToken|internal/function-service/transport/pagination"` returns no legacy references

## Risks and Mitigations

- Behavior drift in error messages/codes: mitigate via baseline tests before refactor and handler-level regression tests.
- Incorrect DI rollout: mitigate by constructor-level compile failures and focused handler tests.
- Generic decode type mismatch: mitigate by typed roundtrip tests per token DTO used in function-service.
