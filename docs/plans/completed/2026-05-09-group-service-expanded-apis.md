# Group Service Expanded APIs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `group-service` with group read, idempotent group soft delete, grouping-rule replacement, and paginated individual member reads.

**Architecture:** Expand the existing `group-service` along current backend boundaries: domain owns identities, cursors, and invariant validation; transport owns JSON DTOs and cursor token encoding; service owns use-case validation, time generation, and error wrapping; repository owns MongoDB filters, transactions, soft deletes, and pagination; handlers stay thin and map service outcomes to HTTP responses. The implementation preserves the existing create workflow while adding active-document reads and soft-delete behavior around the same `groups` and `group_individual_members` collections.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, `log/slog`, `internal/shared/pagination`, `internal/shared/http/exception`, standard `encoding/json`, standard `testing`.

---

## Source Designs

Primary source design: [../../designs/group-service.md](../../designs/group-service.md)

Related source designs:

- [../../designs/group-service-group.md](../../designs/group-service-group.md)
- [../../designs/group-service-individual-members.md](../../designs/group-service-individual-members.md)
- [../../designs/shared-pagination-helper-refactor.md](../../designs/shared-pagination-helper-refactor.md)

Applicable policies:

- [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend implementation must keep handlers thin, place HTTP DTOs in transport packages, keep domain independent from Echo and MongoDB, define repository interfaces at the service consumer side, and treat API and MongoDB schemas as explicit contracts with tests and REST Client examples.
- Implementation plans must live under `docs/plans/active/`, link to source designs, and be committed once finalized. Completed plans move to `docs/plans/completed/` after implementation.

## Scope

Implement:

- `GET /api/v1/workspaces/:workspace_id/groups/:group_id` returning `200` with a group object or `200` with `group: null`.
- `DELETE /api/v1/workspaces/:workspace_id/groups/:group_id` as idempotent `204`, soft-deleting the active group and active individual members in one MongoDB transaction.
- `PUT /api/v1/workspaces/:workspace_id/groups/:group_id/grouping-rules` returning `204`, `404` for missing active group, and `400` when empty rules would leave the group without active individual members.
- `GET /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members` returning cursor-paginated active individual members sorted by `created_at DESC, _id DESC`.
- Domain, transport, service, repository, handler, main wiring, tests, and REST Client examples for the new API surface.

Do not implement:

- Group list API.
- Group name or description update APIs.
- Individual member add, replace, delete, restore, or bulk mutation APIs.
- Workspace existence validation.
- Dynamic grouping-rule evaluation or permission evaluation.
- NATS, JetStream, CloudEvents, or frontend changes.

## File Structure and Responsibilities

- Modify: `internal/domain/group/errors.go`
  - Add `ErrNotFound`.
- Modify: `internal/domain/group/group.go`
  - Add get/delete/update/list input models, member cursor, member page, and normalization helpers.
- Modify: `internal/domain/group/validation.go`
  - Add validation methods for path identities, grouping-rule replacement, and member-list cursors.
- Modify: `internal/domain/group/validation_test.go`
  - Add domain tests for new inputs and cursors.
- Modify: `internal/group-service/transport/group_request.go`
  - Add grouping-rules request decoding and DTO-to-domain mapping.
- Create: `internal/group-service/transport/pagination.go`
  - Add individual-member cursor token encode/decode helpers.
- Create: `internal/group-service/transport/pagination_test.go`
  - Add token encode/decode tests.
- Modify: `internal/group-service/transport/group_response.go`
  - Add nullable GET group response, member list response, and page info DTOs.
- Modify: `internal/group-service/transport/group_response_test.go`
  - Add GET and member-list response tests.
- Modify: `internal/group-service/services/group_service.go`
  - Extend repository interface and add get/delete/update/list service methods.
- Modify: `internal/group-service/services/group_service_test.go`
  - Add service workflow tests for the new methods.
- Modify: `internal/group-service/repositories/mongo_group_repository.go`
  - Add active filters, get, soft delete transaction, grouping-rule replacement transaction, member pagination, and pagination index update.
- Modify: `internal/group-service/repositories/mongo_group_repository_test.go`
  - Add mapping, index, filter, and integration tests for reads, soft deletes, updates, and pagination.
- Modify: `internal/group-service/handlers/group_handler.go`
  - Register new routes, inject pagination helper, and map new service methods to HTTP responses.
- Modify: `internal/group-service/handlers/group_handler_test.go`
  - Add handler tests for new routes and error mappings.
- Modify: `cmd/group-service/main.go`
  - Wire `pagination.New()` into `NewGroupHandler`.
- Modify: `examples/api/groups.http`
  - Add REST examples for group read/delete/update and member pagination.

---

### Task 1: Domain Contracts for Group Reads, Deletes, Updates, and Member Pagination

**Files:**

- Modify: `internal/domain/group/errors.go`
- Modify: `internal/domain/group/group.go`
- Modify: `internal/domain/group/validation.go`
- Modify: `internal/domain/group/validation_test.go`

- [ ] **Step 1: Write failing domain tests for group identity inputs**

Append these tests to `internal/domain/group/validation_test.go`:

```go
func TestGetQueryValidate(t *testing.T) {
	query := GetQuery{WorkspaceID: "workspace-1", GroupID: "group-1"}
	if err := query.Normalize().Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestGroupIdentityValidationRejectsBlankFields(t *testing.T) {
	tests := []struct {
		name        string
		validate    func() error
		wantMessage string
	}{
		{
			name: "get blank workspace",
			validate: func() error {
				return GetQuery{WorkspaceID: " ", GroupID: "group-1"}.Normalize().Validate()
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "get blank group id",
			validate: func() error {
				return GetQuery{WorkspaceID: "workspace-1", GroupID: " "}.Normalize().Validate()
			},
			wantMessage: "group id is required",
		},
		{
			name: "delete blank group id",
			validate: func() error {
				return DeleteInput{WorkspaceID: "workspace-1", GroupID: " "}.Normalize().Validate()
			},
			wantMessage: "group id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireInvalidInput(t, tt.validate(), tt.wantMessage)
		})
	}
}
```

- [ ] **Step 2: Run the domain tests and confirm they fail**

Run:

```bash
go test ./internal/domain/group -run 'Test(GetQuery|GroupIdentity)' -v
```

Expected: FAIL because `GetQuery` and `DeleteInput` are undefined.

- [ ] **Step 3: Add domain errors and input/page types**

Update `internal/domain/group/errors.go`:

```go
package group

import "errors"

var (
	ErrInvalidInput  = errors.New("invalid group input")
	ErrDuplicateName = errors.New("duplicate group name")
	ErrNotFound      = errors.New("group not found")
)
```

Append these types to `internal/domain/group/group.go`:

```go
type GetQuery struct {
	WorkspaceID string
	GroupID     string
}

type DeleteInput struct {
	WorkspaceID string
	GroupID     string
}

type UpdateGroupingRuleInput struct {
	WorkspaceID     string
	GroupID         string
	Rules           []Rule
	ExpirationDate  time.Time
}

type IndividualMemberCursor struct {
	CreatedAt time.Time
	ID        string
}

type ListIndividualMembersQuery struct {
	WorkspaceID string
	GroupID     string
	Limit       int
	Cursor      *IndividualMemberCursor
}

type IndividualMemberPage struct {
	Members     []IndividualMember
	HasNextPage bool
	NextCursor  *IndividualMemberCursor
}
```

- [ ] **Step 4: Add normalization helpers**

Append these helpers to `internal/domain/group/group.go`:

```go
func (query GetQuery) Normalize() GetQuery {
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.GroupID = strings.TrimSpace(query.GroupID)
	return query
}

func (input DeleteInput) Normalize() DeleteInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	return input
}

func (input UpdateGroupingRuleInput) Normalize() UpdateGroupingRuleInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	for i := range input.Rules {
		input.Rules[i] = input.Rules[i].Normalize()
	}
	return input
}

func (query ListIndividualMembersQuery) Normalize() ListIndividualMembersQuery {
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.GroupID = strings.TrimSpace(query.GroupID)
	if query.Cursor != nil {
		query.Cursor.ID = strings.TrimSpace(query.Cursor.ID)
	}
	return query
}
```

- [ ] **Step 5: Add identity validation**

Append these functions to `internal/domain/group/validation.go`:

```go
func (query GetQuery) Validate() error {
	return validateGroupIdentity(query.WorkspaceID, query.GroupID)
}

func (input DeleteInput) Validate() error {
	return validateGroupIdentity(input.WorkspaceID, input.GroupID)
}

func validateGroupIdentity(workspaceID string, groupID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(groupID) == "" {
		return invalidInput("group id is required")
	}
	return nil
}
```

- [ ] **Step 6: Run identity tests and confirm they pass**

Run:

```bash
go test ./internal/domain/group -run 'Test(GetQuery|GroupIdentity)' -v
```

Expected: PASS.

- [ ] **Step 7: Write failing domain tests for grouping-rule replacement**

Append these tests to `internal/domain/group/validation_test.go`:

```go
func TestUpdateGroupingRuleInputValidate(t *testing.T) {
	input := UpdateGroupingRuleInput{
		WorkspaceID:    " workspace-1 ",
		GroupID:        " group-1 ",
		ExpirationDate: futureTime(),
		Rules: []Rule{{
			AttributeKey: " department ",
			Operator:     OperatorEq,
			Multi:        false,
			Value:        "ABCD-123",
		}},
	}.Normalize()

	if err := input.Validate(validationNow(), WithMaxGroupingRules(1)); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" {
		t.Fatalf("identity = %q/%q, want trimmed values", input.WorkspaceID, input.GroupID)
	}
	if input.Rules[0].AttributeKey != "department" {
		t.Fatalf("AttributeKey = %q, want department", input.Rules[0].AttributeKey)
	}
}

func TestUpdateGroupingRuleInputAllowsEmptyRulesAtDomainBoundary(t *testing.T) {
	input := UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		ExpirationDate: futureTime(),
		Rules:          nil,
	}

	if err := input.Validate(validationNow()); err != nil {
		t.Fatalf("Validate error = %v, want nil because active member count is repository-backed", err)
	}
}

func TestUpdateGroupingRuleInputRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		input       UpdateGroupingRuleInput
		wantMessage string
	}{
		{
			name:        "blank group id",
			input:       UpdateGroupingRuleInput{WorkspaceID: "workspace-1", GroupID: " ", ExpirationDate: futureTime()},
			wantMessage: "group id is required",
		},
		{
			name:        "missing expiration",
			input:       UpdateGroupingRuleInput{WorkspaceID: "workspace-1", GroupID: "group-1"},
			wantMessage: "grouping rule expiration date is required",
		},
		{
			name:        "past expiration",
			input:       UpdateGroupingRuleInput{WorkspaceID: "workspace-1", GroupID: "group-1", ExpirationDate: validationNow()},
			wantMessage: "grouping rule expiration date must be in the future",
		},
		{
			name: "too many rules",
			input: UpdateGroupingRuleInput{
				WorkspaceID:    "workspace-1",
				GroupID:        "group-1",
				ExpirationDate: futureTime(),
				Rules: []Rule{
					{AttributeKey: "department", Operator: OperatorEq, Multi: false, Value: "ABCD-123"},
					{AttributeKey: "level", Operator: OperatorGte, Multi: false, Value: 5},
				},
			},
			wantMessage: "grouping rules must not exceed 1 items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Normalize().Validate(validationNow(), WithMaxGroupingRules(1))
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}
```

- [ ] **Step 8: Run grouping-rule tests and confirm they fail**

Run:

```bash
go test ./internal/domain/group -run TestUpdateGroupingRuleInput -v
```

Expected: FAIL because `UpdateGroupingRuleInput.Validate` is undefined.

- [ ] **Step 9: Add grouping-rule replacement validation**

Append this method to `internal/domain/group/validation.go`:

```go
func (input UpdateGroupingRuleInput) Validate(now time.Time, opts ...ValidateOption) error {
	if err := validateGroupIdentity(input.WorkspaceID, input.GroupID); err != nil {
		return err
	}
	options := defaultValidateOptions()
	for _, opt := range opts {
		if opt != nil {
			opt.applyValidateOption(&options)
		}
	}
	options = options.withDefaults()
	if input.ExpirationDate.IsZero() {
		return invalidInput("grouping rule expiration date is required")
	}
	if !input.ExpirationDate.After(now) {
		return invalidInput("grouping rule expiration date must be in the future")
	}
	if len(input.Rules) > options.maxGroupingRules {
		return invalidInput(fmt.Sprintf("grouping rules must not exceed %d items", options.maxGroupingRules))
	}
	for _, rule := range input.Rules {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 10: Write failing domain tests for member list query**

Append these tests to `internal/domain/group/validation_test.go`:

```go
func TestListIndividualMembersQueryValidate(t *testing.T) {
	query := ListIndividualMembersQuery{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		Limit:       20,
		Cursor: &IndividualMemberCursor{
			CreatedAt: validationNow(),
			ID:        " member-1 ",
		},
	}.Normalize()

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if query.WorkspaceID != "workspace-1" || query.GroupID != "group-1" || query.Cursor.ID != "member-1" {
		t.Fatalf("query = %+v, want trimmed identity and cursor", query)
	}
}

func TestListIndividualMembersQueryRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		query       ListIndividualMembersQuery
		wantMessage string
	}{
		{
			name:        "blank group id",
			query:       ListIndividualMembersQuery{WorkspaceID: "workspace-1", GroupID: " ", Limit: 20},
			wantMessage: "group id is required",
		},
		{
			name:        "zero limit",
			query:       ListIndividualMembersQuery{WorkspaceID: "workspace-1", GroupID: "group-1"},
			wantMessage: "limit must be greater than zero",
		},
		{
			name: "cursor missing created at",
			query: ListIndividualMembersQuery{
				WorkspaceID: "workspace-1",
				GroupID:     "group-1",
				Limit:       20,
				Cursor:      &IndividualMemberCursor{ID: "member-1"},
			},
			wantMessage: "cursor created_at is required",
		},
		{
			name: "cursor missing id",
			query: ListIndividualMembersQuery{
				WorkspaceID: "workspace-1",
				GroupID:     "group-1",
				Limit:       20,
				Cursor:      &IndividualMemberCursor{CreatedAt: validationNow()},
			},
			wantMessage: "cursor id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Normalize().Validate()
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}
```

- [ ] **Step 11: Run member list domain tests and confirm they fail**

Run:

```bash
go test ./internal/domain/group -run TestListIndividualMembersQuery -v
```

Expected: FAIL because `ListIndividualMembersQuery.Validate` is undefined.

- [ ] **Step 12: Add member list validation**

Append this method to `internal/domain/group/validation.go`:

```go
func (query ListIndividualMembersQuery) Validate() error {
	if err := validateGroupIdentity(query.WorkspaceID, query.GroupID); err != nil {
		return err
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
```

- [ ] **Step 13: Run all group domain tests**

Run:

```bash
go test ./internal/domain/group -v
```

Expected: PASS.

- [ ] **Step 14: Commit domain contract changes**

```bash
git add internal/domain/group/errors.go internal/domain/group/group.go internal/domain/group/validation.go internal/domain/group/validation_test.go
git commit -m "feat: add group service domain contracts"
```

---

### Task 2: Transport DTOs, Nullable Group Response, and Member Cursor Tokens

**Files:**

- Modify: `internal/group-service/transport/group_request.go`
- Create: `internal/group-service/transport/pagination.go`
- Create: `internal/group-service/transport/pagination_test.go`
- Modify: `internal/group-service/transport/group_response.go`
- Modify: `internal/group-service/transport/group_request_test.go`
- Modify: `internal/group-service/transport/group_response_test.go`

- [ ] **Step 1: Write failing grouping-rules request tests**

Append this test to `internal/group-service/transport/group_request_test.go`:

```go
func TestGroupGroupingRulesRequestToDomain(t *testing.T) {
	multi := false
	request := GroupGroupingRulesRequest{
		Rules: []RuleRequest{{
			AttributeKey: "department",
			Operator:     "eq",
			Multi:        &multi,
			Value:        "ABCD-123",
		}},
		ExpirationDate: JSONTime{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
	}

	input, err := request.ToDomain("workspace-1", "group-1")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" {
		t.Fatalf("identity = %q/%q, want workspace-1/group-1", input.WorkspaceID, input.GroupID)
	}
	if len(input.Rules) != 1 || input.Rules[0].Operator != group.OperatorEq {
		t.Fatalf("rules = %+v, want one eq rule", input.Rules)
	}
}

func TestGroupGroupingRulesRequestRejectsMissingMulti(t *testing.T) {
	request := GroupGroupingRulesRequest{
		Rules: []RuleRequest{{
			AttributeKey: "department",
			Operator:     "eq",
			Value:        "ABCD-123",
		}},
		ExpirationDate: JSONTime{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
	}

	_, err := request.ToDomain("workspace-1", "group-1")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}
```

- [ ] **Step 2: Run request tests and confirm they fail**

Run:

```bash
go test ./internal/group-service/transport -run TestGroupGroupingRulesRequest -v
```

Expected: FAIL because `GroupGroupingRulesRequest` is undefined.

- [ ] **Step 3: Add grouping-rules request DTO and mapper**

Append this code to `internal/group-service/transport/group_request.go`:

```go
type GroupGroupingRulesRequest struct {
	Rules          []RuleRequest `json:"rules"`
	ExpirationDate JSONTime      `json:"expiration_date"`
}

func DecodeGroupGroupingRulesRequest(body io.Reader) (GroupGroupingRulesRequest, error) {
	var request GroupGroupingRulesRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return GroupGroupingRulesRequest{}, fmt.Errorf("decode group grouping rules request: %w", err)
	}
	return request, nil
}

func (request GroupGroupingRulesRequest) ToDomain(workspaceID string, groupID string) (group.UpdateGroupingRuleInput, error) {
	rules, err := newDomainRules(request.Rules)
	if err != nil {
		return group.UpdateGroupingRuleInput{}, err
	}
	return group.UpdateGroupingRuleInput{
		WorkspaceID:    workspaceID,
		GroupID:        groupID,
		Rules:          rules,
		ExpirationDate: request.ExpirationDate.Time,
	}, nil
}

func newDomainRules(input []RuleRequest) ([]group.Rule, error) {
	rules := make([]group.Rule, 0, len(input))
	for _, rule := range input {
		if rule.Multi == nil {
			return nil, invalidGroupRequest("rule multi is required")
		}
		rules = append(rules, group.Rule{
			AttributeKey: rule.AttributeKey,
			Operator:     group.Operator(rule.Operator),
			Multi:        *rule.Multi,
			Value:        rule.Value,
		})
	}
	return rules, nil
}
```

Then update `GroupCreateRequest.ToDomain` to call `newDomainRules(request.GroupingRule.Rules)` instead of duplicating the rule mapping loop.

- [ ] **Step 4: Run request tests**

Run:

```bash
go test ./internal/group-service/transport -run 'TestGroup(CreateRequest|GroupingRulesRequest)' -v
```

Expected: PASS.

- [ ] **Step 5: Write failing cursor token tests**

Create `internal/group-service/transport/pagination_test.go`:

```go
package transport

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func TestEncodeDecodeIndividualMemberNextToken(t *testing.T) {
	cursor := &group.IndividualMemberCursor{
		CreatedAt: time.Date(2026, 5, 9, 7, 31, 0, 0, time.UTC),
		ID:        "member-123",
	}

	token, err := EncodeIndividualMemberNextToken(cursor)
	if err != nil {
		t.Fatalf("Encode error = %v, want nil", err)
	}
	got, err := DecodeIndividualMemberNextToken(token)
	if err != nil {
		t.Fatalf("Decode error = %v, want nil", err)
	}
	if got.ID != cursor.ID || !got.CreatedAt.Equal(cursor.CreatedAt) {
		t.Fatalf("cursor = %+v, want %+v", got, cursor)
	}
}

func TestDecodeIndividualMemberNextTokenEmpty(t *testing.T) {
	cursor, err := DecodeIndividualMemberNextToken("")
	if err != nil {
		t.Fatalf("Decode error = %v, want nil", err)
	}
	if cursor != nil {
		t.Fatalf("cursor = %+v, want nil", cursor)
	}
}

func TestDecodeIndividualMemberNextTokenRejectsInvalidToken(t *testing.T) {
	_, err := DecodeIndividualMemberNextToken("not-base64")
	if err == nil {
		t.Fatal("Decode error = nil, want error")
	}
}
```

- [ ] **Step 6: Run cursor tests and confirm they fail**

Run:

```bash
go test ./internal/group-service/transport -run IndividualMemberNextToken -v
```

Expected: FAIL because the token helper functions are undefined.

- [ ] **Step 7: Add cursor token helpers**

Create `internal/group-service/transport/pagination.go`:

```go
package transport

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/pagination"
)

type individualMemberNextTokenPayload struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func EncodeIndividualMemberNextToken(cursor *group.IndividualMemberCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	payload := individualMemberNextTokenPayload{
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        cursor.ID,
	}
	return pagination.EncodeNextToken(payload)
}

func DecodeIndividualMemberNextToken(token string) (*group.IndividualMemberCursor, error) {
	payload, err := pagination.DecodeNextToken[individualMemberNextTokenPayload](token)
	if err != nil {
		if errors.Is(err, pagination.ErrEmptyToken) {
			return nil, nil
		}
		return nil, err
	}
	if strings.TrimSpace(payload.ID) == "" {
		return nil, fmt.Errorf("next_token.id is required")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("next_token.created_at must be RFC3339 timestamp")
	}
	return &group.IndividualMemberCursor{CreatedAt: createdAt, ID: strings.TrimSpace(payload.ID)}, nil
}
```

- [ ] **Step 8: Write failing response tests**

Append these tests to `internal/group-service/transport/group_response_test.go`:

```go
func TestNewGroupGetResponseFound(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	model := group.Group{
		ID:          "group-1",
		WorkspaceID: "workspace-1",
		Name:        "Design Reviewers",
		Description: "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: "department",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: expiration,
		},
	}

	response := NewGroupGetResponse(&model)
	if response.Group == nil {
		t.Fatal("Group = nil, want group")
	}
	if response.Group.ID != "group-1" {
		t.Fatalf("ID = %q, want group-1", response.Group.ID)
	}
}

func TestNewGroupGetResponseMissing(t *testing.T) {
	response := NewGroupGetResponse(nil)
	if response.Group != nil {
		t.Fatalf("Group = %+v, want nil", response.Group)
	}
}

func TestNewIndividualMemberListResponse(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	page := group.IndividualMemberPage{
		Members: []group.IndividualMember{{
			ID:             "member-1",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: expiration,
		}},
		HasNextPage: true,
		NextCursor: &group.IndividualMemberCursor{
			CreatedAt: time.Date(2026, 5, 9, 7, 31, 0, 0, time.UTC),
			ID:        "member-1",
		},
	}

	response, err := NewIndividualMemberListResponse(page)
	if err != nil {
		t.Fatalf("response error = %v, want nil", err)
	}
	if len(response.Members) != 1 || response.Members[0].NTAccount != "user1" {
		t.Fatalf("Members = %+v, want user1", response.Members)
	}
	if !response.PageInfo.HasNextPage || response.PageInfo.NextToken == "" {
		t.Fatalf("PageInfo = %+v, want next page token", response.PageInfo)
	}
}
```

- [ ] **Step 9: Run response tests and confirm they fail**

Run:

```bash
go test ./internal/group-service/transport -run 'TestNew(GroupGet|IndividualMemberList)' -v
```

Expected: FAIL because response constructors are undefined.

- [ ] **Step 10: Add GET and member-list response DTOs**

Append these DTOs and constructors to `internal/group-service/transport/group_response.go`:

```go
type GroupGetResponse struct {
	Group *GroupSummaryResponse `json:"group"`
}

type GroupSummaryResponse struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	GroupingRule GroupingRuleResponse `json:"grouping_rule"`
}

