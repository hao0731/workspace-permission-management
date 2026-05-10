# Function Resource Permissions Get and Timestamps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `GET /api/v1/workspaces/:workspace_id/functions/:function_key/permissions` and persist `created_at` / `updated_at` metadata for function resource permission documents.

**Architecture:** Extend the existing permission aggregate without changing its ownership boundaries. Domain owns query validation and persistence metadata fields, transport owns nullable GET response JSON, service owns timestamp assignment and read workflow, repository owns MongoDB timestamp preservation/read behavior, and handlers stay thin by mapping service results to HTTP responses.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, `log/slog`, standard `encoding/json`, standard `testing`.

---

## Source Designs

Primary source design: [../../designs/function-resource-permissions.md](../../designs/function-resource-permissions.md)

Related source design: [../../designs/function-service.md](../../designs/function-service.md)

Applicable policies:

- [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend code must keep handlers thin, place HTTP DTOs in transport, keep domain independent from Echo and MongoDB, define repository interfaces at the service consumer side, and treat API and MongoDB schemas as explicit contracts with tests and REST Client examples.
- Design and implementation plan documents must stay under `docs/designs/` and `docs/plans/active/`, and implementation plans must link to their source design documents.

## Scope

Implement:

- `GET /api/v1/workspaces/:workspace_id/functions/:function_key/permissions`.
- `200 OK` with a normal `permissions` object when a permission document exists.
- `200 OK` with `{ "permissions": null }` when no `function_resource_permissions` document exists for `workspace_id + function_key`.
- No `function_resources` lookup for GET.
- `created_at` and `updated_at` fields in the permission domain model and MongoDB document.
- PUT insert behavior that stores both `created_at` and `updated_at`.
- PUT replace behavior that preserves existing `created_at`, sets missing legacy `created_at` on next replace, and refreshes `updated_at`.
- Tests for domain query validation, transport nullable responses, service timestamp/read behavior, repository mapping/update helpers, handler GET behavior, and API examples.

Do not implement:

- Permission evaluation.
- Frontend changes.
- Partial update, PATCH, delete, history, or audit APIs.
- Workspace, function, group, action, or resource tag existence checks.
- Public response fields for `created_at` or `updated_at`.
- A migration job for existing permission documents.

## File Structure and Responsibilities

- Modify: `internal/domain/permission/permission.go`
  - Add `CreatedAt`, `UpdatedAt`, and `GetQuery`.
- Modify: `internal/domain/permission/validation.go`
  - Add `GetQuery.Validate`.
- Modify: `internal/domain/permission/validation_test.go`
  - Add query validation tests.
- Modify: `internal/function-service/transport/permission_response.go`
  - Share permission object mapping across PUT and GET responses.
  - Add nullable GET response constructor.
- Modify: `internal/function-service/transport/permission_response_test.go`
  - Cover GET found, GET not found, and timestamp exclusion.
- Modify: `internal/function-service/services/permission_service.go`
  - Add clock injection, timestamp assignment, repository `Get`, and service `GetPermission`.
- Modify: `internal/function-service/services/permission_service_test.go`
  - Cover deterministic timestamps, GET found, GET not found, invalid query, and GET repository failure.
- Modify: `internal/function-service/repositories/mongo_permission_repository.go`
  - Add timestamp fields to the document, update pipeline preserving `created_at`, and `Get`.
- Modify: `internal/function-service/repositories/mongo_permission_repository_test.go`
  - Cover timestamp mapping and update pipeline behavior.
- Modify: `internal/function-service/handlers/permission_handler.go`
  - Add GET service interface method, route registration, and handler method.
- Modify: `internal/function-service/handlers/permission_handler_test.go`
  - Cover GET found, GET not found, and GET failure behavior.
- Modify: `examples/api/function_resource_permissions.http`
  - Add GET examples for found and not-yet-configured permission documents.

---

### Task 1: Domain Query and Metadata Contract

**Files:**

- Modify: `internal/domain/permission/permission.go`
- Modify: `internal/domain/permission/validation.go`
- Modify: `internal/domain/permission/validation_test.go`

- [ ] **Step 1: Write failing tests for GET query validation**

Append these tests to `internal/domain/permission/validation_test.go`:

```go
func TestGetQueryValidateAcceptsValidQuery(t *testing.T) {
	query := GetQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
	}

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestGetQueryValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		query       GetQuery
		wantMessage string
	}{
		{
			name: "blank workspace id",
			query: GetQuery{
				WorkspaceID: "   ",
				FunctionKey: "todo",
			},
			wantMessage: "workspace id is required",
		},
		{
			name: "blank function key",
			query: GetQuery{
				WorkspaceID: "workspace-1",
				FunctionKey: "   ",
			},
			wantMessage: "function key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			requireInvalidInput(t, err, tt.wantMessage)
		})
	}
}
```

- [ ] **Step 2: Run domain tests and confirm the new tests fail**

Run:

```bash
go test ./internal/domain/permission -run 'Test(GetQuery|SaveInput)' -v
```

Expected: FAIL because `GetQuery` is undefined.

- [ ] **Step 3: Add permission metadata fields and query type**

Update `internal/domain/permission/permission.go` so the `Permission` type includes timestamps and the file contains `GetQuery`:

```go
package permission

import "time"

type Permission struct {
	ID               string
	WorkspaceID      string
	FunctionKey      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	OfficePermission PermissionSection
	RemotePermission PermissionSection
}

type SaveInput struct {
	WorkspaceID      string
	FunctionKey      string
	OfficePermission *PermissionSection
	RemotePermission *PermissionSection
}

type GetQuery struct {
	WorkspaceID string
	FunctionKey string
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

- [ ] **Step 4: Add query validation**

Append this method to `internal/domain/permission/validation.go`:

```go
func (query GetQuery) Validate() error {
	if strings.TrimSpace(query.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(query.FunctionKey) == "" {
		return invalidInput("function key is required")
	}
	return nil
}
```

- [ ] **Step 5: Run domain tests and confirm they pass**

Run:

```bash
go test ./internal/domain/permission -v
```

Expected: PASS.

- [ ] **Step 6: Commit domain contract changes**

```bash
git add internal/domain/permission/permission.go internal/domain/permission/validation.go internal/domain/permission/validation_test.go
git commit -m "feat: add permission get query contract"
```

---

### Task 2: Transport GET Response Shape

**Files:**

- Modify: `internal/function-service/transport/permission_response.go`
- Modify: `internal/function-service/transport/permission_response_test.go`

- [ ] **Step 1: Write failing transport tests for GET responses**

Add `strings` to the imports in `internal/function-service/transport/permission_response_test.go`, then append these tests:

```go
func TestNewPermissionGetResponse(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	response := NewPermissionGetResponse(permission.Permission{
		ID:          "permission-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		CreatedAt:   time.Date(2026, 5, 9, 1, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 9, 2, 0, 0, 0, time.UTC),
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

	if response.Permissions == nil {
		t.Fatal("permissions = nil, want response object")
	}
	if response.Permissions.OfficePermission.ExtraRules[0].RuleID != "rule-1" {
		t.Fatalf("rule_id = %q, want rule-1", response.Permissions.OfficePermission.ExtraRules[0].RuleID)
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(data), "created_at") || strings.Contains(string(data), "updated_at") {
		t.Fatalf("response exposes persistence timestamps: %s", data)
	}
}

func TestNewPermissionGetNotFoundResponse(t *testing.T) {
	response := NewPermissionGetNotFoundResponse()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	got := string(data)
	want := `{"permissions":null}`
	if got != want {
		t.Fatalf("json = %s, want %s", got, want)
	}
}
```

- [ ] **Step 2: Run transport response tests and confirm failure**

Run:

```bash
go test ./internal/function-service/transport -run 'TestNewPermission(Get|Save)' -v
```

Expected: FAIL because `NewPermissionGetResponse` and `NewPermissionGetNotFoundResponse` are undefined.

- [ ] **Step 3: Add nullable GET response constructors**

Update `internal/function-service/transport/permission_response.go` to share the permission object mapper:

```go
type PermissionSaveResponse struct {
	Permissions PermissionResponse `json:"permissions"`
}

type PermissionGetResponse struct {
	Permissions *PermissionResponse `json:"permissions"`
}

func NewPermissionSaveResponse(model permission.Permission) PermissionSaveResponse {
	return PermissionSaveResponse{
		Permissions: newPermissionResponse(model),
	}
}

func NewPermissionGetResponse(model permission.Permission) PermissionGetResponse {
	permissions := newPermissionResponse(model)
	return PermissionGetResponse{
		Permissions: &permissions,
	}
}

func NewPermissionGetNotFoundResponse() PermissionGetResponse {
	return PermissionGetResponse{}
}

func newPermissionResponse(model permission.Permission) PermissionResponse {
	return PermissionResponse{
		OfficePermission: permissionSectionResponse(model.OfficePermission),
		RemotePermission: permissionSectionResponse(model.RemotePermission),
	}
}
```

Keep the existing `PermissionResponse`, `PermissionSectionResponse`, `BaselineRuleResponse`, `ExtraRuleResponse`, and `permissionSectionResponse` definitions.

- [ ] **Step 4: Run transport tests and confirm they pass**

Run:

```bash
go test ./internal/function-service/transport -v
```

Expected: PASS.

- [ ] **Step 5: Commit transport response changes**

```bash
git add internal/function-service/transport/permission_response.go internal/function-service/transport/permission_response_test.go
git commit -m "feat: add nullable permission get response"
```

---

### Task 3: Service Timestamp and Read Workflow

**Files:**

- Modify: `internal/function-service/services/permission_service.go`
- Modify: `internal/function-service/services/permission_service_test.go`

- [ ] **Step 1: Extend the fake repository in service tests**

Update `fakePermissionRepository` in `internal/function-service/services/permission_service_test.go`:

```go
type fakePermissionRepository struct {
	input    permission.Permission
	query    permission.GetQuery
	model    permission.Permission
	found    bool
	calls    int
	getCalls int
	err      error
	getErr   error
}

func (f *fakePermissionRepository) Save(ctx context.Context, input permission.Permission) (permission.Permission, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return permission.Permission{}, f.err
	}
	return input, nil
}

func (f *fakePermissionRepository) Get(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error) {
	f.getCalls++
	f.query = query
	if f.getErr != nil {
		return permission.Permission{}, false, f.getErr
	}
	return f.model, f.found, nil
}
```

- [ ] **Step 2: Write failing service tests**

Append these tests to `internal/function-service/services/permission_service_test.go`:

```go
func TestPermissionServiceSavePermissionAssignsTimestamps(t *testing.T) {
	repo := &fakePermissionRepository{}
	now := time.Date(2026, 5, 9, 1, 2, 3, 0, time.UTC)
	ids := []string{"permission-1", "rule-generated-1"}
	service := NewPermissionService(repo,
		WithPermissionIDGenerator(func() string {
			id := ids[0]
			ids = ids[1:]
			return id
		}),
		WithPermissionClock(func() time.Time {
			return now
		}),
	)

	got, err := service.SavePermission(context.Background(), validPermissionSaveInput())
	if err != nil {
		t.Fatalf("SavePermission error = %v, want nil", err)
	}
	if !got.CreatedAt.Equal(now) || !got.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = %s/%s, want %s", got.CreatedAt, got.UpdatedAt, now)
	}
	if !repo.input.CreatedAt.Equal(now) || !repo.input.UpdatedAt.Equal(now) {
		t.Fatalf("repository timestamps = %s/%s, want %s", repo.input.CreatedAt, repo.input.UpdatedAt, now)
	}
}

func TestPermissionServiceGetPermissionReturnsFoundModel(t *testing.T) {
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
		},
		RemotePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	}
	repo := &fakePermissionRepository{model: model, found: true}
	service := NewPermissionService(repo)

	got, found, err := service.GetPermission(context.Background(), permission.GetQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
	})
	if err != nil {
		t.Fatalf("GetPermission error = %v, want nil", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if got.ID != "permission-1" {
		t.Fatalf("permission id = %q, want permission-1", got.ID)
	}
	if repo.query.WorkspaceID != "workspace-1" || repo.query.FunctionKey != "todo" {
		t.Fatalf("query = %+v, want workspace-1/todo", repo.query)
	}
}

func TestPermissionServiceGetPermissionReturnsNotFound(t *testing.T) {
	repo := &fakePermissionRepository{found: false}
	service := NewPermissionService(repo)

	_, found, err := service.GetPermission(context.Background(), permission.GetQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
	})
	if err != nil {
		t.Fatalf("GetPermission error = %v, want nil", err)
	}
	if found {
		t.Fatal("found = true, want false")
	}
}

