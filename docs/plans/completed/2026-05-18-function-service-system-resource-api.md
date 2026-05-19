# Function Service System Resource API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add system-scoped resource definition APIs to `function-service`, persist definitions and derived resource attributes, and keep `system_id` aligned with the existing `function_key` identity.

**Architecture:** Keep resource definition and attribute domain behavior in `internal/domain/resource`; transport owns HTTP DTOs; handlers remain thin; services own validation orchestration, transaction workflow, ID/time seams, count limits, and attribute derivation; repositories own MongoDB documents, indexes, upserts, reads, and transaction sessions. The new APIs are independent of the existing workspace-scoped `function_key` routes, which remain unchanged.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, Viper, `log/slog`, standard `encoding/json`, standard `testing`, REST Client `.http` examples.

---

## Source Designs And Policies

- Source design: [../../designs/function-service-system-resource-api-design.md](../../designs/function-service-system-resource-api-design.md)
- Related design: [../../designs/function-service.md](../../designs/function-service.md)
- Related design: [../../designs/function-resource-permissions.md](../../designs/function-resource-permissions.md)
- Policy: [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- Policy: [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- This is backend implementation plus design-plan documentation work.
- Domain packages must not depend on Echo, MongoDB, NATS, JetStream, transport DTOs, or service-private packages.
- Transport packages may depend on domain packages for DTO-to-domain mapping.
- Services depend on domain contracts and consumer-side repository interfaces.
- Repositories may depend on MongoDB driver types and domain packages, but not handlers, transport, or services.
- Public JSON fields use `snake_case`; request/response DTOs stay in transport packages.
- REST API changes require `examples/api/*.http` examples.
- This implementation plan lives under `docs/plans/active/` and links to its source design.

## Scope

Implement:

- `POST /api/v1/systems/:system_id/resources`.
- `GET /api/v1/systems/:system_id/resources`.
- `GET /api/v1/systems/:system_id/resource-attributes`.
- `system_resources` MongoDB collection with unique `{ system_id, type, key }`.
- `system_resource_attributes` MongoDB collection with unique `{ system_id }`.
- Configurable resource definition limits with defaults: type `3`, action `5`, tag `20`.
- Partial upsert semantics, preserving omitted existing definitions.
- Request-order POST responses.
- `ResourceAttribute` and `NewResourceAttribute(action, tag, resourceType string)` in `internal/domain/resource/resource_attribute.go`.
- REST Client examples in `examples/api/system_resources.http`.

Do not implement:

- Migration of existing workspace-scoped routes from `function_key` to `system_id`.
- Resource definition delete APIs.
- Registry existence validation for systems.
- Permission evaluation changes.
- Frontend changes.

## File Structure And Responsibilities

- Create: `internal/domain/resource/resource_definition.go`
  - Resource definition models, input/query structs, limit model, normalization, validation, duplicate request detection, and post-write count validation.
- Create: `internal/domain/resource/resource_attribute.go`
  - `ResourceAttribute` string type and `NewResourceAttribute` constructor.
- Modify: `internal/domain/resource/validation_test.go`
  - Domain tests for resource definitions, count limits, and attributes.
- Modify: `internal/function-service/config/config.go`
  - System resource limit config, defaults, and validation.
- Modify: `internal/function-service/config/config_test.go`
  - Config load/default/reject tests for the new limits.
- Create: `internal/function-service/transport/system_resource_request.go`
  - POST request DTO decoding and DTO-to-domain mapping.
- Create: `internal/function-service/transport/system_resource_response.go`
  - POST/GET resources response DTOs and attributes response DTO.
- Create: `internal/function-service/transport/system_resource_request_test.go`
  - Request decode and mapping tests.
- Create: `internal/function-service/transport/system_resource_response_test.go`
  - Response shape and timestamp tests.
- Create: `internal/function-service/services/system_resource_service.go`
  - Save/list/get-attributes workflows, transaction orchestration, ID/time injection, count validation, and attribute derivation.
- Create: `internal/function-service/services/system_resource_service_test.go`
  - Service tests with a fake repository.
- Create: `internal/function-service/repositories/mongo_system_resource_repository.go`
  - MongoDB collection mapping, index models, transaction runner, definition upserts, list reads, attribute reads, and attribute upserts.
- Create: `internal/function-service/repositories/mongo_system_resource_repository_test.go`
  - Repository helper, mapping, index, filter, update, and transaction-shape tests.
- Create: `internal/function-service/handlers/system_resource_handler.go`
  - Echo route registration and thin HTTP handlers.
- Create: `internal/function-service/handlers/system_resource_handler_test.go`
  - POST/GET handler success, empty, validation, and failure tests.
- Modify: `cmd/function-service/main.go`
  - Build repository/service, ensure indexes, and register routes.
- Modify: `.env.example`
  - Add system resource limit defaults.
- Create: `examples/api/system_resources.http`
  - Manual API examples for success and validation cases.

---

### Task 1: Resource Domain Models, Validation, And Attributes

**Files:**

- Create: `internal/domain/resource/resource_definition.go`
- Create: `internal/domain/resource/resource_attribute.go`
- Modify: `internal/domain/resource/validation_test.go`

- [ ] **Step 1: Write failing domain tests**

Append these tests to `internal/domain/resource/validation_test.go`:

```go
func validResourceDefinitionSaveInput() ResourceDefinitionSaveInput {
	return ResourceDefinitionSaveInput{
		SystemID: "todo",
		Resources: []ResourceDefinitionInput{
			{Type: ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit", Description: "Allows editing."},
			{Type: ResourceDefinitionKindTag, Label: "Private", Key: "private"},
			{Type: ResourceDefinitionKindType, Label: "Repository", Key: "repo"},
		},
	}
}

func TestResourceDefinitionSaveInputValidateAcceptsValidInput(t *testing.T) {
	input := validResourceDefinitionSaveInput()

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestResourceDefinitionSaveInputNormalizeTrimsValues(t *testing.T) {
	input := ResourceDefinitionSaveInput{
		SystemID: " todo ",
		Resources: []ResourceDefinitionInput{{
			Type:        ResourceDefinitionType(" action "),
			Label:       " Can Edit ",
			Key:         " can_edit ",
			Description: " Allows editing. ",
		}},
	}

	normalized := input.Normalize()
	if normalized.SystemID != "todo" {
		t.Fatalf("SystemID = %q, want todo", normalized.SystemID)
	}
	got := normalized.Resources[0]
	if got.Type != ResourceDefinitionKindAction || got.Label != "Can Edit" || got.Key != "can_edit" || got.Description != "Allows editing." {
		t.Fatalf("resource = %+v, want trimmed action", got)
	}
}

func TestResourceDefinitionSaveInputValidateRejectsInvalidFields(t *testing.T) {
	longLabel := strings.Repeat("測", 21)
	longDescription := strings.Repeat("a", 2001)
	tests := []struct {
		name        string
		mutate      func(*ResourceDefinitionSaveInput)
		wantMessage string
	}{
		{name: "blank system id", mutate: func(i *ResourceDefinitionSaveInput) { i.SystemID = "   " }, wantMessage: "system id is required"},
		{name: "dotted system id", mutate: func(i *ResourceDefinitionSaveInput) { i.SystemID = "app.todo" }, wantMessage: "system id must be a single subject token"},
		{name: "whitespace system id", mutate: func(i *ResourceDefinitionSaveInput) { i.SystemID = "todo app" }, wantMessage: "system id must be a single subject token"},
		{name: "empty resources", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources = nil }, wantMessage: "resources are required"},
		{name: "invalid resource type", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Type = "scope" }, wantMessage: "resource type must be type, tag, or action"},
		{name: "blank key", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Key = "   " }, wantMessage: "resource key is required"},
		{name: "uppercase key", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Key = "Can_Edit" }, wantMessage: "resource key must contain only lower-case letters, numbers, and underscores"},
		{name: "long key", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Key = "abcdefghijklmnop" }, wantMessage: "resource key must be at most 15 characters"},
		{name: "blank label", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Label = "   " }, wantMessage: "resource label is required"},
		{name: "long unicode label", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Label = longLabel }, wantMessage: "resource label must be at most 20 characters"},
		{name: "long description", mutate: func(i *ResourceDefinitionSaveInput) { i.Resources[0].Description = longDescription }, wantMessage: "resource description must be at most 2000 characters"},
		{name: "duplicate request identity", mutate: func(i *ResourceDefinitionSaveInput) {
			i.Resources = append(i.Resources, ResourceDefinitionInput{Type: ResourceDefinitionKindAction, Label: "Can Update", Key: "can_edit"})
		}, wantMessage: "duplicate resource definition action/can_edit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validResourceDefinitionSaveInput()
			tt.mutate(&input)
			requireInvalidInput(t, input.Validate(), tt.wantMessage)
		})
	}
}

func TestResourceDefinitionLimitsValidateAndCounts(t *testing.T) {
	limits := ResourceDefinitionLimits{Types: 1, Actions: 1, Tags: 1}
	definitions := []ResourceDefinition{
		{SystemID: "todo", Type: ResourceDefinitionKindType, Key: "repo"},
		{SystemID: "todo", Type: ResourceDefinitionKindType, Key: "page"},
	}

	err := ValidateResourceDefinitionCounts(definitions, limits)
	requireInvalidInput(t, err, "resource type limit exceeded")
}

func TestResourceDefinitionLimitsRejectsInvalidLimit(t *testing.T) {
	err := ResourceDefinitionLimits{Types: 0, Actions: 1, Tags: 1}.Validate()
	requireInvalidInput(t, err, "resource type limit must be greater than zero")
}

func TestNewResourceAttribute(t *testing.T) {
	got := NewResourceAttribute("can_edit", "private", "repo")
	want := ResourceAttribute("can_edit_private_repo")

	if got != want {
		t.Fatalf("ResourceAttribute = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run domain tests and confirm failure**

Run:

```bash
go test ./internal/domain/resource -run 'ResourceDefinition|ResourceAttribute' -v
```

Expected: FAIL with undefined symbols such as `ResourceDefinitionSaveInput`, `ResourceDefinitionKindAction`, and `NewResourceAttribute`.

- [ ] **Step 3: Add resource definition domain code**

Create `internal/domain/resource/resource_definition.go` with this import list and initial model definitions:

```go
package resource

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type ResourceDefinitionType string

const (
	ResourceDefinitionKindType   ResourceDefinitionType = "type"
	ResourceDefinitionKindTag    ResourceDefinitionType = "tag"
	ResourceDefinitionKindAction ResourceDefinitionType = "action"
)

var resourceDefinitionKeyPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

type ResourceDefinition struct {
	ID          string
	SystemID    string
	Type        ResourceDefinitionType
	Label       string
	Key         string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

Add these input and validation definitions in the same file:

```go
type ResourceDefinitionInput struct {
	Type        ResourceDefinitionType
	Label       string
	Key         string
	Description string
}

type ResourceDefinitionSaveInput struct {
	SystemID  string
	Resources []ResourceDefinitionInput
}

type ResourceDefinitionsQuery struct {
	SystemID string
}

type ResourceAttributesQuery struct {
	SystemID string
}

type ResourceDefinitionLimits struct {
	Types   int
	Actions int
	Tags    int
}

type ResourceAttributes struct {
	ID        string
	SystemID  string
	Values    []ResourceAttribute
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (input ResourceDefinitionSaveInput) Normalize() ResourceDefinitionSaveInput {
	input.SystemID = strings.TrimSpace(input.SystemID)
	normalized := make([]ResourceDefinitionInput, 0, len(input.Resources))
	for _, item := range input.Resources {
		normalized = append(normalized, ResourceDefinitionInput{
			Type:        ResourceDefinitionType(strings.TrimSpace(string(item.Type))),
			Label:       strings.TrimSpace(item.Label),
			Key:         strings.TrimSpace(item.Key),
			Description: strings.TrimSpace(item.Description),
		})
	}
	input.Resources = normalized
	return input
}

func (input ResourceDefinitionSaveInput) Validate() error {
	normalized := input.Normalize()
	if err := validateSystemID(normalized.SystemID); err != nil {
		return err
	}
	if len(normalized.Resources) == 0 {
		return invalidInput("resources are required")
	}
	seen := map[string]struct{}{}
	for _, item := range normalized.Resources {
		if err := item.Validate(); err != nil {
			return err
		}
		identity := string(item.Type) + "/" + item.Key
		if _, ok := seen[identity]; ok {
			return invalidInput(fmt.Sprintf("duplicate resource definition %s", identity))
		}
		seen[identity] = struct{}{}
	}
	return nil
}

func (input ResourceDefinitionInput) Validate() error {
	if !input.Type.IsValid() {
		return invalidInput("resource type must be type, tag, or action")
	}
	if strings.TrimSpace(input.Key) == "" {
		return invalidInput("resource key is required")
	}
	if utf8.RuneCountInString(input.Key) > 15 {
		return invalidInput("resource key must be at most 15 characters")
	}
	if !resourceDefinitionKeyPattern.MatchString(input.Key) {
		return invalidInput("resource key must contain only lower-case letters, numbers, and underscores")
	}
	if strings.TrimSpace(input.Label) == "" {
		return invalidInput("resource label is required")
	}
	if utf8.RuneCountInString(input.Label) > 20 {
		return invalidInput("resource label must be at most 20 characters")
	}
	if utf8.RuneCountInString(input.Description) > 2000 {
		return invalidInput("resource description must be at most 2000 characters")
	}
	return nil
}

func (query ResourceDefinitionsQuery) Validate() error {
	return validateSystemID(strings.TrimSpace(query.SystemID))
}

func (query ResourceAttributesQuery) Validate() error {
	return validateSystemID(strings.TrimSpace(query.SystemID))
}

func (t ResourceDefinitionType) IsValid() bool {
	return t == ResourceDefinitionKindType || t == ResourceDefinitionKindTag || t == ResourceDefinitionKindAction
}

func (limits ResourceDefinitionLimits) Validate() error {
	if limits.Types <= 0 {
		return invalidInput("resource type limit must be greater than zero")
	}
	if limits.Actions <= 0 {
		return invalidInput("resource action limit must be greater than zero")
	}
	if limits.Tags <= 0 {
		return invalidInput("resource tag limit must be greater than zero")
	}
	return nil
}

func ValidateResourceDefinitionCounts(definitions []ResourceDefinition, limits ResourceDefinitionLimits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	counts := map[ResourceDefinitionType]int{}
	for _, definition := range definitions {
		counts[definition.Type]++
	}
	if counts[ResourceDefinitionKindType] > limits.Types {
		return invalidInput("resource type limit exceeded")
	}
	if counts[ResourceDefinitionKindAction] > limits.Actions {
		return invalidInput("resource action limit exceeded")
	}
	if counts[ResourceDefinitionKindTag] > limits.Tags {
		return invalidInput("resource tag limit exceeded")
	}
	return nil
}

func validateSystemID(systemID string) error {
	if strings.TrimSpace(systemID) == "" {
		return invalidInput("system id is required")
	}
	if strings.Contains(systemID, ".") || strings.IndexFunc(systemID, unicode.IsSpace) >= 0 {
		return invalidInput("system id must be a single subject token")
	}
	return nil
}
```

- [ ] **Step 4: Add resource attribute domain code**

Create `internal/domain/resource/resource_attribute.go`:

```go
package resource

type ResourceAttribute string

func NewResourceAttribute(action, tag, resourceType string) ResourceAttribute {
	return ResourceAttribute(action + "_" + tag + "_" + resourceType)
}
```

- [ ] **Step 5: Run domain tests and confirm pass**

Run:

```bash
go test ./internal/domain/resource -run 'ResourceDefinition|ResourceAttribute' -v
```

Expected: PASS.

- [ ] **Step 6: Commit domain contract**

```bash
git add internal/domain/resource/resource_definition.go internal/domain/resource/resource_attribute.go internal/domain/resource/validation_test.go
git commit -m "feat: add system resource definition domain model"
```

---

### Task 2: Function-Service Configuration

**Files:**

- Modify: `internal/function-service/config/config.go`
- Modify: `internal/function-service/config/config_test.go`
- Modify: `.env.example`

- [ ] **Step 1: Write failing config tests**

In `internal/function-service/config/config_test.go`, update `TestLoadReadsRequiredEnvironment` to set:

```go
t.Setenv("FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT", "4")
t.Setenv("FUNCTION_SERVICE_SYSTEM_RESOURCE_ACTION_LIMIT", "6")
t.Setenv("FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT", "21")
```

Then add assertions after `ResourceDeletedSubject`:

```go
if cfg.SystemResourceLimits.Type != 4 {
	t.Fatalf("SystemResourceLimits.Type = %d, want 4", cfg.SystemResourceLimits.Type)
}
if cfg.SystemResourceLimits.Action != 6 {
	t.Fatalf("SystemResourceLimits.Action = %d, want 6", cfg.SystemResourceLimits.Action)
}
if cfg.SystemResourceLimits.Tag != 21 {
	t.Fatalf("SystemResourceLimits.Tag = %d, want 21", cfg.SystemResourceLimits.Tag)
}
```

In `TestLoadAppliesOptionalDefaults`, add:

```go
if cfg.SystemResourceLimits.Type != 3 {
	t.Fatalf("SystemResourceLimits.Type = %d, want 3", cfg.SystemResourceLimits.Type)
}
if cfg.SystemResourceLimits.Action != 5 {
	t.Fatalf("SystemResourceLimits.Action = %d, want 5", cfg.SystemResourceLimits.Action)
}
if cfg.SystemResourceLimits.Tag != 20 {
	t.Fatalf("SystemResourceLimits.Tag = %d, want 20", cfg.SystemResourceLimits.Tag)
}
```

Append:

```go
func TestLoadRejectsInvalidSystemResourceLimits(t *testing.T) {
	t.Setenv("FUNCTION_SERVICE_HTTP_ADDR", ":8080")
	t.Setenv("FUNCTION_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("FUNCTION_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("FUNCTION_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_STREAM", "FUNCTION_RESOURCES")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_DURABLE", "function-service-resource-upserter")
	t.Setenv("FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT", "0")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}
```

- [ ] **Step 2: Run config tests and confirm failure**

Run:

```bash
go test ./internal/function-service/config -run 'SystemResourceLimits|LoadReadsRequiredEnvironment|LoadAppliesOptionalDefaults' -v
```

Expected: FAIL with `cfg.SystemResourceLimits undefined`.

- [ ] **Step 3: Add config fields, defaults, and validation**

In `internal/function-service/config/config.go`, add:

```go
type SystemResourceLimitsConfig struct {
	Type   int
	Action int
	Tag    int
}
```

Add this field to `Config`:

```go
SystemResourceLimits SystemResourceLimitsConfig
```

In `Load`, add defaults:

```go
v.SetDefault("FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT", 3)
v.SetDefault("FUNCTION_SERVICE_SYSTEM_RESOURCE_ACTION_LIMIT", 5)
v.SetDefault("FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT", 20)
```

Add this to the `cfg := Config{...}` literal:

```go
SystemResourceLimits: SystemResourceLimitsConfig{
	Type:   v.GetInt("FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT"),
	Action: v.GetInt("FUNCTION_SERVICE_SYSTEM_RESOURCE_ACTION_LIMIT"),
	Tag:    v.GetInt("FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT"),
},
```

Add validation after the fetch count check:

```go
if c.SystemResourceLimits.Type <= 0 {
	return fmt.Errorf("FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT must be greater than zero")
}
if c.SystemResourceLimits.Action <= 0 {
	return fmt.Errorf("FUNCTION_SERVICE_SYSTEM_RESOURCE_ACTION_LIMIT must be greater than zero")
}
if c.SystemResourceLimits.Tag <= 0 {
	return fmt.Errorf("FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT must be greater than zero")
}
```

- [ ] **Step 4: Add local environment defaults**

In `.env.example`, add these after `FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT`:

```env
FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT=3
FUNCTION_SERVICE_SYSTEM_RESOURCE_ACTION_LIMIT=5
FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT=20
```

- [ ] **Step 5: Run config tests and confirm pass**

Run:

```bash
go test ./internal/function-service/config -v
```

Expected: PASS.

- [ ] **Step 6: Commit config work**

```bash
git add internal/function-service/config/config.go internal/function-service/config/config_test.go .env.example
git commit -m "feat: configure system resource limits"
```

---

### Task 3: Transport Request And Response DTOs

**Files:**

- Create: `internal/function-service/transport/system_resource_request.go`
- Create: `internal/function-service/transport/system_resource_request_test.go`
- Create: `internal/function-service/transport/system_resource_response.go`
- Create: `internal/function-service/transport/system_resource_response_test.go`

- [ ] **Step 1: Write failing request tests**

Create `internal/function-service/transport/system_resource_request_test.go`:

```go
package transport

import (
	"errors"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func validSystemResourceRequestJSON() string {
	return `{
		"resources": [
			{"type": "action", "label": "Can Edit", "key": "can_edit", "description": "Allows editing."},
			{"type": "tag", "label": "Private", "key": "private"},
			{"type": "type", "label": "Repository", "key": "repo"}
		]
	}`
}

func TestDecodeSystemResourceSaveRequest(t *testing.T) {
	req, err := DecodeSystemResourceSaveRequest(strings.NewReader(validSystemResourceRequestJSON()))
	if err != nil {
		t.Fatalf("DecodeSystemResourceSaveRequest error = %v, want nil", err)
	}
	if len(req.Resources) != 3 {
		t.Fatalf("resources len = %d, want 3", len(req.Resources))
	}
	if req.Resources[0].Type != "action" || req.Resources[0].Key != "can_edit" {
		t.Fatalf("first resource = %+v, want action/can_edit", req.Resources[0])
	}
}

func TestDecodeSystemResourceSaveRequestRejectsInvalidJSON(t *testing.T) {
	if _, err := DecodeSystemResourceSaveRequest(strings.NewReader(`{"resources":`)); err == nil {
		t.Fatal("DecodeSystemResourceSaveRequest error = nil, want error")
	}
}

func TestSystemResourceSaveRequestToDomain(t *testing.T) {
	req, err := DecodeSystemResourceSaveRequest(strings.NewReader(validSystemResourceRequestJSON()))
	if err != nil {
		t.Fatalf("decode request: %v", err)
	}

	input, err := req.ToDomain("todo")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.SystemID != "todo" {
		t.Fatalf("SystemID = %q, want todo", input.SystemID)
	}
	if input.Resources[0].Type != resource.ResourceDefinitionKindAction {
		t.Fatalf("type = %q, want action", input.Resources[0].Type)
	}
	if input.Resources[0].Description != "Allows editing." {
		t.Fatalf("description = %q, want Allows editing.", input.Resources[0].Description)
	}
}

func TestSystemResourceSaveRequestToDomainReturnsDomainValidationError(t *testing.T) {
	req := SystemResourceSaveRequest{
		Resources: []SystemResourceRequest{{Type: "ACTION", Label: "Can Edit", Key: "can_edit"}},
	}

	_, err := req.ToDomain("todo")
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}
```

- [ ] **Step 2: Write failing response tests**

Create `internal/function-service/transport/system_resource_response_test.go`:

```go
package transport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func TestNewSystemResourcesResponse(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	response := NewSystemResourcesResponse([]resource.ResourceDefinition{{
		SystemID:    "todo",
		Type:        resource.ResourceDefinitionKindAction,
		Label:       "Can Edit",
		Key:         "can_edit",
		Description: "Allows editing.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}})

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, `"description":"Allows editing."`) {
		t.Fatalf("response missing description: %s", body)
	}
	if !strings.Contains(body, `"created_at":"2026-05-18T10:00:00Z"`) {
		t.Fatalf("response missing created_at: %s", body)
	}
}

func TestNewSystemResourcesResponseOmitsEmptyDescription(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	response := NewSystemResourcesResponse([]resource.ResourceDefinition{{
		SystemID:  "todo",
		Type:      resource.ResourceDefinitionKindTag,
		Label:     "Private",
		Key:       "private",
		CreatedAt: now,
		UpdatedAt: now,
	}})

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(data), "description") {
		t.Fatalf("response includes empty description: %s", data)
	}
}

func TestNewSystemResourceAttributesResponse(t *testing.T) {
	response := NewSystemResourceAttributesResponse([]resource.ResourceAttribute{
		resource.ResourceAttribute("can_edit_private_repo"),
	})

	if len(response.ResourceAttributes) != 1 || response.ResourceAttributes[0] != "can_edit_private_repo" {
		t.Fatalf("attributes = %#v, want can_edit_private_repo", response.ResourceAttributes)
	}
}
```

- [ ] **Step 3: Run transport tests and confirm failure**

Run:

```bash
go test ./internal/function-service/transport -run 'SystemResource' -v
```

Expected: FAIL with undefined request and response constructors.

- [ ] **Step 4: Add request DTO mapping**

Create `internal/function-service/transport/system_resource_request.go`:

```go
package transport

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type SystemResourceSaveRequest struct {
	Resources []SystemResourceRequest `json:"resources"`
}

type SystemResourceRequest struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Key         string `json:"key"`
	Description string `json:"description,omitempty"`
}

func DecodeSystemResourceSaveRequest(body io.Reader) (SystemResourceSaveRequest, error) {
	var request SystemResourceSaveRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return SystemResourceSaveRequest{}, fmt.Errorf("decode system resource request: %w", err)
	}
	return request, nil
}

func (request SystemResourceSaveRequest) ToDomain(systemID string) (resource.ResourceDefinitionSaveInput, error) {
	resources := make([]resource.ResourceDefinitionInput, 0, len(request.Resources))
	for _, item := range request.Resources {
		resources = append(resources, resource.ResourceDefinitionInput{
			Type:        resource.ResourceDefinitionType(item.Type),
			Label:       item.Label,
			Key:         item.Key,
			Description: item.Description,
		})
	}
	input := resource.ResourceDefinitionSaveInput{
		SystemID:  systemID,
		Resources: resources,
	}
	if err := input.Validate(); err != nil {
		return resource.ResourceDefinitionSaveInput{}, err
	}
	return input.Normalize(), nil
}
```

- [ ] **Step 5: Add response DTO mapping**

Create `internal/function-service/transport/system_resource_response.go`:

```go
package transport

import (
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type SystemResourcesResponse struct {
	Resources []SystemResourceResponse `json:"resources"`
}

type SystemResourceResponse struct {
	Type        resource.ResourceDefinitionType `json:"type"`
	Label       string                          `json:"label"`
	Key         string                          `json:"key"`
	Description string                          `json:"description,omitempty"`
	CreatedAt   time.Time                       `json:"created_at"`
	UpdatedAt   time.Time                       `json:"updated_at"`
}

type SystemResourceAttributesResponse struct {
	ResourceAttributes []string `json:"resource_attributes"`
}

func NewSystemResourcesResponse(definitions []resource.ResourceDefinition) SystemResourcesResponse {
	resources := make([]SystemResourceResponse, 0, len(definitions))
	for _, definition := range definitions {
		resources = append(resources, SystemResourceResponse{
			Type:        definition.Type,
			Label:       definition.Label,
			Key:         definition.Key,
			Description: definition.Description,
			CreatedAt:   definition.CreatedAt,
			UpdatedAt:   definition.UpdatedAt,
		})
	}
	return SystemResourcesResponse{Resources: resources}
}

func NewSystemResourceAttributesResponse(attributes []resource.ResourceAttribute) SystemResourceAttributesResponse {
	values := make([]string, 0, len(attributes))
	for _, attribute := range attributes {
		values = append(values, string(attribute))
	}
	return SystemResourceAttributesResponse{ResourceAttributes: values}
}
```

- [ ] **Step 6: Run transport tests and confirm pass**

Run:

```bash
go test ./internal/function-service/transport -run 'SystemResource' -v
```

Expected: PASS.

- [ ] **Step 7: Commit transport DTOs**

```bash
git add internal/function-service/transport/system_resource_request.go internal/function-service/transport/system_resource_request_test.go internal/function-service/transport/system_resource_response.go internal/function-service/transport/system_resource_response_test.go
git commit -m "feat: add system resource transport DTOs"
```

---

### Task 4: Service Workflow And Attribute Derivation

**Files:**

- Create: `internal/function-service/services/system_resource_service.go`
- Create: `internal/function-service/services/system_resource_service_test.go`

- [ ] **Step 1: Write failing service tests**

Create `internal/function-service/services/system_resource_service_test.go` with a fake repository and these tests:

```go
package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type fakeSystemResourceRepository struct {
	transactionCalls int
	existing         []resource.ResourceDefinition
	latest          []resource.ResourceDefinition
	saved           []resource.ResourceDefinition
	attributes      resource.ResourceAttributes
	attributesFound bool
	attributesSaved resource.ResourceAttributes
	err             error
}

func (f *fakeSystemResourceRepository) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	f.transactionCalls++
	return fn(ctx)
}

func (f *fakeSystemResourceRepository) ListResourceDefinitions(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.saved != nil {
		return append([]resource.ResourceDefinition(nil), f.latest...), nil
	}
	return append([]resource.ResourceDefinition(nil), f.existing...), nil
}

func (f *fakeSystemResourceRepository) UpsertResourceDefinitions(ctx context.Context, definitions []resource.ResourceDefinition) ([]resource.ResourceDefinition, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.saved = append([]resource.ResourceDefinition(nil), definitions...)
	if f.latest == nil {
		f.latest = append(append([]resource.ResourceDefinition(nil), f.existing...), definitions...)
	}
	return append([]resource.ResourceDefinition(nil), definitions...), nil
}

func (f *fakeSystemResourceRepository) UpsertResourceAttributes(ctx context.Context, attributes resource.ResourceAttributes) (resource.ResourceAttributes, error) {
	f.attributesSaved = attributes
	return attributes, nil
}

func (f *fakeSystemResourceRepository) GetResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) (resource.ResourceAttributes, bool, error) {
	if f.err != nil {
		return resource.ResourceAttributes{}, false, f.err
	}
	return f.attributes, f.attributesFound, nil
}

func TestSystemResourceServiceSaveSystemResources(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeSystemResourceRepository{
		existing: []resource.ResourceDefinition{{ID: "existing-1", SystemID: "todo", Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private", CreatedAt: now, UpdatedAt: now}},
		latest: []resource.ResourceDefinition{
			{ID: "new-action", SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit", CreatedAt: now, UpdatedAt: now},
			{ID: "existing-1", SystemID: "todo", Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private", CreatedAt: now, UpdatedAt: now},
			{ID: "new-type", SystemID: "todo", Type: resource.ResourceDefinitionKindType, Label: "Repository", Key: "repo", CreatedAt: now, UpdatedAt: now},
		},
	}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20},
		WithSystemResourceClock(func() time.Time { return now }),
		WithSystemResourceIDGenerator(sequenceIDs("new-action", "new-type", "attributes-1")),
	)

	saved, err := service.SaveSystemResources(context.Background(), resource.ResourceDefinitionSaveInput{
		SystemID: "todo",
		Resources: []resource.ResourceDefinitionInput{
			{Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit"},
			{Type: resource.ResourceDefinitionKindType, Label: "Repository", Key: "repo"},
		},
	})
	if err != nil {
		t.Fatalf("SaveSystemResources error = %v, want nil", err)
	}
	if repo.transactionCalls != 1 {
		t.Fatalf("transaction calls = %d, want 1", repo.transactionCalls)
	}
	if len(saved) != 2 || saved[0].Key != "can_edit" || saved[1].Key != "repo" {
		t.Fatalf("saved = %+v, want request order can_edit/repo", saved)
	}
	wantAttributes := []resource.ResourceAttribute{resource.ResourceAttribute("can_edit_private_repo")}
	if !reflect.DeepEqual(repo.attributesSaved.Values, wantAttributes) {
		t.Fatalf("attributes = %#v, want %#v", repo.attributesSaved.Values, wantAttributes)
	}
}

func TestSystemResourceServiceDoesNotWriteAttributesWhenIncomplete(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeSystemResourceRepository{}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20},
		WithSystemResourceClock(func() time.Time { return now }),
		WithSystemResourceIDGenerator(sequenceIDs("definition-1", "attributes-1")),
	)

	_, err := service.SaveSystemResources(context.Background(), resource.ResourceDefinitionSaveInput{
		SystemID:   "todo",
		Resources: []resource.ResourceDefinitionInput{{Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit"}},
	})
	if err != nil {
		t.Fatalf("SaveSystemResources error = %v, want nil", err)
	}
	if len(repo.attributesSaved.Values) != 0 {
		t.Fatalf("attributes = %#v, want none", repo.attributesSaved.Values)
	}
}

func TestSystemResourceServiceRejectsLimitViolation(t *testing.T) {
	repo := &fakeSystemResourceRepository{
		existing: []resource.ResourceDefinition{{SystemID: "todo", Type: resource.ResourceDefinitionKindType, Key: "repo"}},
	}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 1, Actions: 5, Tags: 20})

	_, err := service.SaveSystemResources(context.Background(), resource.ResourceDefinitionSaveInput{
		SystemID:   "todo",
		Resources: []resource.ResourceDefinitionInput{{Type: resource.ResourceDefinitionKindType, Label: "Page", Key: "page"}},
	})
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestSystemResourceServiceListAndGetAttributes(t *testing.T) {
	repo := &fakeSystemResourceRepository{
		existing: []resource.ResourceDefinition{{SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Key: "can_edit"}},
		attributes: resource.ResourceAttributes{
			SystemID: "todo",
			Values:   []resource.ResourceAttribute{resource.ResourceAttribute("can_edit_private_repo")},
		},
		attributesFound: true,
	}
	service := NewSystemResourceService(repo, resource.ResourceDefinitionLimits{Types: 3, Actions: 5, Tags: 20})

	definitions, err := service.ListSystemResources(context.Background(), resource.ResourceDefinitionsQuery{SystemID: "todo"})
	if err != nil {
		t.Fatalf("ListSystemResources error = %v, want nil", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("definitions len = %d, want 1", len(definitions))
	}
	attributes, err := service.GetSystemResourceAttributes(context.Background(), resource.ResourceAttributesQuery{SystemID: "todo"})
	if err != nil {
		t.Fatalf("GetSystemResourceAttributes error = %v, want nil", err)
	}
	if len(attributes) != 1 || attributes[0] != "can_edit_private_repo" {
		t.Fatalf("attributes = %#v, want can_edit_private_repo", attributes)
	}
}

func sequenceIDs(values ...string) func() string {
	index := 0
	return func() string {
		value := values[index]
		index++
		return value
	}
}
```

- [ ] **Step 2: Run service tests and confirm failure**

Run:

```bash
go test ./internal/function-service/services -run 'SystemResource' -v
```

Expected: FAIL with undefined `NewSystemResourceService`.

- [ ] **Step 3: Add service workflow**

Create `internal/function-service/services/system_resource_service.go` with the service interface and constructor:

```go
package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type SystemResourceRepository interface {
	RunInTransaction(ctx context.Context, fn func(context.Context) error) error
	ListResourceDefinitions(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error)
	UpsertResourceDefinitions(ctx context.Context, definitions []resource.ResourceDefinition) ([]resource.ResourceDefinition, error)
	UpsertResourceAttributes(ctx context.Context, attributes resource.ResourceAttributes) (resource.ResourceAttributes, error)
	GetResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) (resource.ResourceAttributes, bool, error)
}

type SystemResourceOption func(*SystemResourceService)

func WithSystemResourceClock(clock func() time.Time) SystemResourceOption {
	return func(s *SystemResourceService) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func WithSystemResourceIDGenerator(generator func() string) SystemResourceOption {
	return func(s *SystemResourceService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}

type SystemResourceService struct {
	repository  SystemResourceRepository
	limits      resource.ResourceDefinitionLimits
	clock       func() time.Time
	idGenerator func() string
}

func NewSystemResourceService(repository SystemResourceRepository, limits resource.ResourceDefinitionLimits, opts ...SystemResourceOption) *SystemResourceService {
	service := &SystemResourceService{
		repository:  repository,
		limits:      limits,
		clock:       func() time.Time { return time.Now().UTC() },
		idGenerator: uuid.NewString,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}
```

Add the save/list/get methods:

```go
func (s *SystemResourceService) SaveSystemResources(ctx context.Context, input resource.ResourceDefinitionSaveInput) ([]resource.ResourceDefinition, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if err := s.limits.Validate(); err != nil {
		return nil, err
	}

	normalized := input.Normalize()
	now := s.clock()
	definitions := make([]resource.ResourceDefinition, 0, len(normalized.Resources))
	for _, item := range normalized.Resources {
		definitions = append(definitions, resource.ResourceDefinition{
			ID:          s.idGenerator(),
			SystemID:    normalized.SystemID,
			Type:        item.Type,
			Label:       item.Label,
			Key:         item.Key,
			Description: item.Description,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	attributeID := s.idGenerator()

	var saved []resource.ResourceDefinition
	if err := s.repository.RunInTransaction(ctx, func(tx context.Context) error {
		existing, err := s.repository.ListResourceDefinitions(tx, resource.ResourceDefinitionsQuery{SystemID: normalized.SystemID})
		if err != nil {
			return fmt.Errorf("list existing system resources: %w", err)
		}
		merged := mergeResourceDefinitions(existing, definitions)
		if err := resource.ValidateResourceDefinitionCounts(merged, s.limits); err != nil {
			return err
		}
		saved, err = s.repository.UpsertResourceDefinitions(tx, definitions)
		if err != nil {
			return fmt.Errorf("upsert system resources: %w", err)
		}
		latest, err := s.repository.ListResourceDefinitions(tx, resource.ResourceDefinitionsQuery{SystemID: normalized.SystemID})
		if err != nil {
			return fmt.Errorf("list latest system resources: %w", err)
		}
		attributes := deriveResourceAttributes(latest)
		if len(attributes) == 0 {
			return nil
		}
		if _, err := s.repository.UpsertResourceAttributes(tx, resource.ResourceAttributes{
			ID:        attributeID,
			SystemID:  normalized.SystemID,
			Values:    attributes,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("upsert system resource attributes: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *SystemResourceService) ListSystemResources(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	definitions, err := s.repository.ListResourceDefinitions(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list system resources: %w", err)
	}
	return definitions, nil
}

func (s *SystemResourceService) GetSystemResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) ([]resource.ResourceAttribute, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	attributes, found, err := s.repository.GetResourceAttributes(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get system resource attributes: %w", err)
	}
	if !found {
		return []resource.ResourceAttribute{}, nil
	}
	return append([]resource.ResourceAttribute(nil), attributes.Values...), nil
}
```

Add helper functions in the same file:

```go
func mergeResourceDefinitions(existing, updates []resource.ResourceDefinition) []resource.ResourceDefinition {
	byIdentity := map[string]resource.ResourceDefinition{}
	for _, definition := range existing {
		byIdentity[resourceDefinitionIdentity(definition.Type, definition.Key)] = definition
	}
	for _, definition := range updates {
		byIdentity[resourceDefinitionIdentity(definition.Type, definition.Key)] = definition
	}
	merged := make([]resource.ResourceDefinition, 0, len(byIdentity))
	for _, definition := range byIdentity {
		merged = append(merged, definition)
	}
	return merged
}

func deriveResourceAttributes(definitions []resource.ResourceDefinition) []resource.ResourceAttribute {
	actions := resourceDefinitionKeys(definitions, resource.ResourceDefinitionKindAction)
	tags := resourceDefinitionKeys(definitions, resource.ResourceDefinitionKindTag)
	types := resourceDefinitionKeys(definitions, resource.ResourceDefinitionKindType)
	if len(actions) == 0 || len(tags) == 0 || len(types) == 0 {
		return nil
	}
	attributes := make([]resource.ResourceAttribute, 0, len(actions)*len(tags)*len(types))
	for _, action := range actions {
		for _, tag := range tags {
			for _, resourceType := range types {
				attributes = append(attributes, resource.NewResourceAttribute(action, tag, resourceType))
			}
		}
	}
	return attributes
}

func resourceDefinitionKeys(definitions []resource.ResourceDefinition, kind resource.ResourceDefinitionType) []string {
	keys := make([]string, 0)
	for _, definition := range definitions {
		if definition.Type == kind {
			keys = append(keys, definition.Key)
		}
	}
	sort.Strings(keys)
	return keys
}

func resourceDefinitionIdentity(kind resource.ResourceDefinitionType, key string) string {
	return string(kind) + "\x00" + key
}
```

- [ ] **Step 4: Run service tests and confirm pass**

Run:

```bash
go test ./internal/function-service/services -run 'SystemResource' -v
```

Expected: PASS.

- [ ] **Step 5: Commit service workflow**

```bash
git add internal/function-service/services/system_resource_service.go internal/function-service/services/system_resource_service_test.go
git commit -m "feat: add system resource service workflow"
```

---

### Task 5: MongoDB Repository And Transaction Runner

**Files:**

- Create: `internal/function-service/repositories/mongo_system_resource_repository.go`
- Create: `internal/function-service/repositories/mongo_system_resource_repository_test.go`

- [ ] **Step 1: Write repository helper tests**

Create `internal/function-service/repositories/mongo_system_resource_repository_test.go`:

```go
package repositories

import (
	"reflect"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestSystemResourceFilter(t *testing.T) {
	got := buildSystemResourceFilter("todo", resource.ResourceDefinitionKindAction, "can_edit")
	want := bson.M{"system_id": "todo", "type": resource.ResourceDefinitionKindAction, "key": "can_edit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestSystemResourceUpdateSetsDescriptionWhenPresent(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	doc := systemResourceDocument{
		ID:          "definition-1",
		SystemID:    "todo",
		Type:        resource.ResourceDefinitionKindAction,
		Label:       "Can Edit",
		Key:         "can_edit",
		Description: "Allows editing.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	update := buildSystemResourceUpdate(doc)
	set := update["$set"].(bson.M)
	if set["description"] != "Allows editing." {
		t.Fatalf("description = %#v, want Allows editing.", set["description"])
	}
	if _, ok := update["$unset"]; ok {
		t.Fatalf("update contains unset: %#v", update)
	}
}

func TestSystemResourceUpdateClearsDescriptionWhenEmpty(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	doc := systemResourceDocument{
		ID:        "definition-1",
		SystemID:  "todo",
		Type:      resource.ResourceDefinitionKindAction,
		Label:     "Can Edit",
		Key:       "can_edit",
		CreatedAt: now,
		UpdatedAt: now,
	}
	update := buildSystemResourceUpdate(doc)
	unset := update["$unset"].(bson.M)
	if _, ok := unset["description"]; !ok {
		t.Fatalf("unset = %#v, want description unset", unset)
	}
}

func TestSystemResourceIndexModelsAreUnique(t *testing.T) {
	models := systemResourceIndexModels()
	if len(models) != 2 {
		t.Fatalf("models len = %d, want 2", len(models))
	}
	for _, model := range models {
		opts := &options.IndexOptions{}
		for _, setter := range model.Options.List() {
			if err := setter(opts); err != nil {
				t.Fatalf("apply index option: %v", err)
			}
		}
		if opts.Unique == nil || !*opts.Unique {
			t.Fatalf("model %#v unique = %v, want true", model.Keys, opts.Unique)
		}
	}
}

func TestSystemResourceDocumentMapping(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	doc := systemResourceDocument{
		ID:          "definition-1",
		SystemID:    "todo",
		Type:        resource.ResourceDefinitionKindAction,
		Label:       "Can Edit",
		Key:         "can_edit",
		Description: "Allows editing.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	got := doc.toDomain()
	if got.ID != "definition-1" || got.SystemID != "todo" || got.Type != resource.ResourceDefinitionKindAction || got.Key != "can_edit" {
		t.Fatalf("domain = %+v, want definition-1/todo/action/can_edit", got)
	}
}

func TestSystemResourceAttributesDocumentMapping(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	doc := systemResourceAttributesDocument{
		ID:                 "attributes-1",
		SystemID:           "todo",
		ResourceAttributes: []string{"can_edit_private_repo"},
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	got := doc.toDomain()
	if got.Values[0] != resource.ResourceAttribute("can_edit_private_repo") {
		t.Fatalf("attributes = %#v, want can_edit_private_repo", got.Values)
	}
}
```

- [ ] **Step 2: Run repository tests and confirm failure**

Run:

```bash
go test ./internal/function-service/repositories -run 'SystemResource' -v
```

Expected: FAIL with undefined repository helpers.

- [ ] **Step 3: Add repository implementation**

Create `internal/function-service/repositories/mongo_system_resource_repository.go` with:

```go
package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	systemResourcesCollectionName          = "system_resources"
	systemResourceAttributesCollectionName = "system_resource_attributes"
)

type MongoSystemResourceRepository struct {
	client               *mongo.Client
	resourcesCollection  *mongo.Collection
	attributesCollection *mongo.Collection
}

type systemResourceDocument struct {
	ID          string                          `bson:"_id"`
	SystemID    string                          `bson:"system_id"`
	Type        resource.ResourceDefinitionType `bson:"type"`
	Label       string                          `bson:"label"`
	Key         string                          `bson:"key"`
	Description string                          `bson:"description,omitempty"`
	CreatedAt   time.Time                       `bson:"created_at"`
	UpdatedAt   time.Time                       `bson:"updated_at"`
}

type systemResourceAttributesDocument struct {
	ID                 string    `bson:"_id"`
	SystemID           string    `bson:"system_id"`
	ResourceAttributes []string  `bson:"resource_attributes"`
	CreatedAt          time.Time `bson:"created_at"`
	UpdatedAt          time.Time `bson:"updated_at"`
}

func NewMongoSystemResourceRepository(db *mongo.Database) *MongoSystemResourceRepository {
	return &MongoSystemResourceRepository{
		client:               db.Client(),
		resourcesCollection:  db.Collection(systemResourcesCollectionName),
		attributesCollection: db.Collection(systemResourceAttributesCollectionName),
	}
}

func (r *MongoSystemResourceRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.resourcesCollection.Indexes().CreateOne(ctx, systemResourceIndexModels()[0]); err != nil {
		return fmt.Errorf("create system_resources index: %w", err)
	}
	if _, err := r.attributesCollection.Indexes().CreateOne(ctx, systemResourceIndexModels()[1]); err != nil {
		return fmt.Errorf("create system_resource_attributes index: %w", err)
	}
	return nil
}

func (r *MongoSystemResourceRepository) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf("start mongo session: %w", err)
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		return nil, fn(tx)
	})
	if err != nil {
		return fmt.Errorf("run mongo transaction: %w", err)
	}
	return nil
}
```

Add read and write methods in the same file:

```go
func (r *MongoSystemResourceRepository) ListResourceDefinitions(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error) {
	cursor, err := r.resourcesCollection.Find(ctx,
		bson.M{"system_id": query.SystemID},
		options.Find().SetSort(bson.D{{Key: "type", Value: 1}, {Key: "key", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find system resources: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()
	var docs []systemResourceDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode system resources: %w", err)
	}
	definitions := make([]resource.ResourceDefinition, 0, len(docs))
	for _, doc := range docs {
		definitions = append(definitions, doc.toDomain())
	}
	return definitions, nil
}

func (r *MongoSystemResourceRepository) UpsertResourceDefinitions(ctx context.Context, definitions []resource.ResourceDefinition) ([]resource.ResourceDefinition, error) {
	saved := make([]resource.ResourceDefinition, 0, len(definitions))
	for _, definition := range definitions {
		doc := newSystemResourceDocument(definition)
		if _, err := r.resourcesCollection.UpdateOne(ctx, buildSystemResourceFilter(doc.SystemID, doc.Type, doc.Key), buildSystemResourceUpdate(doc), options.UpdateOne().SetUpsert(true)); err != nil {
			return nil, fmt.Errorf("upsert system resource: %w", err)
		}
		var persisted systemResourceDocument
		if err := r.resourcesCollection.FindOne(ctx, buildSystemResourceFilter(doc.SystemID, doc.Type, doc.Key)).Decode(&persisted); err != nil {
			return nil, fmt.Errorf("find upserted system resource: %w", err)
		}
		saved = append(saved, persisted.toDomain())
	}
	return saved, nil
}

func (r *MongoSystemResourceRepository) GetResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) (resource.ResourceAttributes, bool, error) {
	var doc systemResourceAttributesDocument
	if err := r.attributesCollection.FindOne(ctx, bson.M{"system_id": query.SystemID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return resource.ResourceAttributes{}, false, nil
		}
		return resource.ResourceAttributes{}, false, fmt.Errorf("find system resource attributes: %w", err)
	}
	return doc.toDomain(), true, nil
}

func (r *MongoSystemResourceRepository) UpsertResourceAttributes(ctx context.Context, attributes resource.ResourceAttributes) (resource.ResourceAttributes, error) {
	doc := newSystemResourceAttributesDocument(attributes)
	if _, err := r.attributesCollection.UpdateOne(ctx, bson.M{"system_id": doc.SystemID}, buildSystemResourceAttributesUpdate(doc), options.UpdateOne().SetUpsert(true)); err != nil {
		return resource.ResourceAttributes{}, fmt.Errorf("upsert system resource attributes: %w", err)
	}
	var persisted systemResourceAttributesDocument
	if err := r.attributesCollection.FindOne(ctx, bson.M{"system_id": doc.SystemID}).Decode(&persisted); err != nil {
		return resource.ResourceAttributes{}, fmt.Errorf("find upserted system resource attributes: %w", err)
	}
	return persisted.toDomain(), nil
}
```

Add helper functions:

```go
func systemResourceIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "system_id", Value: 1}, {Key: "type", Value: 1}, {Key: "key", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "system_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}
}

func buildSystemResourceFilter(systemID string, kind resource.ResourceDefinitionType, key string) bson.M {
	return bson.M{"system_id": systemID, "type": kind, "key": key}
}

func buildSystemResourceUpdate(doc systemResourceDocument) bson.M {
	set := bson.M{"label": doc.Label, "updated_at": doc.UpdatedAt}
	update := bson.M{
		"$set": set,
		"$setOnInsert": bson.M{
			"_id":        doc.ID,
			"system_id":  doc.SystemID,
			"type":       doc.Type,
			"key":        doc.Key,
			"created_at": doc.CreatedAt,
		},
	}
	if doc.Description == "" {
		update["$unset"] = bson.M{"description": ""}
	} else {
		set["description"] = doc.Description
	}
	return update
}

func buildSystemResourceAttributesUpdate(doc systemResourceAttributesDocument) bson.M {
	return bson.M{
		"$set": bson.M{
			"resource_attributes": append([]string(nil), doc.ResourceAttributes...),
			"updated_at":          doc.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"_id":        doc.ID,
			"system_id":  doc.SystemID,
			"created_at": doc.CreatedAt,
		},
	}
}
```

Add document mapping helpers:

```go
func newSystemResourceDocument(definition resource.ResourceDefinition) systemResourceDocument {
	return systemResourceDocument{
		ID:          definition.ID,
		SystemID:    definition.SystemID,
		Type:        definition.Type,
		Label:       definition.Label,
		Key:         definition.Key,
		Description: definition.Description,
		CreatedAt:   definition.CreatedAt,
		UpdatedAt:   definition.UpdatedAt,
	}
}

func (d systemResourceDocument) toDomain() resource.ResourceDefinition {
	return resource.ResourceDefinition{
		ID:          d.ID,
		SystemID:    d.SystemID,
		Type:        d.Type,
		Label:       d.Label,
		Key:         d.Key,
		Description: d.Description,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func newSystemResourceAttributesDocument(attributes resource.ResourceAttributes) systemResourceAttributesDocument {
	values := make([]string, 0, len(attributes.Values))
	for _, value := range attributes.Values {
		values = append(values, string(value))
	}
	return systemResourceAttributesDocument{
		ID:                 attributes.ID,
		SystemID:           attributes.SystemID,
		ResourceAttributes: values,
		CreatedAt:          attributes.CreatedAt,
		UpdatedAt:          attributes.UpdatedAt,
	}
}

func (d systemResourceAttributesDocument) toDomain() resource.ResourceAttributes {
	values := make([]resource.ResourceAttribute, 0, len(d.ResourceAttributes))
	for _, value := range d.ResourceAttributes {
		values = append(values, resource.ResourceAttribute(value))
	}
	return resource.ResourceAttributes{
		ID:        d.ID,
		SystemID:  d.SystemID,
		Values:    values,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}
```

- [ ] **Step 4: Run repository tests and confirm pass**

Run:

```bash
go test ./internal/function-service/repositories -run 'SystemResource' -v
```

Expected: PASS.

- [ ] **Step 5: Commit repository**

```bash
git add internal/function-service/repositories/mongo_system_resource_repository.go internal/function-service/repositories/mongo_system_resource_repository_test.go
git commit -m "feat: persist system resource definitions"
```

---

### Task 6: HTTP Handlers And Routes

**Files:**

- Create: `internal/function-service/handlers/system_resource_handler.go`
- Create: `internal/function-service/handlers/system_resource_handler_test.go`

- [ ] **Step 1: Write failing handler tests**

Create `internal/function-service/handlers/system_resource_handler_test.go`:

```go
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/labstack/echo/v5"
)

type fakeHTTPSystemResourceService struct {
	saveInput  resource.ResourceDefinitionSaveInput
	saveResult []resource.ResourceDefinition
	saveErr    error
	listQuery  resource.ResourceDefinitionsQuery
	listResult []resource.ResourceDefinition
	listErr    error
	attrQuery  resource.ResourceAttributesQuery
	attrs      []resource.ResourceAttribute
	attrErr    error
}

func (f *fakeHTTPSystemResourceService) SaveSystemResources(ctx context.Context, input resource.ResourceDefinitionSaveInput) ([]resource.ResourceDefinition, error) {
	f.saveInput = input
	if f.saveErr != nil {
		return nil, f.saveErr
	}
	return f.saveResult, nil
}

func (f *fakeHTTPSystemResourceService) ListSystemResources(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error) {
	f.listQuery = query
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}

func (f *fakeHTTPSystemResourceService) GetSystemResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) ([]resource.ResourceAttribute, error) {
	f.attrQuery = query
	if f.attrErr != nil {
		return nil, f.attrErr
	}
	return f.attrs, nil
}

func TestSystemResourceHandlerSaveSystemResources(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	service := &fakeHTTPSystemResourceService{
		saveResult: []resource.ResourceDefinition{{SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit", CreatedAt: now, UpdatedAt: now}},
	}
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(service, newTestLogger()))
	body := bytes.NewBufferString(`{"resources":[{"type":"action","label":"Can Edit","key":"can_edit"}]}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/systems/todo/resources", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.saveInput.SystemID != "todo" || service.saveInput.Resources[0].Key != "can_edit" {
		t.Fatalf("input = %+v, want todo/can_edit", service.saveInput)
	}
}

func TestSystemResourceHandlerRejectsInvalidJSON(t *testing.T) {
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(&fakeHTTPSystemResourceService{}, newTestLogger()))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/systems/todo/resources", bytes.NewBufferString(`{"resources":`))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSystemResourceHandlerListSystemResources(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	service := &fakeHTTPSystemResourceService{
		listResult: []resource.ResourceDefinition{{SystemID: "todo", Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private", CreatedAt: now, UpdatedAt: now}},
	}
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(service, newTestLogger()))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/systems/todo/resources", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.listQuery.SystemID != "todo" {
		t.Fatalf("query = %+v, want todo", service.listQuery)
	}
}

