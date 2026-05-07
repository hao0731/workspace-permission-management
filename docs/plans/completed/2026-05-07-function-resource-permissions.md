# Function Resource Permissions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions` to save one complete function permission configuration per workspace/function pair.

**Architecture:** Extend `function-service` with a new permission aggregate while preserving the existing dependency direction. Transport owns JSON DTOs and missing-field checks, domain owns framework-independent permission invariants, service owns normalization and ID generation, repositories own MongoDB persistence, handlers stay thin, and `cmd/function-service` wires the new repository/service/routes at startup.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, `github.com/google/uuid`, `log/slog`, standard `encoding/json`, standard `testing`.

---

## Source Designs

Primary source design: [../../designs/function-resource-permissions.md](../../designs/function-resource-permissions.md)

Related source design: [../../designs/function-service.md](../../designs/function-service.md)

Applicable policies:

- [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend code must keep handlers thin, put HTTP DTOs in transport, keep domain independent from Echo and MongoDB, define service interfaces at the consumer side, and treat API and MongoDB schemas as contracts with tests and API examples.
- Implementation plans created from design documents must live under `docs/plans/active/` and link back to their source design.

## Scope

Implement only the permission save feature from the source design:

- New `PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions` endpoint.
- New MongoDB collection: `function_resource_permissions`.
- One logical document per `{ workspace_id, function_key }` with a unique index.
- Full replace semantics for `office_permission` and `remote_permission`.
- Preserve existing document `_id` on replace.
- Generate document IDs and missing extra-rule `rule_id` values in the service.
- Preserve request-provided baseline `enabled` values, including `false`.
- Reject duplicate request-provided `rule_id` values with `400`.
- Deduplicate semantically identical extra rules inside each permission section, keeping the first occurrence.
- Return `200` with the normalized persisted permission model.

Do not implement permission evaluation, permission read APIs, partial update APIs, permission change events, frontend changes, workspace/function/group/action/resource-tag existence checks, rule history, or migrations for existing `function_resources`.

## File Structure

Create:

- `internal/domain/permission/errors.go`: permission domain sentinel errors.
- `internal/domain/permission/permission.go`: permission aggregate and save input types.
- `internal/domain/permission/validation.go`: domain validation and duplicate `rule_id` checks.
- `internal/domain/permission/validation_test.go`: domain validation coverage.
- `internal/function-service/transport/permission_request.go`: request DTOs, body decoding, missing `enabled` checks, and request-to-domain mapping.
- `internal/function-service/transport/permission_request_test.go`: request decoding and mapping tests.
- `internal/function-service/transport/permission_response.go`: response DTOs and domain-to-response mapping.
- `internal/function-service/transport/permission_response_test.go`: response shape tests.
- `internal/function-service/services/permission_service.go`: save workflow, semantic deduplication, and ID generation.
- `internal/function-service/services/permission_service_test.go`: save workflow tests.
- `internal/function-service/repositories/mongo_permission_repository.go`: MongoDB document mapping, unique index, update-then-insert save behavior, and duplicate-key retry.
- `internal/function-service/repositories/mongo_permission_repository_test.go`: repository helper and document mapping tests.
- `internal/function-service/handlers/permission_handler.go`: Echo route and save handler.
- `internal/function-service/handlers/permission_handler_test.go`: HTTP handler tests.
- `examples/api/function_resource_permissions.http`: executable REST Client examples.

Modify:

- `cmd/function-service/main.go`: construct permission repository/service, ensure indexes, and register permission route.

## Task 1: Domain Permission Contract

**Files:**

- Create: `internal/domain/permission/errors.go`
- Create: `internal/domain/permission/permission.go`
- Create: `internal/domain/permission/validation_test.go`
- Create: `internal/domain/permission/validation.go`

- [ ] **Step 1: Write failing domain validation tests**

Create `internal/domain/permission/validation_test.go` with this exact content:

```go
package permission

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validSaveInput() SaveInput {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return SaveInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		OfficePermission: &PermissionSection{
			BaselineRule: BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []ExtraRule{{
				RuleID:         "rule-office-1",
				GroupIDs:       []string{"group-1"},
				ActionID:       "edit",
				ResourceTags:   []string{"section_1"},
				ExpirationDate: expiration,
			}},
		},
		RemotePermission: &PermissionSection{
			BaselineRule: BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
			ExtraRules: []ExtraRule{{
				GroupIDs:       []string{"group-2"},
				ActionID:       "delete",
				ResourceTags:   []string{"remote"},
				ExpirationDate: expiration,
			}},
		},
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

func TestSaveInputValidateAcceptsValidInput(t *testing.T) {
	input := validSaveInput()

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestSaveInputValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*SaveInput)
		wantMessage string
	}{
		{
			name: "blank workspace id",
			mutate: func(input *SaveInput) {
				input.WorkspaceID = "   "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			mutate: func(input *SaveInput) {
				input.FunctionKey = "   "
			},
			wantMessage: "function key is required",
		},
		{
			name: "missing office permission",
			mutate: func(input *SaveInput) {
				input.OfficePermission = nil
			},
			wantMessage: "office permission is required",
		},
		{
			name: "missing remote permission",
			mutate: func(input *SaveInput) {
				input.RemotePermission = nil
			},
			wantMessage: "remote permission is required",
		},
		{
			name: "blank baseline action",
			mutate: func(input *SaveInput) {
				input.OfficePermission.BaselineRule.ActionID = "   "
			},
			wantMessage: "office baseline action id is required",
		},
		{
			name: "empty baseline tags",
			mutate: func(input *SaveInput) {
				input.OfficePermission.BaselineRule.ResourceTags = nil
			},
			wantMessage: "office baseline resource tags are required",
		},
		{
			name: "blank baseline tag",
			mutate: func(input *SaveInput) {
				input.OfficePermission.BaselineRule.ResourceTags = []string{"section_1", "   "}
			},
			wantMessage: "office baseline resource tags must be non-empty strings",
		},
		{
			name: "empty extra group ids",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].GroupIDs = nil
			},
			wantMessage: "office extra rule group ids are required",
		},
		{
			name: "blank extra group id",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].GroupIDs = []string{"group-1", "   "}
			},
			wantMessage: "office extra rule group ids must be non-empty strings",
		},
		{
			name: "blank extra action",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].ActionID = "   "
			},
			wantMessage: "office extra rule action id is required",
		},
		{
			name: "empty extra resource tags",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].ResourceTags = nil
			},
			wantMessage: "office extra rule resource tags are required",
		},
		{
			name: "zero extra expiration",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].ExpirationDate = time.Time{}
			},
			wantMessage: "office extra rule expiration date is required",
		},
		{
			name: "blank extra rule id",
			mutate: func(input *SaveInput) {
				input.OfficePermission.ExtraRules[0].RuleID = "   "
			},
			wantMessage: "office extra rule rule id must be non-empty when provided",
		},
		{
			name: "duplicate provided rule id across sections",
			mutate: func(input *SaveInput) {
				input.RemotePermission.ExtraRules[0].RuleID = "rule-office-1"
			},
			wantMessage: "duplicate rule id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validSaveInput()
			tt.mutate(&input)

			err := input.Validate()

			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}
```

- [ ] **Step 2: Run domain validation tests and verify they fail**

Run:

```bash
go test ./internal/domain/permission -run TestSaveInputValidate -count=1
```

Expected: FAIL because the `internal/domain/permission` package does not exist yet.

- [ ] **Step 3: Create permission domain errors**

Create `internal/domain/permission/errors.go` with this exact content:

```go
package permission

import "errors"

var ErrInvalidInput = errors.New("invalid permission input")
```

- [ ] **Step 4: Create permission domain models**

Create `internal/domain/permission/permission.go` with this exact content:

```go
package permission

import "time"

type Permission struct {
	ID               string
	WorkspaceID      string
	FunctionKey      string
	OfficePermission PermissionSection
	RemotePermission PermissionSection
}

type SaveInput struct {
	WorkspaceID       string
	FunctionKey       string
	OfficePermission  *PermissionSection
	RemotePermission  *PermissionSection
}

type PermissionSection struct {
	BaselineRule BaselineRule
	ExtraRules   []ExtraRule
}

type BaselineRule struct {
	ActionID     string
	ResourceTags []string
	Enabled      bool
}

type ExtraRule struct {
	RuleID         string
	GroupIDs       []string
	ActionID       string
	ResourceTags   []string
	ExpirationDate time.Time
}
```

- [ ] **Step 5: Implement domain validation**

Create `internal/domain/permission/validation.go` with this exact content:

```go
package permission

import (
	"fmt"
	"strings"
)

func (input SaveInput) Validate() error {
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return invalidInput("function key is required")
	}
	if input.OfficePermission == nil {
		return invalidInput("office permission is required")
	}
	if input.RemotePermission == nil {
		return invalidInput("remote permission is required")
	}
	if err := validateSection("office", *input.OfficePermission); err != nil {
		return err
	}
	if err := validateSection("remote", *input.RemotePermission); err != nil {
		return err
	}
	if err := validateUniqueRuleIDs(input); err != nil {
		return err
	}
	return nil
}

func validateSection(label string, section PermissionSection) error {
	if strings.TrimSpace(section.BaselineRule.ActionID) == "" {
		return invalidInput(fmt.Sprintf("%s baseline action id is required", label))
	}
	if len(section.BaselineRule.ResourceTags) == 0 {
		return invalidInput(fmt.Sprintf("%s baseline resource tags are required", label))
	}
	for _, tag := range section.BaselineRule.ResourceTags {
		if strings.TrimSpace(tag) == "" {
			return invalidInput(fmt.Sprintf("%s baseline resource tags must be non-empty strings", label))
		}
	}
	for _, rule := range section.ExtraRules {
		if strings.TrimSpace(rule.RuleID) == "" && rule.RuleID != "" {
			return invalidInput(fmt.Sprintf("%s extra rule rule id must be non-empty when provided", label))
		}
		if len(rule.GroupIDs) == 0 {
			return invalidInput(fmt.Sprintf("%s extra rule group ids are required", label))
		}
		for _, groupID := range rule.GroupIDs {
			if strings.TrimSpace(groupID) == "" {
				return invalidInput(fmt.Sprintf("%s extra rule group ids must be non-empty strings", label))
			}
		}
		if strings.TrimSpace(rule.ActionID) == "" {
			return invalidInput(fmt.Sprintf("%s extra rule action id is required", label))
		}
		if len(rule.ResourceTags) == 0 {
			return invalidInput(fmt.Sprintf("%s extra rule resource tags are required", label))
		}
		for _, tag := range rule.ResourceTags {
			if strings.TrimSpace(tag) == "" {
				return invalidInput(fmt.Sprintf("%s extra rule resource tags must be non-empty strings", label))
			}
		}
		if rule.ExpirationDate.IsZero() {
			return invalidInput(fmt.Sprintf("%s extra rule expiration date is required", label))
		}
	}
	return nil
}

func validateUniqueRuleIDs(input SaveInput) error {
	seen := map[string]struct{}{}
	for _, rule := range input.OfficePermission.ExtraRules {
		if err := checkRuleID(rule.RuleID, seen); err != nil {
			return err
		}
	}
	for _, rule := range input.RemotePermission.ExtraRules {
		if err := checkRuleID(rule.RuleID, seen); err != nil {
			return err
		}
	}
	return nil
}

func checkRuleID(ruleID string, seen map[string]struct{}) error {
	if ruleID == "" {
		return nil
	}
	if _, ok := seen[ruleID]; ok {
		return invalidInput(fmt.Sprintf("duplicate rule id %q", ruleID))
	}
	seen[ruleID] = struct{}{}
	return nil
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
```

- [ ] **Step 6: Run domain validation tests and verify they pass**

Run:

```bash
go test ./internal/domain/permission -run TestSaveInputValidate -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit domain contract**

Run:

```bash
git add internal/domain/permission/errors.go internal/domain/permission/permission.go internal/domain/permission/validation.go internal/domain/permission/validation_test.go
git commit -m "feat: add permission domain contract"
```

## Task 2: Transport Request and Response DTOs

**Files:**

- Create: `internal/function-service/transport/permission_request_test.go`
- Create: `internal/function-service/transport/permission_request.go`
- Create: `internal/function-service/transport/permission_response_test.go`
- Create: `internal/function-service/transport/permission_response.go`

- [ ] **Step 1: Write failing request transport tests**

Create `internal/function-service/transport/permission_request_test.go` with this exact content:

```go
package transport

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

func validPermissionRequestJSON() string {
	return `{
		"office_permission": {
			"baseline_rule": {
				"action_id": "view",
				"resource_tags": ["section_1"],
				"enabled": true
			},
			"extra_rules": [
				{
					"rule_id": "rule-1",
					"group_ids": ["group-1"],
					"action_id": "edit",
					"resource_tags": ["section_1"],
					"expiration_date": "2026-06-01T00:00:00Z"
				}
			]
		},
		"remote_permission": {
			"baseline_rule": {
				"action_id": "view",
				"resource_tags": ["remote"],
				"enabled": false
			},
			"extra_rules": []
		}
	}`
}

func TestDecodePermissionSaveRequest(t *testing.T) {
	req, err := DecodePermissionSaveRequest(strings.NewReader(validPermissionRequestJSON()))
	if err != nil {
		t.Fatalf("DecodePermissionSaveRequest error = %v, want nil", err)
	}
	if req.OfficePermission == nil || req.RemotePermission == nil {
		t.Fatal("permissions were not decoded")
	}
	if req.OfficePermission.BaselineRule == nil {
		t.Fatal("office baseline was not decoded")
	}
	if req.OfficePermission.BaselineRule.Enabled == nil || !*req.OfficePermission.BaselineRule.Enabled {
		t.Fatal("office enabled = nil/false, want true")
	}
	if req.RemotePermission.BaselineRule.Enabled == nil || *req.RemotePermission.BaselineRule.Enabled {
		t.Fatal("remote enabled = nil/true, want false")
	}
}

func TestDecodePermissionSaveRequestRejectsInvalidJSON(t *testing.T) {
	if _, err := DecodePermissionSaveRequest(strings.NewReader(`{"office_permission":`)); err == nil {
		t.Fatal("DecodePermissionSaveRequest error = nil, want error")
	}
}

func TestPermissionSaveRequestToDomain(t *testing.T) {
	req, err := DecodePermissionSaveRequest(strings.NewReader(validPermissionRequestJSON()))
	if err != nil {
		t.Fatalf("decode request: %v", err)
	}

	input, err := req.ToDomain("workspace-1", "todo")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.FunctionKey != "todo" {
		t.Fatalf("identity = %s/%s, want workspace-1/todo", input.WorkspaceID, input.FunctionKey)
	}
	if input.OfficePermission == nil || input.RemotePermission == nil {
		t.Fatal("domain permissions = nil, want values")
	}
	if !input.OfficePermission.BaselineRule.Enabled {
		t.Fatal("office enabled = false, want true")
	}
	if input.RemotePermission.BaselineRule.Enabled {
		t.Fatal("remote enabled = true, want false")
	}
	if len(input.OfficePermission.ExtraRules) != 1 {
		t.Fatalf("office extra rules len = %d, want 1", len(input.OfficePermission.ExtraRules))
	}
	if input.OfficePermission.ExtraRules[0].ExpirationDate != time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("expiration = %s, want 2026-06-01T00:00:00Z", input.OfficePermission.ExtraRules[0].ExpirationDate)
	}
}

