# Group Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `group-service` with `POST /api/v1/workspaces/:workspace_id/groups`, MongoDB transactional persistence, active-name/member uniqueness, and liveness probing.

**Architecture:** Add an independent Go/Echo service that follows the repository backend boundaries: domain owns group invariants, transport owns HTTP DTOs and JSON shape checks, service owns ID/time generation and create workflow, repository owns MongoDB documents, indexes, and transaction mechanics, and handlers stay thin. The create workflow writes `groups` and `group_individual_members` in one MongoDB transaction and returns a `201 Created` response without exposing persistence metadata or `workspace_id`.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, `viper`, `log/slog`, standard `encoding/json`, standard `testing`.

---

## Source Designs

Primary source design: [../../designs/group-service.md](../../designs/group-service.md)

Related source context:

- [../../designs/function-resource-permissions.md](../../designs/function-resource-permissions.md)
- [../../concept.md](../../concept.md)

Applicable policies:

- [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend implementation must keep handlers thin, keep request/response DTOs in transport packages, keep domain independent from Echo and MongoDB, keep repository driver details behind service-owned interfaces, inject time and ID generators where deterministic tests need them, and include REST Client examples for REST API contract changes.
- Design and implementation plan documents must live under `docs/designs/` and `docs/plans/active/`; plans must link to source design documents and be committed once finalized.

## Scope

Implement:

- `cmd/group-service` executable.
- `internal/domain/group` model, invariants, and domain errors.
- `internal/group-service/transport` request/response DTOs and mapping.
- `internal/group-service/services` create workflow with injected ID generator and clock.
- `internal/group-service/repositories` MongoDB documents, indexes, duplicate-name mapping, and transaction-backed create.
- `internal/group-service/handlers` route registration, HTTP status mapping, and response rendering.
- `internal/group-service/config` environment-based configuration.
- `GET /health/liveness` using `internal/shared/health`.
- `examples/api/groups.http`.

Do not implement:

- Workspace existence validation.
- Group list, get, update, delete, or soft-delete APIs.
- Group membership materialization or employee attribute evaluation.
- Employee attribute catalog/type validation.
- NATS, JetStream, or CloudEvents for `group-service`.
- Frontend changes.

## File Structure and Responsibilities

- Create: `internal/domain/group/errors.go`
  - Stable domain errors.
- Create: `internal/domain/group/group.go`
  - Group aggregate, create input, grouping rule, rule, operator, member models, and normalization helpers.
- Create: `internal/domain/group/validation.go`
  - Domain/business validation.
- Create: `internal/domain/group/validation_test.go`
  - Domain invariant tests.
- Create: `internal/group-service/transport/group_request.go`
  - JSON request DTOs, RFC3339 parsing, required `multi` detection, and DTO-to-domain mapping.
- Create: `internal/group-service/transport/group_request_test.go`
  - Request decoding and mapping tests.
- Create: `internal/group-service/transport/group_response.go`
  - Public create response DTOs.
- Create: `internal/group-service/transport/group_response_test.go`
  - Response shape tests.
- Create: `internal/group-service/services/group_service.go`
  - Create workflow, repository interface, ID generation, timestamping, validation, error wrapping.
- Create: `internal/group-service/services/group_service_test.go`
  - Service workflow tests.
- Create: `internal/group-service/repositories/mongo_group_repository.go`
  - MongoDB documents, index models, mapping, duplicate-key classification, transaction create.
- Create: `internal/group-service/repositories/mongo_group_repository_test.go`
  - Mapping/index unit tests and opt-in Mongo transaction integration tests.
- Create: `internal/group-service/handlers/group_handler.go`
  - Echo route and HTTP status mapping.
- Create: `internal/group-service/handlers/group_handler_test.go`
  - Handler tests.
- Create: `internal/group-service/handlers/test_logger_test.go`
  - Handler test logger helper.
- Create: `internal/group-service/config/config.go`
  - Environment loading and validation.
- Create: `internal/group-service/config/config_test.go`
  - Config tests.
- Create: `cmd/group-service/main.go`
  - Service wiring, health route, MongoDB connection, graceful shutdown.
- Create: `cmd/group-service/main_test.go`
  - Process indicator and health registration tests.
- Create: `examples/api/groups.http`
  - REST Client examples for success and validation/conflict cases.

---

### Task 1: Domain Model and Validation

**Files:**

- Create: `internal/domain/group/errors.go`
- Create: `internal/domain/group/group.go`
- Create: `internal/domain/group/validation.go`
- Create: `internal/domain/group/validation_test.go`

- [ ] **Step 1: Write failing domain validation tests**

Create `internal/domain/group/validation_test.go` with this content:

```go
package group

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func futureTime() time.Time {
	return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
}

func validationNow() time.Time {
	return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
}

func validCreateInput() CreateInput {
	return CreateInput{
		WorkspaceID: "workspace-1",
		Name:        " Design Reviewers ",
		Description: "Employees who can review design documents.",
		GroupingRule: GroupingRule{
			Rules: []Rule{{
				AttributeKey: " department ",
				Operator:     OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: futureTime(),
		},
		IndividualMembers: []IndividualMember{{
			NTAccount:      " user1 ",
			ExpirationDate: futureTime(),
		}},
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

func TestCreateInputNormalizeTrimsNamesAndKeys(t *testing.T) {
	input := validCreateInput().Normalize()

	if input.WorkspaceID != "workspace-1" {
		t.Fatalf("WorkspaceID = %q, want workspace-1", input.WorkspaceID)
	}
	if input.Name != "Design Reviewers" {
		t.Fatalf("Name = %q, want Design Reviewers", input.Name)
	}
	if input.GroupingRule.Rules[0].AttributeKey != "department" {
		t.Fatalf("AttributeKey = %q, want department", input.GroupingRule.Rules[0].AttributeKey)
	}
	if input.IndividualMembers[0].NTAccount != "user1" {
		t.Fatalf("NTAccount = %q, want user1", input.IndividualMembers[0].NTAccount)
	}
}

func TestCreateInputValidateAcceptsValidInput(t *testing.T) {
	input := validCreateInput().Normalize()

	if err := input.Validate(validationNow()); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestCreateInputValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*CreateInput)
		wantMessage string
	}{
		{
			name: "blank workspace id",
			mutate: func(input *CreateInput) {
				input.WorkspaceID = " "
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank name",
			mutate: func(input *CreateInput) {
				input.Name = " "
			},
			wantMessage: "name is required",
		},
		{
			name: "zero grouping expiration",
			mutate: func(input *CreateInput) {
				input.GroupingRule.ExpirationDate = time.Time{}
			},
			wantMessage: "grouping rule expiration date is required",
		},
		{
			name: "past grouping expiration",
			mutate: func(input *CreateInput) {
				input.GroupingRule.ExpirationDate = validationNow()
			},
			wantMessage: "grouping rule expiration date must be in the future",
		},
		{
			name: "no membership source",
			mutate: func(input *CreateInput) {
				input.GroupingRule.Rules = nil
				input.IndividualMembers = nil
			},
			wantMessage: "at least one membership source is required",
		},
		{
			name: "blank member nt account",
			mutate: func(input *CreateInput) {
				input.IndividualMembers = []IndividualMember{{
					NTAccount:      " ",
					ExpirationDate: futureTime(),
				}}
			},
			wantMessage: "individual member nt account is required",
		},
		{
			name: "duplicate member nt account",
			mutate: func(input *CreateInput) {
				input.IndividualMembers = []IndividualMember{
					{NTAccount: "user1", ExpirationDate: futureTime()},
					{NTAccount: "user1", ExpirationDate: futureTime()},
				}
			},
			wantMessage: "duplicate individual member nt account",
		},
		{
			name: "past member expiration",
			mutate: func(input *CreateInput) {
				input.IndividualMembers = []IndividualMember{{
					NTAccount:      "user1",
					ExpirationDate: validationNow(),
				}}
			},
			wantMessage: "individual member expiration date must be in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validCreateInput().Normalize()
			tt.mutate(&input)

			err := input.Validate(validationNow())
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestRuleValidate(t *testing.T) {
	tests := []struct {
		name        string
		rule        Rule
		wantMessage string
	}{
		{
			name: "blank attribute",
			rule: Rule{
				AttributeKey: " ",
				Operator:     OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			},
			wantMessage: "rule attribute key is required",
		},
		{
			name: "invalid operator",
			rule: Rule{
				AttributeKey: "department",
				Operator:     Operator("contains"),
				Multi:        false,
				Value:        "ABCD-123",
			},
			wantMessage: "rule operator is invalid",
		},
		{
			name: "single value null",
			rule: Rule{
				AttributeKey: "department",
				Operator:     OperatorEq,
				Multi:        false,
				Value:        nil,
			},
			wantMessage: "single-value rule value is required",
		},
		{
			name: "single value array",
			rule: Rule{
				AttributeKey: "department",
				Operator:     OperatorEq,
				Multi:        false,
				Value:        []any{"ABCD-123"},
			},
			wantMessage: "single-value rule value must not be an array",
		},
		{
			name: "multi value scalar",
			rule: Rule{
				AttributeKey: "department",
				Operator:     OperatorEq,
				Multi:        true,
				Value:        "ABCD-123",
			},
			wantMessage: "multi-value rule value must be a non-empty array",
		},
		{
			name: "multi value empty array",
			rule: Rule{
				AttributeKey: "department",
				Operator:     OperatorEq,
				Multi:        true,
				Value:        []any{},
			},
			wantMessage: "multi-value rule value must be a non-empty array",
		},
		{
			name: "multi value null item",
			rule: Rule{
				AttributeKey: "department",
				Operator:     OperatorEq,
				Multi:        true,
				Value:        []any{"ABCD-123", nil},
			},
			wantMessage: "multi-value rule value items must not be null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}

func TestRuleValidateAcceptsAllowedOperators(t *testing.T) {
	operators := []Operator{
		OperatorEq,
		OperatorNotEq,
		OperatorGt,
		OperatorGte,
		OperatorLt,
		OperatorLte,
	}

	for _, operator := range operators {
		t.Run(string(operator), func(t *testing.T) {
			rule := Rule{
				AttributeKey: "department",
				Operator:     operator,
				Multi:        false,
				Value:        "ABCD-123",
			}

			if err := rule.Validate(); err != nil {
				t.Fatalf("Validate error = %v, want nil", err)
			}
		})
	}
}
```

- [ ] **Step 2: Run domain tests and verify failure**

Run:

```bash
go test ./internal/domain/group -v
```

Expected: FAIL because the `internal/domain/group` package does not exist.

- [ ] **Step 3: Add domain errors**

Create `internal/domain/group/errors.go` with this content:

```go
package group

import "errors"

var (
	ErrInvalidInput  = errors.New("invalid group input")
	ErrDuplicateName = errors.New("duplicate group name")
)
```

- [ ] **Step 4: Add domain models and normalization**

Create `internal/domain/group/group.go` with this content:

```go
package group

import (
	"strings"
	"time"
)

type Operator string

const (
	OperatorEq    Operator = "eq"
	OperatorNotEq Operator = "not_eq"
	OperatorGt    Operator = "gt"
	OperatorGte   Operator = "gte"
	OperatorLt    Operator = "lt"
	OperatorLte   Operator = "lte"
)

type Group struct {
	ID                string
	WorkspaceID       string
	Name              string
	NormalizedName    string
	Description       string
	GroupingRule      GroupingRule
	IndividualMembers []IndividualMember
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

type CreateInput struct {
	WorkspaceID       string
	Name              string
	Description       string
	GroupingRule      GroupingRule
	IndividualMembers []IndividualMember
}

type GroupingRule struct {
	Rules          []Rule
	ExpirationDate time.Time
}

type Rule struct {
	AttributeKey string
	Operator     Operator
	Multi        bool
	Value        any
}

type IndividualMember struct {
	ID             string
	GroupID        string
	NTAccount      string
	ExpirationDate time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

func (input CreateInput) Normalize() CreateInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.Name = strings.TrimSpace(input.Name)
	input.GroupingRule = input.GroupingRule.Normalize()
	for i := range input.IndividualMembers {
		input.IndividualMembers[i] = input.IndividualMembers[i].Normalize()
	}
	return input
}

func (rule GroupingRule) Normalize() GroupingRule {
	for i := range rule.Rules {
		rule.Rules[i] = rule.Rules[i].Normalize()
	}
	return rule
}

func (rule Rule) Normalize() Rule {
	rule.AttributeKey = strings.TrimSpace(rule.AttributeKey)
	return rule
}

func (member IndividualMember) Normalize() IndividualMember {
	member.NTAccount = strings.TrimSpace(member.NTAccount)
	return member
}
```

- [ ] **Step 5: Add domain validation**

Create `internal/domain/group/validation.go` with this content:

```go
package group

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

func (input CreateInput) Validate(now time.Time) error {
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(input.Name) == "" {
		return invalidInput("name is required")
	}
	if input.GroupingRule.ExpirationDate.IsZero() {
		return invalidInput("grouping rule expiration date is required")
	}
	if !input.GroupingRule.ExpirationDate.After(now) {
		return invalidInput("grouping rule expiration date must be in the future")
	}
	if len(input.GroupingRule.Rules) == 0 && len(input.IndividualMembers) == 0 {
		return invalidInput("at least one membership source is required")
	}
	for _, rule := range input.GroupingRule.Rules {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return validateIndividualMembers(input.IndividualMembers, now)
}

func (rule Rule) Validate() error {
	if strings.TrimSpace(rule.AttributeKey) == "" {
		return invalidInput("rule attribute key is required")
	}
	if !IsValidOperator(rule.Operator) {
		return invalidInput(fmt.Sprintf("rule operator is invalid: %s", rule.Operator))
	}
	if rule.Multi {
		length, valueAt, ok := arrayValue(rule.Value)
		if !ok || length == 0 {
			return invalidInput("multi-value rule value must be a non-empty array")
		}
		for i := 0; i < length; i++ {
			if isNilValue(valueAt(i)) {
				return invalidInput("multi-value rule value items must not be null")
			}
		}
		return nil
	}
	if isNilValue(rule.Value) {
		return invalidInput("single-value rule value is required")
	}
	if _, _, ok := arrayValue(rule.Value); ok {
		return invalidInput("single-value rule value must not be an array")
	}
	return nil
}

func IsValidOperator(operator Operator) bool {
	switch operator {
	case OperatorEq, OperatorNotEq, OperatorGt, OperatorGte, OperatorLt, OperatorLte:
		return true
	default:
		return false
	}
}

func validateIndividualMembers(members []IndividualMember, now time.Time) error {
	seen := map[string]struct{}{}
	for _, member := range members {
		account := strings.TrimSpace(member.NTAccount)
		if account == "" {
			return invalidInput("individual member nt account is required")
		}
		if _, ok := seen[account]; ok {
			return invalidInput(fmt.Sprintf("duplicate individual member nt account %q", account))
		}
		seen[account] = struct{}{}
		if member.ExpirationDate.IsZero() {
			return invalidInput("individual member expiration date is required")
		}
		if !member.ExpirationDate.After(now) {
			return invalidInput("individual member expiration date must be in the future")
		}
	}
	return nil
}

func arrayValue(value any) (int, func(int) any, bool) {
	if value == nil {
		return 0, nil, false
	}
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return 0, nil, false
	}
	return v.Len(), func(index int) any {
		return v.Index(index).Interface()
	}, true
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
```

- [ ] **Step 6: Run domain tests and verify pass**

Run:

```bash
go test ./internal/domain/group -v
```

Expected: PASS.

- [ ] **Step 7: Commit domain package**

```bash
git add internal/domain/group/errors.go internal/domain/group/group.go internal/domain/group/validation.go internal/domain/group/validation_test.go
git commit -m "feat: add group domain model"
```

---

### Task 2: Transport Request and Response Contracts

**Files:**

- Create: `internal/group-service/transport/group_request.go`
- Create: `internal/group-service/transport/group_request_test.go`
- Create: `internal/group-service/transport/group_response.go`
- Create: `internal/group-service/transport/group_response_test.go`

- [ ] **Step 1: Write failing request mapping tests**

Create `internal/group-service/transport/group_request_test.go` with this content:

```go
package transport

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func TestDecodeGroupCreateRequestToDomain(t *testing.T) {
	body := strings.NewReader(`{
		"name": " Design Reviewers ",
		"description": "Employees who can review design documents.",
		"grouping_rule": {
			"rules": [
				{
					"attribute_key": " department ",
					"operator": "eq",
					"multi": false,
					"value": "ABCD-123"
				},
				{
					"attribute_key": "level",
					"operator": "gte",
					"multi": true,
					"value": [5, 6]
				}
			],
			"expiration_date": "2026-06-01T00:00:00Z"
		},
		"individual_members": [
			{
				"nt_account": " user1 ",
				"expiration_date": "2026-06-02T00:00:00Z"
			}
		]
	}`)

	request, err := DecodeGroupCreateRequest(body)
	if err != nil {
		t.Fatalf("DecodeGroupCreateRequest error = %v, want nil", err)
	}
	input, err := request.ToDomain("workspace-1")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}

	if input.WorkspaceID != "workspace-1" {
		t.Fatalf("WorkspaceID = %q, want workspace-1", input.WorkspaceID)
	}
	if input.Name != " Design Reviewers " {
		t.Fatalf("Name = %q, want original request value", input.Name)
	}
	if len(input.GroupingRule.Rules) != 2 {
		t.Fatalf("rules len = %d, want 2", len(input.GroupingRule.Rules))
	}
	if input.GroupingRule.Rules[0].Operator != group.OperatorEq {
		t.Fatalf("operator = %q, want eq", input.GroupingRule.Rules[0].Operator)
	}
	if input.GroupingRule.Rules[1].Multi != true {
		t.Fatal("second rule Multi = false, want true")
	}
	values, ok := input.GroupingRule.Rules[1].Value.([]any)
	if !ok || len(values) != 2 {
		t.Fatalf("second rule value = %#v, want two JSON array items", input.GroupingRule.Rules[1].Value)
	}
	wantGroupingExpiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !input.GroupingRule.ExpirationDate.Equal(wantGroupingExpiration) {
		t.Fatalf("grouping expiration = %s, want %s", input.GroupingRule.ExpirationDate, wantGroupingExpiration)
	}
	wantMemberExpiration := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	if !input.IndividualMembers[0].ExpirationDate.Equal(wantMemberExpiration) {
		t.Fatalf("member expiration = %s, want %s", input.IndividualMembers[0].ExpirationDate, wantMemberExpiration)
	}
}

func TestDecodeGroupCreateRequestRejectsMalformedJSON(t *testing.T) {
	_, err := DecodeGroupCreateRequest(strings.NewReader(`{"name":`))
	if err == nil {
		t.Fatal("DecodeGroupCreateRequest error = nil, want error")
	}
}

func TestGroupCreateRequestToDomainRejectsMissingGroupingRule(t *testing.T) {
	request := GroupCreateRequest{Name: "Design Reviewers"}

	_, err := request.ToDomain("workspace-1")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}

func TestGroupCreateRequestToDomainRejectsMissingRuleMulti(t *testing.T) {
	request := GroupCreateRequest{
		Name: "Design Reviewers",
		GroupingRule: &GroupingRuleRequest{
			Rules: []RuleRequest{{
				AttributeKey: "department",
				Operator:     "eq",
				Value:        "ABCD-123",
			}},
			ExpirationDate: JSONTime{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		},
	}

	_, err := request.ToDomain("workspace-1")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}

func TestJSONTimeRejectsInvalidTimestamp(t *testing.T) {
	_, err := DecodeGroupCreateRequest(strings.NewReader(`{
		"name": "Design Reviewers",
		"grouping_rule": {
			"rules": [],
			"expiration_date": "not-a-time"
		},
		"individual_members": []
	}`))
	if err == nil {
		t.Fatal("DecodeGroupCreateRequest error = nil, want error")
	}
}
```

- [ ] **Step 2: Write failing response tests**

Create `internal/group-service/transport/group_response_test.go` with this content:

```go
package transport

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func TestNewGroupCreateResponse(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	model := group.Group{
		ID:             "group-1",
		WorkspaceID:    "workspace-1",
		Name:           "Design Reviewers",
		NormalizedName: "Design Reviewers",
		Description:    "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: "department",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: expiration,
		},
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-1",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: expiration,
		}},
		CreatedAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	}

	response := NewGroupCreateResponse(model)
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}

	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}
	groupBody := body["group"].(map[string]any)
	if groupBody["id"] != "group-1" {
		t.Fatalf("id = %v, want group-1", groupBody["id"])
	}
	if _, ok := groupBody["workspace_id"]; ok {
		t.Fatal("workspace_id is present, want omitted")
	}
	if _, ok := groupBody["created_at"]; ok {
		t.Fatal("created_at is present, want omitted")
	}
	if groupBody["name"] != "Design Reviewers" {
		t.Fatalf("name = %v, want Design Reviewers", groupBody["name"])
	}
}
```

- [ ] **Step 3: Run transport tests and verify failure**

Run:

```bash
go test ./internal/group-service/transport -v
```

Expected: FAIL because the `internal/group-service/transport` package does not exist.

- [ ] **Step 4: Add create request DTOs and mapping**

Create `internal/group-service/transport/group_request.go` with this content:

```go
package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type GroupCreateRequest struct {
	Name              string                    `json:"name"`
	Description       string                    `json:"description"`
	GroupingRule      *GroupingRuleRequest      `json:"grouping_rule"`
	IndividualMembers []IndividualMemberRequest `json:"individual_members"`
}

type GroupingRuleRequest struct {
	Rules          []RuleRequest `json:"rules"`
	ExpirationDate JSONTime      `json:"expiration_date"`
}

type RuleRequest struct {
	AttributeKey string `json:"attribute_key"`
	Operator     string `json:"operator"`
	Multi        *bool  `json:"multi"`
	Value        any    `json:"value"`
}

type IndividualMemberRequest struct {
	NTAccount      string   `json:"nt_account"`
	ExpirationDate JSONTime `json:"expiration_date"`
}

func DecodeGroupCreateRequest(body io.Reader) (GroupCreateRequest, error) {
	var request GroupCreateRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return GroupCreateRequest{}, fmt.Errorf("decode group create request: %w", err)
	}
	return request, nil
}

func (request GroupCreateRequest) ToDomain(workspaceID string) (group.CreateInput, error) {
	if request.GroupingRule == nil {
		return group.CreateInput{}, invalidGroupRequest("grouping rule is required")
	}
	rules := make([]group.Rule, 0, len(request.GroupingRule.Rules))
	for _, rule := range request.GroupingRule.Rules {
		if rule.Multi == nil {
			return group.CreateInput{}, invalidGroupRequest("rule multi is required")
		}
		rules = append(rules, group.Rule{
			AttributeKey: rule.AttributeKey,
			Operator:     group.Operator(rule.Operator),
			Multi:        *rule.Multi,
			Value:        rule.Value,
		})
	}
	members := make([]group.IndividualMember, 0, len(request.IndividualMembers))
	for _, member := range request.IndividualMembers {
		members = append(members, group.IndividualMember{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate.Time,
		})
	}
	return group.CreateInput{
		WorkspaceID:  workspaceID,
		Name:         request.Name,
		Description:  request.Description,
		GroupingRule: group.GroupingRule{Rules: rules, ExpirationDate: request.GroupingRule.ExpirationDate.Time},
		IndividualMembers: members,
	}, nil
}

func invalidGroupRequest(message string) error {
	return fmt.Errorf("%w: %s", group.ErrInvalidInput, message)
}

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

- [ ] **Step 5: Add create response DTOs**

Create `internal/group-service/transport/group_response.go` with this content:

```go
package transport

import (
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type GroupCreateResponse struct {
	Group GroupResponse `json:"group"`
}

type GroupResponse struct {
	ID                string                     `json:"id"`
	Name              string                     `json:"name"`
	Description       string                     `json:"description"`
	GroupingRule      GroupingRuleResponse       `json:"grouping_rule"`
	IndividualMembers []IndividualMemberResponse `json:"individual_members"`
}

type GroupingRuleResponse struct {
	Rules          []RuleResponse `json:"rules"`
	ExpirationDate time.Time      `json:"expiration_date"`
}

type RuleResponse struct {
	AttributeKey string         `json:"attribute_key"`
	Operator     group.Operator `json:"operator"`
	Multi        bool           `json:"multi"`
	Value        any            `json:"value"`
}

type IndividualMemberResponse struct {
	NTAccount      string    `json:"nt_account"`
	ExpirationDate time.Time `json:"expiration_date"`
}

func NewGroupCreateResponse(model group.Group) GroupCreateResponse {
	return GroupCreateResponse{Group: newGroupResponse(model)}
}

func newGroupResponse(model group.Group) GroupResponse {
	rules := make([]RuleResponse, 0, len(model.GroupingRule.Rules))
	for _, rule := range model.GroupingRule.Rules {
		rules = append(rules, RuleResponse{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	members := make([]IndividualMemberResponse, 0, len(model.IndividualMembers))
	for _, member := range model.IndividualMembers {
		members = append(members, IndividualMemberResponse{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
		})
	}
	return GroupResponse{
		ID:          model.ID,
		Name:        model.Name,
		Description: model.Description,
		GroupingRule: GroupingRuleResponse{
			Rules:          rules,
			ExpirationDate: model.GroupingRule.ExpirationDate,
		},
		IndividualMembers: members,
	}
}
```

- [ ] **Step 6: Run transport tests and verify pass**

Run:

```bash
go test ./internal/group-service/transport -v
```

Expected: PASS.

- [ ] **Step 7: Commit transport package**

```bash
git add internal/group-service/transport/group_request.go internal/group-service/transport/group_request_test.go internal/group-service/transport/group_response.go internal/group-service/transport/group_response_test.go
git commit -m "feat: add group transport contract"
```

---

### Task 3: Service Create Workflow

**Files:**

- Create: `internal/group-service/services/group_service.go`
- Create: `internal/group-service/services/group_service_test.go`

- [ ] **Step 1: Write failing service tests**

Create `internal/group-service/services/group_service_test.go` with this content:

```go
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type fakeGroupRepository struct {
	input group.Group
	err   error
	calls int
}

func (f *fakeGroupRepository) Create(ctx context.Context, input group.Group) (group.Group, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return group.Group{}, f.err
	}
	return input, nil
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
}

func serviceFutureTime() time.Time {
	return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
}

func validServiceCreateInput() group.CreateInput {
	return group.CreateInput{
		WorkspaceID: "workspace-1",
		Name:        " Design Reviewers ",
		Description: "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: " department ",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: serviceFutureTime(),
		},
		IndividualMembers: []group.IndividualMember{{
			NTAccount:      " user1 ",
			ExpirationDate: serviceFutureTime(),
		}},
	}
}

func TestGroupServiceCreateGroup(t *testing.T) {
	repository := &fakeGroupRepository{}
	ids := []string{"group-1", "member-1"}
	service := NewGroupService(repository,
		WithGroupClock(fixedNow),
		WithGroupIDGenerator(func() string {
			id := ids[0]
			ids = ids[1:]
			return id
		}),
	)

	model, err := service.CreateGroup(context.Background(), validServiceCreateInput())
	if err != nil {
		t.Fatalf("CreateGroup error = %v, want nil", err)
	}
	if repository.calls != 1 {
		t.Fatalf("repository calls = %d, want 1", repository.calls)
	}
	if model.ID != "group-1" {
		t.Fatalf("ID = %q, want group-1", model.ID)
	}
	if model.Name != "Design Reviewers" {
		t.Fatalf("Name = %q, want Design Reviewers", model.Name)
	}
	if model.NormalizedName != "Design Reviewers" {
		t.Fatalf("NormalizedName = %q, want Design Reviewers", model.NormalizedName)
	}
	if model.GroupingRule.Rules[0].AttributeKey != "department" {
		t.Fatalf("AttributeKey = %q, want department", model.GroupingRule.Rules[0].AttributeKey)
	}
	if model.IndividualMembers[0].ID != "member-1" {
		t.Fatalf("member ID = %q, want member-1", model.IndividualMembers[0].ID)
	}
	if model.IndividualMembers[0].GroupID != "group-1" {
		t.Fatalf("member GroupID = %q, want group-1", model.IndividualMembers[0].GroupID)
	}
	if !model.CreatedAt.Equal(fixedNow()) || !model.UpdatedAt.Equal(fixedNow()) {
		t.Fatalf("timestamps = %s/%s, want fixed now", model.CreatedAt, model.UpdatedAt)
	}
}

func TestGroupServiceCreateGroupValidationFailureDoesNotCallRepository(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))
	input := validServiceCreateInput()
	input.Name = " "

	_, err := service.CreateGroup(context.Background(), input)
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("CreateGroup error = %v, want ErrInvalidInput", err)
	}
	if repository.calls != 0 {
		t.Fatalf("repository calls = %d, want 0", repository.calls)
	}
}

func TestGroupServiceCreateGroupDuplicateName(t *testing.T) {
	repository := &fakeGroupRepository{err: group.ErrDuplicateName}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	_, err := service.CreateGroup(context.Background(), validServiceCreateInput())
	if !errors.Is(err, group.ErrDuplicateName) {
		t.Fatalf("CreateGroup error = %v, want ErrDuplicateName", err)
	}
}

func TestGroupServiceCreateGroupRepositoryFailure(t *testing.T) {
	repository := &fakeGroupRepository{err: errors.New("database unavailable")}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	_, err := service.CreateGroup(context.Background(), validServiceCreateInput())
	if err == nil {
		t.Fatal("CreateGroup error = nil, want error")
	}
	if errors.Is(err, group.ErrDuplicateName) {
		t.Fatalf("CreateGroup error = %v, should not be ErrDuplicateName", err)
	}
}
```

- [ ] **Step 2: Run service tests and verify failure**

Run:

```bash
go test ./internal/group-service/services -v
```

Expected: FAIL because `NewGroupService` is undefined.

- [ ] **Step 3: Add service workflow**

Create `internal/group-service/services/group_service.go` with this content:

```go
package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type GroupRepository interface {
	Create(ctx context.Context, input group.Group) (group.Group, error)
}

type GroupOption func(*GroupService)

func WithGroupIDGenerator(generator func() string) GroupOption {
	return func(s *GroupService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}

func WithGroupClock(clock func() time.Time) GroupOption {
	return func(s *GroupService) {
		if clock != nil {
			s.now = clock
		}
	}
}

type GroupService struct {
	repository  GroupRepository
	idGenerator func() string
	now         func() time.Time
}

func NewGroupService(repository GroupRepository, opts ...GroupOption) *GroupService {
	service := &GroupService{
		repository:  repository,
		idGenerator: uuid.NewString,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *GroupService) CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error) {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(now); err != nil {
		return group.Group{}, err
	}

	model := group.Group{
		ID:             s.idGenerator(),
		WorkspaceID:    input.WorkspaceID,
		Name:           input.Name,
		NormalizedName: input.Name,
		Description:    input.Description,
		GroupingRule:   input.GroupingRule,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	model.IndividualMembers = s.newIndividualMembers(model.ID, input.IndividualMembers, now)

	saved, err := s.repository.Create(ctx, model)
	if err != nil {
		if errors.Is(err, group.ErrDuplicateName) {
			return group.Group{}, err
		}
		return group.Group{}, fmt.Errorf("create group: %w", err)
	}
	return saved, nil
}

func (s *GroupService) newIndividualMembers(groupID string, input []group.IndividualMember, now time.Time) []group.IndividualMember {
	members := make([]group.IndividualMember, 0, len(input))
	for _, member := range input {
		members = append(members, group.IndividualMember{
			ID:             s.idGenerator(),
			GroupID:        groupID,
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	return members
}
```

- [ ] **Step 4: Run service tests and verify pass**

Run:

```bash
go test ./internal/group-service/services -v
```

Expected: PASS.

- [ ] **Step 5: Commit service workflow**

```bash
git add internal/group-service/services/group_service.go internal/group-service/services/group_service_test.go
git commit -m "feat: add group create service"
```

---

### Task 4: MongoDB Repository, Indexes, and Transaction

**Files:**

- Create: `internal/group-service/repositories/mongo_group_repository.go`
- Create: `internal/group-service/repositories/mongo_group_repository_test.go`

- [ ] **Step 1: Write failing repository tests**

Create `internal/group-service/repositories/mongo_group_repository_test.go` with this content:

```go
package repositories

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func repositoryTime() time.Time {
	return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
}

func repositoryGroup() group.Group {
	now := repositoryTime()
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return group.Group{
		ID:             "group-1",
		WorkspaceID:    "workspace-1",
		Name:           "Design Reviewers",
		NormalizedName: "Design Reviewers",
		Description:    "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: "department",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: expiration,
		},
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-1",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: expiration,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestNewGroupDocumentMapping(t *testing.T) {
	model := repositoryGroup()
	doc := newGroupDocument(model)

	if doc.ID != "group-1" || doc.WorkspaceID != "workspace-1" {
		t.Fatalf("doc identity = %+v, want group-1/workspace-1", doc)
	}
	if doc.DeletedAt != nil {
		t.Fatal("DeletedAt != nil, want nil")
	}
	if len(doc.GroupingRule.Rules) != 1 {
		t.Fatalf("rules len = %d, want 1", len(doc.GroupingRule.Rules))
	}

	got := doc.toDomain(model.IndividualMembers)
	if got.ID != model.ID || got.Name != model.Name {
		t.Fatalf("domain = %+v, want ID/name copied", got)
	}
}

func TestNewIndividualMemberDocuments(t *testing.T) {
	model := repositoryGroup()
	docs := newIndividualMemberDocuments(model)

	if len(docs) != 1 {
		t.Fatalf("docs len = %d, want 1", len(docs))
	}
	if docs[0].ID != "member-1" || docs[0].GroupID != "group-1" {
		t.Fatalf("member doc = %+v, want member-1/group-1", docs[0])
	}
	if docs[0].DeletedAt != nil {
		t.Fatal("DeletedAt != nil, want nil")
	}
}

func TestIndexModels(t *testing.T) {
	groupIndexes := groupIndexModels()
	if len(groupIndexes) != 2 {
		t.Fatalf("group indexes len = %d, want 2", len(groupIndexes))
	}
	if *groupIndexes[0].Options.Name != groupsActiveNameUniqueIndexName {
		t.Fatalf("group unique index name = %q, want %q", *groupIndexes[0].Options.Name, groupsActiveNameUniqueIndexName)
	}
	if groupIndexes[0].Options.Unique == nil || !*groupIndexes[0].Options.Unique {
		t.Fatal("group unique index Unique = false, want true")
	}

	memberIndexes := individualMemberIndexModels()
	if len(memberIndexes) != 2 {
		t.Fatalf("member indexes len = %d, want 2", len(memberIndexes))
	}
	if *memberIndexes[0].Options.Name != membersActiveGroupAccountUniqueIndexName {
		t.Fatalf("member unique index name = %q, want %q", *memberIndexes[0].Options.Name, membersActiveGroupAccountUniqueIndexName)
	}
}

func TestIsDuplicateIndex(t *testing.T) {
	err := mongo.WriteException{
		WriteErrors: []mongo.WriteError{{
			Code:    11000,
			Message: "E11000 duplicate key error collection: groups index: " + groupsActiveNameUniqueIndexName + " dup key",
		}},
	}

	if !isDuplicateIndex(err, groupsActiveNameUniqueIndexName) {
		t.Fatal("isDuplicateIndex = false, want true")
	}
	if isDuplicateIndex(err, membersActiveGroupAccountUniqueIndexName) {
		t.Fatal("isDuplicateIndex for member index = true, want false")
	}
}

func TestGroupDocumentBSONKeys(t *testing.T) {
	doc := newGroupDocument(repositoryGroup())
	data, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var raw bson.M
	if err := bson.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}

	for _, key := range []string{"_id", "workspace_id", "name", "normalized_name", "description", "grouping_rule", "created_at", "updated_at", "deleted_at"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("BSON key %q missing from %#v", key, raw)
		}
	}
}

func TestMongoGroupRepositoryCreateIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	model := repositoryGroup()
	got, err := repository.Create(context.Background(), model)
	if err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, model) {
		t.Fatalf("created group = %#v, want %#v", got, model)
	}

	groupCount, err := db.Collection(groupCollectionName).CountDocuments(context.Background(), bson.M{"_id": "group-1"})
	if err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if groupCount != 1 {
		t.Fatalf("group count = %d, want 1", groupCount)
	}
	memberCount, err := db.Collection(groupIndividualMemberCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1"})
	if err != nil {
		t.Fatalf("count members: %v", err)
	}
	if memberCount != 1 {
		t.Fatalf("member count = %d, want 1", memberCount)
	}
}

func TestMongoGroupRepositoryDuplicateActiveNameIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	first := repositoryGroup()
	if _, err := repository.Create(context.Background(), first); err != nil {
		t.Fatalf("first Create error = %v, want nil", err)
	}
	second := repositoryGroup()
	second.ID = "group-2"
	second.IndividualMembers = nil

	_, err := repository.Create(context.Background(), second)
	if !errors.Is(err, group.ErrDuplicateName) {
		t.Fatalf("second Create error = %v, want ErrDuplicateName", err)
	}
}

func TestMongoGroupRepositorySameNameDifferentWorkspaceIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	first := repositoryGroup()
	second := repositoryGroup()
	second.ID = "group-2"
	second.WorkspaceID = "workspace-2"
	second.IndividualMembers = nil

	if _, err := repository.Create(context.Background(), first); err != nil {
		t.Fatalf("first Create error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), second); err != nil {
		t.Fatalf("second Create error = %v, want nil", err)
	}
}

func TestMongoGroupRepositoryRollsBackMemberInsertFailureIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	model := repositoryGroup()
	model.IndividualMembers = []group.IndividualMember{
		{ID: "member-duplicate", GroupID: "group-1", NTAccount: "user1", ExpirationDate: repositoryTime().Add(time.Hour), CreatedAt: repositoryTime(), UpdatedAt: repositoryTime()},
		{ID: "member-duplicate", GroupID: "group-1", NTAccount: "user2", ExpirationDate: repositoryTime().Add(time.Hour), CreatedAt: repositoryTime(), UpdatedAt: repositoryTime()},
	}

	if _, err := repository.Create(context.Background(), model); err == nil {
		t.Fatal("Create error = nil, want duplicate _id error")
	}
	groupCount, err := db.Collection(groupCollectionName).CountDocuments(context.Background(), bson.M{"_id": "group-1"})
	if err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if groupCount != 0 {
		t.Fatalf("group count = %d, want rollback to 0", groupCount)
	}
}

func newIntegrationDatabase(t *testing.T) (*mongo.Client, *mongo.Database) {
	t.Helper()
	uri := os.Getenv("GROUP_SERVICE_MONGODB_TEST_URI")
	if strings.TrimSpace(uri) == "" {
		t.Skip("GROUP_SERVICE_MONGODB_TEST_URI is not set")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect mongodb: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Fatalf("disconnect mongodb: %v", err)
		}
	})
	db := client.Database("workspace_permission_management_group_service_test_" + strings.ReplaceAll(t.Name(), "/", "_"))
	t.Cleanup(func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Fatalf("drop database: %v", err)
		}
	})
	return client, db
}
```

- [ ] **Step 2: Run repository tests and verify failure**

Run:

```bash
go test ./internal/group-service/repositories -v
```

Expected: FAIL because repository symbols are undefined. Integration tests skip unless `GROUP_SERVICE_MONGODB_TEST_URI` is set.

- [ ] **Step 3: Add Mongo repository**

Create `internal/group-service/repositories/mongo_group_repository.go` with this content:

```go
package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	groupCollectionName                       = "groups"
	groupIndividualMemberCollectionName       = "group_individual_members"
	groupsActiveNameUniqueIndexName           = "groups_active_workspace_normalized_name_unique"
	groupsWorkspaceCreatedIndexName           = "groups_workspace_created_id"
	membersActiveGroupAccountUniqueIndexName  = "group_individual_members_active_group_account_unique"
	membersGroupIDIndexName                   = "group_individual_members_group_id"
)

