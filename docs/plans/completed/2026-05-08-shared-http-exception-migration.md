# Shared HTTP Exception Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce `internal/shared/http/exception` and migrate existing HTTP handlers to use it while preserving the current error response contract.

**Architecture:** Add a shared exception builder package that owns HTTP error payload types and construction (`Exception`, options, and response wrapper). Remove module-local duplicated error response types from `internal/function-service/transport`, then update handlers to build responses through the shared package. Keep response JSON field names and HTTP status mapping unchanged to avoid API contract drift.

**Tech Stack:** Go, Echo v4 handlers, existing repository test framework (`go test ./...`).

---

## File Structure and Responsibilities

- Create: `internal/shared/http/exception/exception.go`
  - Defines `Exception`, option type(s), `New`, `WithDetails`, `ErrorResponse`, `WrapResponse`.
- Create: `internal/shared/http/exception/exception_test.go`
  - Verifies payload shape, option behavior, and wrapper behavior.
- Modify: `internal/function-service/handlers/resource_handler.go`
  - Replace `transport.ErrorResponse/ErrorBody` usage with `exception` package usage.
- Modify: `internal/function-service/handlers/permission_handler.go`
  - Replace `transport.ErrorResponse/ErrorBody` usage with `exception` package usage.
- Modify: `internal/function-service/transport/resource_response.go`
  - Remove `ErrorResponse` and `ErrorBody` declarations that are superseded by shared package.
- Modify: `internal/function-service/handlers/resource_handler_test.go`
  - Keep tests green while asserting unchanged error JSON structure.
- Modify: `internal/function-service/handlers/permission_handler_test.go`
  - Keep tests green while asserting unchanged error JSON structure.
- Modify: `docs/designs/function-service.md`
  - If needed, align implementation details with finalized structure.
- Modify: `docs/designs/function-resource-permissions.md`
  - If needed, align implementation details with finalized structure.

---

### Task 1: Add shared exception package with TDD

**Files:**
- Create: `internal/shared/http/exception/exception.go`
- Create: `internal/shared/http/exception/exception_test.go`

- [ ] **Step 1: Write failing tests for constructor, options, and response wrapping**

```go
package exception

import "testing"

func TestNew_BuildsExceptionWithoutDetails(t *testing.T) {
	ex := New("validation_failed", "invalid request")
	if ex.Code != "validation_failed" || ex.Message != "invalid request" {
		t.Fatalf("unexpected exception: %+v", ex)
	}
	if ex.Details != nil {
		t.Fatalf("expected nil details, got %+v", ex.Details)
	}
}

func TestNew_WithDetails(t *testing.T) {
	details := map[string]any{"field": "workspace_id"}
	ex := New("validation_failed", "invalid request", WithDetails(details))
	if ex.Details["field"] != "workspace_id" {
		t.Fatalf("unexpected details: %+v", ex.Details)
	}
}

func TestWrapResponse(t *testing.T) {
	ex := New("internal_error", "failed")
	resp := WrapResponse(ex)
	if resp.Error.Code != "internal_error" {
		t.Fatalf("unexpected wrapped response: %+v", resp)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shared/http/exception -run TestNew -v`
Expected: FAIL with package/file/symbol not found.

- [ ] **Step 3: Implement minimal shared package**

```go
package exception

type Exception struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error Exception `json:"error"`
}

type Option func(*Exception)

func New(code, message string, opts ...Option) Exception {
	ex := Exception{Code: code, Message: message}
	for _, opt := range opts {
		if opt != nil {
			opt(&ex)
		}
	}
	return ex
}

func WithDetails(details map[string]any) Option {
	return func(ex *Exception) {
		ex.Details = details
	}
}

func WrapResponse(ex Exception) ErrorResponse {
	return ErrorResponse{Error: ex}
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/shared/http/exception -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/shared/http/exception/exception.go internal/shared/http/exception/exception_test.go
git commit -m "feat: add shared http exception package"
```

---

### Task 2: Migrate resource handler to shared exception package

**Files:**
- Modify: `internal/function-service/handlers/resource_handler.go`
- Modify: `internal/function-service/handlers/resource_handler_test.go`

- [ ] **Step 1: Add/adjust failing handler tests for error payload shape (if not already explicit)**

```go
func TestUpsertResource_ValidationErrorShape(t *testing.T) {
	// Assert status 400 and body.error.code/message/details shape stays unchanged.
}
```