func TestSystemResourceHandlerGetAttributesEmpty(t *testing.T) {
	service := &fakeHTTPSystemResourceService{}
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(service, newTestLogger()))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/systems/todo/resource-attributes", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response map[string][]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response["resource_attributes"]) != 0 {
		t.Fatalf("attributes = %#v, want empty", response["resource_attributes"])
	}
}

func TestSystemResourceHandlerServiceFailure(t *testing.T) {
	service := &fakeHTTPSystemResourceService{listErr: errors.New("database unavailable")}
	e := echo.New()
	RegisterSystemResourceRoutes(e, NewSystemResourceHandler(service, newTestLogger()))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/systems/todo/resources", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
```

- [ ] **Step 2: Run handler tests and confirm failure**

Run:

```bash
go test ./internal/function-service/handlers -run 'SystemResource' -v
```

Expected: FAIL with undefined handler symbols.

- [ ] **Step 3: Add system resource handler**

Create `internal/function-service/handlers/system_resource_handler.go`:

```go
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/http/exception"
	"github.com/labstack/echo/v5"
)

type HTTPSystemResourceService interface {
	SaveSystemResources(ctx context.Context, input resource.ResourceDefinitionSaveInput) ([]resource.ResourceDefinition, error)
	ListSystemResources(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error)
	GetSystemResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) ([]resource.ResourceAttribute, error)
}