type MongoGroupRepository struct {
	client  *mongo.Client
	groups  *mongo.Collection
	members *mongo.Collection
}

type groupDocument struct {
	ID             string               `bson:"_id"`
	WorkspaceID    string               `bson:"workspace_id"`
	Name           string               `bson:"name"`
	NormalizedName string               `bson:"normalized_name"`
	Description    string               `bson:"description"`
	GroupingRule   groupingRuleDocument `bson:"grouping_rule"`
	CreatedAt      time.Time            `bson:"created_at"`
	UpdatedAt      time.Time            `bson:"updated_at"`
	DeletedAt      *time.Time           `bson:"deleted_at"`
}

type groupingRuleDocument struct {
	Rules          []ruleDocument `bson:"rules"`
	ExpirationDate time.Time      `bson:"expiration_date"`
}

type ruleDocument struct {
	AttributeKey string         `bson:"attribute_key"`
	Operator     group.Operator `bson:"operator"`
	Multi        bool           `bson:"multi"`
	Value        any            `bson:"value"`
}

type individualMemberDocument struct {
	ID             string     `bson:"_id"`
	GroupID        string     `bson:"group_id"`
	NTAccount      string     `bson:"nt_account"`
	ExpirationDate time.Time  `bson:"expiration_date"`
	CreatedAt      time.Time  `bson:"created_at"`
	UpdatedAt      time.Time  `bson:"updated_at"`
	DeletedAt      *time.Time `bson:"deleted_at"`
}