func TestPermissionSaveRequestToDomainRejectsMissingEnabled(t *testing.T) {
	req := PermissionSaveRequest{
		OfficePermission: &PermissionSectionRequest{
			BaselineRule: &BaselineRuleRequest{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
			},
		},
		RemotePermission: &PermissionSectionRequest{
			BaselineRule: &BaselineRuleRequest{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      boolPtr(false),
			},
		},
	}

	_, err := req.ToDomain("workspace-1", "todo")
	if !errors.Is(err, permission.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestPermissionSaveRequestToDomainRejectsMissingSection(t *testing.T) {
	req := PermissionSaveRequest{}

	_, err := req.ToDomain("workspace-1", "todo")
	if !errors.Is(err, permission.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
```

- [ ] **Step 2: Run request transport tests and verify they fail**

Run:

```bash
go test ./internal/function-service/transport -run 'Test(DecodePermissionSaveRequest|PermissionSaveRequestToDomain)' -count=1
```

Expected: FAIL because the permission request DTOs do not exist yet.

- [ ] **Step 3: Implement request transport DTOs**

Create `internal/function-service/transport/permission_request.go` with this exact content:

```go
package transport

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

type PermissionSaveRequest struct {
	OfficePermission *PermissionSectionRequest `json:"office_permission"`
	RemotePermission *PermissionSectionRequest `json:"remote_permission"`
}

type PermissionSectionRequest struct {
	BaselineRule *BaselineRuleRequest `json:"baseline_rule"`
	ExtraRules   []ExtraRuleRequest   `json:"extra_rules"`
}

type BaselineRuleRequest struct {
	ActionID     string   `json:"action_id"`
	ResourceTags []string `json:"resource_tags"`
	Enabled      *bool    `json:"enabled"`
}

type ExtraRuleRequest struct {
	RuleID         string   `json:"rule_id,omitempty"`
	GroupIDs       []string `json:"group_ids"`
	ActionID       string   `json:"action_id"`
	ResourceTags   []string `json:"resource_tags"`
	ExpirationDate JSONTime `json:"expiration_date"`
}

func DecodePermissionSaveRequest(body io.Reader) (PermissionSaveRequest, error) {
	var request PermissionSaveRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return PermissionSaveRequest{}, fmt.Errorf("decode permission request: %w", err)
	}
	return request, nil
}

func (request PermissionSaveRequest) ToDomain(workspaceID, functionKey string) (permission.SaveInput, error) {
	officePermission, err := requestSectionToDomain("office", request.OfficePermission)
	if err != nil {
		return permission.SaveInput{}, err
	}
	remotePermission, err := requestSectionToDomain("remote", request.RemotePermission)
	if err != nil {
		return permission.SaveInput{}, err
	}
	return permission.SaveInput{
		WorkspaceID:       workspaceID,
		FunctionKey:       functionKey,
		OfficePermission:  officePermission,
		RemotePermission:  remotePermission,
	}, nil
}

func requestSectionToDomain(label string, request *PermissionSectionRequest) (*permission.PermissionSection, error) {
	if request == nil {
		return nil, invalidPermissionRequest(fmt.Sprintf("%s permission is required", label))
	}
	if request.BaselineRule == nil {
		return nil, invalidPermissionRequest(fmt.Sprintf("%s baseline rule is required", label))
	}
	if request.BaselineRule.Enabled == nil {
		return nil, invalidPermissionRequest(fmt.Sprintf("%s baseline enabled is required", label))
	}
	extraRules := make([]permission.ExtraRule, 0, len(request.ExtraRules))
	for _, rule := range request.ExtraRules {
		extraRules = append(extraRules, permission.ExtraRule{
			RuleID:         rule.RuleID,
			GroupIDs:       append([]string(nil), rule.GroupIDs...),
			ActionID:       rule.ActionID,
			ResourceTags:   append([]string(nil), rule.ResourceTags...),
			ExpirationDate: rule.ExpirationDate.Time,
		})
	}
	return &permission.PermissionSection{
		BaselineRule: permission.BaselineRule{
			ActionID:     request.BaselineRule.ActionID,
			ResourceTags: append([]string(nil), request.BaselineRule.ResourceTags...),
			Enabled:      *request.BaselineRule.Enabled,
		},
		ExtraRules: extraRules,
	}, nil
}

func invalidPermissionRequest(message string) error {
	return fmt.Errorf("%w: %s", permission.ErrInvalidInput, message)
}
```

- [ ] **Step 4: Add JSONTime helper for request timestamp decoding**

Append this exact code to `internal/function-service/transport/permission_request.go`:

```go

type JSONTime struct {
	Time time.Time
}

func (t *JSONTime) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("expiration_date must be RFC3339 timestamp")
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return fmt.Errorf("expiration_date must be RFC3339 timestamp")
	}
	t.Time = parsed
	return nil
}
```

Add `time` to the standard-library import group in `internal/function-service/transport/permission_request.go`:

```go
import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)
```

- [ ] **Step 5: Run request transport tests and verify they pass**

Run:

```bash
go test ./internal/function-service/transport -run 'Test(DecodePermissionSaveRequest|PermissionSaveRequestToDomain)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Write failing response transport tests**

