# Group Service Individual Member Mutations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `POST`, `PATCH`, and `DELETE` APIs for explicit individual members on `group-service`.

**Architecture:** Extend the existing `group-service` backend layers without introducing a new service boundary. Domain owns member mutation inputs and validation, transport owns JSON DTOs, services own ID/time generation and error preservation, repositories own MongoDB transactions and active-document filters, and handlers stay thin with HTTP status mapping.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, `log/slog`, `internal/shared/http/exception`, standard `encoding/json`, standard `testing`.

---

## Source Designs

Primary source designs:

- [../../designs/group-service.md](../../designs/group-service.md)
- [../../designs/group-service-individual-members.md](../../designs/group-service-individual-members.md)

Related source design:

- [../../designs/group-service-group.md](../../designs/group-service-group.md)

Applicable policies:

- [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend work keeps handlers thin, transport DTOs separate from domain models, domain independent from Echo and MongoDB, services independent from transport and database drivers, and repositories isolated around MongoDB mechanics.
- REST API contracts, error shapes, MongoDB documents, and indexes are explicit contracts requiring tests and REST Client examples.
- Implementation plans live under `docs/plans/active/` and link back to source designs.

## Scope

Implement:

- `POST /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members`
  - Request body: `{ "individual_members": [{ "nt_account": string, "expiration_date": RFC3339 timestamp }] }`
  - Response: `201 Created` with `{ "members": [{ "nt_account": string, "expiration_date": RFC3339 timestamp }] }`
  - Missing active group: `404 Not Found`
  - Duplicate account in request or active group: `409 Conflict`
- `PATCH /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account`
  - Request body: `{ "expiration_date": RFC3339 timestamp }`
  - Response: `204 No Content`
  - Missing active group or active member: `404 Not Found`
- `DELETE /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account`
  - Response: `204 No Content`
  - Missing active group or active member: `204 No Content`

Do not implement:

- Member restore, hard-delete, history, search, or bulk replacement.
- Workspace existence validation.
- Dynamic grouping-rule evaluation.
- NATS, JetStream, CloudEvents, or frontend changes.

## File Structure and Responsibilities

- Modify: `internal/domain/group/errors.go`
  - Add `ErrDuplicateMember`.
- Modify: `internal/domain/group/group.go`
  - Add mutation input types and normalization helpers.
- Modify: `internal/domain/group/validation.go`
  - Add add/update/delete member validation and duplicate-member conflict validation for add requests.
- Modify: `internal/domain/group/validation_test.go`
  - Add domain tests for add/update/delete inputs.
- Modify: `internal/group-service/transport/group_request.go`
  - Add request DTOs, decoders, and DTO-to-domain mapping for add and update member APIs.
- Modify: `internal/group-service/transport/group_request_test.go`
  - Add transport decode and mapping tests.
- Modify: `internal/group-service/transport/group_response.go`
  - Add `{ "members": [{ "nt_account": string, "expiration_date": RFC3339 timestamp }] }` response mapping for add member.
- Modify: `internal/group-service/transport/group_response_test.go`
  - Add response mapping and metadata omission tests.
- Modify: `internal/group-service/services/group_service.go`
  - Extend the repository interface and add service methods for add/update/delete member APIs.
- Modify: `internal/group-service/services/group_service_test.go`
  - Add service workflow, validation, and error preservation tests.
- Modify: `internal/group-service/repositories/mongo_group_repository.go`
  - Add transaction-backed member add/update/delete repository methods and duplicate-member error mapping.
- Modify: `internal/group-service/repositories/mongo_group_repository_test.go`
  - Add repository filter, duplicate mapping, and integration tests.
- Modify: `internal/group-service/handlers/group_handler.go`
  - Register routes and add handler methods with HTTP status mapping.
- Modify: `internal/group-service/handlers/group_handler_test.go`
  - Add route, body, and error mapping tests.
- Modify: `examples/api/groups.http`
  - Add executable REST Client examples for member add, update, delete, and validation/conflict cases.

---

### Task 1: Domain Contracts for Individual Member Mutations

**Files:**

- Modify: `internal/domain/group/errors.go`
- Modify: `internal/domain/group/group.go`
- Modify: `internal/domain/group/validation.go`
- Modify: `internal/domain/group/validation_test.go`

- [ ] **Step 1: Write failing domain tests**

Append this test code to `internal/domain/group/validation_test.go`:

```go
func TestAddIndividualMembersInputValidate(t *testing.T) {
	input := AddIndividualMembersInput{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		IndividualMembers: []IndividualMember{{
			NTAccount:      " user2 ",
			ExpirationDate: futureTime(),
		}},
	}.Normalize()

	if err := input.Validate(validationNow(), WithMaxIndividualMembers(1)); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" {
		t.Fatalf("identity = %q/%q, want trimmed values", input.WorkspaceID, input.GroupID)
	}
	if input.IndividualMembers[0].NTAccount != "user2" {
		t.Fatalf("NTAccount = %q, want user2", input.IndividualMembers[0].NTAccount)
	}
}

func TestAddIndividualMembersInputRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		input       AddIndividualMembersInput
		wantError   error
		wantMessage string
	}{
		{
			name:        "blank workspace id",
			input:       AddIndividualMembersInput{WorkspaceID: " ", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: futureTime()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "workspace id is required",
		},
		{
			name:        "blank group id",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: " ", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: futureTime()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "group id is required",
		},
		{
			name:        "empty members",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1"},
			wantError:   ErrInvalidInput,
			wantMessage: "individual members are required",
		},
		{
			name:        "blank account",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: " ", ExpirationDate: futureTime()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "individual member nt account is required",
		},
		{
			name:        "duplicate account",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: futureTime()}, {NTAccount: "user1", ExpirationDate: futureTime()}}},
			wantError:   ErrDuplicateMember,
			wantMessage: "duplicate individual member nt account",
		},
		{
			name:        "missing expiration",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1"}}},
			wantError:   ErrInvalidInput,
			wantMessage: "individual member expiration date is required",
		},
		{
			name:        "past expiration",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: validationNow()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "individual member expiration date must be in the future",
		},
		{
			name:        "limit exceeded",
			input:       AddIndividualMembersInput{WorkspaceID: "workspace-1", GroupID: "group-1", IndividualMembers: []IndividualMember{{NTAccount: "user1", ExpirationDate: futureTime()}, {NTAccount: "user2", ExpirationDate: futureTime()}}},
			wantError:   ErrInvalidInput,
			wantMessage: "individual members must not exceed 1 items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Normalize().Validate(validationNow(), WithMaxIndividualMembers(1))
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("error = %v, want %v", err, tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("error = %q, want message containing %q", err.Error(), tt.wantMessage)
			}
		})
	}
}

func TestUpdateIndividualMemberExpirationInputValidate(t *testing.T) {
	input := UpdateIndividualMemberExpirationInput{
		WorkspaceID:    " workspace-1 ",
		GroupID:        " group-1 ",
		NTAccount:      " user2 ",
		ExpirationDate: futureTime(),
	}.Normalize()

	if err := input.Validate(validationNow()); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" || input.NTAccount != "user2" {
		t.Fatalf("input = %+v, want trimmed identity and account", input)
	}
}

func TestUpdateIndividualMemberExpirationInputRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		input       UpdateIndividualMemberExpirationInput
		wantMessage string
	}{
		{
			name:        "blank account",
			input:       UpdateIndividualMemberExpirationInput{WorkspaceID: "workspace-1", GroupID: "group-1", NTAccount: " ", ExpirationDate: futureTime()},
			wantMessage: "individual member nt account is required",
		},
		{
			name:        "missing expiration",
			input:       UpdateIndividualMemberExpirationInput{WorkspaceID: "workspace-1", GroupID: "group-1", NTAccount: "user1"},
			wantMessage: "individual member expiration date is required",
		},
		{
			name:        "past expiration",
			input:       UpdateIndividualMemberExpirationInput{WorkspaceID: "workspace-1", GroupID: "group-1", NTAccount: "user1", ExpirationDate: validationNow()},
			wantMessage: "individual member expiration date must be in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireInvalidInput(t, tt.input.Normalize().Validate(validationNow()), tt.wantMessage)
		})
	}
}

func TestDeleteIndividualMemberInputValidate(t *testing.T) {
	input := DeleteIndividualMemberInput{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		NTAccount:   " user2 ",
	}.Normalize()

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" || input.NTAccount != "user2" {
		t.Fatalf("input = %+v, want trimmed identity and account", input)
	}
}

func TestDeleteIndividualMemberInputRejectsBlankAccount(t *testing.T) {
	err := DeleteIndividualMemberInput{WorkspaceID: "workspace-1", GroupID: "group-1", NTAccount: " "}.Normalize().Validate()
	requireInvalidInput(t, err, "individual member nt account is required")
}
```

- [ ] **Step 2: Run the focused domain tests and confirm they fail**

Run:

```bash
go test ./internal/domain/group -run 'Test(AddIndividualMembersInput|UpdateIndividualMemberExpirationInput|DeleteIndividualMemberInput)' -v
```

Expected: FAIL because `AddIndividualMembersInput`, `UpdateIndividualMemberExpirationInput`, `DeleteIndividualMemberInput`, and `ErrDuplicateMember` are undefined.

- [ ] **Step 3: Add the domain error and input types**

Update `internal/domain/group/errors.go`:

```go
var (
	ErrInvalidInput     = errors.New("invalid group input")
	ErrDuplicateName    = errors.New("duplicate group name")
	ErrDuplicateMember  = errors.New("duplicate group individual member")
	ErrNotFound         = errors.New("group not found")
)
```

Append these types to `internal/domain/group/group.go` after `ListIndividualMembersQuery`:

```go
type AddIndividualMembersInput struct {
	WorkspaceID       string
	GroupID           string
	IndividualMembers []IndividualMember
}

type UpdateIndividualMemberExpirationInput struct {
	WorkspaceID    string
	GroupID        string
	NTAccount      string
	ExpirationDate time.Time
}

type DeleteIndividualMemberInput struct {
	WorkspaceID string
	GroupID     string
	NTAccount   string
}
```

Append these normalization methods to `internal/domain/group/group.go`:

```go
func (input AddIndividualMembersInput) Normalize() AddIndividualMembersInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	for i := range input.IndividualMembers {
		input.IndividualMembers[i] = input.IndividualMembers[i].Normalize()
	}
	return input
}

func (input UpdateIndividualMemberExpirationInput) Normalize() UpdateIndividualMemberExpirationInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	input.NTAccount = strings.TrimSpace(input.NTAccount)
	return input
}

func (input DeleteIndividualMemberInput) Normalize() DeleteIndividualMemberInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	input.NTAccount = strings.TrimSpace(input.NTAccount)
	return input
}
```

- [ ] **Step 4: Add validation methods**

Append this code to `internal/domain/group/validation.go` after `ListIndividualMembersQuery.Validate`:

```go
func (input AddIndividualMembersInput) Validate(now time.Time, opts ...ValidateOption) error {
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
	if len(input.IndividualMembers) == 0 {
		return invalidInput("individual members are required")
	}
	if len(input.IndividualMembers) > options.maxIndividualMembers {
		return invalidInput(fmt.Sprintf("individual members must not exceed %d items", options.maxIndividualMembers))
	}
	return validateIndividualMembersForAdd(input.IndividualMembers, now)
}

func (input UpdateIndividualMemberExpirationInput) Validate(now time.Time) error {
	if err := validateGroupIdentity(input.WorkspaceID, input.GroupID); err != nil {
		return err
	}
	if err := validateIndividualMemberAccount(input.NTAccount); err != nil {
		return err
	}
	if input.ExpirationDate.IsZero() {
		return invalidInput("individual member expiration date is required")
	}
	if !input.ExpirationDate.After(now) {
		return invalidInput("individual member expiration date must be in the future")
	}
	return nil
}

func (input DeleteIndividualMemberInput) Validate() error {
	if err := validateGroupIdentity(input.WorkspaceID, input.GroupID); err != nil {
		return err
	}
	return validateIndividualMemberAccount(input.NTAccount)
}
```

Append these helper functions near `validateIndividualMembers`:

```go
func validateIndividualMembersForAdd(members []IndividualMember, now time.Time) error {
	seen := map[string]struct{}{}
	for _, member := range members {
		account := strings.TrimSpace(member.NTAccount)
		if err := validateIndividualMemberAccount(account); err != nil {
			return err
		}
		if _, ok := seen[account]; ok {
			return fmt.Errorf("%w: duplicate individual member nt account %q", ErrDuplicateMember, account)
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

func validateIndividualMemberAccount(account string) error {
	if strings.TrimSpace(account) == "" {
		return invalidInput("individual member nt account is required")
	}
	return nil
}
```

- [ ] **Step 5: Run the focused domain tests and confirm they pass**

Run:

```bash
go test ./internal/domain/group -run 'Test(AddIndividualMembersInput|UpdateIndividualMemberExpirationInput|DeleteIndividualMemberInput)' -v
```

Expected: PASS.

- [ ] **Step 6: Run all domain tests**

Run:

```bash
go test ./internal/domain/group -v
```

Expected: PASS.

- [ ] **Step 7: Commit domain contracts**

```bash
git add internal/domain/group/errors.go internal/domain/group/group.go internal/domain/group/validation.go internal/domain/group/validation_test.go
git commit -m "feat(group): add individual member mutation contracts"
```

### Task 2: Transport DTOs for Member Add and Expiration Update

**Files:**

- Modify: `internal/group-service/transport/group_request.go`
- Modify: `internal/group-service/transport/group_request_test.go`
- Modify: `internal/group-service/transport/group_response.go`
- Modify: `internal/group-service/transport/group_response_test.go`

- [ ] **Step 1: Write failing request DTO tests**

Append this code to `internal/group-service/transport/group_request_test.go`:

```go
func TestDecodeIndividualMembersAddRequestToDomain(t *testing.T) {
	body := strings.NewReader(`{
		"individual_members": [
			{
				"nt_account": " user2 ",
				"expiration_date": "2026-06-01T00:00:00Z"
			}
		]
	}`)

	request, err := DecodeIndividualMembersAddRequest(body)
	if err != nil {
		t.Fatalf("DecodeIndividualMembersAddRequest error = %v, want nil", err)
	}
	input, err := request.ToDomain("workspace-1", "group-1")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" {
		t.Fatalf("identity = %q/%q, want workspace-1/group-1", input.WorkspaceID, input.GroupID)
	}
	if len(input.IndividualMembers) != 1 || input.IndividualMembers[0].NTAccount != " user2 " {
		t.Fatalf("members = %+v, want original account value before domain normalization", input.IndividualMembers)
	}
}

func TestDecodeIndividualMembersAddRequestRejectsInvalidTimestamp(t *testing.T) {
	_, err := DecodeIndividualMembersAddRequest(strings.NewReader(`{
		"individual_members": [
			{
				"nt_account": "user2",
				"expiration_date": "not-a-time"
			}
		]
	}`))
	if err == nil {
		t.Fatal("DecodeIndividualMembersAddRequest error = nil, want error")
	}
}

func TestIndividualMembersAddRequestToDomainRejectsMissingExpiration(t *testing.T) {
	request := IndividualMembersAddRequest{
		IndividualMembers: []IndividualMemberRequest{{NTAccount: "user2"}},
	}

	_, err := request.ToDomain("workspace-1", "group-1")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}

func TestDecodeIndividualMemberExpirationUpdateRequestToDomain(t *testing.T) {
	request, err := DecodeIndividualMemberExpirationUpdateRequest(strings.NewReader(`{
		"expiration_date": "2026-07-01T00:00:00Z"
	}`))
	if err != nil {
		t.Fatalf("DecodeIndividualMemberExpirationUpdateRequest error = %v, want nil", err)
	}

	input, err := request.ToDomain("workspace-1", "group-1", "user2")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" || input.NTAccount != "user2" {
		t.Fatalf("input = %+v, want path identity", input)
	}
	want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !input.ExpirationDate.Equal(want) {
		t.Fatalf("ExpirationDate = %s, want %s", input.ExpirationDate, want)
	}
}

func TestIndividualMemberExpirationUpdateRequestToDomainRejectsMissingExpiration(t *testing.T) {
	request := IndividualMemberExpirationUpdateRequest{}

	_, err := request.ToDomain("workspace-1", "group-1", "user2")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}
```

- [ ] **Step 2: Write failing add response test**

Append this code to `internal/group-service/transport/group_response_test.go`:

```go
func TestNewIndividualMembersAddResponse(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	response := NewIndividualMembersAddResponse([]group.IndividualMember{{
		ID:             "member-2",
		GroupID:        "group-1",
		NTAccount:      "user2",
		ExpirationDate: expiration,
		CreatedAt:      time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
	}})

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}
	members, ok := body["members"].([]any)
	if !ok || len(members) != 1 {
		t.Fatalf("members = %#v, want one member", body["members"])
	}
	member, ok := members[0].(map[string]any)
	if !ok {
		t.Fatalf("member = %#v, want object", members[0])
	}
	if member["nt_account"] != "user2" {
		t.Fatalf("nt_account = %v, want user2", member["nt_account"])
	}
	for _, field := range []string{"id", "group_id", "created_at", "updated_at", "deleted_at"} {
		if _, ok := member[field]; ok {
			t.Fatalf("field %q is present in %#v, want omitted", field, member)
		}
	}
}
```

- [ ] **Step 3: Run transport tests and confirm they fail**

Run:

```bash
go test ./internal/group-service/transport -run 'Test(DecodeIndividualMembersAddRequest|IndividualMembersAddRequest|DecodeIndividualMemberExpirationUpdateRequest|IndividualMemberExpirationUpdateRequest|NewIndividualMembersAddResponse)' -v
```

Expected: FAIL because the new request/response types and functions are undefined.

- [ ] **Step 4: Add request DTOs and mapping**

Add these types to `internal/group-service/transport/group_request.go` after `GroupGroupingRulesRequest`:

```go
type IndividualMembersAddRequest struct {
	IndividualMembers []IndividualMemberRequest `json:"individual_members"`
}

type IndividualMemberExpirationUpdateRequest struct {
	ExpirationDate JSONTime `json:"expiration_date"`
}
```

Add these functions after `DecodeGroupGroupingRulesRequest`:

```go
func DecodeIndividualMembersAddRequest(body io.Reader) (IndividualMembersAddRequest, error) {
	var request IndividualMembersAddRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return IndividualMembersAddRequest{}, fmt.Errorf("decode individual members add request: %w", err)
	}
	return request, nil
}

func (request IndividualMembersAddRequest) ToDomain(workspaceID string, groupID string) (group.AddIndividualMembersInput, error) {
	members := make([]group.IndividualMember, 0, len(request.IndividualMembers))
	for _, member := range request.IndividualMembers {
		if member.ExpirationDate.Time.IsZero() {
			return group.AddIndividualMembersInput{}, invalidGroupRequest("individual member expiration_date is required")
		}
		members = append(members, group.IndividualMember{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate.Time,
		})
	}
	return group.AddIndividualMembersInput{
		WorkspaceID:       workspaceID,
		GroupID:           groupID,
		IndividualMembers: members,
	}, nil
}

func DecodeIndividualMemberExpirationUpdateRequest(body io.Reader) (IndividualMemberExpirationUpdateRequest, error) {
	var request IndividualMemberExpirationUpdateRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return IndividualMemberExpirationUpdateRequest{}, fmt.Errorf("decode individual member expiration update request: %w", err)
	}
	return request, nil
}

func (request IndividualMemberExpirationUpdateRequest) ToDomain(workspaceID string, groupID string, ntAccount string) (group.UpdateIndividualMemberExpirationInput, error) {
	if request.ExpirationDate.Time.IsZero() {
		return group.UpdateIndividualMemberExpirationInput{}, invalidGroupRequest("expiration_date is required")
	}
	return group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    workspaceID,
		GroupID:        groupID,
		NTAccount:      ntAccount,
		ExpirationDate: request.ExpirationDate.Time,
	}, nil
}
```

- [ ] **Step 5: Add add-response DTO**

Add this type to `internal/group-service/transport/group_response.go` after `IndividualMemberListResponse`:

```go
type IndividualMembersAddResponse struct {
	Members []IndividualMemberResponse `json:"members"`
}
```

Add this function after `NewIndividualMemberListResponse`:

```go
func NewIndividualMembersAddResponse(members []group.IndividualMember) IndividualMembersAddResponse {
	responseMembers := make([]IndividualMemberResponse, 0, len(members))
	for _, member := range members {
		responseMembers = append(responseMembers, IndividualMemberResponse{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
		})
	}
	return IndividualMembersAddResponse{Members: responseMembers}
}
```

- [ ] **Step 6: Run transport tests**

Run:

```bash
go test ./internal/group-service/transport -v
```

Expected: PASS.

- [ ] **Step 7: Commit transport DTOs**

```bash
git add internal/group-service/transport/group_request.go internal/group-service/transport/group_request_test.go internal/group-service/transport/group_response.go internal/group-service/transport/group_response_test.go
git commit -m "feat(group-service): add individual member transport DTOs"
```

### Task 3: Service Use Cases for Member Mutations

**Files:**

- Modify: `internal/group-service/services/group_service.go`
- Modify: `internal/group-service/services/group_service_test.go`

- [ ] **Step 1: Write failing service tests**

Update `fakeGroupRepository` in `internal/group-service/services/group_service_test.go` with these fields:

```go
	addInput       group.AddIndividualMembersInput
	memberUpdate   group.UpdateIndividualMemberExpirationInput
	memberDelete   group.DeleteIndividualMemberInput
	addedMembers   []group.IndividualMember
	memberAddCalls int
	memberUpdCalls int
	memberDelCalls int
	memberUpdTime  time.Time
	memberDelTime  time.Time
```

Add these fake repository methods:

```go
func (f *fakeGroupRepository) AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error) {
	f.memberAddCalls++
	f.addInput = input
	if f.err != nil {
		return nil, f.err
	}
	if f.addedMembers != nil {
		return f.addedMembers, nil
	}
	return input.IndividualMembers, nil
}

func (f *fakeGroupRepository) UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput, updatedAt time.Time) error {
	f.memberUpdCalls++
	f.memberUpdate = input
	f.memberUpdTime = updatedAt
	return f.err
}

func (f *fakeGroupRepository) DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput, deletedAt time.Time) error {
	f.memberDelCalls++
	f.memberDelete = input
	f.memberDelTime = deletedAt
	return f.err
}
```

Append these tests:

```go
func TestGroupServiceAddIndividualMembers(t *testing.T) {
	repository := &fakeGroupRepository{}
	ids := []string{"member-2"}
	service := NewGroupService(repository,
		WithGroupClock(fixedNow),
		WithGroupIDGenerator(func() string {
			id := ids[0]
			ids = ids[1:]
			return id
		}),
	)

	members, err := service.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		IndividualMembers: []group.IndividualMember{{
			NTAccount:      " user2 ",
			ExpirationDate: serviceFutureTime(),
		}},
	})
	if err != nil {
		t.Fatalf("AddIndividualMembers error = %v, want nil", err)
	}
	if repository.memberAddCalls != 1 {
		t.Fatalf("member add calls = %d, want 1", repository.memberAddCalls)
	}
	if len(members) != 1 || members[0].ID != "member-2" || members[0].GroupID != "group-1" {
		t.Fatalf("members = %+v, want generated member for group-1", members)
	}
	if !members[0].CreatedAt.Equal(fixedNow()) || !members[0].UpdatedAt.Equal(fixedNow()) {
		t.Fatalf("timestamps = %s/%s, want fixed now", members[0].CreatedAt, members[0].UpdatedAt)
	}
}

func TestGroupServiceAddIndividualMembersValidationFailureDoesNotCallRepository(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	_, err := service.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
	})
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("AddIndividualMembers error = %v, want ErrInvalidInput", err)
	}
	if repository.memberAddCalls != 0 {
		t.Fatalf("member add calls = %d, want 0", repository.memberAddCalls)
	}
}

func TestGroupServiceAddIndividualMembersPreservesKnownErrors(t *testing.T) {
	for _, knownErr := range []error{group.ErrDuplicateMember, group.ErrNotFound} {
		t.Run(knownErr.Error(), func(t *testing.T) {
			repository := &fakeGroupRepository{err: knownErr}
			service := NewGroupService(repository, WithGroupClock(fixedNow))

			_, err := service.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
				WorkspaceID: "workspace-1",
				GroupID:     "group-1",
				IndividualMembers: []group.IndividualMember{{
					NTAccount:      "user2",
					ExpirationDate: serviceFutureTime(),
				}},
			})
			if !errors.Is(err, knownErr) {
				t.Fatalf("AddIndividualMembers error = %v, want %v", err, knownErr)
			}
		})
	}
}

func TestGroupServiceUpdateIndividualMemberExpiration(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    " workspace-1 ",
		GroupID:        " group-1 ",
		NTAccount:      " user2 ",
		ExpirationDate: serviceFutureTime(),
	})
	if err != nil {
		t.Fatalf("UpdateIndividualMemberExpiration error = %v, want nil", err)
	}
	if repository.memberUpdCalls != 1 {
		t.Fatalf("member update calls = %d, want 1", repository.memberUpdCalls)
	}
	if repository.memberUpdate.NTAccount != "user2" {
		t.Fatalf("member update = %+v, want trimmed user2", repository.memberUpdate)
	}
	if !repository.memberUpdTime.Equal(fixedNow()) {
		t.Fatalf("updatedAt = %s, want fixed now", repository.memberUpdTime)
	}
}

func TestGroupServiceUpdateIndividualMemberExpirationNotFound(t *testing.T) {
	repository := &fakeGroupRepository{err: group.ErrNotFound}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		NTAccount:      "user2",
		ExpirationDate: serviceFutureTime(),
	})
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("UpdateIndividualMemberExpiration error = %v, want ErrNotFound", err)
	}
}

func TestGroupServiceDeleteIndividualMember(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.DeleteIndividualMember(context.Background(), group.DeleteIndividualMemberInput{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		NTAccount:   " user2 ",
	})
	if err != nil {
		t.Fatalf("DeleteIndividualMember error = %v, want nil", err)
	}
	if repository.memberDelCalls != 1 {
		t.Fatalf("member delete calls = %d, want 1", repository.memberDelCalls)
	}
	if repository.memberDelete.NTAccount != "user2" {
		t.Fatalf("member delete = %+v, want trimmed user2", repository.memberDelete)
	}
	if !repository.memberDelTime.Equal(fixedNow()) {
		t.Fatalf("deletedAt = %s, want fixed now", repository.memberDelTime)
	}
}

func TestGroupServiceDeleteIndividualMemberValidationFailureDoesNotCallRepository(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.DeleteIndividualMember(context.Background(), group.DeleteIndividualMemberInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		NTAccount:   " ",
	})
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("DeleteIndividualMember error = %v, want ErrInvalidInput", err)
	}
	if repository.memberDelCalls != 0 {
		t.Fatalf("member delete calls = %d, want 0", repository.memberDelCalls)
	}
}
```

- [ ] **Step 2: Run focused service tests and confirm they fail**

Run:

```bash
go test ./internal/group-service/services -run 'TestGroupService(AddIndividualMembers|UpdateIndividualMemberExpiration|DeleteIndividualMember)' -v
```

Expected: FAIL because the repository interface and service methods do not include member mutations.

- [ ] **Step 3: Extend the service repository interface**

Add these methods to `GroupRepository` in `internal/group-service/services/group_service.go`:

```go
	AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error)
	UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput, updatedAt time.Time) error
	DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput, deletedAt time.Time) error
```

- [ ] **Step 4: Add service methods**

Append these methods to `internal/group-service/services/group_service.go` before `newIndividualMembers`:

```go
func (s *GroupService) AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error) {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(now, s.validateOptions...); err != nil {
		return nil, err
	}
	input.IndividualMembers = s.newIndividualMembers(input.GroupID, input.IndividualMembers, now)
	members, err := s.repository.AddIndividualMembers(ctx, input)
	if err != nil {
		if errors.Is(err, group.ErrDuplicateMember) || errors.Is(err, group.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("add group individual members: %w", err)
	}
	return members, nil
}

func (s *GroupService) UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput) error {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(now); err != nil {
		return err
	}
	if err := s.repository.UpdateIndividualMemberExpiration(ctx, input, now); err != nil {
		if errors.Is(err, group.ErrNotFound) {
			return err
		}
		return fmt.Errorf("update group individual member expiration: %w", err)
	}
	return nil
}

func (s *GroupService) DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput) error {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}
	if err := s.repository.DeleteIndividualMember(ctx, input, s.now().UTC()); err != nil {
		return fmt.Errorf("delete group individual member: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run service tests**

Run:

```bash
go test ./internal/group-service/services -v
```

Expected: PASS.

- [ ] **Step 6: Commit service use cases**

```bash
git add internal/group-service/services/group_service.go internal/group-service/services/group_service_test.go
git commit -m "feat(group-service): add individual member service workflows"
```

### Task 4: MongoDB Repository Mutations

**Files:**

- Modify: `internal/group-service/repositories/mongo_group_repository.go`
- Modify: `internal/group-service/repositories/mongo_group_repository_test.go`

- [ ] **Step 1: Write failing repository unit tests**

Append this code to `internal/group-service/repositories/mongo_group_repository_test.go`:

```go
func TestActiveIndividualMemberFilter(t *testing.T) {
	filter := activeIndividualMemberFilter("group-1", "user2")
	want := bson.M{"group_id": "group-1", "nt_account": "user2", "deleted_at": nil}

	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestMapMemberInsertError(t *testing.T) {
	err := mongo.WriteException{
		WriteErrors: []mongo.WriteError{{
			Code:    11000,
			Message: "E11000 duplicate key error collection: group_individual_members index: " + membersActiveGroupAccountUniqueIndexName + " dup key",
		}},
	}

	mapped := mapMemberInsertError(err)
	if !errors.Is(mapped, group.ErrDuplicateMember) {
		t.Fatalf("mapped error = %v, want ErrDuplicateMember", mapped)
	}
}
```

- [ ] **Step 2: Write failing repository integration tests**

Append this code to `internal/group-service/repositories/mongo_group_repository_test.go`:

```go
func TestMongoGroupRepositoryAddIndividualMembersIntegration(t *testing.T) {
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

	member := group.IndividualMember{
		ID:             "member-2",
		GroupID:        "group-1",
		NTAccount:      "user2",
		ExpirationDate: repositoryTime().Add(24 * time.Hour),
		CreatedAt:      repositoryTime(),
		UpdatedAt:      repositoryTime(),
	}
	members, err := repository.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID:       "workspace-1",
		GroupID:           "group-1",
		IndividualMembers: []group.IndividualMember{member},
	})
	if err != nil {
		t.Fatalf("AddIndividualMembers error = %v, want nil", err)
	}
	if len(members) != 1 || members[0].NTAccount != "user2" {
		t.Fatalf("members = %+v, want user2", members)
	}
	count, err := db.Collection(groupIndividualMemberCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1", "nt_account": "user2", "deleted_at": nil})
	if err != nil {
		t.Fatalf("count members: %v", err)
	}
	if count != 1 {
		t.Fatalf("active member count = %d, want 1", count)
	}
}

func TestMongoGroupRepositoryAddIndividualMembersMissingGroupIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	_, err := repository.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID: "workspace-1",
		GroupID:     "missing",
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-2",
			GroupID:        "missing",
			NTAccount:      "user2",
			ExpirationDate: repositoryTime().Add(24 * time.Hour),
			CreatedAt:      repositoryTime(),
			UpdatedAt:      repositoryTime(),
		}},
	})
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("AddIndividualMembers error = %v, want ErrNotFound", err)
	}
}