func NewMongoGroupRepository(client *mongo.Client, db *mongo.Database) *MongoGroupRepository {
	return &MongoGroupRepository{
		client:  client,
		groups:  db.Collection(groupCollectionName),
		members: db.Collection(groupIndividualMemberCollectionName),
	}
}

func (r *MongoGroupRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.groups.Indexes().CreateMany(ctx, groupIndexModels()); err != nil {
		return fmt.Errorf("create group indexes: %w", err)
	}
	if _, err := r.members.Indexes().CreateMany(ctx, individualMemberIndexModels()); err != nil {
		return fmt.Errorf("create group individual member indexes: %w", err)
	}
	return nil
}

func (r *MongoGroupRepository) Create(ctx context.Context, input group.Group) (group.Group, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return group.Group{}, fmt.Errorf("start group create session: %w", err)
	}
	defer session.EndSession(ctx)

	if err := mongo.WithSession(ctx, session, func(sessionCtx context.Context) error {
		if err := session.StartTransaction(); err != nil {
			return fmt.Errorf("start group create transaction: %w", err)
		}
		if _, err := r.groups.InsertOne(sessionCtx, newGroupDocument(input)); err != nil {
			return r.abortTransaction(sessionCtx, session, mapGroupInsertError(err))
		}
		memberDocs := newIndividualMemberDocuments(input)
		if len(memberDocs) > 0 {
			docs := make([]any, 0, len(memberDocs))
			for _, doc := range memberDocs {
				docs = append(docs, doc)
			}
			if _, err := r.members.InsertMany(sessionCtx, docs); err != nil {
				return r.abortTransaction(sessionCtx, session, fmt.Errorf("insert group individual members: %w", err))
			}
		}
		if err := session.CommitTransaction(sessionCtx); err != nil {
			return fmt.Errorf("commit group create transaction: %w", err)
		}
		return nil
	}); err != nil {
		return group.Group{}, err
	}
	return input, nil
}