func TestPermissionServiceGetPermissionRejectsInvalidQuery(t *testing.T) {
	repo := &fakePermissionRepository{}
	service := NewPermissionService(repo)

	_, _, err := service.GetPermission(context.Background(), permission.GetQuery{
		WorkspaceID: "",
		FunctionKey: "todo",
	})
	if !errors.Is(err, permission.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if repo.getCalls != 0 {
		t.Fatalf("repository get calls = %d, want 0", repo.getCalls)
	}
}

func TestPermissionServiceGetPermissionWrapsRepositoryError(t *testing.T) {
	repo := &fakePermissionRepository{getErr: errors.New("database unavailable")}
	service := NewPermissionService(repo)

	_, _, err := service.GetPermission(context.Background(), permission.GetQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
	})
	if err == nil {
		t.Fatal("GetPermission error = nil, want error")
	}
}
```

- [ ] **Step 3: Run service tests and confirm failure**

Run:

```bash
go test ./internal/function-service/services -run Permission -v
```

Expected: FAIL because `WithPermissionClock`, repository `Get`, and `GetPermission` are missing.

- [ ] **Step 4: Implement clock injection and get workflow**

Update the imports in `internal/function-service/services/permission_service.go` to include `time`.

Update the repository interface, option set, service struct, constructor, and workflows:

```go
type PermissionRepository interface {
	Save(ctx context.Context, input permission.Permission) (permission.Permission, error)
	Get(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error)
}