type IndividualMemberListResponse struct {
	Members  []IndividualMemberResponse `json:"members"`
	PageInfo PageInfoResponse           `json:"page_info"`
}

type PageInfoResponse struct {
	HasNextPage bool   `json:"has_next_page"`
	NextToken   string `json:"next_token"`
}

func NewGroupGetResponse(model *group.Group) GroupGetResponse {
	if model == nil {
		return GroupGetResponse{Group: nil}
	}
	return GroupGetResponse{Group: newGroupSummaryResponse(*model)}
}

func newGroupSummaryResponse(model group.Group) *GroupSummaryResponse {
	return &GroupSummaryResponse{
		ID:           model.ID,
		Name:         model.Name,
		Description:  model.Description,
		GroupingRule: newGroupingRuleResponse(model.GroupingRule),
	}
}

func newGroupingRuleResponse(rule group.GroupingRule) GroupingRuleResponse {
	rules := make([]RuleResponse, 0, len(rule.Rules))
	for _, item := range rule.Rules {
		rules = append(rules, RuleResponse{
			AttributeKey: item.AttributeKey,
			Operator:     item.Operator,
			Multi:        item.Multi,
			Value:        item.Value,
		})
	}
	return GroupingRuleResponse{Rules: rules, ExpirationDate: rule.ExpirationDate}
}