func (r *MongoGroupRepository) abortTransaction(ctx context.Context, session *mongo.Session, cause error) error {
	if err := session.AbortTransaction(ctx); err != nil {
		return fmt.Errorf("%w; abort group create transaction: %v", cause, err)
	}
	return cause
}

func groupIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "workspace_id", Value: 1},
				{Key: "normalized_name", Value: 1},
			},
			Options: options.Index().
				SetName(groupsActiveNameUniqueIndexName).
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"deleted_at": nil}),
		},
		{
			Keys: bson.D{
				{Key: "workspace_id", Value: 1},
				{Key: "created_at", Value: -1},
				{Key: "_id", Value: -1},
			},
			Options: options.Index().SetName(groupsWorkspaceCreatedIndexName),
		},
	}
}

func individualMemberIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "group_id", Value: 1},
				{Key: "nt_account", Value: 1},
			},
			Options: options.Index().
				SetName(membersActiveGroupAccountUniqueIndexName).
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"deleted_at": nil}),
		},
		{
			Keys: bson.D{{Key: "group_id", Value: 1}},
			Options: options.Index().SetName(membersGroupIDIndexName),
		},
	}
}

func mapGroupInsertError(err error) error {
	if isDuplicateIndex(err, groupsActiveNameUniqueIndexName) {
		return fmt.Errorf("%w: active group name already exists", group.ErrDuplicateName)
	}
	return fmt.Errorf("insert group: %w", err)
}