Create `internal/function-service/transport/permission_response_test.go` with this exact content:

```go
package transport

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

func TestNewPermissionSaveResponse(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	response := NewPermissionSaveResponse(permission.Permission{
		ID:          "permission-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		OfficePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []permission.ExtraRule{{
				RuleID:         "rule-1",
				GroupIDs:       []string{"group-1"},
				ActionID:       "edit",
				ResourceTags:   []string{"section_1"},
				ExpirationDate: expiration,
			}},
		},
		RemotePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	})

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := body["permissions"]; !ok {
		t.Fatal("response missing permissions key")
	}
	if response.Permissions.OfficePermission.BaselineRule.Enabled != true {
		t.Fatal("office enabled = false, want true")
	}
	if response.Permissions.RemotePermission.BaselineRule.Enabled != false {
		t.Fatal("remote enabled = true, want false")
	}
	if got := response.Permissions.OfficePermission.ExtraRules[0].RuleID; got != "rule-1" {
		t.Fatalf("rule_id = %q, want rule-1", got)
	}
}
```

- [ ] **Step 7: Run response transport tests and verify they fail**

Run:

```bash
go test ./internal/function-service/transport -run TestNewPermissionSaveResponse -count=1
```

Expected: FAIL because response DTOs do not exist yet.