func WithPermissionClock(clock func() time.Time) PermissionOption {
	return func(s *PermissionService) {
		if clock != nil {
			s.now = clock
		}
	}
}

type PermissionService struct {
	repository  PermissionRepository
	idGenerator func() string
	now         func() time.Time
}

func NewPermissionService(repository PermissionRepository, opts ...PermissionOption) *PermissionService {
	service := &PermissionService{
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

func (s *PermissionService) SavePermission(ctx context.Context, input permission.SaveInput) (permission.Permission, error) {
	if err := input.Validate(); err != nil {
		return permission.Permission{}, err
	}

	now := s.now()
	model := permission.Permission{
		ID:               s.idGenerator(),
		WorkspaceID:      input.WorkspaceID,
		FunctionKey:      input.FunctionKey,
		CreatedAt:        now,
		UpdatedAt:        now,
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

func (s *PermissionService) GetPermission(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error) {
	if err := query.Validate(); err != nil {
		return permission.Permission{}, false, err
	}

	model, found, err := s.repository.Get(ctx, query)
	if err != nil {
		return permission.Permission{}, false, fmt.Errorf("get permissions: %w", err)
	}
	return model, found, nil
}
```

- [ ] **Step 5: Run service tests and confirm they pass**

Run:

```bash
go test ./internal/function-service/services -v
```

Expected: PASS.

- [ ] **Step 6: Commit service changes**

```bash
git add internal/function-service/services/permission_service.go internal/function-service/services/permission_service_test.go
git commit -m "feat: add permission service read workflow"
```

---

### Task 4: MongoDB Persistence Metadata and Read

**Files:**

- Modify: `internal/function-service/repositories/mongo_permission_repository.go`
- Modify: `internal/function-service/repositories/mongo_permission_repository_test.go`

- [ ] **Step 1: Write failing repository helper and mapping tests**

Replace `TestBuildPermissionUpdate` in `internal/function-service/repositories/mongo_permission_repository_test.go` with:

```go
func TestBuildPermissionUpdateSetsPermissionBodyAndTimestamps(t *testing.T) {
	updatedAt := time.Date(2026, 5, 9, 2, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 5, 9, 1, 0, 0, 0, time.UTC)
	doc := permissionDocument{
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
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

	pipeline := buildPermissionUpdate(doc)
	if len(pipeline) != 1 {
		t.Fatalf("pipeline length = %d, want 1", len(pipeline))
	}
	stage := pipeline[0]
	if len(stage) != 1 || stage[0].Key != "$set" {
		t.Fatalf("pipeline stage = %#v, want $set stage", stage)
	}
	set, ok := stage[0].Value.(bson.D)
	if !ok {
		t.Fatalf("$set = %#v, want bson.D", stage[0].Value)
	}
	if _, ok := bsonDValue(set, "office_permission").(permissionSectionDocument); !ok {
		t.Fatalf("$set office_permission missing or wrong type: %#v", set)
	}
	if _, ok := bsonDValue(set, "remote_permission").(permissionSectionDocument); !ok {
		t.Fatalf("$set remote_permission missing or wrong type: %#v", set)
	}
	if got, ok := bsonDValue(set, "updated_at").(time.Time); !ok || !got.Equal(updatedAt) {
		t.Fatalf("$set updated_at = %#v, want %s", bsonDValue(set, "updated_at"), updatedAt)
	}
	createdAtExpr, ok := bsonDValue(set, "created_at").(bson.D)
	if !ok {
		t.Fatalf("$set created_at = %#v, want $ifNull expression", bsonDValue(set, "created_at"))
	}
	if len(createdAtExpr) != 1 || createdAtExpr[0].Key != "$ifNull" {
		t.Fatalf("created_at expression = %#v, want $ifNull", createdAtExpr)
	}
}

func bsonDValue(doc bson.D, key string) any {
	for _, element := range doc {
		if element.Key == key {
			return element.Value
		}
	}
	return nil
}
```

Update `TestPermissionDocumentMapping` by setting and asserting timestamps:

```go
createdAt := time.Date(2026, 5, 9, 1, 0, 0, 0, time.UTC)
updatedAt := time.Date(2026, 5, 9, 2, 0, 0, 0, time.UTC)
model := permission.Permission{
	ID:          "permission-1",
	WorkspaceID: "workspace-1",
	FunctionKey: "todo",
	CreatedAt:   createdAt,
	UpdatedAt:   updatedAt,
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
```

Add this assertion after `got := doc.toDomain()`:

```go
if !got.CreatedAt.Equal(createdAt) || !got.UpdatedAt.Equal(updatedAt) {
	t.Fatalf("timestamps = %s/%s, want %s/%s", got.CreatedAt, got.UpdatedAt, createdAt, updatedAt)
}
```

- [ ] **Step 2: Run repository tests and confirm failure**

Run:

```bash
go test ./internal/function-service/repositories -run Permission -v
```

Expected: FAIL because permission documents do not include timestamps and `buildPermissionUpdate` does not return a pipeline.

- [ ] **Step 3: Implement timestamp document fields, update pipeline, and Get**

Update imports in `internal/function-service/repositories/mongo_permission_repository.go` to include `errors`.

Add timestamps to `permissionDocument`:

```go
type permissionDocument struct {
	ID               string                    `bson:"_id"`
	WorkspaceID      string                    `bson:"workspace_id"`
	FunctionKey      string                    `bson:"function_key"`
	CreatedAt        time.Time                 `bson:"created_at"`
	UpdatedAt        time.Time                 `bson:"updated_at"`
	OfficePermission permissionSectionDocument `bson:"office_permission"`
	RemotePermission permissionSectionDocument `bson:"remote_permission"`
}
```

Add the repository read method:

```go
func (r *MongoPermissionRepository) Get(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error) {
	model, found, err := r.findOptionalByWorkspaceFunction(ctx, query.WorkspaceID, query.FunctionKey)
	if err != nil {
		return permission.Permission{}, false, err
	}
	return model, found, nil
}
```

Replace `findByWorkspaceFunction` with these helpers:

```go
func (r *MongoPermissionRepository) findByWorkspaceFunction(ctx context.Context, workspaceID, functionKey string) (permission.Permission, error) {
	model, found, err := r.findOptionalByWorkspaceFunction(ctx, workspaceID, functionKey)
	if err != nil {
		return permission.Permission{}, err
	}
	if !found {
		return permission.Permission{}, fmt.Errorf("find permissions: document not found")
	}
	return model, nil
}

func (r *MongoPermissionRepository) findOptionalByWorkspaceFunction(ctx context.Context, workspaceID, functionKey string) (permission.Permission, bool, error) {
	var doc permissionDocument
	if err := r.collection.FindOne(ctx, buildPermissionFilter(workspaceID, functionKey)).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return permission.Permission{}, false, nil
		}
		return permission.Permission{}, false, fmt.Errorf("find permissions: %w", err)
	}
	return doc.toDomain(), true, nil
}
```

Change `buildPermissionUpdate` to preserve existing `created_at`, set missing legacy `created_at`, and refresh `updated_at`:

```go
func buildPermissionUpdate(doc permissionDocument) mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "office_permission", Value: doc.OfficePermission},
				{Key: "remote_permission", Value: doc.RemotePermission},
				{Key: "updated_at", Value: doc.UpdatedAt},
				{Key: "created_at", Value: bson.D{
					{Key: "$ifNull", Value: bson.A{"$created_at", doc.CreatedAt}},
				}},
			}},
		},
	}
}
```

Update `newPermissionDocument` and `toDomain` to map timestamps:

```go
func newPermissionDocument(model permission.Permission) permissionDocument {
	return permissionDocument{
		ID:               model.ID,
		WorkspaceID:      model.WorkspaceID,
		FunctionKey:      model.FunctionKey,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
		OfficePermission: newPermissionSectionDocument(model.OfficePermission),
		RemotePermission: newPermissionSectionDocument(model.RemotePermission),
	}
}

