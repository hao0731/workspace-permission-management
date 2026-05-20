# System Group API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Add system-scoped group creation and paginated list APIs to `group-service`, including deterministic permission relationship projection persistence.

**Architecture:** Keep public API DTOs in `internal/group-service/transport`, validation and normalized models in `internal/domain/group`, use-case orchestration and relationship projection in `internal/group-service/services`, and MongoDB documents, indexes, transactions, and pagination queries in `internal/group-service/repositories`. The handler remains thin and the composition root wires the same concrete repository into both existing workspace group workflows and new system group workflows.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, `log/slog`, standard `encoding/json`, `crypto/sha256`, `encoding/hex`, standard `testing`, shared `internal/shared/pagination`, REST Client `.http` examples.

---

## Source Designs And Policies

- Source design: [../../designs/system-group-api-design.md](../../designs/system-group-api-design.md)
- Entry design: [../../designs/group-service.md](../../designs/group-service.md)
- Related design: [../../designs/function-service-system-resource-api-design.md](../../designs/function-service-system-resource-api-design.md)
- Related design: [../../designs/permission-api-client-design.md](../../designs/permission-api-client-design.md)
- Policy: [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- Policy: [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- This is backend implementation plus design-plan documentation work.
- Domain models must not import Echo, MongoDB, transport DTOs, service packages, repository packages, or permission API transport packages.
- Transport DTOs map HTTP JSON shape into domain inputs and may import domain packages.
- Services own use-case orchestration, ID and time seams, relationship generation, checksum generation, and repository interface calls.
- Repositories own MongoDB collection names, documents, indexes, cursor query predicates, transactions, and document/domain mapping.
- Public JSON fields use `snake_case`.
- REST API changes require matching `examples/api/*.http` files.
- This implementation plan lives under `docs/plans/active/` and links to its source design.

## Scope

Implement:

- `POST /api/v1/systems/:system_id/groups`.
- `GET /api/v1/systems/:system_id/groups?limit=<LIMIT>&next_token=<TOKEN>`.
- MongoDB collection `system_groups`.
- MongoDB collection `system_group_relationships`.
- Rule validation for `organization`, `job_type`, `job_level`, and `job_tag`.
- First-phase rejection of `operator: "not_eq"`.
- Relationship generation using existing permission API helpers.
- SHA256 checksum generation over each relationship JSON object.
- Cursor pagination using `created_at DESC, _id DESC`.
- REST Client examples in `examples/api/system-groups.http`.

Do not implement:

- `not_eq` relationship semantics.
- System existence checks against a registry.
- System group get-by-ID, update, delete, restore, or recalculation APIs.
- External permission API registration for generated relationships.
- Frontend changes.

## File Structure And Responsibilities

- Create: `internal/domain/group/system_group.go`
  - System group models, rule constants, cursor/page types, relationship projection DTOs, normalization helpers.
- Create: `internal/domain/group/system_group_validation.go`
  - System group create/list validation, system ID validation, rule shape validation, duplicate `job_type` detection.
- Create: `internal/domain/group/system_group_test.go`
  - Domain normalization, validation, and pagination cursor tests.
- Create: `internal/group-service/transport/system_group_request.go`
  - Create request DTOs, JSON decoding, `multi` presence checks, DTO-to-domain mapping.
- Create: `internal/group-service/transport/system_group_response.go`
  - Create/list response DTOs and domain-to-response mapping.
- Create: `internal/group-service/transport/system_group_pagination.go`
  - Encode/decode next token for system group list cursors.
- Create: `internal/group-service/transport/system_group_request_test.go`
  - Request decode and mapping tests.
- Create: `internal/group-service/transport/system_group_response_test.go`
  - Response shape tests.
- Create: `internal/group-service/transport/system_group_pagination_test.go`
  - Next token encode/decode and invalid token tests.
- Create: `internal/group-service/services/system_group_relationship_builder.go`
  - Relationship generation, dedupe/sort behavior, checksum computation.
- Create: `internal/group-service/services/system_group_service.go`
  - `SystemGroupRepository` interface, service methods, ID/time seams, validation orchestration.
- Create: `internal/group-service/services/system_group_service_test.go`
  - Service workflow, relationship generation, checksum, and list tests with fake repository.
- Create: `internal/group-service/repositories/mongo_system_group_repository.go`
  - Documents, indexes, create transaction, list query, cursor predicate, page builder.
- Create: `internal/group-service/repositories/mongo_system_group_repository_test.go`
  - Mapping, index, filter, page builder, and transaction integration tests.
- Modify: `internal/group-service/handlers/group_handler.go`
  - Extend service interface, register system routes, add create/list handlers.
- Modify: `internal/group-service/handlers/group_handler_test.go`
  - Add fake service methods and system route handler tests.
- Modify: `cmd/group-service/main.go`
  - No new repository type is needed if `MongoGroupRepository` implements both interfaces; ensure new indexes are created through existing `EnsureIndexes`.
- Create: `examples/api/system-groups.http`
  - Success, empty-rule, pagination, and validation examples.

---

### Task 1: Domain Models And Validation

**Files:**

- Create: `internal/domain/group/system_group.go`
- Create: `internal/domain/group/system_group_validation.go`
- Create: `internal/domain/group/system_group_test.go`

- [x] **Step 1: Write failing domain tests**

Create `internal/domain/group/system_group_test.go`:

```go
package group

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func systemGroupNow() time.Time {
	return time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
}

func validSystemGroupCreateInput() SystemGroupCreateInput {
	return SystemGroupCreateInput{
		SystemID: " system-a ",
		Name:     " System Admins ",
		GroupingRules: []SystemGroupRule{
			{AttributeKey: GroupAttributeOrganization, Operator: OperatorEq, Multi: true, Value: []string{" ORG-200 ", "ORG-100", "ORG-100"}},
			{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: false, Value: " DL "},
			{AttributeKey: GroupAttributeJobLevel, Operator: OperatorEq, Multi: false, Value: " M2 "},
			{AttributeKey: GroupAttributeJobTag, Operator: OperatorEq, Multi: true, Value: []string{"a4_reviewer", "_internal_secretary_"}},
		},
	}
}

func requireSystemGroupInvalidInput(t *testing.T, err error, contains string) {
	t.Helper()
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if contains != "" && !strings.Contains(err.Error(), contains) {
		t.Fatalf("error = %q, want containing %q", err.Error(), contains)
	}
}

func TestSystemGroupCreateInputNormalize(t *testing.T) {
	input := validSystemGroupCreateInput().Normalize()

	if input.SystemID != "system-a" {
		t.Fatalf("SystemID = %q, want system-a", input.SystemID)
	}
	if input.Name != "System Admins" {
		t.Fatalf("Name = %q, want System Admins", input.Name)
	}
	if input.GroupingRules[0].Value.([]string)[0] != "ORG-200" {
		t.Fatalf("first org value = %q, want ORG-200", input.GroupingRules[0].Value.([]string)[0])
	}
	if input.GroupingRules[1].Value.(string) != "DL" {
		t.Fatalf("job type = %q, want DL", input.GroupingRules[1].Value.(string))
	}
}

func TestSystemGroupCreateInputValidateAcceptsValidAndEmptyRules(t *testing.T) {
	if err := validSystemGroupCreateInput().Normalize().Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	emptyRules := SystemGroupCreateInput{SystemID: "system-a", Name: "Everyone", GroupingRules: []SystemGroupRule{}}
	if err := emptyRules.Normalize().Validate(); err != nil {
		t.Fatalf("Validate empty rules error = %v, want nil", err)
	}
}

func TestSystemGroupCreateInputValidateRejectsInvalidIdentityAndName(t *testing.T) {
	tests := []struct {
		name   string
		input  SystemGroupCreateInput
		reason string
	}{
		{name: "empty system id", input: SystemGroupCreateInput{SystemID: " ", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "system id is required"},
		{name: "system id has whitespace", input: SystemGroupCreateInput{SystemID: "system a", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "system id must be a single subject token"},
		{name: "system id has dot", input: SystemGroupCreateInput{SystemID: "system.a", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "system id must be a single subject token"},
		{name: "empty name", input: SystemGroupCreateInput{SystemID: "system-a", Name: " ", GroupingRules: []SystemGroupRule{}}, reason: "name is required"},
		{name: "nil grouping rules", input: SystemGroupCreateInput{SystemID: "system-a", Name: "Group", GroupingRules: nil}, reason: "grouping_rules is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireSystemGroupInvalidInput(t, tt.input.Normalize().Validate(), tt.reason)
		})
	}
}

func TestSystemGroupCreateInputValidateRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name   string
		rules  []SystemGroupRule
		reason string
	}{
		{
			name:   "not eq rejected",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeOrganization, Operator: OperatorNotEq, Multi: true, Value: []string{"ORG-100"}}},
			reason: "system group rule operator must be eq",
		},
		{
			name:   "unknown attribute",
			rules:  []SystemGroupRule{{AttributeKey: "department", Operator: OperatorEq, Multi: false, Value: "D100"}},
			reason: "system group rule attribute_key is invalid",
		},
		{
			name:   "organization must be multi",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeOrganization, Operator: OperatorEq, Multi: false, Value: "ORG-100"}},
			reason: "organization rule must be multi",
		},
		{
			name:   "job type must be single",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: true, Value: []string{"DL"}}},
			reason: "job_type rule must not be multi",
		},
		{
			name:   "invalid job type value",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: false, Value: "CONTRACTOR"}},
			reason: "job_type value must be DL, IDL, or ALL",
		},
		{
			name: "duplicate job type",
			rules: []SystemGroupRule{
				{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: false, Value: "DL"},
				{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: false, Value: "IDL"},
			},
			reason: "only one job_type rule is allowed",
		},
		{
			name:   "empty string value",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeJobLevel, Operator: OperatorEq, Multi: false, Value: " "}},
			reason: "system group rule value must not be empty",
		},
		{
			name:   "empty array item",
			rules:  []SystemGroupRule{{AttributeKey: GroupAttributeJobTag, Operator: OperatorEq, Multi: true, Value: []string{"a4", " "}}},
			reason: "system group rule value must not be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := SystemGroupCreateInput{SystemID: "system-a", Name: "Group", GroupingRules: tt.rules}
			requireSystemGroupInvalidInput(t, input.Normalize().Validate(), tt.reason)
		})
	}
}

func TestSystemGroupListQueryValidate(t *testing.T) {
	query := SystemGroupListQuery{
		SystemID: " system-a ",
		Limit:    20,
		Cursor:   &SystemGroupCursor{CreatedAt: systemGroupNow(), ID: "group-1"},
	}.Normalize()

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if query.SystemID != "system-a" || query.Cursor.ID != "group-1" {
		t.Fatalf("query = %+v, want normalized", query)
	}
}

func TestSystemGroupListQueryValidateRejectsInvalidCursor(t *testing.T) {
	query := SystemGroupListQuery{SystemID: "system-a", Limit: 20, Cursor: &SystemGroupCursor{ID: "group-1"}}
	requireSystemGroupInvalidInput(t, query.Normalize().Validate(), "cursor created_at is required")
}
```

- [x] **Step 2: Run domain tests and confirm failure**

Run:

```bash
go test ./internal/domain/group -run 'SystemGroup' -v
```

Expected: FAIL with undefined symbols such as `SystemGroupCreateInput`, `SystemGroupRule`, and `GroupAttributeOrganization`.

- [x] **Step 3: Add system group domain models**

Create `internal/domain/group/system_group.go`:

```go
package group

import (
	"strings"
	"time"
)

type GroupAttributeKey string

const (
	GroupAttributeOrganization GroupAttributeKey = "organization"
	GroupAttributeJobLevel     GroupAttributeKey = "job_level"
	GroupAttributeJobType      GroupAttributeKey = "job_type"
	GroupAttributeJobTag       GroupAttributeKey = "job_tag"
	SystemGroupSecretarySentinel     string            = "_internal_secretary_"
	SystemGroupJobTypeDL             string            = "DL"
	SystemGroupJobTypeIDL            string            = "IDL"
	SystemGroupJobTypeALL            string            = "ALL"
)

type SystemGroup struct {
	ID            string
	SystemID      string
	Name          string
	GroupingRules []SystemGroupRule
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SystemGroupRule struct {
	AttributeKey GroupAttributeKey
	Operator     Operator
	Multi        bool
	Value        any
}

type SystemGroupCreateInput struct {
	SystemID      string
	Name          string
	GroupingRules []SystemGroupRule
}

type SystemGroupCursor struct {
	CreatedAt time.Time
	ID        string
}

type SystemGroupListQuery struct {
	SystemID string
	Limit    int
	Cursor   *SystemGroupCursor
}

type SystemGroupPage struct {
	Groups      []SystemGroup
	HasNextPage bool
	NextCursor  *SystemGroupCursor
}

type RelationshipInfo struct {
	Relationship any
	Checksum     string
}

type SystemGroupRelationshipProjection struct {
	SystemID      string
	GroupID       string
	Relationships []RelationshipInfo
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (input SystemGroupCreateInput) Normalize() SystemGroupCreateInput {
	input.SystemID = strings.TrimSpace(input.SystemID)
	input.Name = strings.TrimSpace(input.Name)
	for i := range input.GroupingRules {
		input.GroupingRules[i] = input.GroupingRules[i].Normalize()
	}
	return input
}

func (rule SystemGroupRule) Normalize() SystemGroupRule {
	rule.AttributeKey = GroupAttributeKey(strings.TrimSpace(string(rule.AttributeKey)))
	rule.Operator = Operator(strings.TrimSpace(string(rule.Operator)))
	if rule.Multi {
		if values, ok := stringSliceValue(rule.Value); ok {
			out := make([]string, 0, len(values))
			for _, value := range values {
				out = append(out, strings.TrimSpace(value))
			}
			rule.Value = out
		}
		return rule
	}
	if value, ok := rule.Value.(string); ok {
		rule.Value = strings.TrimSpace(value)
	}
	return rule
}

func (query SystemGroupListQuery) Normalize() SystemGroupListQuery {
	query.SystemID = strings.TrimSpace(query.SystemID)
	if query.Cursor != nil {
		query.Cursor.ID = strings.TrimSpace(query.Cursor.ID)
	}
	return query
}

func stringSliceValue(value any) ([]string, bool) {
	values, ok := value.([]string)
	return values, ok
}
```

- [x] **Step 4: Add system group validation**

Create `internal/domain/group/system_group_validation.go`:

```go
package group

import (
	"fmt"
	"strings"
)

func (input SystemGroupCreateInput) Validate() error {
	if err := validateSystemID(input.SystemID); err != nil {
		return err
	}
	if strings.TrimSpace(input.Name) == "" {
		return invalidInput("name is required")
	}
	if input.GroupingRules == nil {
		return invalidInput("grouping_rules is required")
	}
	jobTypeCount := 0
	for _, rule := range input.GroupingRules {
		if rule.AttributeKey == GroupAttributeJobType {
			jobTypeCount++
			if jobTypeCount > 1 {
				return invalidInput("only one job_type rule is allowed")
			}
		}
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (query SystemGroupListQuery) Validate() error {
	if err := validateSystemID(query.SystemID); err != nil {
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

func (rule SystemGroupRule) Validate() error {
	if rule.Operator != OperatorEq {
		return invalidInput("system group rule operator must be eq")
	}
	switch rule.AttributeKey {
	case GroupAttributeOrganization:
		return validateSystemGroupMultiRule(rule, "organization")
	case GroupAttributeJobTag:
		return validateSystemGroupMultiRule(rule, "job_tag")
	case GroupAttributeJobLevel:
		return validateSystemGroupSingleRule(rule, "job_level")
	case GroupAttributeJobType:
		if err := validateSystemGroupSingleRule(rule, "job_type"); err != nil {
			return err
		}
		value := rule.Value.(string)
		if !IsValidSystemGroupJobType(value) {
			return invalidInput("job_type value must be DL, IDL, or ALL")
		}
		return nil
	default:
		return invalidInput(fmt.Sprintf("system group rule attribute_key is invalid: %s", rule.AttributeKey))
	}
}

func IsValidSystemGroupJobType(value string) bool {
	switch value {
	case SystemGroupJobTypeDL, SystemGroupJobTypeIDL, SystemGroupJobTypeALL:
		return true
	default:
		return false
	}
}

func validateSystemGroupMultiRule(rule SystemGroupRule, name string) error {
	if !rule.Multi {
		return invalidInput(fmt.Sprintf("%s rule must be multi", name))
	}
	values, ok := stringSliceValue(rule.Value)
	if !ok {
		return invalidInput(fmt.Sprintf("%s rule value must be a string array", name))
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return invalidInput("system group rule value must not be empty")
		}
	}
	return nil
}

func validateSystemGroupSingleRule(rule SystemGroupRule, name string) error {
	if rule.Multi {
		return invalidInput(fmt.Sprintf("%s rule must not be multi", name))
	}
	value, ok := rule.Value.(string)
	if !ok {
		return invalidInput(fmt.Sprintf("%s rule value must be a string", name))
	}
	if strings.TrimSpace(value) == "" {
		return invalidInput("system group rule value must not be empty")
	}
	return nil
}

func validateSystemID(systemID string) error {
	trimmed := strings.TrimSpace(systemID)
	if trimmed == "" {
		return invalidInput("system id is required")
	}
	if strings.ContainsAny(trimmed, " \t\n\r.") {
		return invalidInput("system id must be a single subject token")
	}
	return nil
}
```

- [x] **Step 5: Run domain tests and verify pass**

Run:

```bash
go test ./internal/domain/group -run 'SystemGroup' -v
```

Expected: PASS.

- [x] **Step 6: Commit domain foundation**

```bash
git add internal/domain/group/system_group.go internal/domain/group/system_group_validation.go internal/domain/group/system_group_test.go
git commit -m "feat: add system group domain model"
```

---

### Task 2: Transport DTOs, Responses, And Pagination Tokens

**Files:**

- Create: `internal/group-service/transport/system_group_request.go`
- Create: `internal/group-service/transport/system_group_response.go`
- Create: `internal/group-service/transport/system_group_pagination.go`
- Create: `internal/group-service/transport/system_group_request_test.go`
- Create: `internal/group-service/transport/system_group_response_test.go`
- Create: `internal/group-service/transport/system_group_pagination_test.go`

- [x] **Step 1: Write failing transport request tests**

Create `internal/group-service/transport/system_group_request_test.go`:

```go
package transport

import (
	"errors"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func TestDecodeSystemGroupCreateRequestToDomain(t *testing.T) {
	request, err := DecodeSystemGroupCreateRequest(strings.NewReader(`{
		"name": " System Admins ",
		"grouping_rules": [
			{"attribute_key": "organization", "operator": "eq", "multi": true, "value": [" ORG-100 ", "ORG-200"]},
			{"attribute_key": "job_type", "operator": "eq", "multi": false, "value": " DL "},
			{"attribute_key": "job_level", "operator": "eq", "multi": false, "value": " M2 "},
			{"attribute_key": "job_tag", "operator": "eq", "multi": true, "value": ["a4_reviewer", "_internal_secretary_"]}
		]
	}`))
	if err != nil {
		t.Fatalf("DecodeSystemGroupCreateRequest error = %v, want nil", err)
	}
	input, err := request.ToDomain("system-a")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.SystemID != "system-a" || input.Name != " System Admins " {
		t.Fatalf("input identity/name = %+v, want original request values", input)
	}
	if len(input.GroupingRules) != 4 {
		t.Fatalf("rules len = %d, want 4", len(input.GroupingRules))
	}
	if input.GroupingRules[0].AttributeKey != group.GroupAttributeOrganization {
		t.Fatalf("first attribute = %q, want organization", input.GroupingRules[0].AttributeKey)
	}
	if values, ok := input.GroupingRules[0].Value.([]string); !ok || values[0] != " ORG-100 " {
		t.Fatalf("organization values = %#v, want string slice preserving transport value", input.GroupingRules[0].Value)
	}
}

func TestDecodeSystemGroupCreateRequestRejectsMalformedJSON(t *testing.T) {
	_, err := DecodeSystemGroupCreateRequest(strings.NewReader(`{"name":`))
	if err == nil {
		t.Fatal("DecodeSystemGroupCreateRequest error = nil, want error")
	}
}

func TestSystemGroupCreateRequestToDomainRejectsMissingGroupingRules(t *testing.T) {
	request := SystemGroupCreateRequest{Name: "System Admins"}
	_, err := request.ToDomain("system-a")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}

func TestSystemGroupCreateRequestToDomainRejectsMissingMulti(t *testing.T) {
	request := SystemGroupCreateRequest{
		Name: "System Admins",
		GroupingRules: []SystemGroupRuleRequest{{
			AttributeKey: "organization",
			Operator:     "eq",
			Value:        []string{"ORG-100"},
		}},
	}
	_, err := request.ToDomain("system-a")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}
```

- [x] **Step 2: Write failing response and pagination tests**

Create `internal/group-service/transport/system_group_response_test.go`:

```go
package transport

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func transportSystemGroupModel() group.SystemGroup {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	return group.SystemGroup{
		ID:       "group-1",
		SystemID: "system-a",
		Name:     "System Admins",
		GroupingRules: []group.SystemGroupRule{{
			AttributeKey: group.GroupAttributeOrganization,
			Operator:     group.OperatorEq,
			Multi:        true,
			Value:        []string{"ORG-100", "ORG-200"},
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestNewSystemGroupCreateResponse(t *testing.T) {
	response := NewSystemGroupCreateResponse(transportSystemGroupModel())
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}
	groupBody, ok := body["group"].(map[string]any)
	if !ok {
		t.Fatalf("group = %#v, want object", body["group"])
	}
	if groupBody["name"] != "System Admins" {
		t.Fatalf("name = %v, want System Admins", groupBody["name"])
	}
	if _, ok := groupBody["system_id"]; ok {
		t.Fatal("system_id present, want omitted")
	}
	if groupBody["created_at"] == "" || groupBody["updated_at"] == "" {
		t.Fatalf("timestamps missing in %#v", groupBody)
	}
}

func TestNewSystemGroupListResponse(t *testing.T) {
	page := group.SystemGroupPage{
		Groups:      []group.SystemGroup{transportSystemGroupModel()},
		HasNextPage: true,
		NextCursor:  &group.SystemGroupCursor{CreatedAt: time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC), ID: "group-1"},
	}
	response, err := NewSystemGroupListResponse(page)
	if err != nil {
		t.Fatalf("NewSystemGroupListResponse error = %v, want nil", err)
	}
	if len(response.Groups) != 1 || response.Groups[0].ID != "group-1" {
		t.Fatalf("Groups = %+v, want group-1", response.Groups)
	}
	if !response.PageInfo.HasNextPage || response.PageInfo.NextToken == "" {
		t.Fatalf("PageInfo = %+v, want next token", response.PageInfo)
	}
}
```

Create `internal/group-service/transport/system_group_pagination_test.go`:

```go
package transport

import (
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

func TestEncodeDecodeSystemGroupNextToken(t *testing.T) {
	cursor := &group.SystemGroupCursor{CreatedAt: time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC), ID: "group-1"}
	token, err := EncodeSystemGroupNextToken(cursor)
	if err != nil {
		t.Fatalf("EncodeSystemGroupNextToken error = %v, want nil", err)
	}
	out, err := DecodeSystemGroupNextToken(token)
	if err != nil {
		t.Fatalf("DecodeSystemGroupNextToken error = %v, want nil", err)
	}
	if !out.CreatedAt.Equal(cursor.CreatedAt) || out.ID != cursor.ID {
		t.Fatalf("cursor = %+v, want %+v", out, cursor)
	}
}

func TestDecodeSystemGroupNextTokenEmpty(t *testing.T) {
	cursor, err := DecodeSystemGroupNextToken("")
	if err != nil {
		t.Fatalf("DecodeSystemGroupNextToken error = %v, want nil", err)
	}
	if cursor != nil {
		t.Fatalf("cursor = %+v, want nil", cursor)
	}
}

func TestDecodeSystemGroupNextTokenInvalid(t *testing.T) {
	_, err := DecodeSystemGroupNextToken("not-base64")
	if err == nil || errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("DecodeSystemGroupNextToken error = %v, want raw pagination decode error for handler mapping", err)
	}
}
```

- [x] **Step 3: Run transport tests and confirm failure**

Run:

```bash
go test ./internal/group-service/transport -run 'SystemGroup' -v
```

Expected: FAIL with undefined symbols such as `DecodeSystemGroupCreateRequest`, `SystemGroupCreateRequest`, `NewSystemGroupCreateResponse`, and `EncodeSystemGroupNextToken`.

- [x] **Step 4: Add request DTOs and mapping**

Create `internal/group-service/transport/system_group_request.go`:

```go
package transport

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type SystemGroupCreateRequest struct {
	Name          string                   `json:"name"`
	GroupingRules []SystemGroupRuleRequest `json:"grouping_rules"`
}

type SystemGroupRuleRequest struct {
	AttributeKey string `json:"attribute_key"`
	Operator     string `json:"operator"`
	Multi        *bool  `json:"multi"`
	Value        any    `json:"value"`
}

func DecodeSystemGroupCreateRequest(body io.Reader) (SystemGroupCreateRequest, error) {
	var request SystemGroupCreateRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return SystemGroupCreateRequest{}, fmt.Errorf("decode system group create request: %w", err)
	}
	return request, nil
}

func (request SystemGroupCreateRequest) ToDomain(systemID string) (group.SystemGroupCreateInput, error) {
	if request.GroupingRules == nil {
		return group.SystemGroupCreateInput{}, invalidGroupRequest("grouping_rules is required")
	}
	rules := make([]group.SystemGroupRule, 0, len(request.GroupingRules))
	for _, rule := range request.GroupingRules {
		if rule.Multi == nil {
			return group.SystemGroupCreateInput{}, invalidGroupRequest("rule multi is required")
		}
		value, err := systemGroupRuleValue(rule.Value, *rule.Multi)
		if err != nil {
			return group.SystemGroupCreateInput{}, err
		}
		rules = append(rules, group.SystemGroupRule{
			AttributeKey: group.GroupAttributeKey(rule.AttributeKey),
			Operator:     group.Operator(rule.Operator),
			Multi:        *rule.Multi,
			Value:        value,
		})
	}
	return group.SystemGroupCreateInput{
		SystemID:      systemID,
		Name:          request.Name,
		GroupingRules: rules,
	}, nil
}

func systemGroupRuleValue(value any, multi bool) (any, error) {
	if multi {
		rawValues, ok := value.([]any)
		if !ok {
			return nil, invalidGroupRequest("multi rule value must be an array")
		}
		values := make([]string, 0, len(rawValues))
		for _, raw := range rawValues {
			valueString, ok := raw.(string)
			if !ok {
				return nil, invalidGroupRequest("multi rule value items must be strings")
			}
			values = append(values, valueString)
		}
		return values, nil
	}
	valueString, ok := value.(string)
	if !ok {
		return nil, invalidGroupRequest("single rule value must be a string")
	}
	return valueString, nil
}
```

- [x] **Step 5: Add response DTOs and mapping**

Create `internal/group-service/transport/system_group_response.go`:

```go
package transport

import (
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type SystemGroupCreateResponse struct {
	Group SystemGroupResponse `json:"group"`
}

type SystemGroupListResponse struct {
	Groups   []SystemGroupResponse `json:"groups"`
	PageInfo PageInfoResponse      `json:"page_info"`
}

type SystemGroupResponse struct {
	ID            string                    `json:"id"`
	Name          string                    `json:"name"`
	GroupingRules []SystemGroupRuleResponse `json:"grouping_rules"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
}

type SystemGroupRuleResponse struct {
	AttributeKey group.GroupAttributeKey `json:"attribute_key"`
	Operator     group.Operator                `json:"operator"`
	Multi        bool                          `json:"multi"`
	Value        any                           `json:"value"`
}

func NewSystemGroupCreateResponse(model group.SystemGroup) SystemGroupCreateResponse {
	return SystemGroupCreateResponse{Group: newSystemGroupResponse(model)}
}

func NewSystemGroupListResponse(page group.SystemGroupPage) (SystemGroupListResponse, error) {
	groups := make([]SystemGroupResponse, 0, len(page.Groups))
	for _, model := range page.Groups {
		groups = append(groups, newSystemGroupResponse(model))
	}
	nextToken, err := EncodeSystemGroupNextToken(page.NextCursor)
	if err != nil {
		return SystemGroupListResponse{}, err
	}
	return SystemGroupListResponse{
		Groups: groups,
		PageInfo: PageInfoResponse{
			HasNextPage: page.HasNextPage,
			NextToken:   nextToken,
		},
	}, nil
}

func newSystemGroupResponse(model group.SystemGroup) SystemGroupResponse {
	rules := make([]SystemGroupRuleResponse, 0, len(model.GroupingRules))
	for _, rule := range model.GroupingRules {
		rules = append(rules, SystemGroupRuleResponse{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	return SystemGroupResponse{
		ID:            model.ID,
		Name:          model.Name,
		GroupingRules: rules,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}
```

- [x] **Step 6: Add pagination token helpers**

Create `internal/group-service/transport/system_group_pagination.go`:

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

type systemGroupNextTokenPayload struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func EncodeSystemGroupNextToken(cursor *group.SystemGroupCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	payload := systemGroupNextTokenPayload{
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        cursor.ID,
	}
	return pagination.EncodeNextToken(payload)
}

func DecodeSystemGroupNextToken(token string) (*group.SystemGroupCursor, error) {
	payload, err := pagination.DecodeNextToken[systemGroupNextTokenPayload](token)
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
	return &group.SystemGroupCursor{CreatedAt: createdAt, ID: strings.TrimSpace(payload.ID)}, nil
}
```

- [x] **Step 7: Run transport tests and verify pass**

Run:

```bash
go test ./internal/group-service/transport -run 'SystemGroup' -v
```

Expected: PASS.

- [x] **Step 8: Commit transport layer**

```bash
git add internal/group-service/transport/system_group_request.go internal/group-service/transport/system_group_response.go internal/group-service/transport/system_group_pagination.go internal/group-service/transport/system_group_request_test.go internal/group-service/transport/system_group_response_test.go internal/group-service/transport/system_group_pagination_test.go
git commit -m "feat: add system group transport contract"
```

---

### Task 3: Relationship Builder And Service Workflow

**Files:**

- Create: `internal/group-service/services/system_group_relationship_builder.go`
- Create: `internal/group-service/services/system_group_service.go`
- Create: `internal/group-service/services/system_group_service_test.go`

- [x] **Step 1: Write failing service tests**

Create `internal/group-service/services/system_group_service_test.go`:

```go
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type fakeSystemGroupRepository struct {
	createGroup      group.SystemGroup
	createProjection group.SystemGroupRelationshipProjection
	listQuery       group.SystemGroupListQuery
	page            group.SystemGroupPage
	err             error
	createCalls     int
	listCalls       int
}

func (f *fakeSystemGroupRepository) CreateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error) {
	f.createCalls++
	f.createGroup = model
	f.createProjection = projection
	if f.err != nil {
		return group.SystemGroup{}, f.err
	}
	return model, nil
}

func (f *fakeSystemGroupRepository) ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error) {
	f.listCalls++
	f.listQuery = query
	if f.err != nil {
		return group.SystemGroupPage{}, f.err
	}
	return f.page, nil
}

func fixedSystemGroupNow() time.Time {
	return time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
}

func validServiceSystemGroupInput() group.SystemGroupCreateInput {
	return group.SystemGroupCreateInput{
		SystemID: "system-a",
		Name:     "System Admins",
		GroupingRules: []group.SystemGroupRule{
			{AttributeKey: group.GroupAttributeOrganization, Operator: group.OperatorEq, Multi: true, Value: []string{"ORG-200", "ORG-100", "ORG-100"}},
			{AttributeKey: group.GroupAttributeJobLevel, Operator: group.OperatorEq, Multi: false, Value: "M2"},
			{AttributeKey: group.GroupAttributeJobTag, Operator: group.OperatorEq, Multi: true, Value: []string{"a4_reviewer", group.SystemGroupSecretarySentinel}},
		},
	}
}

func TestSystemGroupServiceCreateSystemGroup(t *testing.T) {
	repository := &fakeSystemGroupRepository{}
	service := NewSystemGroupService(repository,
		WithSystemGroupClock(fixedSystemGroupNow),
		WithSystemGroupIDGenerator(func() string { return "group-1" }),
	)

	model, err := service.CreateSystemGroup(context.Background(), validServiceSystemGroupInput())
	if err != nil {
		t.Fatalf("CreateSystemGroup error = %v, want nil", err)
	}
	if repository.createCalls != 1 {
		t.Fatalf("CreateSystemGroup repository calls = %d, want 1", repository.createCalls)
	}
	if model.ID != "group-1" || model.SystemID != "system-a" || model.Name != "System Admins" {
		t.Fatalf("model = %+v, want normalized group", model)
	}
	if !model.CreatedAt.Equal(fixedSystemGroupNow()) || !model.UpdatedAt.Equal(fixedSystemGroupNow()) {
		t.Fatalf("timestamps = %s/%s, want fixed now", model.CreatedAt, model.UpdatedAt)
	}
	if repository.createProjection.SystemID != "system-a" || repository.createProjection.GroupID != "group-1" {
		t.Fatalf("projection identity = %+v, want system-a/group-1", repository.createProjection)
	}
	if len(repository.createProjection.Relationships) != 4 {
		t.Fatalf("relationships len = %d, want 4", len(repository.createProjection.Relationships))
	}
}

func TestBuildSystemGroupRelationshipProjectionFallbacks(t *testing.T) {
	projection, err := buildSystemGroupRelationshipProjection("system-a", "group-1", []group.SystemGroupRule{}, fixedSystemGroupNow())
	if err != nil {
		t.Fatalf("projection error = %v, want nil", err)
	}
	if len(projection.Relationships) != 2 {
		t.Fatalf("relationships len = %d, want HR and A4 all employee fallbacks", len(projection.Relationships))
	}
}

func TestBuildSystemGroupRelationshipProjectionSecretaryOnlyBuildsStaticAndA4Fallback(t *testing.T) {
	projection, err := buildSystemGroupRelationshipProjection("system-a", "group-1", []group.SystemGroupRule{{
		AttributeKey: group.GroupAttributeJobTag,
		Operator:     group.OperatorEq,
		Multi:        true,
		Value:        []string{group.SystemGroupSecretarySentinel},
	}}, fixedSystemGroupNow())
	if err != nil {
		t.Fatalf("projection error = %v, want nil", err)
	}
	if len(projection.Relationships) != 3 {
		t.Fatalf("relationships len = %d, want HR fallback, static secretary, A4 fallback", len(projection.Relationships))
	}
}

func TestBuildSystemGroupRelationshipProjectionNonSecretaryTagsDoNotBuildStatic(t *testing.T) {
	projection, err := buildSystemGroupRelationshipProjection("system-a", "group-1", []group.SystemGroupRule{{
		AttributeKey: group.GroupAttributeJobTag,
		Operator:     group.OperatorEq,
		Multi:        true,
		Value:        []string{"a4_writer", "a4_reader"},
	}}, fixedSystemGroupNow())
	if err != nil {
		t.Fatalf("projection error = %v, want nil", err)
	}
	if len(projection.Relationships) != 3 {
		t.Fatalf("relationships len = %d, want HR fallback plus two A4 roles", len(projection.Relationships))
	}
}

func TestRelationshipChecksumUsesRelationshipJSON(t *testing.T) {
	projection, err := buildSystemGroupRelationshipProjection("system-a", "group-1", []group.SystemGroupRule{}, fixedSystemGroupNow())
	if err != nil {
		t.Fatalf("projection error = %v, want nil", err)
	}
	raw, err := json.Marshal(projection.Relationships[0].Relationship)
	if err != nil {
		t.Fatalf("marshal relationship: %v", err)
	}
	sum := sha256.Sum256(raw)
	want := hex.EncodeToString(sum[:])
	if projection.Relationships[0].Checksum != want {
		t.Fatalf("checksum = %q, want %q", projection.Relationships[0].Checksum, want)
	}
}

func TestSystemGroupServiceValidationFailureDoesNotCallRepository(t *testing.T) {
	repository := &fakeSystemGroupRepository{}
	service := NewSystemGroupService(repository)

	_, err := service.CreateSystemGroup(context.Background(), group.SystemGroupCreateInput{SystemID: "system-a", Name: " "})
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("CreateSystemGroup error = %v, want ErrInvalidInput", err)
	}
	if repository.createCalls != 0 {
		t.Fatalf("repository calls = %d, want 0", repository.createCalls)
	}
}

func TestSystemGroupServiceListSystemGroups(t *testing.T) {
	repository := &fakeSystemGroupRepository{page: group.SystemGroupPage{Groups: []group.SystemGroup{{ID: "group-1"}}}}
	service := NewSystemGroupService(repository)

	page, err := service.ListSystemGroups(context.Background(), group.SystemGroupListQuery{SystemID: " system-a ", Limit: 20})
	if err != nil {
		t.Fatalf("ListSystemGroups error = %v, want nil", err)
	}
	if len(page.Groups) != 1 || repository.listQuery.SystemID != "system-a" {
		t.Fatalf("page/query = %+v/%+v, want normalized list", page, repository.listQuery)
	}
}
```

- [x] **Step 2: Run service tests and confirm failure**

Run:

```bash
go test ./internal/group-service/services -run 'SystemGroup' -v
```

Expected: FAIL with undefined symbols such as `NewSystemGroupService`, `buildSystemGroupRelationshipProjection`, and `WithSystemGroupClock`.

- [x] **Step 3: Add service workflow**

Create `internal/group-service/services/system_group_service.go`:

```go
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type SystemGroupRepository interface {
	CreateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error)
	ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error)
}

type SystemGroupOption func(*SystemGroupService)

type SystemGroupService struct {
	repository  SystemGroupRepository
	idGenerator func() string
	now         func() time.Time
}

func NewSystemGroupService(repository SystemGroupRepository, opts ...SystemGroupOption) *SystemGroupService {
	service := &SystemGroupService{
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

func WithSystemGroupIDGenerator(generator func() string) SystemGroupOption {
	return func(s *SystemGroupService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}

func WithSystemGroupClock(clock func() time.Time) SystemGroupOption {
	return func(s *SystemGroupService) {
		if clock != nil {
			s.now = clock
		}
	}
}

func (s *SystemGroupService) CreateSystemGroup(ctx context.Context, input group.SystemGroupCreateInput) (group.SystemGroup, error) {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return group.SystemGroup{}, err
	}
	model := group.SystemGroup{
		ID:            s.idGenerator(),
		SystemID:      input.SystemID,
		Name:          input.Name,
		GroupingRules: input.GroupingRules,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	projection, err := buildSystemGroupRelationshipProjection(model.SystemID, model.ID, model.GroupingRules, now)
	if err != nil {
		return group.SystemGroup{}, err
	}
	saved, err := s.repository.CreateSystemGroup(ctx, model, projection)
	if err != nil {
		return group.SystemGroup{}, fmt.Errorf("create system group: %w", err)
	}
	return saved, nil
}

func (s *SystemGroupService) ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error) {
	query = query.Normalize()
	if err := query.Validate(); err != nil {
		return group.SystemGroupPage{}, err
	}
	page, err := s.repository.ListSystemGroups(ctx, query)
	if err != nil {
		return group.SystemGroupPage{}, fmt.Errorf("list system groups: %w", err)
	}
	return page, nil
}
```

- [x] **Step 4: Add relationship builder**

Create `internal/group-service/services/system_group_relationship_builder.go`:

```go
package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/caveat"
	permissionrelationship "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/relationship"
)

func buildSystemGroupRelationshipProjection(systemID string, groupID string, rules []group.SystemGroupRule, now time.Time) (group.SystemGroupRelationshipProjection, error) {
	organizationIDs := dedupeSystemGroupRuleValues(rules, group.GroupAttributeOrganization, "")
	jobLevels := dedupeSystemGroupRuleValues(rules, group.GroupAttributeJobLevel, "")
	jobTags := dedupeSystemGroupRuleValues(rules, group.GroupAttributeJobTag, "")
	jobType := firstSystemGroupRuleValue(rules, group.GroupAttributeJobType)
	containsSecretary := containsString(jobTags, group.SystemGroupSecretarySentinel)
	a4Roles := removeString(jobTags, group.SystemGroupSecretarySentinel)

	relationships := make([]any, 0)
	if len(organizationIDs) == 0 {
		relationships = append(relationships, permissionrelationship.NewAllEmployeeToGroupForHRRelationship(groupID))
	} else {
		for _, organizationID := range organizationIDs {
			relationships = append(relationships, permissionrelationship.NewOrganizationToGroupRelationship(groupID, organizationID))
		}
	}

	if jobType != "" || len(jobLevels) > 0 || containsSecretary {
		options := make([]caveat.StaticAttributesCheckOption, 0, 3)
		if jobType != "" {
			options = append(options, caveat.WithAllowedTypes([]string{jobType}))
		}
		if len(jobLevels) > 0 {
			options = append(options, caveat.WithAllowedLevels(jobLevels))
		}
		if containsSecretary {
			options = append(options, caveat.WithIsContainSecretary(true))
		}
		relationships = append(relationships, permissionrelationship.NewGroupWithStaticAttributesRelationship(groupID, options...))
	}

	if len(a4Roles) == 0 {
		relationships = append(relationships, permissionrelationship.NewAllEmployeeToGroupForA4Relationship(groupID))
	} else {
		for _, role := range a4Roles {
			relationships = append(relationships, permissionrelationship.NewA4RoleToGroupRelationship(groupID, role))
		}
	}

	infos := make([]group.RelationshipInfo, 0, len(relationships))
	for _, relationship := range relationships {
		checksum, err := relationshipChecksum(relationship)
		if err != nil {
			return group.SystemGroupRelationshipProjection{}, err
		}
		infos = append(infos, group.RelationshipInfo{Relationship: relationship, Checksum: checksum})
	}
	return group.SystemGroupRelationshipProjection{
		SystemID:      systemID,
		GroupID:       groupID,
		Relationships: infos,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func relationshipChecksum(relationship any) (string, error) {
	data, err := json.Marshal(relationship)
	if err != nil {
		return "", fmt.Errorf("marshal relationship for checksum: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func dedupeSystemGroupRuleValues(rules []group.SystemGroupRule, key group.GroupAttributeKey, exclude string) []string {
	seen := map[string]struct{}{}
	for _, rule := range rules {
		if rule.AttributeKey != key {
			continue
		}
		if rule.Multi {
			values, ok := rule.Value.([]string)
			if !ok {
				continue
			}
			for _, value := range values {
				if value == exclude {
					continue
				}
				seen[value] = struct{}{}
			}
			continue
		}
		value, ok := rule.Value.(string)
		if ok && value != exclude {
			seen[value] = struct{}{}
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func firstSystemGroupRuleValue(rules []group.SystemGroupRule, key group.GroupAttributeKey) string {
	for _, rule := range rules {
		if rule.AttributeKey != key {
			continue
		}
		value, ok := rule.Value.(string)
		if ok {
			return value
		}
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}
```

- [x] **Step 5: Run service tests and verify pass**

Run:

```bash
go test ./internal/group-service/services -run 'SystemGroup' -v
```

Expected: PASS.

- [x] **Step 6: Commit service layer**

```bash
git add internal/group-service/services/system_group_relationship_builder.go internal/group-service/services/system_group_service.go internal/group-service/services/system_group_service_test.go
git commit -m "feat: add system group service workflow"
```

---

### Task 4: MongoDB Repository For System Groups

**Files:**

- Create: `internal/group-service/repositories/mongo_system_group_repository.go`
- Create: `internal/group-service/repositories/mongo_system_group_repository_test.go`
- Modify: `internal/group-service/repositories/mongo_group_repository.go`

- [x] **Step 1: Write failing repository tests**

Create `internal/group-service/repositories/mongo_system_group_repository_test.go`:

```go
package repositories

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func repositorySystemGroup() group.SystemGroup {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	return group.SystemGroup{
		ID:       "group-1",
		SystemID: "system-a",
		Name:     "System Admins",
		GroupingRules: []group.SystemGroupRule{{
			AttributeKey: group.GroupAttributeOrganization,
			Operator:     group.OperatorEq,
			Multi:        true,
			Value:        []string{"ORG-100", "ORG-200"},
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func repositorySystemGroupProjection() group.SystemGroupRelationshipProjection {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	return group.SystemGroupRelationshipProjection{
		SystemID: "system-a",
		GroupID:  "group-1",
		Relationships: []group.RelationshipInfo{{
			Relationship: map[string]any{"relation": "hr_member"},
			Checksum:     "checksum-1",
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestNewSystemGroupDocumentMapping(t *testing.T) {
	doc := newSystemGroupDocument(repositorySystemGroup())
	if doc.ID != "group-1" || doc.SystemID != "system-a" || doc.Name != "System Admins" {
		t.Fatalf("doc = %+v, want identity/name copied", doc)
	}
	model := doc.toDomain()
	if model.ID != "group-1" || model.GroupingRules[0].AttributeKey != group.GroupAttributeOrganization {
		t.Fatalf("model = %+v, want domain mapping", model)
	}
}

func TestNewSystemGroupRelationshipDocumentMapping(t *testing.T) {
	doc := newSystemGroupRelationshipDocument(repositorySystemGroupProjection())
	if doc.SystemID != "system-a" || doc.GroupID != "group-1" {
		t.Fatalf("doc = %+v, want projection identity", doc)
	}
	if len(doc.Relationships) != 1 || doc.Relationships[0].Checksum != "checksum-1" {
		t.Fatalf("relationships = %+v, want checksum", doc.Relationships)
	}
}

func TestSystemGroupIndexModels(t *testing.T) {
	groupIndexes := systemGroupIndexModels()
	if len(groupIndexes) != 1 {
		t.Fatalf("system group indexes len = %d, want 1", len(groupIndexes))
	}
	if *indexOptions(t, groupIndexes[0]).Name != systemGroupsSystemCreatedIndexName {
		t.Fatalf("index name = %q, want %q", *indexOptions(t, groupIndexes[0]).Name, systemGroupsSystemCreatedIndexName)
	}

	relationshipIndexes := systemGroupRelationshipIndexModels()
	if len(relationshipIndexes) != 1 {
		t.Fatalf("relationship indexes len = %d, want 1", len(relationshipIndexes))
	}
	options := indexOptions(t, relationshipIndexes[0])
	if options.Unique == nil || !*options.Unique {
		t.Fatal("relationship unique index Unique = false, want true")
	}
}

func TestBuildSystemGroupListFilter(t *testing.T) {
	cursorTime := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	filter := buildSystemGroupListFilter(group.SystemGroupListQuery{
		SystemID: "system-a",
		Cursor:  &group.SystemGroupCursor{CreatedAt: cursorTime, ID: "group-9"},
	})
	want := bson.M{
		"system_id": "system-a",
		"$or": bson.A{
			bson.M{"created_at": bson.M{"$lt": cursorTime}},
			bson.M{"created_at": cursorTime, "_id": bson.M{"$lt": "group-9"}},
		},
	}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestBuildSystemGroupPage(t *testing.T) {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	page := buildSystemGroupPage([]systemGroupDocument{
		{ID: "group-1", SystemID: "system-a", Name: "One", CreatedAt: now, UpdatedAt: now},
		{ID: "group-2", SystemID: "system-a", Name: "Two", CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}, 1)
	if !page.HasNextPage || len(page.Groups) != 1 {
		t.Fatalf("page = %+v, want one item with next page", page)
	}
	if page.NextCursor == nil || page.NextCursor.ID != "group-1" {
		t.Fatalf("next cursor = %+v, want group-1", page.NextCursor)
	}
}

func TestMongoGroupRepositoryCreateAndListSystemGroupsIntegration(t *testing.T) {
	ctx := context.Background()
	repository := newIntegrationRepository(t)
	if err := repository.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes error = %v", err)
	}

	model := repositorySystemGroup()
	projection := repositorySystemGroupProjection()
	saved, err := repository.CreateSystemGroup(ctx, model, projection)
	if err != nil {
		t.Fatalf("CreateSystemGroup error = %v", err)
	}
	if saved.ID != model.ID {
		t.Fatalf("saved ID = %q, want %q", saved.ID, model.ID)
	}
	page, err := repository.ListSystemGroups(ctx, group.SystemGroupListQuery{SystemID: "system-a", Limit: 20})
	if err != nil {
		t.Fatalf("ListSystemGroups error = %v", err)
	}
	if len(page.Groups) != 1 || page.Groups[0].ID != "group-1" {
		t.Fatalf("page = %+v, want group-1", page)
	}
	count, err := repository.systemGroupRelationships.CountDocuments(ctx, bson.M{"system_id": "system-a", "group_id": "group-1"})
	if err != nil {
		t.Fatalf("count relationships: %v", err)
	}
	if count != 1 {
		t.Fatalf("relationship docs = %d, want 1", count)
	}
}
```

- [x] **Step 2: Run repository tests and confirm failure**

Run:

```bash
go test ./internal/group-service/repositories -run 'SystemGroup' -v
```

Expected: FAIL with undefined symbols such as `systemGroupDocument`, `newSystemGroupDocument`, `systemGroupIndexModels`, and `CreateSystemGroup`.

- [x] **Step 3: Extend repository struct and indexes**

Modify `internal/group-service/repositories/mongo_group_repository.go`:

```go
const (
	groupCollectionName                            = "groups"
	groupIndividualMemberCollectionName            = "group_individual_members"
	systemGroupCollectionName                      = "system_groups"
	systemGroupRelationshipCollectionName          = "system_group_relationships"
	groupsActiveNameUniqueIndexName                = "groups_active_workspace_normalized_name_unique"
	groupsWorkspaceCreatedIndexName                = "groups_workspace_created_id"
	membersActiveGroupAccountUniqueIndexName       = "group_individual_members_active_group_account_unique"
	membersActiveUnexpiredGroupIndexName           = "group_individual_members_active_unexpired_group"
	membersGroupCreatedIndexName                   = "group_individual_members_group_created_id"
	systemGroupsSystemCreatedIndexName             = "system_groups_system_created_id"
	systemGroupRelationshipsSystemGroupUniqueName  = "system_group_relationships_system_group_unique"
)
```

Update `MongoGroupRepository` and constructor:

```go
type MongoGroupRepository struct {
	client                   *mongo.Client
	groups                   *mongo.Collection
	members                  *mongo.Collection
	systemGroups             *mongo.Collection
	systemGroupRelationships *mongo.Collection
	expiryRepository         *sharedexpiry.MongoRepository
}

func NewMongoGroupRepository(client *mongo.Client, db *mongo.Database) *MongoGroupRepository {
	return &MongoGroupRepository{
		client:                   client,
		groups:                   db.Collection(groupCollectionName),
		members:                  db.Collection(groupIndividualMemberCollectionName),
		systemGroups:             db.Collection(systemGroupCollectionName),
		systemGroupRelationships: db.Collection(systemGroupRelationshipCollectionName),
		expiryRepository:         sharedexpiry.NewMongoRepository(db),
	}
}
```

Update `EnsureIndexes`:

```go
func (r *MongoGroupRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.groups.Indexes().CreateMany(ctx, groupIndexModels()); err != nil {
		return fmt.Errorf("create group indexes: %w", err)
	}
	if _, err := r.members.Indexes().CreateMany(ctx, individualMemberIndexModels()); err != nil {
		return fmt.Errorf("create group individual member indexes: %w", err)
	}
	if _, err := r.systemGroups.Indexes().CreateMany(ctx, systemGroupIndexModels()); err != nil {
		return fmt.Errorf("create system group indexes: %w", err)
	}
	if _, err := r.systemGroupRelationships.Indexes().CreateMany(ctx, systemGroupRelationshipIndexModels()); err != nil {
		return fmt.Errorf("create system group relationship indexes: %w", err)
	}
	if err := r.expiryRepository.EnsureIndexes(ctx); err != nil {
		return err
	}
	return nil
}
```

- [x] **Step 4: Add system group repository implementation**

Create `internal/group-service/repositories/mongo_system_group_repository.go`:

```go
package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type systemGroupDocument struct {
	ID            string                    `bson:"_id"`
	SystemID      string                    `bson:"system_id"`
	Name          string                    `bson:"name"`
	GroupingRules []systemGroupRuleDocument `bson:"grouping_rules"`
	CreatedAt     time.Time                 `bson:"created_at"`
	UpdatedAt     time.Time                 `bson:"updated_at"`
}

type systemGroupRuleDocument struct {
	AttributeKey group.GroupAttributeKey `bson:"attribute_key"`
	Operator     group.Operator                `bson:"operator"`
	Multi        bool                          `bson:"multi"`
	Value        any                           `bson:"value"`
}

type systemGroupRelationshipDocument struct {
	SystemID      string                             `bson:"system_id"`
	GroupID       string                             `bson:"group_id"`
	Relationships []systemGroupRelationshipInfoDocument `bson:"relationship"`
	CreatedAt     time.Time                          `bson:"created_at"`
	UpdatedAt     time.Time                          `bson:"updated_at"`
}

type systemGroupRelationshipInfoDocument struct {
	Relationship any    `bson:"relationship"`
	Checksum     string `bson:"checksum"`
}

func (r *MongoGroupRepository) CreateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return group.SystemGroup{}, fmt.Errorf("start system group create session: %w", err)
	}
	defer session.EndSession(ctx)

	if _, err := session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		if _, insertErr := r.systemGroups.InsertOne(sessionCtx, newSystemGroupDocument(model)); insertErr != nil {
			return nil, fmt.Errorf("insert system group: %w", insertErr)
		}
		if _, insertErr := r.systemGroupRelationships.InsertOne(sessionCtx, newSystemGroupRelationshipDocument(projection)); insertErr != nil {
			return nil, fmt.Errorf("insert system group relationships: %w", insertErr)
		}
		return nil, nil
	}); err != nil {
		return group.SystemGroup{}, err
	}
	return model, nil
}

func (r *MongoGroupRepository) ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error) {
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(query.Limit + 1))
	cursor, err := r.systemGroups.Find(ctx, buildSystemGroupListFilter(query), findOptions)
	if err != nil {
		return group.SystemGroupPage{}, fmt.Errorf("find system groups: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()
	var docs []systemGroupDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return group.SystemGroupPage{}, fmt.Errorf("decode system groups: %w", err)
	}
	return buildSystemGroupPage(docs, query.Limit), nil
}

func systemGroupIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{{
		Keys: bson.D{
			{Key: "system_id", Value: 1},
			{Key: "created_at", Value: -1},
			{Key: "_id", Value: -1},
		},
		Options: options.Index().SetName(systemGroupsSystemCreatedIndexName),
	}}
}

func systemGroupRelationshipIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{{
		Keys: bson.D{
			{Key: "system_id", Value: 1},
			{Key: "group_id", Value: 1},
		},
		Options: options.Index().SetName(systemGroupRelationshipsSystemGroupUniqueName).SetUnique(true),
	}}
}

func buildSystemGroupListFilter(query group.SystemGroupListQuery) bson.M {
	filter := bson.M{"system_id": query.SystemID}
	if query.Cursor != nil {
		filter["$or"] = bson.A{
			bson.M{"created_at": bson.M{"$lt": query.Cursor.CreatedAt}},
			bson.M{"created_at": query.Cursor.CreatedAt, "_id": bson.M{"$lt": query.Cursor.ID}},
		}
	}
	return filter
}

func newSystemGroupDocument(model group.SystemGroup) systemGroupDocument {
	rules := make([]systemGroupRuleDocument, 0, len(model.GroupingRules))
	for _, rule := range model.GroupingRules {
		rules = append(rules, systemGroupRuleDocument{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	return systemGroupDocument{
		ID:            model.ID,
		SystemID:      model.SystemID,
		Name:          model.Name,
		GroupingRules: rules,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}

func (d systemGroupDocument) toDomain() group.SystemGroup {
	rules := make([]group.SystemGroupRule, 0, len(d.GroupingRules))
	for _, rule := range d.GroupingRules {
		rules = append(rules, group.SystemGroupRule{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	return group.SystemGroup{
		ID:            d.ID,
		SystemID:      d.SystemID,
		Name:          d.Name,
		GroupingRules: rules,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

func newSystemGroupRelationshipDocument(model group.SystemGroupRelationshipProjection) systemGroupRelationshipDocument {
	relationships := make([]systemGroupRelationshipInfoDocument, 0, len(model.Relationships))
	for _, relationship := range model.Relationships {
		relationships = append(relationships, systemGroupRelationshipInfoDocument{
			Relationship: relationship.Relationship,
			Checksum:     relationship.Checksum,
		})
	}
	return systemGroupRelationshipDocument{
		SystemID:      model.SystemID,
		GroupID:       model.GroupID,
		Relationships: relationships,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}

func buildSystemGroupPage(docs []systemGroupDocument, limit int) group.SystemGroupPage {
	hasNext := len(docs) > limit
	if hasNext {
		docs = docs[:limit]
	}
	groups := make([]group.SystemGroup, 0, len(docs))
	for _, doc := range docs {
		groups = append(groups, doc.toDomain())
	}
	var nextCursor *group.SystemGroupCursor
	if hasNext && len(groups) > 0 {
		last := groups[len(groups)-1]
		nextCursor = &group.SystemGroupCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return group.SystemGroupPage{Groups: groups, HasNextPage: hasNext, NextCursor: nextCursor}
}
```

- [x] **Step 5: Run repository tests and fix BSON value mapping if needed**

Run:

```bash
go test ./internal/group-service/repositories -run 'SystemGroup' -v
```

Expected: PASS. If MongoDB decodes `[]string` fields as `bson.A`, adjust `systemGroupRuleDocument.toDomain` mapping to convert `bson.A` string items back to `[]string`, then rerun the same command.

- [x] **Step 6: Run repository package tests**

Run:

```bash
go test ./internal/group-service/repositories -v
```

Expected: PASS.

- [x] **Step 7: Commit repository layer**

```bash
git add internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_system_group_repository.go internal/group-service/repositories/mongo_system_group_repository_test.go
git commit -m "feat: persist system groups"
```

---

### Task 5: HTTP Handlers And Route Registration

**Files:**

- Modify: `internal/group-service/handlers/group_handler.go`
- Modify: `internal/group-service/handlers/group_handler_test.go`

- [x] **Step 1: Extend fake service and write failing handler tests**

Modify `fakeHTTPGroupService` in `internal/group-service/handlers/group_handler_test.go` with these fields:

```go
	systemGroupInput group.SystemGroupCreateInput
	systemGroupQuery group.SystemGroupListQuery
	systemGroupModel group.SystemGroup
	systemGroupPage  group.SystemGroupPage
	systemCreateCalls int
	systemListCalls   int
```

Add these fake methods:

```go
func (f *fakeHTTPGroupService) CreateSystemGroup(ctx context.Context, input group.SystemGroupCreateInput) (group.SystemGroup, error) {
	f.systemCreateCalls++
	f.systemGroupInput = input
	if f.err != nil {
		return group.SystemGroup{}, f.err
	}
	return f.systemGroupModel, nil
}

func (f *fakeHTTPGroupService) ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error) {
	f.systemListCalls++
	f.systemGroupQuery = query
	if f.err != nil {
		return group.SystemGroupPage{}, f.err
	}
	return f.systemGroupPage, nil
}
```

Append these tests:

```go
func systemGroupHandlerModel() group.SystemGroup {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	return group.SystemGroup{
		ID:       "group-1",
		SystemID: "system-a",
		Name:     "System Admins",
		GroupingRules: []group.SystemGroupRule{{
			AttributeKey: group.GroupAttributeOrganization,
			Operator:     group.OperatorEq,
			Multi:        true,
			Value:        []string{"ORG-100"},
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func validSystemGroupRequestBody() string {
	return `{
		"name": "System Admins",
		"grouping_rules": [
			{"attribute_key": "organization", "operator": "eq", "multi": true, "value": ["ORG-100"]}
		]
	}`
}

func TestGroupHandlerCreateSystemGroup(t *testing.T) {
	service := &fakeHTTPGroupService{systemGroupModel: systemGroupHandlerModel()}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/systems/system-a/groups", strings.NewReader(validSystemGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if service.systemCreateCalls != 1 || service.systemGroupInput.SystemID != "system-a" {
		t.Fatalf("service calls/input = %d/%+v, want system create", service.systemCreateCalls, service.systemGroupInput)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["group"]; !ok {
		t.Fatal("response missing group")
	}
}

func TestGroupHandlerCreateSystemGroupValidationError(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrInvalidInput}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/systems/system-a/groups", strings.NewReader(validSystemGroupRequestBody()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGroupHandlerListSystemGroups(t *testing.T) {
	service := &fakeHTTPGroupService{systemGroupPage: group.SystemGroupPage{Groups: []group.SystemGroup{systemGroupHandlerModel()}}}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/systems/system-a/groups?limit=10", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.systemListCalls != 1 || service.systemGroupQuery.SystemID != "system-a" || service.systemGroupQuery.Limit != 10 {
		t.Fatalf("query = %+v, want system-a limit 10", service.systemGroupQuery)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["groups"]; !ok {
		t.Fatal("response missing groups")
	}
}

func TestGroupHandlerListSystemGroupsInvalidLimit(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/systems/system-a/groups?limit=51", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if service.systemListCalls != 0 {
		t.Fatalf("service calls = %d, want 0", service.systemListCalls)
	}
}
```

- [x] **Step 2: Run handler tests and confirm failure**

Run:

```bash
go test ./internal/group-service/handlers -run 'SystemGroup' -v
```

Expected: FAIL because `HTTPGroupService` does not include system group methods and routes are not registered.

- [x] **Step 3: Extend handler interface and routes**

Modify `HTTPGroupService` in `internal/group-service/handlers/group_handler.go`:

```go
type HTTPGroupService interface {
	CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error)
	GetGroup(ctx context.Context, query group.GetQuery) (*group.Group, error)
	DeleteGroup(ctx context.Context, input group.DeleteInput) error
	UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput) error
	ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error)
	AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error)
	UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput) error
	DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput) error
	CreateSystemGroup(ctx context.Context, input group.SystemGroupCreateInput) (group.SystemGroup, error)
	ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error)
}
```

Add routes in `RegisterRoutes`:

```go
	e.POST("/api/v1/systems/:system_id/groups", handler.CreateSystemGroup)
	e.GET("/api/v1/systems/:system_id/groups", handler.ListSystemGroups)
```

- [x] **Step 4: Add system group handler methods**

Add to `internal/group-service/handlers/group_handler.go`:

```go
func (h *GroupHandler) CreateSystemGroup(c *echo.Context) error {
	systemID := c.Param("system_id")
	request, err := transport.DecodeSystemGroupCreateRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(systemID)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}
	model, err := h.service.CreateSystemGroup(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to create system group", "err", err, "system_id", systemID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusCreated, transport.NewSystemGroupCreateResponse(model))
}

func (h *GroupHandler) ListSystemGroups(c *echo.Context) error {
	systemID := c.Param("system_id")
	limit, err := h.paginationHelper.ParseLimit(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	token, err := h.paginationHelper.ParseToken(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	cursor, err := transport.DecodeSystemGroupNextToken(token)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	page, err := h.service.ListSystemGroups(c.Request().Context(), group.SystemGroupListQuery{
		SystemID: systemID,
		Limit:    limit,
		Cursor:   cursor,
	})
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to list system groups", "err", err, "system_id", systemID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	response, err := transport.NewSystemGroupListResponse(page)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, response)
}
```

- [x] **Step 5: Run handler tests and verify pass**

Run:

```bash
go test ./internal/group-service/handlers -run 'SystemGroup|GroupHandler' -v
```

Expected: PASS.

- [x] **Step 6: Commit handler layer**

```bash
git add internal/group-service/handlers/group_handler.go internal/group-service/handlers/group_handler_test.go
git commit -m "feat: expose system group routes"
```

---

### Task 6: Composition Root, REST Examples, And Package Integration

**Files:**

- Modify: `cmd/group-service/main.go`
- Create: `examples/api/system-groups.http`

- [x] **Step 1: Run package tests and confirm composition failure**

Run:

```bash
go test ./internal/group-service/... ./cmd/group-service
```

Expected: FAIL because `*services.GroupService` does not implement the extended handler interface.

- [x] **Step 2: Add a combined service adapter in `cmd/group-service/main.go`**

Add near `processIndicator`:

```go
type groupHTTPService struct {
	*services.GroupService
	*services.SystemGroupService
}
```

Change service construction in `run`:

```go
	groupService := services.NewGroupService(repository,
		services.WithGroupValidationLimits(
			cfg.Validation.MaxIndividualMembers,
			cfg.Validation.MaxGroupingRules,
		),
		services.WithGroupExpiryBucketLocation(cfg.GroupExpiryCommand.BucketLocation),
		services.WithIndividualMemberExpiryBucketLocation(cfg.IndividualMemberExpiryCommand.BucketLocation),
	)
	systemGroupService := services.NewSystemGroupService(repository)
	httpService := groupHTTPService{
		GroupService:       groupService,
		SystemGroupService: systemGroupService,
	}
```

Update route registration:

```go
	handlers.RegisterRoutes(e, handlers.NewGroupHandler(httpService, logger, pagination.New()))
```

Keep event handlers using `groupService`, not `httpService`, because expiry command handlers only need workspace group workflows:

```go
	eventHandler := handlers.NewGroupExpiryEventHandler(groupService, cfg.GroupExpiryCommand.Subject, logger)
	individualMemberExpiryEventHandler := handlers.NewIndividualMemberExpiryEventHandler(groupService, cfg.IndividualMemberExpiryCommand.Subject, logger)
```

- [x] **Step 3: Add REST Client examples**

Create `examples/api/system-groups.http`:

```http
@baseUrl = http://localhost:8082
@systemId = todo
@nextToken = replace-with-next-token

### Create system group
POST {{baseUrl}}/api/v1/systems/{{systemId}}/groups
Content-Type: application/json

{
  "name": "System Admins",
  "grouping_rules": [
    {
      "attribute_key": "organization",
      "operator": "eq",
      "multi": true,
      "value": ["ORG-100", "ORG-200"]
    },
    {
      "attribute_key": "job_type",
      "operator": "eq",
      "multi": false,
      "value": "DL"
    },
    {
      "attribute_key": "job_level",
      "operator": "eq",
      "multi": false,
      "value": "M2"
    },
    {
      "attribute_key": "job_tag",
      "operator": "eq",
      "multi": true,
      "value": ["a4_reviewer", "_internal_secretary_"]
    }
  ]
}

### Create system group with empty rules
POST {{baseUrl}}/api/v1/systems/{{systemId}}/groups
Content-Type: application/json

{
  "name": "All Employees",
  "grouping_rules": []
}

### List system groups
GET {{baseUrl}}/api/v1/systems/{{systemId}}/groups?limit=20

### List next page
GET {{baseUrl}}/api/v1/systems/{{systemId}}/groups?limit=20&next_token={{nextToken}}

### Validation error: not_eq is not supported in this phase
POST {{baseUrl}}/api/v1/systems/{{systemId}}/groups
Content-Type: application/json

{
  "name": "Unsupported Operator",
  "grouping_rules": [
    {
      "attribute_key": "organization",
      "operator": "not_eq",
      "multi": true,
      "value": ["ORG-100"]
    }
  ]
}

### Validation error: duplicate job_type
POST {{baseUrl}}/api/v1/systems/{{systemId}}/groups
Content-Type: application/json

{
  "name": "Duplicate Job Type",
  "grouping_rules": [
    {
      "attribute_key": "job_type",
      "operator": "eq",
      "multi": false,
      "value": "DL"
    },
    {
      "attribute_key": "job_type",
      "operator": "eq",
      "multi": false,
      "value": "IDL"
    }
  ]
}

### Validation error: invalid limit
GET {{baseUrl}}/api/v1/systems/{{systemId}}/groups?limit=51

### Validation error: invalid next_token
GET {{baseUrl}}/api/v1/systems/{{systemId}}/groups?next_token=not-base64
```

- [x] **Step 4: Run integration package tests**

Run:

```bash
go test ./internal/group-service/... ./cmd/group-service
```

Expected: PASS.

- [x] **Step 5: Commit composition and examples**

```bash
git add cmd/group-service/main.go examples/api/system-groups.http
git commit -m "feat: wire system group api"
```

---

### Task 7: Full Verification And Plan Completion

**Files:**

- Verify all changed files.
- Update this plan's checkboxes while executing.

- [x] **Step 1: Run focused verification**

Run:

```bash
go test ./internal/domain/group ./internal/group-service/... ./cmd/group-service
```

Expected: PASS.

- [x] **Step 2: Run repository-wide verification**

Run:

```bash
go test ./...
```

Expected: PASS.

- [x] **Step 3: Verify examples and docs references**

Run:

```bash
test -f docs/designs/system-group-api-design.md
test -f examples/api/system-groups.http
rg -n "system_groups|system_group_relationships|/api/v1/systems/:system_id/groups" docs/designs/system-group-api-design.md docs/designs/group-service.md examples/api/system-groups.http
```

Expected: all commands exit 0 and show the system group design/API references.

- [x] **Step 4: Verify no policy-prohibited strings or placeholders were introduced**

Run:

```bash
bad_system='TS''MC'
bad_tbd='T''BD'
bad_todo='TO''DO'
bad_later='implement ''later'
bad_fill='fill in ''details'
pattern="$bad_system|$bad_tbd|$bad_todo|$bad_later|$bad_fill"
rg -n "$pattern" docs/plans/active/2026-05-20-system-group-api.md docs/designs/system-group-api-design.md examples/api/system-groups.http internal/domain/group internal/group-service cmd/group-service
```

Expected: exit 1 with no matches.

- [x] **Step 5: Inspect git diff**

Run:

```bash
git diff --stat
git diff --check
```

Expected: `git diff --check` exits 0. Confirm the diff contains only system group implementation, examples, and this plan.

- [x] **Step 6: Commit finalized implementation state**

```bash
git add .
git commit -m "feat: add system group api"
```

Skip this commit if the previous task commits already cover all changes and repository policy for the current branch prefers one commit per task.

---

## Self-Review Checklist

- [x] Source design coverage: create API, list API, rule validation, `not_eq` rejection, relationship projection, checksum, transaction, persistence, examples, and tests are all covered by tasks.
- [x] Boundary coverage: domain, transport, service, repository, handler, and composition root responsibilities are separated.
- [x] API contract coverage: `snake_case` JSON, response wrapper, timestamps, pagination, and error shape are covered.
- [x] Persistence coverage: `system_groups`, `system_group_relationships`, indexes, and transaction behavior are covered.
- [x] Verification coverage: focused package tests, full `go test ./...`, REST example presence, placeholder scan, and `git diff --check` are included.