func isDuplicateIndex(err error, indexName string) bool {
	return mongo.IsDuplicateKeyError(err) && strings.Contains(err.Error(), indexName)
}

func newGroupDocument(model group.Group) groupDocument {
	rules := make([]ruleDocument, 0, len(model.GroupingRule.Rules))
	for _, rule := range model.GroupingRule.Rules {
		rules = append(rules, ruleDocument{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	return groupDocument{
		ID:             model.ID,
		WorkspaceID:    model.WorkspaceID,
		Name:           model.Name,
		NormalizedName: model.NormalizedName,
		Description:    model.Description,
		GroupingRule: groupingRuleDocument{
			Rules:          rules,
			ExpirationDate: model.GroupingRule.ExpirationDate,
		},
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		DeletedAt: model.DeletedAt,
	}
}

func newIndividualMemberDocuments(model group.Group) []individualMemberDocument {
	docs := make([]individualMemberDocument, 0, len(model.IndividualMembers))
	for _, member := range model.IndividualMembers {
		docs = append(docs, individualMemberDocument{
			ID:             member.ID,
			GroupID:        member.GroupID,
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
			CreatedAt:      member.CreatedAt,
			UpdatedAt:      member.UpdatedAt,
			DeletedAt:      member.DeletedAt,
		})
	}
	return docs
}

func (d groupDocument) toDomain(members []group.IndividualMember) group.Group {
	rules := make([]group.Rule, 0, len(d.GroupingRule.Rules))
	for _, rule := range d.GroupingRule.Rules {
		rules = append(rules, group.Rule{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	return group.Group{
		ID:             d.ID,
		WorkspaceID:    d.WorkspaceID,
		Name:           d.Name,
		NormalizedName: d.NormalizedName,
		Description:    d.Description,
		GroupingRule: group.GroupingRule{
			Rules:          rules,
			ExpirationDate: d.GroupingRule.ExpirationDate,
		},
		IndividualMembers: members,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
		DeletedAt:         d.DeletedAt,
	}
}
```

- [ ] **Step 4: Run repository unit tests and verify pass**

Run:

```bash
go test ./internal/group-service/repositories -v
```

Expected: PASS with Mongo integration tests skipped when `GROUP_SERVICE_MONGODB_TEST_URI` is not set.

- [ ] **Step 5: Run transaction integration tests when MongoDB is available**

Run:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -run Integration -v
```

Expected: PASS when local MongoDB is running as a replica set from `docker-compose.yml`.

- [ ] **Step 6: Commit repository package**

```bash
git add internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_group_repository_test.go
git commit -m "feat: persist groups transactionally"
```

---

### Task 5: HTTP Handler and Route

**Files:**

- Create: `internal/group-service/handlers/group_handler.go`
- Create: `internal/group-service/handlers/group_handler_test.go`
- Create: `internal/group-service/handlers/test_logger_test.go`

- [ ] **Step 1: Write failing handler tests**

Create `internal/group-service/handlers/test_logger_test.go` with this content:

```go
package handlers

import (
	"io"
	"log/slog"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
```

Create `internal/group-service/handlers/group_handler_test.go` with this content:

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

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/labstack/echo/v5"
)

type fakeHTTPGroupService struct {
	input group.CreateInput
	model group.Group
	err   error
	calls int
}

func (f *fakeHTTPGroupService) CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return group.Group{}, f.err
	}
	return f.model, nil
}

func validGroupRequestBody() string {
	return `{
		"name": "Design Reviewers",
		"description": "Employees who can review design documents.",
		"grouping_rule": {
			"rules": [
				{
					"attribute_key": "department",
					"operator": "eq",
					"multi": false,
					"value": "ABCD-123"
				}
			],
			"expiration_date": "2026-06-01T00:00:00Z"
		},
		"individual_members": [
			{
				"nt_account": "user1",
				"expiration_date": "2026-06-01T00:00:00Z"
			}
		]
	}`
}

func groupModel() group.Group {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return group.Group{
		ID:             "group-1",
		WorkspaceID:    "workspace-1",
		Name:           "Design Reviewers",
		NormalizedName: "Design Reviewers",
		Description:    "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: "department",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: expiration,
		},
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-1",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: expiration,
		}},
	}
}

