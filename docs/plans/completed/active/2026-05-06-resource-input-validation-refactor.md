# Resource Input Validation Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move resource workflow validation from private service helper functions to `resource.UpsertInput.Validate()`, `resource.DeleteInput.Validate()`, and `resource.ListQuery.Validate()` without changing external behavior.

**Architecture:** Keep transport parsing in `internal/function-service/transport`, service workflow sequencing in `internal/function-service/services`, and framework-independent input/query invariants in `internal/domain/resource`. Services will call domain `Validate` methods before repositories or publishers, then continue wrapping repository and publisher failures with workflow context.

**Tech Stack:** Go 1.25, standard `testing`, standard `errors`, `fmt`, `strings`, and `time`.

---

## Source Designs

Primary source design: [../../designs/resource-input-validation-refactor.md](../../designs/resource-input-validation-refactor.md)

Related source design: [../../designs/function-service.md](../../designs/function-service.md)

Applicable policies:

- [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend work must keep transport concerns separate from domain and service logic.
- Domain packages must not depend on Echo, MongoDB, NATS, JetStream, service packages, transport packages, or infrastructure clients.
- Services may depend on domain packages and consumer-side interfaces, but must not depend on transport DTOs or infrastructure clients.
- Plans created from design documents must live under `docs/plans/active/` and link back to source design documents.

## Scope

Implement only the validation placement refactor:

- Add domain validation methods for `UpsertInput`, `DeleteInput`, and `ListQuery`.
- Preserve existing validation messages and `errors.Is(err, resource.ErrInvalidInput)` behavior.
- Update `ResourceService` to call the new domain methods.
- Remove `validateUpsertInput`, `validateDeleteInput`, and `validateListQuery` from `resource_service.go`.
- Keep HTTP `limit`, max-limit, `next_token`, and CloudEvent parsing validation in transport packages.

Do not change HTTP routes, API response bodies, status codes, CloudEvent contracts, MongoDB schema, repository interfaces, package dependencies outside the listed files, or external dependencies.

## File Structure

Create:

- `internal/domain/resource/validation.go`: domain input/query validation methods.
- `internal/domain/resource/validation_test.go`: domain validation coverage for valid inputs, invalid fields, whitespace-only values, blank tags, zero event time, and invalid cursors.

Modify:

- `internal/function-service/services/resource_service.go`: call `Validate()` methods and remove private helper validators.
- `internal/function-service/services/resource_service_test.go`: strengthen existing invalid-input service tests to prove repositories and publishers are not invoked for invalid inputs.

## Task 1: Domain Validation Tests

**Files:**

- Create: `internal/domain/resource/validation_test.go`

- [ ] **Step 1: Write failing domain validation tests**

Create `internal/domain/resource/validation_test.go` with this exact content:

```go
package resource

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validUpsertInput() UpsertInput {
	return UpsertInput{
		ID:          "resource-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		DisplayName: "Spec",
		Type:        "document",
		Tags:        []string{"section_1"},
		EventTime:   time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
	}
}

func validDeleteInput() DeleteInput {
	return DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "resource-1",
	}
}

func validListQuery() ListQuery {
	return ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       20,
	}
}

func requireInvalidInput(t *testing.T, err error, wantMessage string) {
	t.Helper()

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if !strings.Contains(err.Error(), wantMessage) {
		t.Fatalf("error = %q, want message containing %q", err.Error(), wantMessage)
	}
}

func TestUpsertInputValidateAcceptsValidInput(t *testing.T) {
	input := validUpsertInput()

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestUpsertInputValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*UpsertInput)
		wantMessage string
	}{
		{
			name: "blank resource id",
			mutate: func(input *UpsertInput) {
				input.ID = "   "
			},
			wantMessage: "resource id is required",
		},
		{
			name: "blank workspace id",
			mutate: func(input *UpsertInput) {
				input.WorkspaceID = "   "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			mutate: func(input *UpsertInput) {
				input.FunctionKey = "   "
			},
			wantMessage: "function key is required",
		},
		{
			name: "blank display name",
			mutate: func(input *UpsertInput) {
				input.DisplayName = "   "
			},
			wantMessage: "display name is required",
		},
		{
			name: "blank resource type",
			mutate: func(input *UpsertInput) {
				input.Type = "   "
			},
			wantMessage: "resource type is required",
		},
		{
			name: "zero event time",
			mutate: func(input *UpsertInput) {
				input.EventTime = time.Time{}
			},
			wantMessage: "event time is required",
		},
		{
			name: "blank resource tag",
			mutate: func(input *UpsertInput) {
				input.Tags = []string{"section_1", "   "}
			},
			wantMessage: "resource tags must be non-empty strings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validUpsertInput()
			tt.mutate(&input)

			err := input.Validate()

			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestDeleteInputValidateAcceptsValidInput(t *testing.T) {
	input := validDeleteInput()

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestDeleteInputValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*DeleteInput)
		wantMessage string
	}{
		{
			name: "blank workspace id",
			mutate: func(input *DeleteInput) {
				input.WorkspaceID = "   "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			mutate: func(input *DeleteInput) {
				input.FunctionKey = "   "
			},
			wantMessage: "function key is required",
		},
		{
			name: "blank resource id",
			mutate: func(input *DeleteInput) {
				input.ResourceID = "   "
			},
			wantMessage: "resource id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validDeleteInput()
			tt.mutate(&input)

			err := input.Validate()

			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestListQueryValidateAcceptsValidQuery(t *testing.T) {
	query := validListQuery()

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestListQueryValidateAcceptsValidCursor(t *testing.T) {
	query := validListQuery()
	query.Cursor = &Cursor{
		CreatedAt: time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
		ID:        "resource-1",
	}

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestListQueryValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*ListQuery)
		wantMessage string
	}{
		{
			name: "blank workspace id",
			mutate: func(query *ListQuery) {
				query.WorkspaceID = "   "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			mutate: func(query *ListQuery) {
				query.FunctionKey = "   "
			},
			wantMessage: "function key is required",
		},
		{
			name: "zero limit",
			mutate: func(query *ListQuery) {
				query.Limit = 0
			},
			wantMessage: "limit must be greater than zero",
		},
		{
			name: "negative limit",
			mutate: func(query *ListQuery) {
				query.Limit = -1
			},
			wantMessage: "limit must be greater than zero",
		},
		{
			name: "cursor missing created_at",
			mutate: func(query *ListQuery) {
				query.Cursor = &Cursor{ID: "resource-1"}
			},
			wantMessage: "cursor created_at is required",
		},
		{
			name: "cursor missing id",
			mutate: func(query *ListQuery) {
				query.Cursor = &Cursor{
					CreatedAt: time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
					ID:        "   ",
				}
			},
			wantMessage: "cursor id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := validListQuery()
			tt.mutate(&query)

			err := query.Validate()

			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}
```

- [ ] **Step 2: Run the domain package tests and verify they fail for missing methods**

Run:

```bash
go test ./internal/domain/resource
```

Expected: FAIL with compile errors containing:

```txt
input.Validate undefined
query.Validate undefined
```

## Task 2: Domain Validation Methods

**Files:**

- Create: `internal/domain/resource/validation.go`
- Test: `internal/domain/resource/validation_test.go`

- [ ] **Step 1: Implement domain validation methods**

Create `internal/domain/resource/validation.go` with this exact content:

```go
package resource

import (
	"fmt"
	"strings"
)

func (input UpsertInput) Validate() error {
	if strings.TrimSpace(input.ID) == "" {
		return invalidInput("resource id is required")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return invalidInput("function key is required")
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		return invalidInput("display name is required")
	}
	if strings.TrimSpace(input.Type) == "" {
		return invalidInput("resource type is required")
	}
	if input.EventTime.IsZero() {
		return invalidInput("event time is required")
	}
	for _, tag := range input.Tags {
		if strings.TrimSpace(tag) == "" {
			return invalidInput("resource tags must be non-empty strings")
		}
	}
	return nil
}

func (input DeleteInput) Validate() error {
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return invalidInput("function key is required")
	}
	if strings.TrimSpace(input.ResourceID) == "" {
		return invalidInput("resource id is required")
	}
	return nil
}

func (query ListQuery) Validate() error {
	if strings.TrimSpace(query.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(query.FunctionKey) == "" {
		return invalidInput("function key is required")
	}
	if query.Limit <= 0 {
		return invalidInput("limit must be greater than zero")
	}
	if query.Cursor != nil {
		if query.Cursor.CreatedAt.IsZero() {
			return invalidInput("cursor created_at is required")
		}
		if strings.TrimSpace(query.Cursor.ID) == "" {
			return invalidInput("cursor id is required")
		}
	}
	return nil
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
```

- [ ] **Step 2: Format domain validation files**

Run:

```bash
gofmt -w internal/domain/resource/validation.go internal/domain/resource/validation_test.go
```

Expected: command exits 0.

- [ ] **Step 3: Run the domain package tests and verify they pass**

Run:

```bash
go test ./internal/domain/resource
```

Expected: PASS.

- [ ] **Step 4: Commit the domain validation methods**

Run:

```bash
git add internal/domain/resource/validation.go internal/domain/resource/validation_test.go
git commit -m "refactor: add resource input validation methods"
```

Expected: commit succeeds and includes only the two domain validation files.

## Task 3: Service Delegation Refactor

**Files:**

- Modify: `internal/function-service/services/resource_service.go`
- Modify: `internal/function-service/services/resource_service_test.go`
- Test: `internal/function-service/services/resource_service_test.go`

- [ ] **Step 1: Strengthen fake repository call tracking in service tests**

In `internal/function-service/services/resource_service_test.go`, replace the `fakeResourceRepository` type and its three methods with this exact code:

```go
type fakeResourceRepository struct {
	upsertStatus resource.UpsertStatus
	upsertInput  resource.UpsertInput
	upsertCalls  int
	upsertErr    error
	listQuery    resource.ListQuery
	listCalls    int
	listPage     resource.Page
	listErr      error
	deleteStatus resource.DeleteStatus
	deleteInput  resource.DeleteInput
	deleteCalls  int
	deleteErr    error
}

func (f *fakeResourceRepository) Upsert(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	f.upsertCalls++
	f.upsertInput = input
	if f.upsertErr != nil {
		return "", f.upsertErr
	}
	return f.upsertStatus, nil
}

func (f *fakeResourceRepository) List(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	f.listCalls++
	f.listQuery = query
	if f.listErr != nil {
		return resource.Page{}, f.listErr
	}
	return f.listPage, nil
}

func (f *fakeResourceRepository) Delete(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error) {
	f.deleteCalls++
	f.deleteInput = input
	if f.deleteErr != nil {
		return "", f.deleteErr
	}
	return f.deleteStatus, nil
}
```

- [ ] **Step 2: Strengthen invalid upsert service test**

In `internal/function-service/services/resource_service_test.go`, replace `TestResourceServiceRejectsInvalidUpsertInput` with this exact code:

```go
func TestResourceServiceRejectsInvalidUpsertInput(t *testing.T) {
	repo := &fakeResourceRepository{}
	service := NewResourceService(repo)

	_, err := service.UpsertResource(context.Background(), resource.UpsertInput{
		ID:          "",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		DisplayName: "Spec",
		Type:        "document",
		Tags:        []string{"section_1"},
		EventTime:   time.Now(),
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("repo upsert calls = %d, want 0", repo.upsertCalls)
	}
}
```

- [ ] **Step 3: Strengthen invalid list service test**

In `internal/function-service/services/resource_service_test.go`, replace `TestResourceServiceRejectsInvalidListQuery` with this exact code:

```go
func TestResourceServiceRejectsInvalidListQuery(t *testing.T) {
	repo := &fakeResourceRepository{}
	service := NewResourceService(repo)

	_, err := service.ListResources(context.Background(), resource.ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       0,
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if repo.listCalls != 0 {
		t.Fatalf("repo list calls = %d, want 0", repo.listCalls)
	}
}
```

- [ ] **Step 4: Strengthen invalid delete service test**

In `internal/function-service/services/resource_service_test.go`, replace `TestResourceServiceDeleteResourceRejectsInvalidInput` with this exact code:

```go
func TestResourceServiceDeleteResourceRejectsInvalidInput(t *testing.T) {
	repo := &fakeResourceRepository{}
	publisher := &fakeResourceDeletedPublisher{}
	service := NewResourceService(repo, WithResourceDeletedPublisher(publisher))

	_, err := service.DeleteResource(context.Background(), resource.DeleteInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		ResourceID:  "",
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if repo.deleteCalls != 0 {
		t.Fatalf("repo delete calls = %d, want 0", repo.deleteCalls)
	}
	if publisher.calls != 0 {
		t.Fatalf("publisher calls = %d, want 0", publisher.calls)
	}
}
```

- [ ] **Step 5: Run service tests before changing service implementation**

Run:

```bash
go test ./internal/function-service/services
```

Expected: PASS. These service tests document existing behavior before the delegation refactor.

- [ ] **Step 6: Update service methods to call domain validation**

In `internal/function-service/services/resource_service.go`, update the import block to remove `strings`:

```go
import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)
```

Replace the validation guard in `UpsertResource` with:

```go
if err := input.Validate(); err != nil {
	return "", err
}
```

Replace the validation guard in `ListResources` with:

```go
if err := query.Validate(); err != nil {
	return resource.Page{}, err
}
```

Replace the validation guard in `DeleteResource` with:

```go
if err := input.Validate(); err != nil {
	return "", err
}
```

Remove the full definitions of these private helper functions from `resource_service.go`:

```go
func validateUpsertInput(input resource.UpsertInput) error {
	if strings.TrimSpace(input.ID) == "" {
		return fmt.Errorf("%w: resource id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return fmt.Errorf("%w: function key is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		return fmt.Errorf("%w: display name is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.Type) == "" {
		return fmt.Errorf("%w: resource type is required", resource.ErrInvalidInput)
	}
	if input.EventTime.IsZero() {
		return fmt.Errorf("%w: event time is required", resource.ErrInvalidInput)
	}
	for _, tag := range input.Tags {
		if strings.TrimSpace(tag) == "" {
			return fmt.Errorf("%w: resource tags must be non-empty strings", resource.ErrInvalidInput)
		}
	}
	return nil
}

func validateDeleteInput(input resource.DeleteInput) error {
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return fmt.Errorf("%w: function key is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(input.ResourceID) == "" {
		return fmt.Errorf("%w: resource id is required", resource.ErrInvalidInput)
	}
	return nil
}

func validateListQuery(query resource.ListQuery) error {
	if strings.TrimSpace(query.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace id is required", resource.ErrInvalidInput)
	}
	if strings.TrimSpace(query.FunctionKey) == "" {
		return fmt.Errorf("%w: function key is required", resource.ErrInvalidInput)
	}
	if query.Limit <= 0 {
		return fmt.Errorf("%w: limit must be greater than zero", resource.ErrInvalidInput)
	}
	if query.Cursor != nil {
		if query.Cursor.CreatedAt.IsZero() {
			return fmt.Errorf("%w: cursor created_at is required", resource.ErrInvalidInput)
		}
		if strings.TrimSpace(query.Cursor.ID) == "" {
			return fmt.Errorf("%w: cursor id is required", resource.ErrInvalidInput)
		}
	}
	return nil
}
```

- [ ] **Step 7: Format service files**

Run:

```bash
gofmt -w internal/function-service/services/resource_service.go internal/function-service/services/resource_service_test.go
```

Expected: command exits 0.

- [ ] **Step 8: Run service tests and verify they pass**

Run:

```bash
go test ./internal/function-service/services
```

Expected: PASS.

- [ ] **Step 9: Verify old service helper names are gone from implementation code**

Run:

```bash
rg -n "validateUpsertInput|validateDeleteInput|validateListQuery" internal
```

Expected: no matches in `internal/`. `rg` exits 1 when there are no matches; that exit code is acceptable for this check.

- [ ] **Step 10: Commit the service delegation refactor**

Run:

```bash
git add internal/function-service/services/resource_service.go internal/function-service/services/resource_service_test.go
git commit -m "refactor: delegate resource input validation to domain"
```

Expected: commit succeeds and includes only the service files.

## Task 4: Full Verification

**Files:**

- Verify: `internal/domain/resource/validation.go`
- Verify: `internal/domain/resource/validation_test.go`
- Verify: `internal/function-service/services/resource_service.go`
- Verify: `internal/function-service/services/resource_service_test.go`

- [ ] **Step 1: Run full Go test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run diff whitespace check**

Run:

```bash
git diff --check
```

Expected: command exits 0 with no whitespace errors.

- [ ] **Step 3: Review final diff scope**

Run:

```bash
git status --short
```

Expected changed implementation files, if not already committed:

```txt
?? internal/domain/resource/validation.go
?? internal/domain/resource/validation_test.go
 M internal/function-service/services/resource_service.go
 M internal/function-service/services/resource_service_test.go
```

If Task 2 and Task 3 commits were already created, these files should not appear as uncommitted changes. Existing unrelated changes, such as `lefthook.yml`, should not be modified or reverted.

## Self-Review Checklist

- [ ] `UpsertInput.Validate()` covers all existing `validateUpsertInput` rules and messages.
- [ ] `DeleteInput.Validate()` covers all existing `validateDeleteInput` rules and messages.
- [ ] `ListQuery.Validate()` covers all existing `validateListQuery` rules and messages.
- [ ] Every invalid domain validation error wraps `ErrInvalidInput`.
- [ ] `ResourceService` returns validation errors unchanged.
- [ ] Repository and publisher errors remain wrapped with existing workflow context.
- [ ] Transport parsing remains in transport packages.
- [ ] No API, event, MongoDB, or repository contract changes were introduced.