func TestMongoGroupRepositoryAddIndividualMembersDuplicateActiveMemberIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	_, err := repository.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-2",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: repositoryTime().Add(24 * time.Hour),
			CreatedAt:      repositoryTime(),
			UpdatedAt:      repositoryTime(),
		}},
	})
	if !errors.Is(err, group.ErrDuplicateMember) {
		t.Fatalf("AddIndividualMembers error = %v, want ErrDuplicateMember", err)
	}
}

func TestMongoGroupRepositoryUpdateIndividualMemberExpirationIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	expiration := repositoryTime().Add(48 * time.Hour)
	updatedAt := repositoryTime().Add(time.Hour)
	err := repository.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		NTAccount:      "user1",
		ExpirationDate: expiration,
	}, updatedAt)
	if err != nil {
		t.Fatalf("UpdateIndividualMemberExpiration error = %v, want nil", err)
	}
	var doc individualMemberDocument
	err = db.Collection(groupIndividualMemberCollectionName).FindOne(context.Background(), bson.M{"group_id": "group-1", "nt_account": "user1"}).Decode(&doc)
	if err != nil {
		t.Fatalf("find member: %v", err)
	}
	if !doc.ExpirationDate.Equal(expiration) || !doc.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("member doc = %+v, want updated expiration and updated_at", doc)
	}
}