func TestGroupHandlerCreateGroup(t *testing.T) {
	service := &fakeHTTPGroupService{model: groupModel()}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if service.calls != 1 {
		t.Fatalf("service calls = %d, want 1", service.calls)
	}
	if service.input.WorkspaceID != "workspace-1" {
		t.Fatalf("WorkspaceID = %q, want workspace-1", service.input.WorkspaceID)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["group"]; !ok {
		t.Fatal("response missing group")
	}
}

func TestGroupHandlerRejectsMalformedJSON(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(`{"name":`))
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

func TestGroupHandlerValidationError(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrInvalidInput}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGroupHandlerDuplicateName(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrDuplicateName}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestGroupHandlerServiceFailure(t *testing.T) {
	service := &fakeHTTPGroupService{err: errors.New("database unavailable")}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
```

- [ ] **Step 2: Run handler tests and verify failure**

Run:

```bash
go test ./internal/group-service/handlers -v
```

Expected: FAIL because handler symbols are undefined.

- [ ] **Step 3: Add group handler**

Create `internal/group-service/handlers/group_handler.go` with this content:

```go
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/group-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/http/exception"
	"github.com/labstack/echo/v5"
)

type HTTPGroupService interface {
	CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error)
}

type GroupHandler struct {
	service HTTPGroupService
	logger  *slog.Logger
}