func NewIndividualMemberListResponse(page group.IndividualMemberPage) (IndividualMemberListResponse, error) {
	members := make([]IndividualMemberResponse, 0, len(page.Members))
	for _, member := range page.Members {
		members = append(members, IndividualMemberResponse{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
		})
	}
	nextToken, err := EncodeIndividualMemberNextToken(page.NextCursor)
	if err != nil {
		return IndividualMemberListResponse{}, err
	}
	return IndividualMemberListResponse{
		Members: members,
		PageInfo: PageInfoResponse{
			HasNextPage: page.HasNextPage,
			NextToken:   nextToken,
		},
	}, nil
}
```

Update `newGroupResponse` to call `newGroupingRuleResponse(model.GroupingRule)` so create and get responses share rule mapping.

- [ ] **Step 11: Run all transport tests**

Run:

```bash
go test ./internal/group-service/transport -v
```

Expected: PASS.

- [ ] **Step 12: Commit transport changes**

```bash
git add internal/group-service/transport/group_request.go internal/group-service/transport/group_request_test.go internal/group-service/transport/group_response.go internal/group-service/transport/group_response_test.go internal/group-service/transport/pagination.go internal/group-service/transport/pagination_test.go
git commit -m "feat: add group service transport contracts"
```

---

### Task 3: MongoDB Repository Reads, Soft Deletes, Updates, and Member Pagination

**Files:**

- Modify: `internal/group-service/repositories/mongo_group_repository.go`
- Modify: `internal/group-service/repositories/mongo_group_repository_test.go`

- [ ] **Step 1: Write failing repository unit tests for filters and pagination index**

Append these tests to `internal/group-service/repositories/mongo_group_repository_test.go`:

```go
func TestActiveGroupFilter(t *testing.T) {
	filter := activeGroupFilter(group.GetQuery{WorkspaceID: "workspace-1", GroupID: "group-1"})
	want := bson.M{"_id": "group-1", "workspace_id": "workspace-1", "deleted_at": nil}

	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestBuildIndividualMemberListFilter(t *testing.T) {
	cursorTime := time.Date(2026, 5, 9, 7, 31, 0, 0, time.UTC)
	filter := buildIndividualMemberListFilter(group.ListIndividualMembersQuery{
		GroupID: "group-1",
		Cursor: &group.IndividualMemberCursor{CreatedAt: cursorTime, ID: "member-9"},
	})

	want := bson.M{
		"group_id":   "group-1",
		"deleted_at": nil,
		"$or": bson.A{
			bson.M{"created_at": bson.M{"$lt": cursorTime}},
			bson.M{"created_at": cursorTime, "_id": bson.M{"$lt": "member-9"}},
		},
	}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestIndividualMemberPaginationIndex(t *testing.T) {
	memberIndexes := individualMemberIndexModels()
	if len(memberIndexes) != 2 {
		t.Fatalf("member indexes len = %d, want 2", len(memberIndexes))
	}
	keys := memberIndexes[1].Keys
	want := bson.D{{Key: "group_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("member pagination index keys = %#v, want %#v", keys, want)
	}
}
```

- [ ] **Step 2: Run repository unit tests and confirm they fail**

Run:

```bash
go test ./internal/group-service/repositories -run 'Test(ActiveGroupFilter|BuildIndividualMemberListFilter|IndividualMemberPaginationIndex)' -v
```

Expected: FAIL because helper functions are undefined and the member index still uses `{ group_id: 1 }`.

- [ ] **Step 3: Add repository constants, filters, and pagination index**

Update the member index constant in `internal/group-service/repositories/mongo_group_repository.go`:

```go
membersGroupCreatedIndexName = "group_individual_members_group_created_id"
```

Add these helpers:

```go
func activeGroupFilter(query group.GetQuery) bson.M {
	return bson.M{
		"_id":          query.GroupID,
		"workspace_id": query.WorkspaceID,
		"deleted_at":   nil,
	}
}

func buildIndividualMemberListFilter(query group.ListIndividualMembersQuery) bson.M {
	filter := bson.M{
		"group_id":   query.GroupID,
		"deleted_at": nil,
	}
	if query.Cursor != nil {
		filter["$or"] = bson.A{
			bson.M{"created_at": bson.M{"$lt": query.Cursor.CreatedAt}},
			bson.M{"created_at": query.Cursor.CreatedAt, "_id": bson.M{"$lt": query.Cursor.ID}},
		}
	}
	return filter
}
```

Update the second member index model:

```go
{
	Keys: bson.D{
		{Key: "group_id", Value: 1},
		{Key: "created_at", Value: -1},
		{Key: "_id", Value: -1},
	},
	Options: options.Index().SetName(membersGroupCreatedIndexName),
},
```

- [ ] **Step 4: Run repository unit tests**

Run:

```bash
go test ./internal/group-service/repositories -run 'Test(ActiveGroupFilter|BuildIndividualMemberListFilter|IndividualMemberPaginationIndex|IndexModels)' -v
```

Expected: PASS.

- [ ] **Step 5: Write failing repository integration tests for get/delete/update/list**

Append these integration tests to `internal/group-service/repositories/mongo_group_repository_test.go`:

```go
func TestMongoGroupRepositoryGetIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	model := repositoryGroup()
	if _, err := repository.Create(context.Background(), model); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	got, err := repository.Get(context.Background(), group.GetQuery{WorkspaceID: "workspace-1", GroupID: "group-1"})
	if err != nil {
		t.Fatalf("Get error = %v, want nil", err)
	}
	if got == nil || got.ID != "group-1" || len(got.IndividualMembers) != 0 {
		t.Fatalf("Get = %+v, want group without embedded individual members", got)
	}
}

func TestMongoGroupRepositoryGetMissingIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)

	got, err := repository.Get(context.Background(), group.GetQuery{WorkspaceID: "workspace-1", GroupID: "missing"})
	if err != nil {
		t.Fatalf("Get error = %v, want nil", err)
	}
	if got != nil {
		t.Fatalf("Get = %+v, want nil", got)
	}
}

func TestMongoGroupRepositoryDeleteIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	deletedAt := repositoryTime().Add(time.Hour)
	if err := repository.Delete(context.Background(), group.DeleteInput{WorkspaceID: "workspace-1", GroupID: "group-1"}, deletedAt); err != nil {
		t.Fatalf("Delete error = %v, want nil", err)
	}

	groupCount, err := db.Collection(groupCollectionName).CountDocuments(context.Background(), bson.M{"_id": "group-1", "deleted_at": deletedAt})
	if err != nil {
		t.Fatalf("count deleted groups: %v", err)
	}
	if groupCount != 1 {
		t.Fatalf("deleted group count = %d, want 1", groupCount)
	}
	memberCount, err := db.Collection(groupIndividualMemberCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1", "deleted_at": deletedAt})
	if err != nil {
		t.Fatalf("count deleted members: %v", err)
	}
	if memberCount != 1 {
		t.Fatalf("deleted member count = %d, want 1", memberCount)
	}
}

func TestMongoGroupRepositoryUpdateGroupingRuleIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	expiration := repositoryTime().Add(48 * time.Hour)
	err := repository.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		ExpirationDate: expiration,
		Rules: []group.Rule{{
			AttributeKey: "level",
			Operator:     group.OperatorGte,
			Multi:        false,
			Value:        int32(5),
		}},
	}, repositoryTime().Add(time.Hour))
	if err != nil {
		t.Fatalf("UpdateGroupingRule error = %v, want nil", err)
	}

	got, err := repository.Get(context.Background(), group.GetQuery{WorkspaceID: "workspace-1", GroupID: "group-1"})
	if err != nil {
		t.Fatalf("Get error = %v, want nil", err)
	}
	if len(got.GroupingRule.Rules) != 1 || got.GroupingRule.Rules[0].AttributeKey != "level" {
		t.Fatalf("rules = %+v, want level rule", got.GroupingRule.Rules)
	}
}