func TestMongoGroupRepositoryUpdateIndividualMemberExpirationMissingMemberIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	err := repository.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		NTAccount:      "missing",
		ExpirationDate: repositoryTime().Add(48 * time.Hour),
	}, repositoryTime().Add(time.Hour))
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("UpdateIndividualMemberExpiration error = %v, want ErrNotFound", err)
	}
}

func TestMongoGroupRepositoryDeleteIndividualMemberIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	deletedAt := repositoryTime().Add(time.Hour)
	err := repository.DeleteIndividualMember(context.Background(), group.DeleteIndividualMemberInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		NTAccount:   "user1",
	}, deletedAt)
	if err != nil {
		t.Fatalf("DeleteIndividualMember error = %v, want nil", err)
	}
	count, err := db.Collection(groupIndividualMemberCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1", "nt_account": "user1", "deleted_at": deletedAt})
	if err != nil {
		t.Fatalf("count deleted member: %v", err)
	}
	if count != 1 {
		t.Fatalf("deleted member count = %d, want 1", count)
	}
}

func TestMongoGroupRepositoryDeleteIndividualMemberMissingTargetIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	err := repository.DeleteIndividualMember(context.Background(), group.DeleteIndividualMemberInput{
		WorkspaceID: "workspace-1",
		GroupID:     "missing",
		NTAccount:   "user1",
	}, repositoryTime().Add(time.Hour))
	if err != nil {
		t.Fatalf("DeleteIndividualMember missing group error = %v, want nil", err)
	}
}
```

- [ ] **Step 3: Run focused repository tests and confirm they fail**

Run:

```bash
go test ./internal/group-service/repositories -run 'Test(ActiveIndividualMemberFilter|MapMemberInsertError|MongoGroupRepository(AddIndividualMembers|UpdateIndividualMemberExpiration|DeleteIndividualMember))' -v
```

Expected: FAIL because repository helpers and mutation methods are undefined. Integration tests skip if `GROUP_SERVICE_MONGODB_TEST_URI` is not set.

- [ ] **Step 4: Add repository methods**

Append these methods to `internal/group-service/repositories/mongo_group_repository.go` after `ListIndividualMembers`:

```go
func (r *MongoGroupRepository) AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return nil, fmt.Errorf("start individual member add session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		exists, existsErr := r.activeGroupExists(sessionCtx, group.GetQuery{WorkspaceID: input.WorkspaceID, GroupID: input.GroupID})
		if existsErr != nil {
			return nil, existsErr
		}
		if !exists {
			return nil, group.ErrNotFound
		}
		docs := make([]any, 0, len(input.IndividualMembers))
		for _, member := range input.IndividualMembers {
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
		if _, insertErr := r.members.InsertMany(sessionCtx, docs); insertErr != nil {
			return nil, mapMemberInsertError(insertErr)
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	return input.IndividualMembers, nil
}

func (r *MongoGroupRepository) UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput, updatedAt time.Time) error {
	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf("start individual member update session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		exists, existsErr := r.activeGroupExists(sessionCtx, group.GetQuery{WorkspaceID: input.WorkspaceID, GroupID: input.GroupID})
		if existsErr != nil {
			return nil, existsErr
		}
		if !exists {
			return nil, group.ErrNotFound
		}
		result, updateErr := r.members.UpdateOne(sessionCtx,
			activeIndividualMemberFilter(input.GroupID, input.NTAccount),
			bson.M{"$set": bson.M{"expiration_date": input.ExpirationDate, "updated_at": updatedAt}},
		)
		if updateErr != nil {
			return nil, fmt.Errorf("update group individual member expiration: %w", updateErr)
		}
		if result.MatchedCount == 0 {
			return nil, group.ErrNotFound
		}
		return nil, nil
	})
	return err
}

