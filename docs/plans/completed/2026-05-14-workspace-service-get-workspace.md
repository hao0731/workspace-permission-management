# Workspace Service Get Workspace API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `GET /api/v1/workspaces/:workspace_id` to `workspace-service`, returning an owner-enriched workspace when found and `{ "workspace": null }` with `200 OK` when missing.

**Architecture:** Keep the existing backend layering: Echo handlers parse path parameters and map errors, services orchestrate validation, persistence, and HR enrichment, repositories own MongoDB access, and transport owns HTTP response DTOs. The read path uses `workspaces._id` as the lookup key, calls HR only after a workspace is found, and never persists `owner.display_name`.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, `log/slog`, shared HR client interface, shared HTTP exception response helpers.

---

## Source Design Documents

- [Workspace Service Design](../../designs/workspace-service.md)
- [Workspace Service API Design](../../designs/workspace-service-api-design.md#get-workspace-api)
- [Backend Architecture Principle](../../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../../policies/design-and-plan-docs-policy.md)

## Policy Classification

This is backend implementation plus plan documentation work.

- Backend policy applies because the change adds a REST API, service workflow, repository read, validation, handler route, and backend tests.
- Design and plan docs policy applies because this implementation plan lives under `docs/plans/active/` and links to the source design documents.

## File Structure

- Modify `internal/domain/workspace/workspace.go`: add `GetQuery`.
- Modify `internal/domain/workspace/validation.go`: add `GetQuery.Normalize` and `GetQuery.Validate`.
- Modify `internal/domain/workspace/validation_test.go`: add get-query validation tests.
- Modify `internal/workspace-service/transport/workspace_response.go`: add nullable get response and share workspace response construction.
- Modify `internal/workspace-service/transport/workspace_response_test.go`: add found and missing get response tests.
- Modify `internal/workspace-service/repositories/mongo_workspace_repository.go`: add read-by-ID repository method and `_id` filter helper.
- Modify `internal/workspace-service/repositories/mongo_workspace_repository_test.go`: add filter, found-read, and missing-read tests.
- Modify `internal/workspace-service/services/workspace_service.go`: add repository `Get`, `GetWorkspaceResult`, and `GetWorkspace`.
- Modify `internal/workspace-service/services/workspace_service_test.go`: add service read orchestration tests.
- Modify `internal/workspace-service/handlers/workspace_handler.go`: add `GET /api/v1/workspaces/:workspace_id` route and error mapping.
- Modify `internal/workspace-service/handlers/workspace_handler_test.go`: add HTTP route tests for found, missing, validation, HR failure, and repository failure.
- Modify `examples/api/workspaces.http`: include executable GET examples with local defaults.

## API Contract

- Route: `GET /api/v1/workspaces/:workspace_id`
- Found status: `200 OK`
- Found body:

```json
{
  "workspace": {
    "id": "workspace-1",
    "name": "Workspace Name",
    "description": "Workspace description",
    "owner": {
      "nt_account": "user1",
      "display_name": "Test User 測試員"
    }
  }
}
```

- Missing status: `200 OK`
- Missing body:

```json
{
  "workspace": null
}
```

- HR failure for a found workspace: `502 Bad Gateway` with error code `hr_lookup_failed`.
- Repository failure: `500 Internal Server Error` with error code `internal_error`.
- Empty `workspace_id` after trimming: `400 Bad Request` with error code `validation_failed`.

### Task 1: Workspace Domain Get Query

**Files:**
- Modify: `internal/domain/workspace/workspace.go`
- Modify: `internal/domain/workspace/validation.go`
- Modify: `internal/domain/workspace/validation_test.go`

- [ ] **Step 1: Write failing get-query validation tests**

Add these tests to `internal/domain/workspace/validation_test.go`:

```go
func TestGetQueryNormalize(t *testing.T) {
	query := GetQuery{ID: " workspace-1 "}

	normalized := query.Normalize()

	if normalized.ID != "workspace-1" {
		t.Fatalf("Normalize().ID = %q, want workspace-1", normalized.ID)
	}
}

func TestGetQueryValidateRejectsEmptyID(t *testing.T) {
	query := GetQuery{ID: "   "}

	if err := query.Normalize().Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Validate() error = %v, want ErrInvalidInput", err)
	}
}

func TestGetQueryValidateAcceptsWorkspaceID(t *testing.T) {
	query := GetQuery{ID: "workspace-1"}

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}
```

- [ ] **Step 2: Run domain tests and verify they fail**

Run:

```bash
go test ./internal/domain/workspace -run 'TestGetQuery'
```

Expected: FAIL because `GetQuery` is undefined.

- [ ] **Step 3: Add the domain query model**

Add this type to `internal/domain/workspace/workspace.go` after `CreateInput`:

```go
type GetQuery struct {
	ID string
}
```

- [ ] **Step 4: Add get-query normalization and validation**

Add these methods to `internal/domain/workspace/validation.go` before `invalidInput`:

```go
func (query GetQuery) Normalize() GetQuery {
	query.ID = strings.TrimSpace(query.ID)
	return query
}

func (query GetQuery) Validate() error {
	normalized := query.Normalize()
	if normalized.ID == "" {
		return invalidInput("workspace id is required")
	}
	return nil
}
```

- [ ] **Step 5: Format and run domain tests**

Run:

```bash
gofmt -w internal/domain/workspace/workspace.go internal/domain/workspace/validation.go internal/domain/workspace/validation_test.go
go test ./internal/domain/workspace
```

Expected: PASS.

- [ ] **Step 6: Commit the domain query contract**

Run:

```bash
git add internal/domain/workspace/workspace.go internal/domain/workspace/validation.go internal/domain/workspace/validation_test.go
git commit -m "feat: add workspace get query validation"
```

### Task 2: Nullable Workspace Get Response DTO

**Files:**
- Modify: `internal/workspace-service/transport/workspace_response.go`
- Modify: `internal/workspace-service/transport/workspace_response_test.go`

- [ ] **Step 1: Write failing transport response tests**

Add these tests to `internal/workspace-service/transport/workspace_response_test.go`:

```go
func TestNewWorkspaceGetResponse(t *testing.T) {
	response := NewWorkspaceGetResponse(workspace.Workspace{
		ID:             " workspace-1 ",
		Name:           " Planning ",
		Description:    " Planning workspace ",
		OwnerNTAccount: " user1 ",
	}, domainhr.User{NTAccount: " user1 ", DisplayName: " Test User 測試員 "})

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	want := `{"workspace":{"id":"workspace-1","name":"Planning","description":"Planning workspace","owner":{"nt_account":"user1","display_name":"Test User 測試員"}}}`
	if string(data) != want {
		t.Fatalf("body = %s, want %s", data, want)
	}
}

func TestNewWorkspaceGetNotFoundResponse(t *testing.T) {
	response := NewWorkspaceGetNotFoundResponse()

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	want := `{"workspace":null}`
	if string(data) != want {
		t.Fatalf("body = %s, want %s", data, want)
	}
}
```

- [ ] **Step 2: Run transport tests and verify they fail**

Run:

```bash
go test ./internal/workspace-service/transport -run 'TestNewWorkspaceGet'
```

Expected: FAIL because `NewWorkspaceGetResponse` and `NewWorkspaceGetNotFoundResponse` are undefined.

- [ ] **Step 3: Add nullable get response DTOs**

Modify `internal/workspace-service/transport/workspace_response.go` so the response types and constructors are:

```go
type WorkspaceCreateResponse struct {
	Workspace WorkspaceResponse `json:"workspace"`
}

type WorkspaceGetResponse struct {
	Workspace *WorkspaceResponse `json:"workspace"`
}

type WorkspaceResponse struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Owner       OwnerResponse `json:"owner"`
}

type OwnerResponse struct {
	NTAccount   string `json:"nt_account"`
	DisplayName string `json:"display_name"`
}

func NewWorkspaceCreateResponse(workspace workspace.Workspace, owner domainhr.User) WorkspaceCreateResponse {
	return WorkspaceCreateResponse{Workspace: newWorkspaceResponse(workspace, owner)}
}

func NewWorkspaceGetResponse(workspace workspace.Workspace, owner domainhr.User) WorkspaceGetResponse {
	response := newWorkspaceResponse(workspace, owner)
	return WorkspaceGetResponse{Workspace: &response}
}

func NewWorkspaceGetNotFoundResponse() WorkspaceGetResponse {
	return WorkspaceGetResponse{}
}

func newWorkspaceResponse(workspace workspace.Workspace, owner domainhr.User) WorkspaceResponse {
	workspace = workspace.Normalize()
	owner = owner.Normalize()
	return WorkspaceResponse{
		ID:          workspace.ID,
		Name:        workspace.Name,
		Description: workspace.Description,
		Owner: OwnerResponse{
			NTAccount:   owner.NTAccount,
			DisplayName: owner.DisplayName,
		},
	}
}
```

- [ ] **Step 4: Format and run transport tests**

Run:

```bash
gofmt -w internal/workspace-service/transport/workspace_response.go internal/workspace-service/transport/workspace_response_test.go
go test ./internal/workspace-service/transport
```

Expected: PASS.

- [ ] **Step 5: Commit the response contract**

Run:

```bash
git add internal/workspace-service/transport/workspace_response.go internal/workspace-service/transport/workspace_response_test.go
git commit -m "feat: add nullable workspace get response"
```

### Task 3: Mongo Workspace Repository Read

**Files:**
- Modify: `internal/workspace-service/repositories/mongo_workspace_repository.go`
- Modify: `internal/workspace-service/repositories/mongo_workspace_repository_test.go`

- [ ] **Step 1: Write failing repository tests**

Add these tests to `internal/workspace-service/repositories/mongo_workspace_repository_test.go`:

```go
func TestWorkspaceIDFilter(t *testing.T) {
	filter := workspaceIDFilter(workspace.GetQuery{ID: "workspace-1"})

	if filter["_id"] != "workspace-1" {
		t.Fatalf("filter = %#v, want _id workspace-1", filter)
	}
}

func TestMongoWorkspaceRepositoryGetIntegration(t *testing.T) {
	db := newIntegrationDatabase(t)
	repo := NewMongoWorkspaceRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	if _, err := repo.Create(ctx, workspace.Workspace{
		ID:             "workspace-1",
		Name:           "Planning",
		Description:    "Planning workspace",
		OwnerNTAccount: "user1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, found, err := repo.Get(ctx, workspace.GetQuery{ID: " workspace-1 "})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found {
		t.Fatal("Get() found = false, want true")
	}
	if got.ID != "workspace-1" || got.Name != "Planning" || got.OwnerNTAccount != "user1" {
		t.Fatalf("Get() = %+v", got)
	}
}

func TestMongoWorkspaceRepositoryGetMissingIntegration(t *testing.T) {
	db := newIntegrationDatabase(t)
	repo := NewMongoWorkspaceRepository(db)

	got, found, err := repo.Get(context.Background(), workspace.GetQuery{ID: "missing-workspace"})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if found {
		t.Fatalf("Get() found = true with workspace %+v, want false", got)
	}
}
```

- [ ] **Step 2: Run repository tests and verify they fail**

Run:

```bash
go test ./internal/workspace-service/repositories -run 'TestWorkspaceIDFilter|TestMongoWorkspaceRepositoryGet'
```

Expected: FAIL because `workspaceIDFilter` and repository `Get` are undefined. If `WORKSPACE_SERVICE_MONGODB_TEST_URI` is unset, the integration tests skip after the compile failure is fixed.

- [ ] **Step 3: Add repository read behavior**

Modify the imports in `internal/workspace-service/repositories/mongo_workspace_repository.go` to include `errors`:

```go
import (
	"context"
	"errors"
	"fmt"
	"time"
)
```

Add this method after `Create`:

```go
func (r *MongoWorkspaceRepository) Get(ctx context.Context, query workspace.GetQuery) (workspace.Workspace, bool, error) {
	query = query.Normalize()
	if err := query.Validate(); err != nil {
		return workspace.Workspace{}, false, err
	}

	var doc workspaceDocument
	if err := r.collection.FindOne(ctx, workspaceIDFilter(query)).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return workspace.Workspace{}, false, nil
		}
		return workspace.Workspace{}, false, fmt.Errorf("find workspace: %w", err)
	}
	return doc.toDomain(), true, nil
}
```

Add this helper before `workspaceIndexModel`:

```go
func workspaceIDFilter(query workspace.GetQuery) bson.M {
	query = query.Normalize()
	return bson.M{"_id": query.ID}
}
```

- [ ] **Step 4: Format and run repository tests**

Run:

```bash
gofmt -w internal/workspace-service/repositories/mongo_workspace_repository.go internal/workspace-service/repositories/mongo_workspace_repository_test.go
go test ./internal/workspace-service/repositories
```

Expected: PASS. Integration tests that require MongoDB may SKIP when `WORKSPACE_SERVICE_MONGODB_TEST_URI` is not set.

- [ ] **Step 5: Commit repository read behavior**

Run:

```bash
git add internal/workspace-service/repositories/mongo_workspace_repository.go internal/workspace-service/repositories/mongo_workspace_repository_test.go
git commit -m "feat: add workspace repository get"
```

### Task 4: Workspace Service Get Workflow

**Files:**
- Modify: `internal/workspace-service/services/workspace_service.go`
- Modify: `internal/workspace-service/services/workspace_service_test.go`

- [ ] **Step 1: Update the fake repository in service tests**

Replace the `fakeWorkspaceRepository` type in `internal/workspace-service/services/workspace_service_test.go` with:

```go
type fakeWorkspaceRepository struct {
	input        workspace.Workspace
	calls        int
	err          error
	getQuery     workspace.GetQuery
	getCalls     int
	getWorkspace workspace.Workspace
	getFound     bool
	getErr       error
}

func (f *fakeWorkspaceRepository) Create(_ context.Context, input workspace.Workspace) (workspace.Workspace, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return workspace.Workspace{}, f.err
	}
	return input, nil
}

func (f *fakeWorkspaceRepository) Get(_ context.Context, query workspace.GetQuery) (workspace.Workspace, bool, error) {
	f.getCalls++
	f.getQuery = query
	if f.getErr != nil {
		return workspace.Workspace{}, false, f.getErr
	}
	return f.getWorkspace, f.getFound, nil
}
```

- [ ] **Step 2: Write failing service workflow tests**

Add these tests to `internal/workspace-service/services/workspace_service_test.go`:

```go
func TestWorkspaceServiceGetWorkspaceFound(t *testing.T) {
	repo := &fakeWorkspaceRepository{
		getWorkspace: workspace.Workspace{
			ID:             "workspace-1",
			Name:           "Planning",
			Description:    "Planning workspace",
			OwnerNTAccount: "user1",
		},
		getFound: true,
	}
	hrClient := &fakeHRClient{user: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"}}
	service := NewWorkspaceService(repo, hrClient, nil)

	result, err := service.GetWorkspace(context.Background(), workspace.GetQuery{ID: " workspace-1 "})
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if !result.Found {
		t.Fatal("GetWorkspace() Found = false, want true")
	}
	if result.Workspace.ID != "workspace-1" || result.Owner.DisplayName != "Test User 測試員" {
		t.Fatalf("result = %+v", result)
	}
	if repo.getQuery.ID != "workspace-1" {
		t.Fatalf("repo query = %+v, want trimmed workspace id", repo.getQuery)
	}
	if hrClient.calls != 1 || hrClient.input != "user1" {
		t.Fatalf("hr calls=%d input=%q, want 1/user1", hrClient.calls, hrClient.input)
	}
}

func TestWorkspaceServiceGetWorkspaceMissingDoesNotCallHR(t *testing.T) {
	repo := &fakeWorkspaceRepository{getFound: false}
	hrClient := &fakeHRClient{user: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"}}
	service := NewWorkspaceService(repo, hrClient, nil)

	result, err := service.GetWorkspace(context.Background(), workspace.GetQuery{ID: "missing-workspace"})
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if result.Found {
		t.Fatalf("GetWorkspace() Found = true with result %+v, want false", result)
	}
	if hrClient.calls != 0 {
		t.Fatalf("hr calls = %d, want 0", hrClient.calls)
	}
}

func TestWorkspaceServiceGetWorkspaceHRFailure(t *testing.T) {
	repo := &fakeWorkspaceRepository{
		getWorkspace: workspace.Workspace{
			ID:             "workspace-1",
			Name:           "Planning",
			Description:    "Planning workspace",
			OwnerNTAccount: "user1",
		},
		getFound: true,
	}
	hrClient := &fakeHRClient{err: errors.New("hr unavailable")}
	service := NewWorkspaceService(repo, hrClient, nil)

	_, err := service.GetWorkspace(context.Background(), workspace.GetQuery{ID: "workspace-1"})
	if !errors.Is(err, ErrHRLookupFailed) {
		t.Fatalf("GetWorkspace() error = %v, want ErrHRLookupFailed", err)
	}
}

func TestWorkspaceServiceGetWorkspaceRepositoryFailure(t *testing.T) {
	repo := &fakeWorkspaceRepository{getErr: errors.New("find failed")}
	hrClient := &fakeHRClient{}
	service := NewWorkspaceService(repo, hrClient, nil)

	_, err := service.GetWorkspace(context.Background(), workspace.GetQuery{ID: "workspace-1"})
	if err == nil {
		t.Fatal("GetWorkspace() error = nil, want error")
	}
	if hrClient.calls != 0 {
		t.Fatalf("hr calls = %d, want 0", hrClient.calls)
	}
}
```

- [ ] **Step 3: Run service tests and verify they fail**

Run:

```bash
go test ./internal/workspace-service/services -run 'TestWorkspaceServiceGetWorkspace'
```

Expected: FAIL because `GetWorkspace` and `GetWorkspaceResult` are undefined, and `WorkspaceRepository` does not require `Get`.

- [ ] **Step 4: Add service get workflow**

Modify the repository interface in `internal/workspace-service/services/workspace_service.go`:

```go
type WorkspaceRepository interface {
	Create(ctx context.Context, input workspace.Workspace) (workspace.Workspace, error)
	Get(ctx context.Context, query workspace.GetQuery) (workspace.Workspace, bool, error)
}
```

Add this result type after `CreateWorkspaceResult`:

```go
type GetWorkspaceResult struct {
	Workspace workspace.Workspace
	Owner     domainhr.User
	Found     bool
}
```

Add this method after `CreateWorkspace`:

```go
func (s *WorkspaceService) GetWorkspace(ctx context.Context, query workspace.GetQuery) (GetWorkspaceResult, error) {
	query = query.Normalize()
	if err := query.Validate(); err != nil {
		return GetWorkspaceResult{}, err
	}

	model, found, err := s.repository.Get(ctx, query)
	if err != nil {
		return GetWorkspaceResult{}, fmt.Errorf("get workspace: %w", err)
	}
	if !found {
		return GetWorkspaceResult{Found: false}, nil
	}

	owner, err := s.hrClient.Get(ctx, model.OwnerNTAccount)
	if err != nil {
		return GetWorkspaceResult{}, fmt.Errorf("%w: %w", ErrHRLookupFailed, err)
	}
	return GetWorkspaceResult{Workspace: model, Owner: owner.Normalize(), Found: true}, nil
}
```

- [ ] **Step 5: Format and run service tests**

Run:

```bash
gofmt -w internal/workspace-service/services/workspace_service.go internal/workspace-service/services/workspace_service_test.go
go test ./internal/workspace-service/services
```

Expected: PASS.

- [ ] **Step 6: Commit service workflow**

Run:

```bash
git add internal/workspace-service/services/workspace_service.go internal/workspace-service/services/workspace_service_test.go
git commit -m "feat: add workspace get service workflow"
```

### Task 5: Workspace HTTP GET Route

**Files:**
- Modify: `internal/workspace-service/handlers/workspace_handler.go`
- Modify: `internal/workspace-service/handlers/workspace_handler_test.go`

- [ ] **Step 1: Update the fake HTTP service in handler tests**

Replace `fakeHTTPWorkspaceService` in `internal/workspace-service/handlers/workspace_handler_test.go` with:

```go
type fakeHTTPWorkspaceService struct {
	result    services.CreateWorkspaceResult
	err       error
	input     workspace.CreateInput
	getResult services.GetWorkspaceResult
	getErr    error
	getInput  workspace.GetQuery
}

func (f *fakeHTTPWorkspaceService) CreateWorkspace(_ context.Context, input workspace.CreateInput) (services.CreateWorkspaceResult, error) {
	f.input = input
	if f.err != nil {
		return services.CreateWorkspaceResult{}, f.err
	}
	return f.result, nil
}

func (f *fakeHTTPWorkspaceService) GetWorkspace(_ context.Context, input workspace.GetQuery) (services.GetWorkspaceResult, error) {
	f.getInput = input
	if f.getErr != nil {
		return services.GetWorkspaceResult{}, f.getErr
	}
	return f.getResult, nil
}
```

- [ ] **Step 2: Write failing handler route tests**

Add these tests to `internal/workspace-service/handlers/workspace_handler_test.go`:

```go
func TestWorkspaceHandlerGetWorkspace(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{
		getResult: services.GetWorkspaceResult{
			Found: true,
			Workspace: workspace.Workspace{
				ID:             "workspace-1",
				Name:           "Planning",
				Description:    "Planning workspace",
				OwnerNTAccount: "user1",
			},
			Owner: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"},
		},
	}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"workspace-1"`) || !strings.Contains(rec.Body.String(), `"display_name":"Test User 測試員"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if service.getInput.ID != "workspace-1" {
		t.Fatalf("service get input = %+v", service.getInput)
	}
}

func TestWorkspaceHandlerGetMissingWorkspace(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{
		getResult: services.GetWorkspaceResult{Found: false},
	}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/missing-workspace", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != `{"workspace":null}` {
		t.Fatalf("body = %s, want workspace null", rec.Body.String())
	}
}

func TestWorkspaceHandlerGetWorkspaceMapsInvalidInput(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{getErr: workspace.ErrInvalidInput}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/bad-workspace", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandlerGetWorkspaceMapsHRLookupFailure(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{getErr: services.ErrHRLookupFailed}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), `"code":"hr_lookup_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandlerGetWorkspaceMapsUnexpectedError(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{getErr: errors.New("database down")}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 3: Run handler tests and verify they fail**

Run:

```bash
go test ./internal/workspace-service/handlers -run 'TestWorkspaceHandlerGetWorkspace|TestWorkspaceHandlerGetMissingWorkspace'
```

Expected: FAIL because the GET route is not registered and the handler interface does not expose `GetWorkspace`.

- [ ] **Step 4: Add the HTTP route and handler**

Modify `HTTPWorkspaceService` in `internal/workspace-service/handlers/workspace_handler.go`:

```go
type HTTPWorkspaceService interface {
	CreateWorkspace(ctx context.Context, input workspace.CreateInput) (services.CreateWorkspaceResult, error)
	GetWorkspace(ctx context.Context, input workspace.GetQuery) (services.GetWorkspaceResult, error)
}
```

Modify `RegisterRoutes`:

```go
func RegisterRoutes(e *echo.Echo, handler *WorkspaceHandler) {
	e.POST("/api/v1/workspaces", handler.CreateWorkspace)
	e.GET("/api/v1/workspaces/:workspace_id", handler.GetWorkspace)
}
```

Add this method after `CreateWorkspace`:

```go
func (h *WorkspaceHandler) GetWorkspace(c *echo.Context) error {
	workspaceID := c.Param("workspace_id")
	result, err := h.service.GetWorkspace(c.Request().Context(), workspace.GetQuery{ID: workspaceID})
	if err != nil {
		if errors.Is(err, workspace.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, services.ErrHRLookupFailed) {
			return c.JSON(http.StatusBadGateway, exception.WrapResponse(exception.New("hr_lookup_failed", "Failed to resolve workspace owner")))
		}
		h.logger.Warn("failed to get workspace", "err", err, "workspace_id", workspaceID)
		return c.JSON(http.StatusInternalServerError, internalError())
	}
	if !result.Found {
		return c.JSON(http.StatusOK, transport.NewWorkspaceGetNotFoundResponse())
	}
	return c.JSON(http.StatusOK, transport.NewWorkspaceGetResponse(result.Workspace, result.Owner))
}
```

- [ ] **Step 5: Format and run handler tests**

Run:

```bash
gofmt -w internal/workspace-service/handlers/workspace_handler.go internal/workspace-service/handlers/workspace_handler_test.go
go test ./internal/workspace-service/handlers
```

Expected: PASS.

- [ ] **Step 6: Commit HTTP route**

Run:

```bash
git add internal/workspace-service/handlers/workspace_handler.go internal/workspace-service/handlers/workspace_handler_test.go
git commit -m "feat: add workspace get route"
```

### Task 6: REST Client Examples

**Files:**
- Modify: `examples/api/workspaces.http`

- [ ] **Step 1: Update workspace REST examples**

Ensure `examples/api/workspaces.http` starts with these variables:

```http
@baseUrl = http://localhost:8083
@workspaceId = workspace-1
```

Ensure the file includes these requests after the create examples:

```http
### Get workspace by ID
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}

### Get missing workspace returns workspace null
GET {{baseUrl}}/api/v1/workspaces/missing-workspace
```

- [ ] **Step 2: Verify the example file contains GET requests**

Run:

```bash
rg 'GET \{\{baseUrl\}\}/api/v1/workspaces' examples/api/workspaces.http
```

Expected output includes:

```text
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}
GET {{baseUrl}}/api/v1/workspaces/missing-workspace
```

- [ ] **Step 3: Commit REST examples**

Run:

```bash
git add examples/api/workspaces.http
git commit -m "docs: add workspace get REST examples"
```

### Task 7: Final Verification

**Files:**
- Verify all files changed by Tasks 1 through 6.

- [ ] **Step 1: Run focused workspace tests**

Run:

```bash
go test ./internal/domain/workspace ./internal/workspace-service/transport ./internal/workspace-service/repositories ./internal/workspace-service/services ./internal/workspace-service/handlers ./cmd/workspace-service
```

Expected: PASS. Repository integration tests may SKIP when `WORKSPACE_SERVICE_MONGODB_TEST_URI` is unset.

- [ ] **Step 2: Run repository-wide tests**

Run:

```bash
go test ./...
```

Expected: PASS. Any skipped MongoDB integration tests must be reported with the environment variable they require.

- [ ] **Step 3: Verify API contract text and examples**

Run:

```bash
rg 'GET /api/v1/workspaces/:workspace_id|workspace: null|hr_lookup_failed' docs/designs/workspace-service.md docs/designs/workspace-service-api-design.md
rg 'GET \{\{baseUrl\}\}/api/v1/workspaces' examples/api/workspaces.http
```

Expected: the first command finds the GET route, nullable missing response, and HR failure mapping in the design docs. The second command finds both found and missing REST Client examples.

- [ ] **Step 4: Verify formatting and whitespace**

Run:

```bash
git diff --check
```

Expected: no output and exit code 0.

- [ ] **Step 5: Commit final verification metadata if files changed**

If Task 7 required any file corrections, run:

```bash
git add internal/domain/workspace internal/workspace-service examples/api/workspaces.http docs/designs/workspace-service.md docs/designs/workspace-service-api-design.md
git commit -m "chore: verify workspace get API implementation"
```

If Task 7 did not require file corrections, do not create an empty commit.

## Execution Notes

- Keep `owner.display_name` out of MongoDB documents.
- Do not call HR when the workspace repository returns `found == false`.
- Do not publish resource-create commands during GET requests.
- Do not add list, update, delete, archive, search, workspace existence checks in other services, or frontend changes.
- Keep service interfaces at the consumer side, matching the existing `workspace-service` pattern.
- Treat `workspace: null` as a successful read result, not as an error.