func TestMongoGroupRepositoryUpdateGroupingRuleRejectsEmptyRulesWithoutMembersIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	model := repositoryGroup()
	model.IndividualMembers = nil
	if _, err := repository.Create(context.Background(), model); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	err := repository.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		ExpirationDate: repositoryTime().Add(48 * time.Hour),
		Rules:          nil,
	}, repositoryTime().Add(time.Hour))
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("UpdateGroupingRule error = %v, want ErrInvalidInput", err)
	}
}

func TestMongoGroupRepositoryListIndividualMembersIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	model := repositoryGroup()
	model.IndividualMembers = []group.IndividualMember{
		{ID: "member-1", GroupID: "group-1", NTAccount: "user1", ExpirationDate: repositoryTime().Add(24 * time.Hour), CreatedAt: repositoryTime().Add(1 * time.Minute), UpdatedAt: repositoryTime()},
		{ID: "member-2", GroupID: "group-1", NTAccount: "user2", ExpirationDate: repositoryTime().Add(24 * time.Hour), CreatedAt: repositoryTime().Add(2 * time.Minute), UpdatedAt: repositoryTime()},
	}
	if _, err := repository.Create(context.Background(), model); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	page, err := repository.ListIndividualMembers(context.Background(), group.ListIndividualMembersQuery{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("ListIndividualMembers error = %v, want nil", err)
	}
	if len(page.Members) != 1 || page.Members[0].ID != "member-2" {
		t.Fatalf("members = %+v, want newest member-2", page.Members)
	}
	if !page.HasNextPage || page.NextCursor == nil || page.NextCursor.ID != "member-2" {
		t.Fatalf("page = %+v, want next cursor for member-2", page)
	}
}
```

- [ ] **Step 6: Run integration tests and confirm they fail**

Run:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -run 'TestMongoGroupRepository(Get|Delete|UpdateGroupingRule|ListIndividualMembers)' -v
```

Expected: FAIL because repository methods are undefined. If MongoDB is unavailable, the tests SKIP through `newIntegrationDatabase`.

- [ ] **Step 7: Add repository methods**

Add these methods to `internal/group-service/repositories/mongo_group_repository.go`:

```go
func (r *MongoGroupRepository) Get(ctx context.Context, query group.GetQuery) (*group.Group, error) {
	var doc groupDocument
	err := r.groups.FindOne(ctx, activeGroupFilter(query)).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("find group: %w", err)
	}
	model := doc.toDomain(nil)
	return &model, nil
}

func (r *MongoGroupRepository) Delete(ctx context.Context, input group.DeleteInput, deletedAt time.Time) error {
	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf("start group delete session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		result, err := r.groups.UpdateOne(sessionCtx,
			activeGroupFilter(group.GetQuery{WorkspaceID: input.WorkspaceID, GroupID: input.GroupID}),
			bson.M{"$set": bson.M{"deleted_at": deletedAt, "updated_at": deletedAt}},
		)
		if err != nil {
			return nil, fmt.Errorf("soft delete group: %w", err)
		}
		if result.MatchedCount == 0 {
			return nil, nil
		}
		if _, err := r.members.UpdateMany(sessionCtx,
			bson.M{"group_id": input.GroupID, "deleted_at": nil},
			bson.M{"$set": bson.M{"deleted_at": deletedAt, "updated_at": deletedAt}},
		); err != nil {
			return nil, fmt.Errorf("soft delete group individual members: %w", err)
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}
```

Then add `UpdateGroupingRule` and `ListIndividualMembers`:

```go
func (r *MongoGroupRepository) UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput, updatedAt time.Time) error {
	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf("start grouping rule update session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		count, err := r.groups.CountDocuments(sessionCtx, activeGroupFilter(group.GetQuery{WorkspaceID: input.WorkspaceID, GroupID: input.GroupID}))
		if err != nil {
			return nil, fmt.Errorf("count active group: %w", err)
		}
		if count == 0 {
			return nil, group.ErrNotFound
		}
		if len(input.Rules) == 0 {
			memberCount, err := r.members.CountDocuments(sessionCtx, bson.M{"group_id": input.GroupID, "deleted_at": nil})
			if err != nil {
				return nil, fmt.Errorf("count active individual members: %w", err)
			}
			if memberCount == 0 {
				return nil, fmt.Errorf("%w: at least one membership source is required", group.ErrInvalidInput)
			}
		}
		_, err = r.groups.UpdateOne(sessionCtx,
			activeGroupFilter(group.GetQuery{WorkspaceID: input.WorkspaceID, GroupID: input.GroupID}),
			bson.M{"$set": bson.M{
				"grouping_rule": newGroupingRuleDocument(group.GroupingRule{Rules: input.Rules, ExpirationDate: input.ExpirationDate}),
				"updated_at":    updatedAt,
			}},
		)
		if err != nil {
			return nil, fmt.Errorf("update grouping rule: %w", err)
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *MongoGroupRepository) ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error) {
	groupDoc, err := r.Get(ctx, group.GetQuery{WorkspaceID: query.WorkspaceID, GroupID: query.GroupID})
	if err != nil {
		return group.IndividualMemberPage{}, err
	}
	if groupDoc == nil {
		return group.IndividualMemberPage{Members: []group.IndividualMember{}}, nil
	}
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(query.Limit + 1))
	cursor, err := r.members.Find(ctx, buildIndividualMemberListFilter(query), findOptions)
	if err != nil {
		return group.IndividualMemberPage{}, fmt.Errorf("find group individual members: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()
	var docs []individualMemberDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return group.IndividualMemberPage{}, fmt.Errorf("decode group individual members: %w", err)
	}
	return buildIndividualMemberPage(docs, query.Limit), nil
}
```