type groupPathParams struct {
	workspaceID string
}

func NewGroupHandler(service HTTPGroupService, logger *slog.Logger) *GroupHandler {
	return &GroupHandler{service: service, logger: logger}
}

func RegisterRoutes(e *echo.Echo, handler *GroupHandler) {
	e.POST("/api/v1/workspaces/:workspace_id/groups", handler.CreateGroup)
}

func newGroupPathParams(c *echo.Context) groupPathParams {
	return groupPathParams{workspaceID: c.Param("workspace_id")}
}

func (h *GroupHandler) CreateGroup(c *echo.Context) error {
	params := newGroupPathParams(c)
	request, err := transport.DecodeGroupCreateRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(params.workspaceID)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}

	model, err := h.service.CreateGroup(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, group.ErrDuplicateName) {
			return c.JSON(http.StatusConflict, exception.WrapResponse(exception.New("conflict", "Group name already exists", exception.WithDetails(map[string]any{}))))
		}
		h.logger.Warn("failed to create group", "err", err, "workspace_id", params.workspaceID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusCreated, transport.NewGroupCreateResponse(model))
}

func validationError(message string) exception.ErrorResponse {
	return exception.WrapResponse(exception.New("validation_failed", message, exception.WithDetails(map[string]any{})))
}
```

- [ ] **Step 4: Run handler tests and verify pass**

Run:

```bash
go test ./internal/group-service/handlers -v
```

Expected: PASS.

- [ ] **Step 5: Commit handler package**

```bash
git add internal/group-service/handlers/group_handler.go internal/group-service/handlers/group_handler_test.go internal/group-service/handlers/test_logger_test.go
git commit -m "feat: add group create handler"
```

---

### Task 6: Configuration and Service Entrypoint

**Files:**

- Create: `internal/group-service/config/config.go`
- Create: `internal/group-service/config/config_test.go`
- Create: `cmd/group-service/main.go`
- Create: `cmd/group-service/main_test.go`

- [ ] **Step 1: Write failing config tests**

Create `internal/group-service/config/config_test.go` with this content:

```go
package config

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

func TestLoadReadsRequiredEnvironment(t *testing.T) {
	t.Setenv("GROUP_SERVICE_ENV", "production")
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":9090")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://example:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "wpm")
	t.Setenv("GROUP_SERVICE_SHUTDOWN_TIMEOUT", "15s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Production {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, environment.Production)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.MongoDB.URI != "mongodb://example:27017" {
		t.Fatalf("MongoDB.URI = %q, want mongodb://example:27017", cfg.MongoDB.URI)
	}
	if cfg.MongoDB.Database != "wpm" {
		t.Fatalf("MongoDB.Database = %q, want wpm", cfg.MongoDB.Database)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 15s", cfg.ShutdownTimeout)
	}
}