func (d permissionDocument) toDomain() permission.Permission {
	return permission.Permission{
		ID:               d.ID,
		WorkspaceID:      d.WorkspaceID,
		FunctionKey:      d.FunctionKey,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
		OfficePermission: d.OfficePermission.toDomain(),
		RemotePermission: d.RemotePermission.toDomain(),
	}
}
```

- [ ] **Step 4: Run repository tests and confirm they pass**

Run:

```bash
go test ./internal/function-service/repositories -v
```

Expected: PASS.

- [ ] **Step 5: Commit repository changes**

```bash
git add internal/function-service/repositories/mongo_permission_repository.go internal/function-service/repositories/mongo_permission_repository_test.go
git commit -m "feat: persist permission timestamps"
```

---

### Task 5: HTTP GET Route and Handler

**Files:**

- Modify: `internal/function-service/handlers/permission_handler.go`
- Modify: `internal/function-service/handlers/permission_handler_test.go`

- [ ] **Step 1: Extend the fake HTTP permission service**

Update `fakeHTTPPermissionService` in `internal/function-service/handlers/permission_handler_test.go`:

```go
type fakeHTTPPermissionService struct {
	input    permission.SaveInput
	query    permission.GetQuery
	model    permission.Permission
	found    bool
	err      error
	getErr   error
	calls    int
	getCalls int
}