type SystemResourceHandler struct {
	service HTTPSystemResourceService
	logger  *slog.Logger
}

func NewSystemResourceHandler(service HTTPSystemResourceService, logger *slog.Logger) *SystemResourceHandler {
	return &SystemResourceHandler{service: service, logger: logger}
}

func RegisterSystemResourceRoutes(e *echo.Echo, handler *SystemResourceHandler) {
	e.POST("/api/v1/systems/:system_id/resources", handler.SaveSystemResources)
	e.GET("/api/v1/systems/:system_id/resources", handler.ListSystemResources)
	e.GET("/api/v1/systems/:system_id/resource-attributes", handler.GetSystemResourceAttributes)
}

func (h *SystemResourceHandler) SaveSystemResources(c *echo.Context) error {
	request, err := transport.DecodeSystemResourceSaveRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(c.Param("system_id"))
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}
	definitions, err := h.service.SaveSystemResources(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to save system resources", "err", err, "system_id", c.Param("system_id"))
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, transport.NewSystemResourcesResponse(definitions))
}

func (h *SystemResourceHandler) ListSystemResources(c *echo.Context) error {
	definitions, err := h.service.ListSystemResources(c.Request().Context(), resource.ResourceDefinitionsQuery{SystemID: c.Param("system_id")})
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to list system resources", "err", err, "system_id", c.Param("system_id"))
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, transport.NewSystemResourcesResponse(definitions))
}