- [ ] **Step 8: Implement response transport DTOs**

Create `internal/function-service/transport/permission_response.go` with this exact content:

```go
package transport

import (
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

type PermissionSaveResponse struct {
	Permissions PermissionResponse `json:"permissions"`
}

type PermissionResponse struct {
	OfficePermission PermissionSectionResponse `json:"office_permission"`
	RemotePermission PermissionSectionResponse `json:"remote_permission"`
}

type PermissionSectionResponse struct {
	BaselineRule BaselineRuleResponse `json:"baseline_rule"`
	ExtraRules   []ExtraRuleResponse  `json:"extra_rules"`
}

type BaselineRuleResponse struct {
	ActionID     string   `json:"action_id"`
	ResourceTags []string `json:"resource_tags"`
	Enabled      bool     `json:"enabled"`
}

type ExtraRuleResponse struct {
	RuleID         string    `json:"rule_id"`
	GroupIDs       []string  `json:"group_ids"`
	ActionID       string    `json:"action_id"`
	ResourceTags   []string  `json:"resource_tags"`
	ExpirationDate time.Time `json:"expiration_date"`
}

func NewPermissionSaveResponse(model permission.Permission) PermissionSaveResponse {
	return PermissionSaveResponse{
		Permissions: PermissionResponse{
			OfficePermission: permissionSectionResponse(model.OfficePermission),
			RemotePermission: permissionSectionResponse(model.RemotePermission),
		},
	}
}

func permissionSectionResponse(section permission.PermissionSection) PermissionSectionResponse {
	extraRules := make([]ExtraRuleResponse, 0, len(section.ExtraRules))
	for _, rule := range section.ExtraRules {
		extraRules = append(extraRules, ExtraRuleResponse{
			RuleID:         rule.RuleID,
			GroupIDs:       append([]string(nil), rule.GroupIDs...),
			ActionID:       rule.ActionID,
			ResourceTags:   append([]string(nil), rule.ResourceTags...),
			ExpirationDate: rule.ExpirationDate,
		})
	}
	return PermissionSectionResponse{
		BaselineRule: BaselineRuleResponse{
			ActionID:     section.BaselineRule.ActionID,
			ResourceTags: append([]string(nil), section.BaselineRule.ResourceTags...),
			Enabled:      section.BaselineRule.Enabled,
		},
		ExtraRules: extraRules,
	}
}
```

- [ ] **Step 9: Run transport tests and verify they pass**

Run:

```bash
go test ./internal/function-service/transport -run 'Test(DecodePermissionSaveRequest|PermissionSaveRequestToDomain|NewPermissionSaveResponse)' -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit transport DTOs**

Run:

```bash
git add internal/function-service/transport/permission_request.go internal/function-service/transport/permission_request_test.go internal/function-service/transport/permission_response.go internal/function-service/transport/permission_response_test.go
git commit -m "feat: add permission transport DTOs"
```

## Task 3: Permission Service Workflow

**Files:**

- Create: `internal/function-service/services/permission_service_test.go`
- Create: `internal/function-service/services/permission_service.go`

- [ ] **Step 1: Write failing service tests**

Create `internal/function-service/services/permission_service_test.go` with this exact content:

```go
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

type fakePermissionRepository struct {
	input permission.Permission
	calls int
	err   error
}

func (f *fakePermissionRepository) Save(ctx context.Context, input permission.Permission) (permission.Permission, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return permission.Permission{}, f.err
	}
	return input, nil
}

func validPermissionSaveInput() permission.SaveInput {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return permission.SaveInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		OfficePermission: &permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []permission.ExtraRule{
				{
					RuleID:         "rule-provided",
					GroupIDs:       []string{"group-1"},
					ActionID:       "edit",
					ResourceTags:   []string{"section_1"},
					ExpirationDate: expiration,
				},
				{
					GroupIDs:       []string{"group-2"},
					ActionID:       "delete",
					ResourceTags:   []string{"section_2"},
					ExpirationDate: expiration,
				},
			},
		},
		RemotePermission: &permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	}
}

func TestPermissionServiceSavePermissionGeneratesIDs(t *testing.T) {
	repo := &fakePermissionRepository{}
	ids := []string{"permission-1", "rule-generated-1"}
	service := NewPermissionService(repo, WithPermissionIDGenerator(func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}))

	got, err := service.SavePermission(context.Background(), validPermissionSaveInput())
	if err != nil {
		t.Fatalf("SavePermission error = %v, want nil", err)
	}
	if got.ID != "permission-1" {
		t.Fatalf("permission id = %q, want permission-1", got.ID)
	}
	if repo.input.OfficePermission.ExtraRules[0].RuleID != "rule-provided" {
		t.Fatalf("first rule id = %q, want rule-provided", repo.input.OfficePermission.ExtraRules[0].RuleID)
	}
	if repo.input.OfficePermission.ExtraRules[1].RuleID != "rule-generated-1" {
		t.Fatalf("second rule id = %q, want rule-generated-1", repo.input.OfficePermission.ExtraRules[1].RuleID)
	}
	if repo.input.RemotePermission.BaselineRule.Enabled {
		t.Fatal("remote baseline enabled = true, want false")
	}
}

func TestPermissionServiceSavePermissionDeduplicatesSemanticExtraRules(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	input := validPermissionSaveInput()
	input.OfficePermission.ExtraRules = []permission.ExtraRule{
		{
			GroupIDs:       []string{"group-1", "group-2"},
			ActionID:       "edit",
			ResourceTags:   []string{"section_1"},
			ExpirationDate: expiration,
		},
		{
			RuleID:         "dropped-rule",
			GroupIDs:       []string{"group-2", "group-1"},
			ActionID:       "edit",
			ResourceTags:   []string{"section_1"},
			ExpirationDate: expiration,
		},
	}
	repo := &fakePermissionRepository{}
	ids := []string{"permission-1", "rule-generated-1"}
	service := NewPermissionService(repo, WithPermissionIDGenerator(func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}))

	got, err := service.SavePermission(context.Background(), input)
	if err != nil {
		t.Fatalf("SavePermission error = %v, want nil", err)
	}
	if len(got.OfficePermission.ExtraRules) != 1 {
		t.Fatalf("office extra rules len = %d, want 1", len(got.OfficePermission.ExtraRules))
	}
	if got.OfficePermission.ExtraRules[0].RuleID != "rule-generated-1" {
		t.Fatalf("kept rule id = %q, want generated id for first rule", got.OfficePermission.ExtraRules[0].RuleID)
	}
}