func (r *MongoGroupRepository) DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput, deletedAt time.Time) error {
	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf("start individual member delete session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		exists, existsErr := r.activeGroupExists(sessionCtx, group.GetQuery{WorkspaceID: input.WorkspaceID, GroupID: input.GroupID})
		if existsErr != nil {
			return nil, existsErr
		}
		if !exists {
			return nil, nil
		}
		if _, updateErr := r.members.UpdateOne(sessionCtx,
			activeIndividualMemberFilter(input.GroupID, input.NTAccount),
			bson.M{"$set": bson.M{"deleted_at": deletedAt, "updated_at": deletedAt}},
		); updateErr != nil {
			return nil, fmt.Errorf("soft delete group individual member: %w", updateErr)
		}
		return nil, nil
	})
	return err
}
```

- [ ] **Step 5: Add repository helpers**

Append these helper functions near `activeGroupFilter`:

```go
func (r *MongoGroupRepository) activeGroupExists(ctx context.Context, query group.GetQuery) (bool, error) {
	var doc groupDocument
	err := r.groups.FindOne(ctx, activeGroupFilter(query)).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, fmt.Errorf("find active group: %w", err)
	}
	return true, nil
}

func activeIndividualMemberFilter(groupID string, ntAccount string) bson.M {
	return bson.M{
		"group_id":    groupID,
		"nt_account":  ntAccount,
		"deleted_at":  nil,
	}
}
```

Append this helper near `mapGroupInsertError`:

```go
func mapMemberInsertError(err error) error {
	if isDuplicateIndex(err, membersActiveGroupAccountUniqueIndexName) {
		return fmt.Errorf("%w: active individual member already exists", group.ErrDuplicateMember)
	}
	return fmt.Errorf("insert group individual members: %w", err)
}
```

- [ ] **Step 6: Run repository unit tests**

Run:

```bash
go test ./internal/group-service/repositories -run 'Test(ActiveIndividualMemberFilter|MapMemberInsertError|IndexModels|BuildIndividualMemberListFilter)' -v
```

Expected: PASS.

- [ ] **Step 7: Run repository package tests**

Run:

```bash
go test ./internal/group-service/repositories -v
```

Expected: PASS. Integration tests may skip when `GROUP_SERVICE_MONGODB_TEST_URI` is not set.

- [ ] **Step 8: Run MongoDB integration tests when MongoDB is available**

Run:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -run Integration -v
```