- [ ] **Step 2: Run targeted test to verify failure (or missing assertion)**

Run: `go test ./internal/function-service/handlers -run TestUpsertResource_ValidationErrorShape -v`
Expected: FAIL or insufficient assertion signal before migration.

- [ ] **Step 3: Replace transport error response construction with shared exception calls**

```go
import sharedexception "github.com/hao0731/workspace-permission-management/internal/shared/http/exception"

return c.JSON(http.StatusBadRequest, sharedexception.WrapResponse(
	sharedexception.New("validation_failed", message),
))
```

- [ ] **Step 4: Run resource handler tests**

Run: `go test ./internal/function-service/handlers -run Resource -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/function-service/handlers/resource_handler.go internal/function-service/handlers/resource_handler_test.go
git commit -m "refactor: use shared exception package in resource handler"
```

---

### Task 3: Migrate permission handler to shared exception package

**Files:**
- Modify: `internal/function-service/handlers/permission_handler.go`
- Modify: `internal/function-service/handlers/permission_handler_test.go`

- [ ] **Step 1: Add/adjust failing tests for permission handler error shape**

```go
func TestSavePermissions_ValidationErrorShape(t *testing.T) {
	// Assert status 400 and body.error fields are unchanged.
}
```

- [ ] **Step 2: Run targeted test before migration**

Run: `go test ./internal/function-service/handlers -run TestSavePermissions_ValidationErrorShape -v`
Expected: FAIL or pre-migration assertion gap confirmed.

- [ ] **Step 3: Replace transport error response construction with shared exception calls**

```go
import sharedexception "github.com/hao0731/workspace-permission-management/internal/shared/http/exception"

return c.JSON(http.StatusInternalServerError, sharedexception.WrapResponse(
	sharedexception.New("internal_error", "failed to persist permissions"),
))
```

- [ ] **Step 4: Run permission handler tests**

Run: `go test ./internal/function-service/handlers -run Permission -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/function-service/handlers/permission_handler.go internal/function-service/handlers/permission_handler_test.go
git commit -m "refactor: use shared exception package in permission handler"
```

---

### Task 4: Remove duplicated transport error types and run regression verification

**Files:**
- Modify: `internal/function-service/transport/resource_response.go`

- [ ] **Step 1: Write/confirm failing compile expectation after removing old types from transport**

```go
// Remove ErrorBody/ErrorResponse declarations from transport package.
// Any missed migration should fail compile/tests.
```

- [ ] **Step 2: Remove duplicated `ErrorBody` and `ErrorResponse` from transport**

```go
// Keep non-error DTOs only.
```

- [ ] **Step 3: Run full tests and ensure no regression**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/function-service/transport/resource_response.go
git commit -m "refactor: remove duplicated transport error response types"
```

---

### Task 5: Final documentation consistency and plan lifecycle

**Files:**
- Modify: `docs/designs/function-service.md`
- Modify: `docs/designs/function-resource-permissions.md`
- Modify: `docs/plans/active/2026-05-08-shared-http-exception-migration.md`

- [ ] **Step 1: Ensure design docs match final implementation details (if naming/signatures changed during coding)**

```md
Update references so docs match final `internal/shared/http/exception` API exactly.
```

- [ ] **Step 2: Run quick consistency checks**

Run: `rg -n "transport.ErrorResponse|transport.ErrorBody|internal/shared/http/exception" docs internal/function-service`
Expected: No stale references to removed transport error types in handlers.

- [ ] **Step 3: Commit docs updates**

```bash
git add docs/designs/function-service.md docs/designs/function-resource-permissions.md docs/plans/active/2026-05-08-shared-http-exception-migration.md
git commit -m "docs: finalize shared exception migration plan and design alignment"
```

- [ ] **Step 4: After implementation completes, move plan to completed**

```bash
git mv docs/plans/active/2026-05-08-shared-http-exception-migration.md docs/plans/completed/2026-05-08-shared-http-exception-migration.md
git commit -m "docs: move shared exception migration plan to completed"
```

---

## Self-Review Checklist

- Spec coverage: covers package creation, option-based details, wrapper API, handler migration, transport type removal, and regression verification.
- Placeholder scan: no `TODO`/`TBD` placeholders in executable steps.
- Type consistency: plan uses consistent names `Exception`, `New`, `WithDetails`, `WrapResponse`, `ErrorResponse`.