func (f *fakeHTTPPermissionService) SavePermission(ctx context.Context, input permission.SaveInput) (permission.Permission, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return permission.Permission{}, f.err
	}
	return f.model, nil
}

func (f *fakeHTTPPermissionService) GetPermission(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error) {
	f.getCalls++
	f.query = query
	if f.getErr != nil {
		return permission.Permission{}, false, f.getErr
	}
	return f.model, f.found, nil
}
```

- [ ] **Step 2: Write failing handler tests for GET**

Append these tests to `internal/function-service/handlers/permission_handler_test.go`:

```go
func TestPermissionHandlerGetPermissionsFound(t *testing.T) {
	service := &fakeHTTPPermissionService{model: permissionModel(), found: true}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/functions/todo/permissions", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.getCalls != 1 {
		t.Fatalf("service get calls = %d, want 1", service.getCalls)
	}
	if service.query.WorkspaceID != "workspace-1" || service.query.FunctionKey != "todo" {
		t.Fatalf("query = %+v, want workspace-1/todo", service.query)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["permissions"] == nil {
		t.Fatal("permissions = nil, want object")
	}
}

func TestPermissionHandlerGetPermissionsNotFound(t *testing.T) {
	service := &fakeHTTPPermissionService{found: false}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/functions/todo/permissions", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["permissions"]; !ok {
		t.Fatal("response missing permissions key")
	}
	if body["permissions"] != nil {
		t.Fatalf("permissions = %#v, want nil", body["permissions"])
	}
}