Expected: PASS when local MongoDB is running as a replica set. If MongoDB is unavailable, record the connection error in the final implementation notes.

- [ ] **Step 9: Commit repository mutations**

```bash
git add internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_group_repository_test.go
git commit -m "feat(group-service): persist individual member mutations"
```

### Task 5: HTTP Routes and Error Mapping

**Files:**

- Modify: `internal/group-service/handlers/group_handler.go`
- Modify: `internal/group-service/handlers/group_handler_test.go`

- [ ] **Step 1: Write failing handler tests**

Update `fakeHTTPGroupService` in `internal/group-service/handlers/group_handler_test.go` with these fields:

```go
	addMembersInput group.AddIndividualMembersInput
	memberUpdate    group.UpdateIndividualMemberExpirationInput
	memberDelete    group.DeleteIndividualMemberInput
	addedMembers    []group.IndividualMember
	memberAddCalls  int
	memberUpdCalls  int
	memberDelCalls  int
```

Add these fake service methods:

```go
func (f *fakeHTTPGroupService) AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error) {
	f.memberAddCalls++
	f.addMembersInput = input
	if f.err != nil {
		return nil, f.err
	}
	return f.addedMembers, nil
}

func (f *fakeHTTPGroupService) UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput) error {
	f.memberUpdCalls++
	f.memberUpdate = input
	return f.err
}

func (f *fakeHTTPGroupService) DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput) error {
	f.memberDelCalls++
	f.memberDelete = input
	return f.err
}
```