func TestPermissionServiceSavePermissionRejectsInvalidInput(t *testing.T) {
	repo := &fakePermissionRepository{}
	service := NewPermissionService(repo)
	input := validPermissionSaveInput()
	input.WorkspaceID = ""

	_, err := service.SavePermission(context.Background(), input)
	if !errors.Is(err, permission.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if repo.calls != 0 {
		t.Fatalf("repository calls = %d, want 0", repo.calls)
	}
}

func TestPermissionServiceSavePermissionWrapsRepositoryError(t *testing.T) {
	repo := &fakePermissionRepository{err: errors.New("database unavailable")}
	service := NewPermissionService(repo)

	_, err := service.SavePermission(context.Background(), validPermissionSaveInput())
	if err == nil {
		t.Fatal("SavePermission error = nil, want error")
	}
}
```

- [ ] **Step 2: Run service tests and verify they fail**

Run:

```bash
go test ./internal/function-service/services -run TestPermissionService -count=1
```

Expected: FAIL because `NewPermissionService` does not exist yet.

- [ ] **Step 3: Implement permission service**

Create `internal/function-service/services/permission_service.go` with this exact content:

```go
package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

type PermissionRepository interface {
	Save(ctx context.Context, input permission.Permission) (permission.Permission, error)
}

type PermissionOption func(*PermissionService)

func WithPermissionIDGenerator(generator func() string) PermissionOption {
	return func(s *PermissionService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}

type PermissionService struct {
	repository  PermissionRepository
	idGenerator func() string
}

func NewPermissionService(repository PermissionRepository, opts ...PermissionOption) *PermissionService {
	service := &PermissionService{
		repository:  repository,
		idGenerator: uuid.NewString,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *PermissionService) SavePermission(ctx context.Context, input permission.SaveInput) (permission.Permission, error) {
	if err := input.Validate(); err != nil {
		return permission.Permission{}, err
	}

	model := permission.Permission{
		ID:               s.idGenerator(),
		WorkspaceID:      input.WorkspaceID,
		FunctionKey:      input.FunctionKey,
		OfficePermission: s.normalizeSection(*input.OfficePermission),
		RemotePermission: s.normalizeSection(*input.RemotePermission),
	}

	s.assignMissingRuleIDs(&model)

	saved, err := s.repository.Save(ctx, model)
	if err != nil {
		return permission.Permission{}, fmt.Errorf("save permissions: %w", err)
	}
	return saved, nil
}

func (s *PermissionService) normalizeSection(section permission.PermissionSection) permission.PermissionSection {
	seen := map[string]struct{}{}
	extraRules := make([]permission.ExtraRule, 0, len(section.ExtraRules))
	for _, rule := range section.ExtraRules {
		key := semanticRuleKey(rule)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		extraRules = append(extraRules, cloneExtraRule(rule))
	}
	return permission.PermissionSection{
		BaselineRule: permission.BaselineRule{
			ActionID:     section.BaselineRule.ActionID,
			ResourceTags: append([]string(nil), section.BaselineRule.ResourceTags...),
			Enabled:      section.BaselineRule.Enabled,
		},
		ExtraRules: extraRules,
	}
}

func (s *PermissionService) assignMissingRuleIDs(model *permission.Permission) {
	used := map[string]struct{}{}
	collectRuleIDs(model.OfficePermission.ExtraRules, used)
	collectRuleIDs(model.RemotePermission.ExtraRules, used)
	assignRuleIDs(model.OfficePermission.ExtraRules, used, s.idGenerator)
	assignRuleIDs(model.RemotePermission.ExtraRules, used, s.idGenerator)
}

func collectRuleIDs(rules []permission.ExtraRule, used map[string]struct{}) {
	for _, rule := range rules {
		if rule.RuleID != "" {
			used[rule.RuleID] = struct{}{}
		}
	}
}

func assignRuleIDs(rules []permission.ExtraRule, used map[string]struct{}, generator func() string) {
	for i := range rules {
		if rules[i].RuleID != "" {
			continue
		}
		rules[i].RuleID = nextUniqueID(used, generator)
	}
}

func nextUniqueID(used map[string]struct{}, generator func() string) string {
	for {
		id := generator()
		if strings.TrimSpace(id) == "" {
			continue
		}
		if _, ok := used[id]; ok {
			continue
		}
		used[id] = struct{}{}
		return id
	}
}

func semanticRuleKey(rule permission.ExtraRule) string {
	return strings.Join([]string{
		strings.Join(canonicalStrings(rule.GroupIDs), "\x00"),
		rule.ActionID,
		strings.Join(canonicalStrings(rule.ResourceTags), "\x00"),
		rule.ExpirationDate.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}, "\x1f")
}

func canonicalStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func cloneExtraRule(rule permission.ExtraRule) permission.ExtraRule {
	return permission.ExtraRule{
		RuleID:         rule.RuleID,
		GroupIDs:       append([]string(nil), rule.GroupIDs...),
		ActionID:       rule.ActionID,
		ResourceTags:   append([]string(nil), rule.ResourceTags...),
		ExpirationDate: rule.ExpirationDate,
	}
}
```

- [ ] **Step 4: Run service tests and verify they pass**

Run:

```bash
go test ./internal/function-service/services -run TestPermissionService -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit permission service**

Run:

```bash
git add internal/function-service/services/permission_service.go internal/function-service/services/permission_service_test.go
git commit -m "feat: add permission save service"
```

## Task 4: MongoDB Permission Repository

**Files:**

- Create: `internal/function-service/repositories/mongo_permission_repository_test.go`
- Create: `internal/function-service/repositories/mongo_permission_repository.go`

- [ ] **Step 1: Write failing repository tests**

Create `internal/function-service/repositories/mongo_permission_repository_test.go` with this exact content:

```go
package repositories

import (
	"reflect"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestBuildPermissionFilter(t *testing.T) {
	got := buildPermissionFilter("workspace-1", "todo")
	want := bson.M{
		"workspace_id": "workspace-1",
		"function_key": "todo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestBuildPermissionUpdate(t *testing.T) {
	doc := permissionDocument{
		OfficePermission: permissionSectionDocument{
			BaselineRule: baselineRuleDocument{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
		},
		RemotePermission: permissionSectionDocument{
			BaselineRule: baselineRuleDocument{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	}

	got := buildPermissionUpdate(doc)
	set, ok := got["$set"].(bson.M)
	if !ok {
		t.Fatalf("update = %#v, want $set", got)
	}
	if _, ok := set["office_permission"]; !ok {
		t.Fatalf("update set = %#v, want office_permission", set)
	}
	if _, ok := set["remote_permission"]; !ok {
		t.Fatalf("update set = %#v, want remote_permission", set)
	}
}

func TestPermissionDocumentMapping(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	model := permission.Permission{
		ID:          "permission-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		OfficePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []permission.ExtraRule{{
				RuleID:         "rule-1",
				GroupIDs:       []string{"group-1"},
				ActionID:       "edit",
				ResourceTags:   []string{"section_1"},
				ExpirationDate: expiration,
			}},
		},
		RemotePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	}

	doc := newPermissionDocument(model)
	got := doc.toDomain()

	if got.ID != "permission-1" || got.WorkspaceID != "workspace-1" || got.FunctionKey != "todo" {
		t.Fatalf("identity = %+v, want permission-1/workspace-1/todo", got)
	}
	if !got.OfficePermission.BaselineRule.Enabled {
		t.Fatal("office enabled = false, want true")
	}
	if got.RemotePermission.BaselineRule.Enabled {
		t.Fatal("remote enabled = true, want false")
	}
	if got.OfficePermission.ExtraRules[0].RuleID != "rule-1" {
		t.Fatalf("rule id = %q, want rule-1", got.OfficePermission.ExtraRules[0].RuleID)
	}
}

func TestPermissionUniqueIndexModel(t *testing.T) {
	model := permissionUniqueIndexModel()
	if model.Options == nil {
		t.Fatal("index options = nil, want unique option")
	}
	opts := &options.IndexOptions{}
	for _, setter := range model.Options.List() {
		if err := setter(opts); err != nil {
			t.Fatalf("apply index option: %v", err)
		}
	}
	if opts.Unique == nil || !*opts.Unique {
		t.Fatalf("unique option = %v, want true", opts.Unique)
	}
}
```

- [ ] **Step 2: Run repository tests and verify they fail**

Run:

```bash
go test ./internal/function-service/repositories -run 'Test(BuildPermission|PermissionDocument|PermissionUnique)' -count=1
```

Expected: FAIL because the Mongo permission repository helpers do not exist yet.

- [ ] **Step 3: Implement MongoDB permission repository**

Create `internal/function-service/repositories/mongo_permission_repository.go` with this exact content:

```go
package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const permissionCollectionName = "function_resource_permissions"

type MongoPermissionRepository struct {
	collection *mongo.Collection
}

type permissionDocument struct {
	ID               string                    `bson:"_id"`
	WorkspaceID      string                    `bson:"workspace_id"`
	FunctionKey      string                    `bson:"function_key"`
	OfficePermission permissionSectionDocument `bson:"office_permission"`
	RemotePermission permissionSectionDocument `bson:"remote_permission"`
}

type permissionSectionDocument struct {
	BaselineRule baselineRuleDocument `bson:"baseline_rule"`
	ExtraRules   []extraRuleDocument  `bson:"extra_rules"`
}

type baselineRuleDocument struct {
	ActionID     string   `bson:"action_id"`
	ResourceTags []string `bson:"resource_tags"`
	Enabled      bool     `bson:"enabled"`
}

type extraRuleDocument struct {
	RuleID         string    `bson:"rule_id"`
	GroupIDs       []string  `bson:"group_ids"`
	ActionID       string    `bson:"action_id"`
	ResourceTags   []string  `bson:"resource_tags"`
	ExpirationDate time.Time `bson:"expiration_date"`
}

func NewMongoPermissionRepository(db *mongo.Database) *MongoPermissionRepository {
	return &MongoPermissionRepository{collection: db.Collection(permissionCollectionName)}
}

func (r *MongoPermissionRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.collection.Indexes().CreateOne(ctx, permissionUniqueIndexModel()); err != nil {
		return fmt.Errorf("create function_resource_permissions index: %w", err)
	}
	return nil
}

func (r *MongoPermissionRepository) Save(ctx context.Context, input permission.Permission) (permission.Permission, error) {
	doc := newPermissionDocument(input)
	filter := buildPermissionFilter(input.WorkspaceID, input.FunctionKey)

	result, err := r.collection.UpdateOne(ctx, filter, buildPermissionUpdate(doc))
	if err != nil {
		return permission.Permission{}, fmt.Errorf("update permissions: %w", err)
	}
	if result.MatchedCount > 0 {
		return r.findByWorkspaceFunction(ctx, input.WorkspaceID, input.FunctionKey)
	}

	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return r.retryPermissionUpdate(ctx, doc)
		}
		return permission.Permission{}, fmt.Errorf("insert permissions: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *MongoPermissionRepository) retryPermissionUpdate(ctx context.Context, doc permissionDocument) (permission.Permission, error) {
	result, err := r.collection.UpdateOne(ctx, buildPermissionFilter(doc.WorkspaceID, doc.FunctionKey), buildPermissionUpdate(doc))
	if err != nil {
		return permission.Permission{}, fmt.Errorf("retry update permissions: %w", err)
	}
	if result.MatchedCount == 0 {
		return permission.Permission{}, fmt.Errorf("retry update permissions: document not found after duplicate key")
	}
	return r.findByWorkspaceFunction(ctx, doc.WorkspaceID, doc.FunctionKey)
}

func (r *MongoPermissionRepository) findByWorkspaceFunction(ctx context.Context, workspaceID, functionKey string) (permission.Permission, error) {
	var doc permissionDocument
	if err := r.collection.FindOne(ctx, buildPermissionFilter(workspaceID, functionKey)).Decode(&doc); err != nil {
		return permission.Permission{}, fmt.Errorf("find permissions: %w", err)
	}
	return doc.toDomain(), nil
}

func permissionUniqueIndexModel() mongo.IndexModel {
	return mongo.IndexModel{
		Keys: bson.D{
			{Key: "workspace_id", Value: 1},
			{Key: "function_key", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
}

func buildPermissionFilter(workspaceID, functionKey string) bson.M {
	return bson.M{
		"workspace_id": workspaceID,
		"function_key": functionKey,
	}
}

func buildPermissionUpdate(doc permissionDocument) bson.M {
	return bson.M{
		"$set": bson.M{
			"office_permission": doc.OfficePermission,
			"remote_permission": doc.RemotePermission,
		},
	}
}

func newPermissionDocument(model permission.Permission) permissionDocument {
	return permissionDocument{
		ID:               model.ID,
		WorkspaceID:      model.WorkspaceID,
		FunctionKey:      model.FunctionKey,
		OfficePermission: newPermissionSectionDocument(model.OfficePermission),
		RemotePermission: newPermissionSectionDocument(model.RemotePermission),
	}
}

func newPermissionSectionDocument(section permission.PermissionSection) permissionSectionDocument {
	extraRules := make([]extraRuleDocument, 0, len(section.ExtraRules))
	for _, rule := range section.ExtraRules {
		extraRules = append(extraRules, extraRuleDocument{
			RuleID:         rule.RuleID,
			GroupIDs:       append([]string(nil), rule.GroupIDs...),
			ActionID:       rule.ActionID,
			ResourceTags:   append([]string(nil), rule.ResourceTags...),
			ExpirationDate: rule.ExpirationDate,
		})
	}
	return permissionSectionDocument{
		BaselineRule: baselineRuleDocument{
			ActionID:     section.BaselineRule.ActionID,
			ResourceTags: append([]string(nil), section.BaselineRule.ResourceTags...),
			Enabled:      section.BaselineRule.Enabled,
		},
		ExtraRules: extraRules,
	}
}

func (d permissionDocument) toDomain() permission.Permission {
	return permission.Permission{
		ID:               d.ID,
		WorkspaceID:      d.WorkspaceID,
		FunctionKey:      d.FunctionKey,
		OfficePermission: d.OfficePermission.toDomain(),
		RemotePermission: d.RemotePermission.toDomain(),
	}
}

func (d permissionSectionDocument) toDomain() permission.PermissionSection {
	extraRules := make([]permission.ExtraRule, 0, len(d.ExtraRules))
	for _, rule := range d.ExtraRules {
		extraRules = append(extraRules, permission.ExtraRule{
			RuleID:         rule.RuleID,
			GroupIDs:       append([]string(nil), rule.GroupIDs...),
			ActionID:       rule.ActionID,
			ResourceTags:   append([]string(nil), rule.ResourceTags...),
			ExpirationDate: rule.ExpirationDate,
		})
	}
	return permission.PermissionSection{
		BaselineRule: permission.BaselineRule{
			ActionID:     d.BaselineRule.ActionID,
			ResourceTags: append([]string(nil), d.BaselineRule.ResourceTags...),
			Enabled:      d.BaselineRule.Enabled,
		},
		ExtraRules: extraRules,
	}
}
```

- [ ] **Step 4: Run repository tests and verify they pass**

Run:

```bash
go test ./internal/function-service/repositories -run 'Test(BuildPermission|PermissionDocument|PermissionUnique)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit MongoDB permission repository**

Run:

```bash
git add internal/function-service/repositories/mongo_permission_repository.go internal/function-service/repositories/mongo_permission_repository_test.go
git commit -m "feat: add permission repository"
```

## Task 5: HTTP Handler and Route

**Files:**

- Create: `internal/function-service/handlers/permission_handler_test.go`
- Create: `internal/function-service/handlers/permission_handler.go`

- [ ] **Step 1: Write failing handler tests**

Create `internal/function-service/handlers/permission_handler_test.go` with this exact content:

```go
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
	"github.com/labstack/echo/v5"
)

type fakeHTTPPermissionService struct {
	input permission.SaveInput
	model permission.Permission
	err   error
	calls int
}

func (f *fakeHTTPPermissionService) SavePermission(ctx context.Context, input permission.SaveInput) (permission.Permission, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return permission.Permission{}, f.err
	}
	return f.model, nil
}

func validPermissionRequestBody() string {
	return `{
		"office_permission": {
			"baseline_rule": {
				"action_id": "view",
				"resource_tags": ["section_1"],
				"enabled": true
			},
			"extra_rules": [
				{
					"group_ids": ["group-1"],
					"action_id": "edit",
					"resource_tags": ["section_1"],
					"expiration_date": "2026-06-01T00:00:00Z"
				}
			]
		},
		"remote_permission": {
			"baseline_rule": {
				"action_id": "view",
				"resource_tags": ["remote"],
				"enabled": false
			},
			"extra_rules": []
		}
	}`
}

func permissionModel() permission.Permission {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return permission.Permission{
		ID:          "permission-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		OfficePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []permission.ExtraRule{{
				RuleID:         "rule-generated-1",
				GroupIDs:       []string{"group-1"},
				ActionID:       "edit",
				ResourceTags:   []string{"section_1"},
				ExpirationDate: expiration,
			}},
		},
		RemotePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	}
}