Add helper mapping functions:

```go
func newGroupingRuleDocument(rule group.GroupingRule) groupingRuleDocument {
	rules := make([]ruleDocument, 0, len(rule.Rules))
	for _, item := range rule.Rules {
		rules = append(rules, ruleDocument{
			AttributeKey: item.AttributeKey,
			Operator:     item.Operator,
			Multi:        item.Multi,
			Value:        item.Value,
		})
	}
	return groupingRuleDocument{Rules: rules, ExpirationDate: rule.ExpirationDate}
}

func (d individualMemberDocument) toDomain() group.IndividualMember {
	return group.IndividualMember{
		ID:             d.ID,
		GroupID:        d.GroupID,
		NTAccount:      d.NTAccount,
		ExpirationDate: d.ExpirationDate,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
		DeletedAt:      d.DeletedAt,
	}
}

func buildIndividualMemberPage(docs []individualMemberDocument, limit int) group.IndividualMemberPage {
	hasNext := len(docs) > limit
	if hasNext {
		docs = docs[:limit]
	}
	members := make([]group.IndividualMember, 0, len(docs))
	for _, doc := range docs {
		members = append(members, doc.toDomain())
	}
	var nextCursor *group.IndividualMemberCursor
	if hasNext && len(members) > 0 {
		last := members[len(members)-1]
		nextCursor = &group.IndividualMemberCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return group.IndividualMemberPage{Members: members, HasNextPage: hasNext, NextCursor: nextCursor}
}
```

Add `errors` to repository imports.

- [ ] **Step 8: Run repository tests**

Run:

```bash
go test ./internal/group-service/repositories -v
```

Expected without `GROUP_SERVICE_MONGODB_TEST_URI`: PASS with integration tests skipped. Expected with MongoDB URI: PASS including integration tests.

- [ ] **Step 9: Commit repository changes**

```bash
git add internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_group_repository_test.go
git commit -m "feat: add group repository expanded APIs"
```

---

### Task 4: Service Workflows for New Group APIs

**Files:**

- Modify: `internal/group-service/services/group_service.go`
- Modify: `internal/group-service/services/group_service_test.go`

- [ ] **Step 1: Extend the fake repository in service tests**

Update `fakeGroupRepository` in `internal/group-service/services/group_service_test.go`:

```go
type fakeGroupRepository struct {
	input              group.Group
	getQuery           group.GetQuery
	deleteInput        group.DeleteInput
	updateInput        group.UpdateGroupingRuleInput
	listQuery          group.ListIndividualMembersQuery
	model              *group.Group
	page               group.IndividualMemberPage
	err                error
	calls              int
	getCalls           int
	deleteCalls        int
	updateCalls        int
	listCalls          int
	deleteTimestamp    time.Time
	updateTimestamp    time.Time
}
```

Add methods:

```go
func (f *fakeGroupRepository) Get(ctx context.Context, query group.GetQuery) (*group.Group, error) {
	f.getCalls++
	f.getQuery = query
	if f.err != nil {
		return nil, f.err
	}
	return f.model, nil
}

func (f *fakeGroupRepository) Delete(ctx context.Context, input group.DeleteInput, deletedAt time.Time) error {
	f.deleteCalls++
	f.deleteInput = input
	f.deleteTimestamp = deletedAt
	return f.err
}

func (f *fakeGroupRepository) UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput, updatedAt time.Time) error {
	f.updateCalls++
	f.updateInput = input
	f.updateTimestamp = updatedAt
	return f.err
}

func (f *fakeGroupRepository) ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error) {
	f.listCalls++
	f.listQuery = query
	if f.err != nil {
		return group.IndividualMemberPage{}, f.err
	}
	return f.page, nil
}
```

- [ ] **Step 2: Write failing service tests for GET and DELETE**

Append these tests:

```go
func TestGroupServiceGetGroup(t *testing.T) {
	model := group.Group{ID: "group-1", WorkspaceID: "workspace-1", Name: "Design Reviewers"}
	repository := &fakeGroupRepository{model: &model}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	got, err := service.GetGroup(context.Background(), group.GetQuery{WorkspaceID: " workspace-1 ", GroupID: " group-1 "})
	if err != nil {
		t.Fatalf("GetGroup error = %v, want nil", err)
	}
	if got == nil || got.ID != "group-1" {
		t.Fatalf("group = %+v, want group-1", got)
	}
	if repository.getQuery.WorkspaceID != "workspace-1" || repository.getQuery.GroupID != "group-1" {
		t.Fatalf("query = %+v, want trimmed values", repository.getQuery)
	}
}

func TestGroupServiceDeleteGroup(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	if err := service.DeleteGroup(context.Background(), group.DeleteInput{WorkspaceID: " workspace-1 ", GroupID: " group-1 "}); err != nil {
		t.Fatalf("DeleteGroup error = %v, want nil", err)
	}
	if repository.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", repository.deleteCalls)
	}
	if !repository.deleteTimestamp.Equal(fixedNow()) {
		t.Fatalf("deletedAt = %s, want fixed now", repository.deleteTimestamp)
	}
}
```

- [ ] **Step 3: Run service tests and confirm they fail**

Run:

```bash
go test ./internal/group-service/services -run 'TestGroupService(GetGroup|DeleteGroup)' -v
```

Expected: FAIL because service methods and repository interface methods are undefined.

- [ ] **Step 4: Extend service repository interface and add GET/DELETE methods**

Update `GroupRepository` in `internal/group-service/services/group_service.go`:

```go
type GroupRepository interface {
	Create(ctx context.Context, input group.Group) (group.Group, error)
	Get(ctx context.Context, query group.GetQuery) (*group.Group, error)
	Delete(ctx context.Context, input group.DeleteInput, deletedAt time.Time) error
	UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput, updatedAt time.Time) error
	ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error)
}
```

Add service methods:

```go
func (s *GroupService) GetGroup(ctx context.Context, query group.GetQuery) (*group.Group, error) {
	query = query.Normalize()
	if err := query.Validate(); err != nil {
		return nil, err
	}
	model, err := s.repository.Get(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}
	return model, nil
}

func (s *GroupService) DeleteGroup(ctx context.Context, input group.DeleteInput) error {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, input, s.now().UTC()); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Write failing service tests for grouping-rule replacement**

Append these tests:

```go
func TestGroupServiceUpdateGroupingRule(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    " workspace-1 ",
		GroupID:        " group-1 ",
		ExpirationDate: serviceFutureTime(),
		Rules: []group.Rule{{
			AttributeKey: " department ",
			Operator:     group.OperatorEq,
			Multi:        false,
			Value:        "ABCD-123",
		}},
	})
	if err != nil {
		t.Fatalf("UpdateGroupingRule error = %v, want nil", err)
	}
	if repository.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", repository.updateCalls)
	}
	if repository.updateInput.Rules[0].AttributeKey != "department" {
		t.Fatalf("rules = %+v, want trimmed department", repository.updateInput.Rules)
	}
	if !repository.updateTimestamp.Equal(fixedNow()) {
		t.Fatalf("updatedAt = %s, want fixed now", repository.updateTimestamp)
	}
}