func TestPermissionHandlerGetPermissionsServiceFailure(t *testing.T) {
	service := &fakeHTTPPermissionService{getErr: errors.New("database unavailable")}
	e := echo.New()
	RegisterPermissionRoutes(e, NewPermissionHandler(service, newTestLogger()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/functions/todo/permissions", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
```

- [ ] **Step 3: Run handler tests and confirm failure**

Run:

```bash
go test ./internal/function-service/handlers -run Permission -v
```

Expected: FAIL because the service interface and GET route are missing.

- [ ] **Step 4: Implement GET route and handler**

Update `HTTPPermissionService` in `internal/function-service/handlers/permission_handler.go`:

```go
type HTTPPermissionService interface {
	SavePermission(ctx context.Context, input permission.SaveInput) (permission.Permission, error)
	GetPermission(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error)
}
```

Update route registration:

```go
func RegisterPermissionRoutes(e *echo.Echo, handler *PermissionHandler) {
	e.PUT("/api/v1/workspaces/:workspace_id/functions/:function_key/permissions", handler.SavePermissions)
	e.GET("/api/v1/workspaces/:workspace_id/functions/:function_key/permissions", handler.GetPermissions)
}
```

Append this handler method:

```go
func (h *PermissionHandler) GetPermissions(c *echo.Context) error {
	params := newPermissionPathParams(c)
	model, found, err := h.service.GetPermission(c.Request().Context(), permission.GetQuery{
		WorkspaceID: params.workspaceID,
		FunctionKey: params.functionKey,
	})
	if err != nil {
		if errors.Is(err, permission.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to get permissions",
			"err", err,
			"workspace_id", params.workspaceID,
			"function_key", params.functionKey,
		)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	if !found {
		return c.JSON(http.StatusOK, transport.NewPermissionGetNotFoundResponse())
	}
	return c.JSON(http.StatusOK, transport.NewPermissionGetResponse(model))
}
```

- [ ] **Step 5: Run handler tests and confirm they pass**

Run:

```bash
go test ./internal/function-service/handlers -v
```

Expected: PASS.

- [ ] **Step 6: Commit handler changes**

```bash
git add internal/function-service/handlers/permission_handler.go internal/function-service/handlers/permission_handler_test.go
git commit -m "feat: add get permission route"
```

---

### Task 6: API Examples and Full Verification

**Files:**

- Modify: `examples/api/function_resource_permissions.http`

- [ ] **Step 1: Add GET examples to the REST Client file**

Append these requests to `examples/api/function_resource_permissions.http`:

```http
### Get saved permissions
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{functionKey}}/permissions

### Get permissions when no permission document exists; response is 200 with {"permissions": null}
@missingFunctionKey = no-permissions-yet
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/functions/{{missingFunctionKey}}/permissions
```

- [ ] **Step 2: Run targeted package tests**

Run:

```bash
go test ./internal/domain/permission ./internal/function-service/transport ./internal/function-service/services ./internal/function-service/repositories ./internal/function-service/handlers -v
```

Expected: PASS.

- [ ] **Step 3: Run full backend verification**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Check formatting and documentation diff**

Run:

```bash
gofmt -w internal/domain/permission/permission.go internal/domain/permission/validation.go internal/domain/permission/validation_test.go internal/function-service/transport/permission_response.go internal/function-service/transport/permission_response_test.go internal/function-service/services/permission_service.go internal/function-service/services/permission_service_test.go internal/function-service/repositories/mongo_permission_repository.go internal/function-service/repositories/mongo_permission_repository_test.go internal/function-service/handlers/permission_handler.go internal/function-service/handlers/permission_handler_test.go
git diff --check
```

Expected: `git diff --check` prints no output and exits with code 0.

- [ ] **Step 5: Review API contract checklist**

Confirm each item in this checklist against the final diff:

- `GET /api/v1/workspaces/:workspace_id/functions/:function_key/permissions` is registered.
- GET found response is `200 OK` with a non-null `permissions` object.
- GET not found response is `200 OK` with `permissions: null`.
- GET does not query `function_resources`.
- PUT insert persists `created_at` and `updated_at`.
- PUT replace preserves existing `created_at`, sets missing legacy `created_at`, and refreshes `updated_at`.
- Public responses do not include `created_at` or `updated_at`.
- `examples/api/function_resource_permissions.http` includes GET found and GET not-yet-configured examples.

- [ ] **Step 6: Commit examples and verification-ready state**

```bash
git add examples/api/function_resource_permissions.http
git commit -m "docs: add permission get api examples"
```

---

## Self-Review Notes

Spec coverage:

- GET API contract is covered by Tasks 2, 3, 4, 5, and 6.
- `permissions: null` not-found behavior is covered by Tasks 2, 3, 5, and 6.
- No `function_resources` lookup is enforced by the service/repository shape in Tasks 3 and 4.
- `created_at` and `updated_at` persistence is covered by Tasks 1, 3, and 4.
- Public response timestamp exclusion is covered by Task 2.
- API examples and full verification are covered by Task 6.

Placeholder scan:

- This plan contains no placeholder markers, deferred implementation notes, or unnamed edge handling instructions.

Type consistency:

- Domain query type is `permission.GetQuery`.
- Service method is `GetPermission(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error)`.
- Repository method is `Get(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error)`.
- Transport constructors are `NewPermissionGetResponse` and `NewPermissionGetNotFoundResponse`.