func TestPermissionHandlerSavePermissions(t *testing.T) {
	service := &fakeHTTPPermissionService{model: permissionModel()}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/functions/todo/permissions", strings.NewReader(validPermissionRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.calls != 1 {
		t.Fatalf("service calls = %d, want 1", service.calls)
	}
	if service.input.WorkspaceID != "workspace-1" || service.input.FunctionKey != "todo" {
		t.Fatalf("input identity = %s/%s, want workspace-1/todo", service.input.WorkspaceID, service.input.FunctionKey)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["permissions"]; !ok {
		t.Fatal("response missing permissions")
	}
}

func TestPermissionHandlerRejectsMalformedJSON(t *testing.T) {
	service := &fakeHTTPPermissionService{}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/functions/todo/permissions", strings.NewReader(`{"office_permission":`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if service.calls != 0 {
		t.Fatalf("service calls = %d, want 0", service.calls)
	}
}

func TestPermissionHandlerValidationError(t *testing.T) {
	service := &fakeHTTPPermissionService{err: permission.ErrInvalidInput}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/functions/todo/permissions", strings.NewReader(validPermissionRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPermissionHandlerServiceFailure(t *testing.T) {
	service := &fakeHTTPPermissionService{err: errors.New("database unavailable")}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/functions/todo/permissions", strings.NewReader(validPermissionRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
```

- [ ] **Step 2: Run handler tests and verify they fail**

Run:

```bash
go test ./internal/function-service/handlers -run TestPermissionHandler -count=1
```

Expected: FAIL because permission handler types and route registration do not exist yet.

- [ ] **Step 3: Implement permission handler**

Create `internal/function-service/handlers/permission_handler.go` with this exact content:

```go
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/labstack/echo/v5"
)