func (h *SystemResourceHandler) GetSystemResourceAttributes(c *echo.Context) error {
	attributes, err := h.service.GetSystemResourceAttributes(c.Request().Context(), resource.ResourceAttributesQuery{SystemID: c.Param("system_id")})
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to get system resource attributes", "err", err, "system_id", c.Param("system_id"))
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, transport.NewSystemResourceAttributesResponse(attributes))
}
```

- [ ] **Step 4: Run handler tests and confirm pass**

Run:

```bash
go test ./internal/function-service/handlers -run 'SystemResource' -v
```

Expected: PASS.

- [ ] **Step 5: Commit handler routes**

```bash
git add internal/function-service/handlers/system_resource_handler.go internal/function-service/handlers/system_resource_handler_test.go
git commit -m "feat: add system resource HTTP handlers"
```

---

### Task 7: Main Wiring And API Examples

**Files:**

- Modify: `cmd/function-service/main.go`
- Create: `examples/api/system_resources.http`

- [ ] **Step 1: Wire repository, service, indexes, and routes**

In `cmd/function-service/main.go`, add `internal/domain/resource` to imports:

```go
"github.com/hao0731/workspace-permission-management/internal/domain/resource"
```

After `permissionRepository.EnsureIndexes(ctx)`, add:

```go
systemResourceRepository := repositories.NewMongoSystemResourceRepository(db)
if ensureIndexErr := systemResourceRepository.EnsureIndexes(ctx); ensureIndexErr != nil {
	return ensureIndexErr
}
```

After `permissionService := services.NewPermissionService(permissionRepository)`, add:

```go
systemResourceService := services.NewSystemResourceService(systemResourceRepository, resource.ResourceDefinitionLimits{
	Types:   cfg.SystemResourceLimits.Type,
	Actions: cfg.SystemResourceLimits.Action,
	Tags:    cfg.SystemResourceLimits.Tag,
})
```

After `handlers.RegisterPermissionRoutes(...)`, add:

```go
handlers.RegisterSystemResourceRoutes(e, handlers.NewSystemResourceHandler(systemResourceService, logger))
```

- [ ] **Step 2: Run compile test for main package**

Run:

```bash
go test ./cmd/function-service -v
```

Expected: PASS.

- [ ] **Step 3: Add REST Client examples**

Create `examples/api/system_resources.http`:

```http
@baseUrl = http://localhost:8080
@systemId = todo