func TestGroupServiceUpdateGroupingRuleNotFound(t *testing.T) {
	repository := &fakeGroupRepository{err: group.ErrNotFound}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		ExpirationDate: serviceFutureTime(),
	})
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("UpdateGroupingRule error = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 6: Add grouping-rule service method**

Append this method to `internal/group-service/services/group_service.go`:

```go
func (s *GroupService) UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput) error {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(now, s.validateOptions...); err != nil {
		return err
	}
	if err := s.repository.UpdateGroupingRule(ctx, input, now); err != nil {
		if errors.Is(err, group.ErrNotFound) || errors.Is(err, group.ErrInvalidInput) {
			return err
		}
		return fmt.Errorf("update grouping rule: %w", err)
	}
	return nil
}
```

- [ ] **Step 7: Write failing service tests for member list**

Append this test:

```go
func TestGroupServiceListIndividualMembers(t *testing.T) {
	page := group.IndividualMemberPage{
		Members: []group.IndividualMember{{ID: "member-1", GroupID: "group-1", NTAccount: "user1"}},
	}
	repository := &fakeGroupRepository{page: page}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	got, err := service.ListIndividualMembers(context.Background(), group.ListIndividualMembersQuery{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		Limit:       20,
	})
	if err != nil {
		t.Fatalf("ListIndividualMembers error = %v, want nil", err)
	}
	if len(got.Members) != 1 || got.Members[0].NTAccount != "user1" {
		t.Fatalf("members = %+v, want user1", got.Members)
	}
	if repository.listQuery.WorkspaceID != "workspace-1" || repository.listQuery.GroupID != "group-1" {
		t.Fatalf("query = %+v, want trimmed identity", repository.listQuery)
	}
}
```

- [ ] **Step 8: Add member-list service method**

Append this method to `internal/group-service/services/group_service.go`:

```go
func (s *GroupService) ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error) {
	query = query.Normalize()
	if err := query.Validate(); err != nil {
		return group.IndividualMemberPage{}, err
	}
	page, err := s.repository.ListIndividualMembers(ctx, query)
	if err != nil {
		return group.IndividualMemberPage{}, fmt.Errorf("list group individual members: %w", err)
	}
	return page, nil
}
```

- [ ] **Step 9: Run service tests**

Run:

```bash
go test ./internal/group-service/services -v
```

Expected: PASS.

- [ ] **Step 10: Commit service changes**

```bash
git add internal/group-service/services/group_service.go internal/group-service/services/group_service_test.go
git commit -m "feat: add group service expanded workflows"
```

---

### Task 5: HTTP Routes and Handler Error Mapping

**Files:**

- Modify: `internal/group-service/handlers/group_handler.go`
- Modify: `internal/group-service/handlers/group_handler_test.go`

- [ ] **Step 1: Extend handler fake service**

Update `fakeHTTPGroupService` in `internal/group-service/handlers/group_handler_test.go`:

```go
type fakeHTTPGroupService struct {
	input       group.CreateInput
	getQuery    group.GetQuery
	deleteInput group.DeleteInput
	updateInput group.UpdateGroupingRuleInput
	listQuery   group.ListIndividualMembersQuery
	model       group.Group
	groupPtr    *group.Group
	page        group.IndividualMemberPage
	err         error
	calls       int
	getCalls    int
	deleteCalls int
	updateCalls int
	listCalls   int
}
```

Add methods:

```go
func (f *fakeHTTPGroupService) GetGroup(ctx context.Context, query group.GetQuery) (*group.Group, error) {
	f.getCalls++
	f.getQuery = query
	if f.err != nil {
		return nil, f.err
	}
	return f.groupPtr, nil
}

func (f *fakeHTTPGroupService) DeleteGroup(ctx context.Context, input group.DeleteInput) error {
	f.deleteCalls++
	f.deleteInput = input
	return f.err
}

func (f *fakeHTTPGroupService) UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput) error {
	f.updateCalls++
	f.updateInput = input
	return f.err
}

func (f *fakeHTTPGroupService) ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error) {
	f.listCalls++
	f.listQuery = query
	if f.err != nil {
		return group.IndividualMemberPage{}, f.err
	}
	return f.page, nil
}
```

- [ ] **Step 2: Write failing handler tests for GET and DELETE**

Append these tests:

```go
func TestGroupHandlerGetGroup(t *testing.T) {
	model := groupModel()
	service := &fakeHTTPGroupService{groupPtr: &model}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/groups/group-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.getQuery.WorkspaceID != "workspace-1" || service.getQuery.GroupID != "group-1" {
		t.Fatalf("query = %+v, want path params", service.getQuery)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["group"] == nil {
		t.Fatal("group = nil, want object")
	}
}

func TestGroupHandlerGetGroupMissing(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/groups/missing", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"group":null`) {
		t.Fatalf("body = %s, want group null", rec.Body.String())
	}
}

func TestGroupHandlerDeleteGroup(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/groups/group-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if service.deleteInput.GroupID != "group-1" {
		t.Fatalf("delete input = %+v, want group-1", service.deleteInput)
	}
}
```

- [ ] **Step 3: Run handler tests and confirm they fail**

Run:

```bash
go test ./internal/group-service/handlers -run 'TestGroupHandler(GetGroup|DeleteGroup)' -v
```

Expected: FAIL because constructor signature, service interface methods, and routes are not implemented.

- [ ] **Step 4: Extend handler constructor, interface, path params, and routes**

Update imports in `internal/group-service/handlers/group_handler.go` to include shared pagination:

```go
	"github.com/hao0731/workspace-permission-management/internal/shared/pagination"
```

Update the service interface and handler struct:

```go
type HTTPGroupService interface {
	CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error)
	GetGroup(ctx context.Context, query group.GetQuery) (*group.Group, error)
	DeleteGroup(ctx context.Context, input group.DeleteInput) error
	UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput) error
	ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error)
}

type GroupHandler struct {
	service          HTTPGroupService
	logger           *slog.Logger
	paginationHelper *pagination.PaginationHelper
}
```

Update constructor and routes:

```go
func NewGroupHandler(service HTTPGroupService, logger *slog.Logger, paginationHelper *pagination.PaginationHelper) *GroupHandler {
	return &GroupHandler{service: service, logger: logger, paginationHelper: paginationHelper}
}

func RegisterRoutes(e *echo.Echo, handler *GroupHandler) {
	e.POST("/api/v1/workspaces/:workspace_id/groups", handler.CreateGroup)
	e.GET("/api/v1/workspaces/:workspace_id/groups/:group_id", handler.GetGroup)
	e.DELETE("/api/v1/workspaces/:workspace_id/groups/:group_id", handler.DeleteGroup)
	e.PUT("/api/v1/workspaces/:workspace_id/groups/:group_id/grouping-rules", handler.UpdateGroupingRule)
	e.GET("/api/v1/workspaces/:workspace_id/groups/:group_id/individual-members", handler.ListIndividualMembers)
}
```

Update path params:

```go
type groupPathParams struct {
	workspaceID string
	groupID     string
}

func newGroupPathParams(c *echo.Context) groupPathParams {
	return groupPathParams{workspaceID: c.Param("workspace_id"), groupID: c.Param("group_id")}
}
```

- [ ] **Step 5: Add GET and DELETE handlers**

Append methods:

```go
func (h *GroupHandler) GetGroup(c *echo.Context) error {
	params := newGroupPathParams(c)
	model, err := h.service.GetGroup(c.Request().Context(), group.GetQuery{
		WorkspaceID: params.workspaceID,
		GroupID:     params.groupID,
	})
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to get group", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, transport.NewGroupGetResponse(model))
}