type HTTPPermissionService interface {
	SavePermission(ctx context.Context, input permission.SaveInput) (permission.Permission, error)
}

type PermissionHandler struct {
	service HTTPPermissionService
	logger  *slog.Logger
}

type permissionPathParams struct {
	workspaceID string
	functionKey string
}

func NewPermissionHandler(service HTTPPermissionService, logger *slog.Logger) *PermissionHandler {
	return &PermissionHandler{service: service, logger: logger}
}

func RegisterPermissionRoutes(e *echo.Echo, handler *PermissionHandler) {
	e.PUT("/api/v1/workspaces/:workspace_id/functions/:function_key/permissions", handler.SavePermissions)
}

func newPermissionPathParams(c *echo.Context) permissionPathParams {
	return permissionPathParams{
		workspaceID: c.Param("workspace_id"),
		functionKey: c.Param("function_key"),
	}
}

func (h *PermissionHandler) SavePermissions(c *echo.Context) error {
	params := newPermissionPathParams(c)
	request, err := transport.DecodePermissionSaveRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}

	input, err := request.ToDomain(params.workspaceID, params.functionKey)
	if err != nil {
		if errors.Is(err, permission.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}

	model, err := h.service.SavePermission(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, permission.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to save permissions",
			"err", err,
			"workspace_id", params.workspaceID,
			"function_key", params.functionKey,
		)
		return c.JSON(http.StatusInternalServerError, transport.ErrorResponse{
			Error: transport.ErrorBody{
				Code:    "internal_error",
				Message: "Internal server error",
			},
		})
	}

	return c.JSON(http.StatusOK, transport.NewPermissionSaveResponse(model))
}
```

- [ ] **Step 4: Run handler tests and verify they pass**

Run:

```bash
go test ./internal/function-service/handlers -run TestPermissionHandler -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit HTTP permission handler**

Run:

```bash
git add internal/function-service/handlers/permission_handler.go internal/function-service/handlers/permission_handler_test.go
git commit -m "feat: add permission save handler"
```

## Task 6: Startup Wiring and API Examples

**Files:**

- Modify: `cmd/function-service/main.go`
- Create: `examples/api/function_resource_permissions.http`

- [ ] **Step 1: Update function-service startup wiring**

Modify `cmd/function-service/main.go` so the MongoDB database is assigned once, both repositories initialize indexes, and the permission route is registered. Replace the repository/service setup block:

```go
	repository := repositories.NewMongoResourceRepository(mongoClient.Database(cfg.MongoDB.Database))
	if ensureIndexErr := repository.EnsureIndexes(ctx); ensureIndexErr != nil {
		return ensureIndexErr
	}
```

with:

```go
	db := mongoClient.Database(cfg.MongoDB.Database)
	repository := repositories.NewMongoResourceRepository(db)
	if ensureIndexErr := repository.EnsureIndexes(ctx); ensureIndexErr != nil {
		return ensureIndexErr
	}
	permissionRepository := repositories.NewMongoPermissionRepository(db)
	if ensureIndexErr := permissionRepository.EnsureIndexes(ctx); ensureIndexErr != nil {
		return ensureIndexErr
	}
```

Then add the permission service after `resourceService`:

```go
	permissionService := services.NewPermissionService(permissionRepository)
```

Then add permission route registration after resource route registration:

```go
	handlers.RegisterPermissionRoutes(e, handlers.NewPermissionHandler(permissionService, logger))
```

- [ ] **Step 2: Run command package tests and verify startup still compiles**

Run:

```bash
go test ./cmd/function-service -count=1
```

Expected: PASS.

- [ ] **Step 3: Create REST Client API examples**

Create `examples/api/function_resource_permissions.http` with this exact content:

```http
@baseUrl = http://localhost:8080
@workspaceId = workspace-1
@functionKey = todo

### Save permissions with provided and generated rule IDs
PUT {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/permissions
Content-Type: application/json

{
  "office_permission": {
    "baseline_rule": {
      "action_id": "view",
      "resource_tags": ["section_1"],
      "enabled": true
    },
    "extra_rules": [
      {
        "rule_id": "rule-office-1",
        "group_ids": ["group-1"],
        "action_id": "edit",
        "resource_tags": ["section_1"],
        "expiration_date": "2026-06-01T00:00:00Z"
      },
      {
        "group_ids": ["group-2"],
        "action_id": "delete",
        "resource_tags": ["section_2"],
        "expiration_date": "2026-07-01T00:00:00Z"
      }
    ]
  },
  "remote_permission": {
    "baseline_rule": {
      "action_id": "view",
      "resource_tags": ["remote"],
      "enabled": false
    },
    "extra_rules": []
  }
}

### Save permissions with duplicate semantic extra rules; response keeps one
PUT {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/permissions
Content-Type: application/json

{
  "office_permission": {
    "baseline_rule": {
      "action_id": "view",
      "resource_tags": ["section_1"],
      "enabled": true
    },
    "extra_rules": [
      {
        "group_ids": ["group-1", "group-2"],
        "action_id": "edit",
        "resource_tags": ["section_1"],
        "expiration_date": "2026-06-01T00:00:00Z"
      },
      {
        "group_ids": ["group-2", "group-1"],
        "action_id": "edit",
        "resource_tags": ["section_1"],
        "expiration_date": "2026-06-01T00:00:00Z"
      }
    ]
  },
  "remote_permission": {
    "baseline_rule": {
      "action_id": "view",
      "resource_tags": ["remote"],
      "enabled": false
    },
    "extra_rules": []
  }
}

### Duplicate rule IDs return 400
PUT {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/permissions
Content-Type: application/json

{
  "office_permission": {
    "baseline_rule": {
      "action_id": "view",
      "resource_tags": ["section_1"],
      "enabled": true
    },
    "extra_rules": [
      {
        "rule_id": "rule-duplicate",
        "group_ids": ["group-1"],
        "action_id": "edit",
        "resource_tags": ["section_1"],
        "expiration_date": "2026-06-01T00:00:00Z"
      }
    ]
  },
  "remote_permission": {
    "baseline_rule": {
      "action_id": "view",
      "resource_tags": ["remote"],
      "enabled": false
    },
    "extra_rules": [
      {
        "rule_id": "rule-duplicate",
        "group_ids": ["group-2"],
        "action_id": "delete",
        "resource_tags": ["remote"],
        "expiration_date": "2026-07-01T00:00:00Z"
      }
    ]
  }
}

### Missing baseline enabled returns 400
PUT {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/permissions
Content-Type: application/json

{
  "office_permission": {
    "baseline_rule": {
      "action_id": "view",
      "resource_tags": ["section_1"]
    },
    "extra_rules": []
  },
  "remote_permission": {
    "baseline_rule": {
      "action_id": "view",
      "resource_tags": ["remote"],
      "enabled": false
    },
    "extra_rules": []
  }
}
```

- [ ] **Step 4: Run focused package tests**

Run:

```bash
go test ./internal/domain/permission ./internal/function-service/transport ./internal/function-service/services ./internal/function-service/repositories ./internal/function-service/handlers ./cmd/function-service -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit startup wiring and API examples**

Run:

```bash
git add cmd/function-service/main.go examples/api/function_resource_permissions.http
git commit -m "feat: wire permission save API"
```

## Task 7: Final Verification

**Files:**

- Verify all files changed in Tasks 1-6.

- [ ] **Step 1: Check formatting**

Run:

```bash
gofmt -w internal/domain/permission internal/function-service/transport/permission_request.go internal/function-service/transport/permission_request_test.go internal/function-service/transport/permission_response.go internal/function-service/transport/permission_response_test.go internal/function-service/services/permission_service.go internal/function-service/services/permission_service_test.go internal/function-service/repositories/mongo_permission_repository.go internal/function-service/repositories/mongo_permission_repository_test.go internal/function-service/handlers/permission_handler.go internal/function-service/handlers/permission_handler_test.go cmd/function-service/main.go
```

Expected: command exits 0 and leaves Go files formatted.

- [ ] **Step 2: Run full Go tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Run Go vet**

Run:

```bash
go vet ./...
```

Expected: PASS.

- [ ] **Step 4: Confirm no unexpected files changed**

Run:

```bash
git status --short
```

Expected: only files from this plan are modified or staged. Existing unrelated local changes, such as `lefthook.yml`, must not be reverted or included in commits unless the user explicitly asks.

- [ ] **Step 5: Move completed plan after implementation**

After Tasks 1-7 pass and the implementation commits are complete, move this plan from `docs/plans/active/2026-05-07-function-resource-permissions.md` to `docs/plans/completed/2026-05-07-function-resource-permissions.md`.

Run:

```bash
git mv docs/plans/active/2026-05-07-function-resource-permissions.md docs/plans/completed/2026-05-07-function-resource-permissions.md
git commit -m "docs: complete function resource permissions plan"
```

Expected: plan status transition is recorded in git history.

## Self-Review Checklist

- Source design coverage:
  - `PUT /api/v1/workspaces/:workspace_id/functions/:function_key/permissions`: Task 5.
  - `function_resource_permissions` collection: Task 4.
  - Unique `{ workspace_id, function_key }` document identity: Task 4.
  - Preserve existing `_id` on replace: Task 4.
  - Generate missing `rule_id`: Task 3.
  - Persist baseline `enabled`, including `false`: Tasks 1-5.
  - Reject duplicate request-provided `rule_id`: Tasks 1, 3, 5, 6.
  - Deduplicate semantic extra rules and keep the first occurrence: Tasks 3, 5, 6.
  - Return `200` response with normalized permissions: Tasks 2 and 5.
  - API examples: Task 6.
  - Startup index initialization: Task 6.
- Placeholder scan: no placeholders are intentionally left in this plan.
- Type consistency: `permission.SaveInput`, `permission.Permission`, `PermissionService.SavePermission`, `MongoPermissionRepository.Save`, `PermissionSaveRequest.ToDomain`, and `NewPermissionSaveResponse` are used consistently across tasks.