### Save system resource definitions
POST {{baseUrl}}/api/v1/systems/{{systemId}}/resources
Content-Type: application/json

{
  "resources": [
    {
      "type": "action",
      "label": "Can Edit",
      "key": "can_edit",
      "description": "Allows editing resources."
    },
    {
      "type": "action",
      "label": "Can View",
      "key": "can_view"
    },
    {
      "type": "tag",
      "label": "Private",
      "key": "private"
    },
    {
      "type": "tag",
      "label": "Public",
      "key": "public"
    },
    {
      "type": "type",
      "label": "Repository",
      "key": "repo"
    }
  ]
}

### List system resource definitions
GET {{baseUrl}}/api/v1/systems/{{systemId}}/resources

### Get derived system resource attributes
GET {{baseUrl}}/api/v1/systems/{{systemId}}/resource-attributes

### Invalid key returns 400
POST {{baseUrl}}/api/v1/systems/{{systemId}}/resources
Content-Type: application/json

{
  "resources": [
    {
      "type": "action",
      "label": "Can Edit",
      "key": "Can-Edit"
    }
  ]
}

### Duplicate request key returns 400
POST {{baseUrl}}/api/v1/systems/{{systemId}}/resources
Content-Type: application/json

{
  "resources": [
    {
      "type": "tag",
      "label": "Private",
      "key": "private"
    },
    {
      "type": "tag",
      "label": "Private Copy",
      "key": "private"
    }
  ]
}