func TestLoadAppliesOptionalDefaults(t *testing.T) {
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Development {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, environment.Development)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 10s", cfg.ShutdownTimeout)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("GROUP_SERVICE_ENV", "staging")
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsMissingRequiredValue(t *testing.T) {
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsInvalidShutdownTimeout(t *testing.T) {
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("GROUP_SERVICE_SHUTDOWN_TIMEOUT", "0s")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}
```

- [ ] **Step 2: Write failing main tests**

Create `cmd/group-service/main_test.go` with this content:

```go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestProcessIndicator(t *testing.T) {
	indicator := processIndicator{}
	if indicator.Name() != "process" {
		t.Fatalf("Name = %q, want process", indicator.Name())
	}
	if !indicator.IsHealthy(context.Background()) {
		t.Fatal("IsHealthy = false, want true")
	}
}

func TestRegisterHealthRoutes(t *testing.T) {
	e := echo.New()
	registerHealthRoutes(e)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/liveness", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
```

- [ ] **Step 3: Run config and main tests and verify failure**

Run:

```bash
go test ./internal/group-service/config ./cmd/group-service -v
```

Expected: FAIL because config and main implementation files are missing.

- [ ] **Step 4: Add group-service config**

Create `internal/group-service/config/config.go` with this content:

```go
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

type Config struct {
	Environment     environment.Environment
	HTTPAddr        string
	MongoDB         MongoDBConfig
	ShutdownTimeout time.Duration
}

type MongoDBConfig struct {
	URI      string
	Database string
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("GROUP_SERVICE_ENV", string(environment.Development))
	v.SetDefault("GROUP_SERVICE_SHUTDOWN_TIMEOUT", "10s")

	cfg := Config{
		Environment: environment.Environment(v.GetString("GROUP_SERVICE_ENV")),
		HTTPAddr:    v.GetString("GROUP_SERVICE_HTTP_ADDR"),
		MongoDB: MongoDBConfig{
			URI:      v.GetString("GROUP_SERVICE_MONGODB_URI"),
			Database: v.GetString("GROUP_SERVICE_MONGODB_DATABASE"),
		},
		ShutdownTimeout: v.GetDuration("GROUP_SERVICE_SHUTDOWN_TIMEOUT"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !environment.IsValidEnvironment(c.Environment) {
		return fmt.Errorf("%w: GROUP_SERVICE_ENV must be %q or %q", environment.ErrInvalidEnv, environment.Development, environment.Production)
	}
	required := map[string]string{
		"GROUP_SERVICE_HTTP_ADDR":        c.HTTPAddr,
		"GROUP_SERVICE_MONGODB_URI":      c.MongoDB.URI,
		"GROUP_SERVICE_MONGODB_DATABASE": c.MongoDB.Database,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("GROUP_SERVICE_SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}
```

- [ ] **Step 5: Add group-service entrypoint**

Create `cmd/group-service/main.go` with this content:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hao0731/workspace-permission-management/internal/group-service/config"
	"github.com/hao0731/workspace-permission-management/internal/group-service/handlers"
	"github.com/hao0731/workspace-permission-management/internal/group-service/repositories"
	"github.com/hao0731/workspace-permission-management/internal/group-service/services"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
	"github.com/labstack/echo/v5"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type processIndicator struct{}

func (processIndicator) Name() string {
	return "process"
}

func (processIndicator) IsHealthy(context.Context) bool {
	return true
}

func main() {
	if err := run(); err != nil {
		slog.Error("group service stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := sharedlogger.New(cfg.Environment)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoDB.URI))
	if err != nil {
		return err
	}
	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if disconnectErr := mongoClient.Disconnect(disconnectCtx); disconnectErr != nil {
			logger.Warn("failed to disconnect mongodb", "err", disconnectErr)
		}
	}()

	db := mongoClient.Database(cfg.MongoDB.Database)
	repository := repositories.NewMongoGroupRepository(mongoClient, db)
	if ensureIndexErr := repository.EnsureIndexes(ctx); ensureIndexErr != nil {
		return ensureIndexErr
	}

	groupService := services.NewGroupService(repository)

	e := echo.New()
	registerHealthRoutes(e)
	handlers.RegisterRoutes(e, handlers.NewGroupHandler(groupService, logger))

	errCh := make(chan error, 1)
	go func() {
		startConfig := echo.StartConfig{
			Address:         cfg.HTTPAddr,
			GracefulTimeout: cfg.ShutdownTimeout,
		}
		if err := startConfig.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			stop()
			return err
		}
	}
	return nil
}

func registerHealthRoutes(e *echo.Echo) {
	health.NewHealthManager(processIndicator{}).RegisterRoutes(e)
}
```

- [ ] **Step 6: Run config and main tests and verify pass**

Run:

```bash
go test ./internal/group-service/config ./cmd/group-service -v
```

Expected: PASS.

- [ ] **Step 7: Commit config and entrypoint**

```bash
git add internal/group-service/config/config.go internal/group-service/config/config_test.go cmd/group-service/main.go cmd/group-service/main_test.go
git commit -m "feat: wire group service"
```

---

### Task 7: REST Client Examples and Full Verification

**Files:**

- Create: `examples/api/groups.http`

- [ ] **Step 1: Add REST Client examples**

Create `examples/api/groups.http` with this content:

```http
@baseUrl = http://localhost:8081
@workspaceId = workspace-1

### Create group with grouping rules and individual members
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups
Content-Type: application/json

{
  "name": "Design Reviewers",
  "description": "Employees who can review design documents.",
  "grouping_rule": {
    "rules": [
      {
        "attribute_key": "department",
        "operator": "eq",
        "multi": false,
        "value": "ABCD-123"
      },
      {
        "attribute_key": "level",
        "operator": "gte",
        "multi": false,
        "value": 5
      }
    ],
    "expiration_date": "2026-06-01T00:00:00Z"
  },
  "individual_members": [
    {
      "nt_account": "user1",
      "expiration_date": "2026-06-01T00:00:00Z"
    }
  ]
}

### Duplicate active group name in the same workspace returns 409
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups
Content-Type: application/json

{
  "name": "Design Reviewers",
  "description": "Duplicate name example.",
  "grouping_rule": {
    "rules": [
      {
        "attribute_key": "department",
        "operator": "eq",
        "multi": false,
        "value": "ABCD-123"
      }
    ],
    "expiration_date": "2026-06-01T00:00:00Z"
  },
  "individual_members": []
}

### Missing membership source returns 400
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups
Content-Type: application/json

{
  "name": "Empty Group",
  "description": "This request has no dynamic rules or individual members.",
  "grouping_rule": {
    "rules": [],
    "expiration_date": "2026-06-01T00:00:00Z"
  },
  "individual_members": []
}

### Duplicate individual nt_account values return 400
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups
Content-Type: application/json

{
  "name": "Duplicate Members",
  "description": "This request repeats the same individual member.",
  "grouping_rule": {
    "rules": [],
    "expiration_date": "2026-06-01T00:00:00Z"
  },
  "individual_members": [
    {
      "nt_account": "user1",
      "expiration_date": "2026-06-01T00:00:00Z"
    },
    {
      "nt_account": "user1",
      "expiration_date": "2026-06-02T00:00:00Z"
    }
  ]
}

### Multi-value rule with empty array returns 400
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups
Content-Type: application/json

{
  "name": "Invalid Rule",
  "description": "This request has an invalid multi-value rule.",
  "grouping_rule": {
    "rules": [
      {
        "attribute_key": "department",
        "operator": "eq",
        "multi": true,
        "value": []
      }
    ],
    "expiration_date": "2026-06-01T00:00:00Z"
  },
  "individual_members": []
}

### Liveness probe
GET {{baseUrl}}/health/liveness
```

- [ ] **Step 2: Run focused package tests**

Run:

```bash
go test ./internal/domain/group ./internal/group-service/transport ./internal/group-service/services ./internal/group-service/repositories ./internal/group-service/handlers ./internal/group-service/config ./cmd/group-service -v
```

Expected: PASS. Repository integration tests skip unless `GROUP_SERVICE_MONGODB_TEST_URI` is set.

- [ ] **Step 3: Run full backend verification**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Run opt-in Mongo transaction integration verification**

When local MongoDB is available, run:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -run Integration -v
```

Expected: PASS with transaction-backed create, duplicate active-name rejection, same-name-different-workspace success, and rollback on member insert failure.

- [ ] **Step 5: Inspect changed files**

Run:

```bash
git status --short
git diff --stat
```

Expected: only group-service implementation files and `examples/api/groups.http` are changed, plus this plan if task checkboxes were updated during execution.

- [ ] **Step 6: Commit REST examples and final verification state**

```bash
git add examples/api/groups.http
git commit -m "docs: add group api examples"
```

If task checkboxes in this plan were updated during implementation, include the plan file in the same commit:

```bash
git add docs/plans/active/2026-05-09-group-service.md examples/api/groups.http
git commit -m "docs: add group api examples"
```

---

## Final Implementation Checklist

- [ ] `POST /api/v1/workspaces/:workspace_id/groups` returns `201 Created`.
- [ ] Response body uses `{ "group": ... }` and omits `workspace_id`, `created_at`, `updated_at`, `deleted_at`, and `normalized_name`.
- [ ] `groups` documents include `_id`, `workspace_id`, `name`, `normalized_name`, `description`, `grouping_rule`, `created_at`, `updated_at`, and `deleted_at: null`.
- [ ] `group_individual_members` documents include `_id`, `group_id`, `nt_account`, `expiration_date`, `created_at`, `updated_at`, and `deleted_at: null`.
- [ ] Repository creates the active group name partial unique index.
- [ ] Repository creates the active `group_id + nt_account` partial unique index.
- [ ] Repository writes group and member documents in one MongoDB transaction.
- [ ] Duplicate active group names in the same workspace map to `409 Conflict`.
- [ ] Same group name in different workspaces is accepted.
- [ ] Empty membership source returns `400 validation_failed`.
- [ ] Duplicate request `individual_members[].nt_account` returns `400 validation_failed`.
- [ ] All expiration dates must be later than service `now`.
- [ ] `GET /health/liveness` returns `200 OK` for the process indicator.
- [ ] `examples/api/groups.http` includes success, conflict, validation, and liveness examples.
- [ ] `go test ./...` passes.

## Verification Record

Record the observed verification output here during implementation:

```txt
go test ./...:
```

```txt
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -run Integration -v:
```