Append these tests:

```go
func TestGroupHandlerAddIndividualMembers(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	service := &fakeHTTPGroupService{addedMembers: []group.IndividualMember{{ID: "member-2", GroupID: "group-1", NTAccount: "user2", ExpirationDate: expiration}}}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members", strings.NewReader(`{
		"individual_members": [{"nt_account": "user2", "expiration_date": "2026-06-01T00:00:00Z"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if service.memberAddCalls != 1 || service.addMembersInput.GroupID != "group-1" {
		t.Fatalf("add input = %+v calls = %d, want group-1 and one call", service.addMembersInput, service.memberAddCalls)
	}
	if !strings.Contains(rec.Body.String(), `"members"`) {
		t.Fatalf("body = %s, want members", rec.Body.String())
	}
}

func TestGroupHandlerAddIndividualMembersDuplicateReturnsConflict(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrDuplicateMember}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members", strings.NewReader(`{
		"individual_members": [{"nt_account": "user2", "expiration_date": "2026-06-01T00:00:00Z"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestGroupHandlerAddIndividualMembersMissingGroupReturnsNotFound(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrNotFound}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups/missing/individual-members", strings.NewReader(`{
		"individual_members": [{"nt_account": "user2", "expiration_date": "2026-06-01T00:00:00Z"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGroupHandlerUpdateIndividualMemberExpiration(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members/user2", strings.NewReader(`{
		"expiration_date": "2026-07-01T00:00:00Z"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if service.memberUpdate.NTAccount != "user2" || service.memberUpdate.GroupID != "group-1" {
		t.Fatalf("member update = %+v, want user2/group-1", service.memberUpdate)
	}
}

func TestGroupHandlerUpdateIndividualMemberExpirationMissingReturnsNotFound(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrNotFound}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members/missing", strings.NewReader(`{
		"expiration_date": "2026-07-01T00:00:00Z"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGroupHandlerDeleteIndividualMember(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members/user2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if service.memberDelete.NTAccount != "user2" || service.memberDelete.GroupID != "group-1" {
		t.Fatalf("member delete = %+v, want user2/group-1", service.memberDelete)
	}
}

func TestGroupHandlerDeleteIndividualMemberMissingReturnsNoContent(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/groups/missing/individual-members/user2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}
```

- [ ] **Step 2: Run focused handler tests and confirm they fail**

Run:

```bash
go test ./internal/group-service/handlers -run 'TestGroupHandler(AddIndividualMembers|UpdateIndividualMemberExpiration|DeleteIndividualMember)' -v
```

Expected: FAIL because HTTP service methods, routes, and handler functions are undefined.

- [ ] **Step 3: Extend handler service interface and route registration**

Add these methods to `HTTPGroupService` in `internal/group-service/handlers/group_handler.go`:

```go
	AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error)
	UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput) error
	DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput) error
```

Add these routes to `RegisterRoutes`:

```go
	e.POST("/api/v1/workspaces/:workspace_id/groups/:group_id/individual-members", handler.AddIndividualMembers)
	e.PATCH("/api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account", handler.UpdateIndividualMemberExpiration)
	e.DELETE("/api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account", handler.DeleteIndividualMember)
```

- [ ] **Step 4: Add handler methods**

Append these methods to `internal/group-service/handlers/group_handler.go` after `ListIndividualMembers`:

```go
func (h *GroupHandler) AddIndividualMembers(c *echo.Context) error {
	params := newGroupPathParams(c)
	request, err := transport.DecodeIndividualMembersAddRequest(c.Request().Body)
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
	members, err := h.service.AddIndividualMembers(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, group.ErrDuplicateMember) {
			return c.JSON(http.StatusConflict, exception.WrapResponse(exception.New("conflict", "Individual member already exists", exception.WithDetails(map[string]any{}))))
		}
		if errors.Is(err, group.ErrNotFound) {
			return c.JSON(http.StatusNotFound, exception.WrapResponse(exception.New("not_found", "Group not found", exception.WithDetails(map[string]any{}))))
		}
		h.logger.Warn("failed to add group individual members", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusCreated, transport.NewIndividualMembersAddResponse(members))
}

func (h *GroupHandler) UpdateIndividualMemberExpiration(c *echo.Context) error {
	params := newGroupPathParams(c)
	request, err := transport.DecodeIndividualMemberExpirationUpdateRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(params.workspaceID, params.groupID, c.Param("nt_account"))
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}
	if err := h.service.UpdateIndividualMemberExpiration(c.Request().Context(), input); err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, group.ErrNotFound) {
			return c.JSON(http.StatusNotFound, exception.WrapResponse(exception.New("not_found", "Individual member not found", exception.WithDetails(map[string]any{}))))
		}
		h.logger.Warn("failed to update group individual member expiration", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID, "nt_account", c.Param("nt_account"))
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *GroupHandler) DeleteIndividualMember(c *echo.Context) error {
	params := newGroupPathParams(c)
	err := h.service.DeleteIndividualMember(c.Request().Context(), group.DeleteIndividualMemberInput{
		WorkspaceID: params.workspaceID,
		GroupID:     params.groupID,
		NTAccount:   c.Param("nt_account"),
	})
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to delete group individual member", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID, "nt_account", c.Param("nt_account"))
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 5: Run handler tests**

Run:

```bash
go test ./internal/group-service/handlers -v
```

Expected: PASS.

- [ ] **Step 6: Commit HTTP routes**

```bash
git add internal/group-service/handlers/group_handler.go internal/group-service/handlers/group_handler_test.go
git commit -m "feat(group-service): expose individual member mutation APIs"
```

### Task 6: REST Client Examples and Final Verification

**Files:**

- Modify: `examples/api/groups.http`

- [ ] **Step 1: Update REST Client examples**

Append these examples to `examples/api/groups.http` after the existing individual member list examples:

```http
### Add individual members
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members
Content-Type: application/json

{
  "individual_members": [
    {
      "nt_account": "user2",
      "expiration_date": "2026-06-01T00:00:00Z"
    }
  ]
}

### Add duplicate individual member returns 409
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members
Content-Type: application/json

{
  "individual_members": [
    {
      "nt_account": "user2",
      "expiration_date": "2026-06-01T00:00:00Z"
    }
  ]
}

### Add individual member with invalid expiration returns 400
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members
Content-Type: application/json

{
  "individual_members": [
    {
      "nt_account": "user3",
      "expiration_date": "not-a-time"
    }
  ]
}

### Update individual member expiration
PATCH {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members/user2
Content-Type: application/json

{
  "expiration_date": "2026-07-01T00:00:00Z"
}

### Update missing individual member returns 404
PATCH {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members/missing-user
Content-Type: application/json

{
  "expiration_date": "2026-07-01T00:00:00Z"
}

### Delete individual member idempotently
DELETE {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members/user2

### Delete missing individual member idempotently
DELETE {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/groups/{{groupId}}/individual-members/missing-user
```

- [ ] **Step 2: Run package tests for changed backend areas**

Run:

```bash
go test ./internal/domain/group ./internal/group-service/transport ./internal/group-service/services ./internal/group-service/repositories ./internal/group-service/handlers -v
```

Expected: PASS. Repository integration tests may skip when `GROUP_SERVICE_MONGODB_TEST_URI` is not set.

- [ ] **Step 3: Run repository-wide tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Run MongoDB integration tests when MongoDB is available**

Run:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -run Integration -v
```

Expected: PASS when local MongoDB is running as a replica set. If MongoDB is unavailable, record that the integration verification could not be completed and include the exact error.

- [ ] **Step 5: Check examples and docs diff**

Run:

```bash
git diff --check -- internal/domain/group internal/group-service examples/api/groups.http docs/designs docs/plans/active/2026-05-10-group-service-individual-member-mutations.md
```

Expected: PASS with no whitespace errors.

- [ ] **Step 6: Commit examples and final verification state**

```bash
git add examples/api/groups.http
git commit -m "docs(api): add group individual member mutation examples"
```

## Final Acceptance Checklist

- [ ] `POST /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members` is registered.
- [ ] `PATCH /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account` is registered.
- [ ] `DELETE /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account` is registered.
- [ ] Add request accepts `individual_members[].nt_account` and RFC3339 `individual_members[].expiration_date`.
- [ ] Add response returns only `members[].nt_account` and `members[].expiration_date`.
- [ ] Add returns `404` when the active group is missing.
- [ ] Add returns `409` for duplicate request accounts and already-active members.
- [ ] Update request accepts RFC3339 `expiration_date`.
- [ ] Update returns `204` on success and `404` when the active group or active member is missing.
- [ ] Delete returns `204` on success and when the active group or active member is missing.
- [ ] Member mutations use active group ownership checks with `_id`, `workspace_id`, and `deleted_at: null`.
- [ ] Member add/update/delete repository operations run inside MongoDB transactions.
- [ ] Soft-deleted members are excluded from active uniqueness and direct update/delete filters.
- [ ] `examples/api/groups.http` covers success, validation, conflict, not-found, and idempotent delete cases.
- [ ] `go test ./...` passes, or any skipped/unavailable integration checks are documented with exact commands and reasons.