### Too many resource types returns 400 with default limit
POST {{baseUrl}}/api/v1/systems/{{systemId}}/resources
Content-Type: application/json

{
  "resources": [
    {"type": "type", "label": "Repo", "key": "repo"},
    {"type": "type", "label": "Page", "key": "page"},
    {"type": "type", "label": "Issue", "key": "issue"},
    {"type": "type", "label": "Build", "key": "build"}
  ]
}
```

- [ ] **Step 4: Run focused function-service tests**

Run:

```bash
go test ./internal/domain/resource ./internal/function-service/config ./internal/function-service/transport ./internal/function-service/services ./internal/function-service/repositories ./internal/function-service/handlers ./cmd/function-service -v
```

Expected: PASS.

- [ ] **Step 5: Commit wiring and examples**

```bash
git add cmd/function-service/main.go examples/api/system_resources.http
git commit -m "feat: wire system resource API"
```

---

### Task 8: Full Verification

**Files:**

- No new files.

- [ ] **Step 1: Run full test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run vet after transaction and startup wiring changes**

Run:

```bash
go vet ./...
```

Expected: PASS.

- [ ] **Step 3: Check API example file exists**

Run:

```bash
test -f examples/api/system_resources.http
```

Expected: exit code `0`.

- [ ] **Step 4: Check no stale systemresource package references remain**

Run:

```bash
rg -n "internal/domain/systemresource|systemresource" internal docs/designs examples
```

Expected: no matches.

- [ ] **Step 5: Inspect final diff**

Run:

```bash
git diff --stat HEAD
git status --short
```

Expected: only intentional changes are present if implementation tasks were not committed one by one; otherwise the worktree is clean.

## Implementation Notes

- MongoDB transactions require a replica set. Repository unit tests in this plan avoid live MongoDB transactions; manual or integration verification should run against an environment that supports transactions before deployment.
- Generate resource definition IDs and the resource attributes document ID before entering `WithTransaction`; the MongoDB driver can retry transaction callbacks.
- Do not expose `_id` in the system resource API response.
- Keep `description,omitempty` in responses so missing or cleared descriptions are omitted.
- Keep `system_id` as string and do not parse it as a UUID.
- Leave existing workspace-scoped `function_key` routes unchanged.

## Plan Self-Review

Spec coverage:

- POST, GET resources, and GET attributes endpoints are covered by Tasks 3, 4, 6, and 7.
- `system_resources` and `system_resource_attributes` schemas and indexes are covered by Task 5.
- Configurable limits are covered by Task 2 and service count checks in Task 4.
- Partial upsert, request-order response, timestamp preservation, description clearing, and attribute derivation are covered by Tasks 4 and 5.
- `.http` examples and verification commands are covered by Tasks 7 and 8.

Placeholder scan:

- No placeholder sections remain in this plan.

Type consistency:

- Domain type names use `resource.ResourceDefinition`, `resource.ResourceDefinitionInput`, `resource.ResourceDefinitionSaveInput`, `resource.ResourceDefinitionsQuery`, `resource.ResourceAttributesQuery`, `resource.ResourceDefinitionLimits`, `resource.ResourceAttributes`, and `resource.ResourceAttribute`.
- Service, transport, handler, and repository snippets consistently use those type names.