func (h *GroupHandler) DeleteGroup(c *echo.Context) error {
	params := newGroupPathParams(c)
	err := h.service.DeleteGroup(c.Request().Context(), group.DeleteInput{
		WorkspaceID: params.workspaceID,
		GroupID:     params.groupID,
	})
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to delete group", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 6: Write failing handler tests for PUT grouping-rules**

Append these tests:

```go
func TestGroupHandlerUpdateGroupingRule(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/groups/group-1/grouping-rules", strings.NewReader(`{
		"rules": [{"attribute_key": "department", "operator": "eq", "multi": false, "value": "ABCD-123"}],
		"expiration_date": "2026-06-01T00:00:00Z"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if service.updateInput.GroupID != "group-1" || len(service.updateInput.Rules) != 1 {
		t.Fatalf("update input = %+v, want group-1 and one rule", service.updateInput)
	}
}

func TestGroupHandlerUpdateGroupingRuleMissingGroup(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrNotFound}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/groups/missing/grouping-rules", strings.NewReader(`{
		"rules": [],
		"expiration_date": "2026-06-01T00:00:00Z"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 7: Add PUT handler**

Append:

```go
func (h *GroupHandler) UpdateGroupingRule(c *echo.Context) error {
	params := newGroupPathParams(c)
	request, err := transport.DecodeGroupGroupingRulesRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(params.workspaceID, params.groupID)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}
	if err := h.service.UpdateGroupingRule(c.Request().Context(), input); err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, group.ErrNotFound) {
			return c.JSON(http.StatusNotFound, exception.WrapResponse(exception.New("not_found", "Group not found", exception.WithDetails(map[string]any{}))))
		}
		h.logger.Warn("failed to update grouping rule", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 8: Write failing handler tests for member list**

Append this test:

```go
func TestGroupHandlerListIndividualMembers(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	service := &fakeHTTPGroupService{page: group.IndividualMemberPage{
		Members: []group.IndividualMember{{ID: "member-1", GroupID: "group-1", NTAccount: "user1", ExpirationDate: expiration}},
	}}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members?limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.listQuery.Limit != 20 || service.listQuery.GroupID != "group-1" {
		t.Fatalf("list query = %+v, want limit 20 and group-1", service.listQuery)
	}
	if !strings.Contains(rec.Body.String(), `"members"`) {
		t.Fatalf("body = %s, want members", rec.Body.String())
	}
}
```

- [ ] **Step 9: Add member-list handler**

Append:

```go
func (h *GroupHandler) ListIndividualMembers(c *echo.Context) error {
	params := newGroupPathParams(c)
	limit, err := h.paginationHelper.ParseLimit(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	token, err := h.paginationHelper.ParseToken(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	cursor, err := transport.DecodeIndividualMemberNextToken(token)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	page, err := h.service.ListIndividualMembers(c.Request().Context(), group.ListIndividualMembersQuery{
		WorkspaceID: params.workspaceID,
		GroupID:     params.groupID,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to list group individual members", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	response, err := transport.NewIndividualMemberListResponse(page)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, response)
}
```

- [ ] **Step 10: Update existing handler tests for constructor signature**

Replace each `NewGroupHandler(service, newTestLogger())` call in `internal/group-service/handlers/group_handler_test.go` with:

```go
NewGroupHandler(service, newTestLogger(), pagination.New())
```

Add this import:

```go
"github.com/hao0731/workspace-permission-management/internal/shared/pagination"
```

- [ ] **Step 11: Run handler tests**

Run:

```bash
go test ./internal/group-service/handlers -v
```

Expected: PASS.

- [ ] **Step 12: Commit handler changes**

```bash
git add internal/group-service/handlers/group_handler.go internal/group-service/handlers/group_handler_test.go
git commit -m "feat: add group service HTTP routes"
```

---

### Task 6: Main Wiring and REST Client Examples

**Files:**

- Modify: `cmd/group-service/main.go`
- Modify: `examples/api/groups.http`

- [ ] **Step 1: Update main wiring**

Add shared pagination to imports in `cmd/group-service/main.go`:

```go
"github.com/hao0731/workspace-permission-management/internal/shared/pagination"
```

Update group handler construction:

```go
handlers.RegisterRoutes(e, handlers.NewGroupHandler(groupService, logger, pagination.New()))
```

- [ ] **Step 2: Run main tests**

Run:

```bash
go test ./cmd/group-service -v
```

Expected: PASS.

- [ ] **Step 3: Update REST Client examples**

Append these examples to `examples/api/groups.http`:

```http
@groupId = 0d5c4f7e-7675-4c90-b495-93655c2d3c40
@nextToken = replace-with-token-from-previous-response

### Get group
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}

### Get missing group returns group null
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/missing-group

### Delete group idempotently
DELETE {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}

### Replace grouping rules
PUT {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/grouping-rules
Content-Type: application/json

{
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
}

### Replace grouping rules on missing group returns 404
PUT {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/missing-group/grouping-rules
Content-Type: application/json

{
  "rules": [],
  "expiration_date": "2026-06-01T00:00:00Z"
}

### List individual members
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members?limit=20

### List individual members with cursor
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members?limit=20&next_token={{nextToken}}

### Invalid individual member list limit returns 400
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members?limit=51

### Invalid individual member next token returns 400
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members?next_token=not-base64
```

- [ ] **Step 4: Run package-level verification**

Run:

```bash
go test ./internal/domain/group ./internal/group-service/transport ./internal/group-service/services ./internal/group-service/handlers ./cmd/group-service -v
```

Expected: PASS.

- [ ] **Step 5: Commit wiring and examples**

```bash
git add cmd/group-service/main.go examples/api/groups.http
git commit -m "docs: add group service expanded API examples"
```

---

### Task 7: Full Verification and Plan Completion

**Files:**

- Verify: all changed Go and REST Client files.
- Move later after implementation completion: `docs/plans/active/2026-05-09-group-service-expanded-apis.md` to `docs/plans/completed/2026-05-09-group-service-expanded-apis.md`.

- [ ] **Step 1: Run full test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run optional MongoDB repository integration tests**

Run when local MongoDB replica set is available:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -v
```

Expected: PASS. If local MongoDB is unavailable, record that repository transaction behavior was covered by opt-in tests but not executed locally.

- [ ] **Step 3: Inspect API contract examples**

Run:

```bash
sed -n '1,260p' examples/api/groups.http
```

Expected: The file includes create, get, delete, grouping-rule replacement, member list, pagination, and validation examples using `baseUrl`, `workspaceId`, `groupId`, and `nextToken` variables.

- [ ] **Step 4: Check changed files**

Run:

```bash
git status --short
git diff --check
```

Expected: `git diff --check` prints no output. `git status --short` shows only intentional implementation files before final commit.

- [ ] **Step 5: Commit final verification notes or cleanup**

If Step 4 shows intentional uncommitted files, commit them:

```bash
git add internal/domain/group internal/group-service cmd/group-service examples/api/groups.http
git commit -m "feat: complete group service expanded APIs"
```

- [ ] **Step 6: Move completed plan after implementation is done**

Run:

```bash
git mv docs/plans/active/2026-05-09-group-service-expanded-apis.md docs/plans/completed/2026-05-09-group-service-expanded-apis.md
git commit -m "docs: complete group service expanded API plan"
```

Expected: The plan transition is auditable in git history, satisfying the design and plan docs policy.

---

## Self-Review Checklist

- Spec coverage: This plan maps every new endpoint from `group-service.md` to domain, transport, service, repository, handler, wiring, examples, and tests.
- Backend boundary coverage: Handlers parse/render only, transport owns DTOs/tokens, service owns validation orchestration and timestamps, repository owns MongoDB transactions and queries, and domain remains framework-independent.
- API contract coverage: GET group nullable response, DELETE idempotent `204`, PUT grouping-rules `204`/`404`/`400`, and member list `members`/`page_info` are all covered.
- Persistence coverage: Active filters, soft-delete timestamps, grouping-rule replacement, active member count check, member pagination index, and cursor boundary are covered.
- Verification coverage: Each task has focused `go test` commands and the final task has `go test ./...`, optional MongoDB integration tests, REST Client inspection, and diff checks.
