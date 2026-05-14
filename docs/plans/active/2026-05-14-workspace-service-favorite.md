# Workspace Service Favorite API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `POST /api/v1/workspaces/:workspace_id/favorite` to let the current user set or clear a workspace favorite.

**Architecture:** Keep the existing backend layering: transport decodes the JSON body, handlers extract path/header data and map errors, services validate and orchestrate workspace existence plus favorite persistence, and repositories own MongoDB collections and indexes. `MongoWorkspaceRepository` will manage both `workspaces` and `user_favorite_workspaces` because the favorite workflow must verify workspace existence and then write user-specific state in the same service-owned persistence boundary.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, `log/slog`, shared HTTP exception helpers, REST Client `.http` examples.

---

## Source Design Documents

- [Workspace Service Design](../../designs/workspace-service.md)
- [Workspace Service API Design](../../designs/workspace-service-api-design.md#favorite-workspace-api)
- [Backend Architecture Principle](../../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../../policies/design-and-plan-docs-policy.md)

## Policy Classification

This is backend implementation plus plan documentation work.

- Backend policy applies because this adds a REST API, transport DTO, handler route, service workflow, MongoDB collection/index contract, REST Client example, and backend tests.
- Design and plan docs policy applies because this implementation plan lives under `docs/plans/active/` and links to the source design documents.

## File Structure

- Modify `internal/domain/workspace/errors.go`: add `ErrNotFound` for favorite mutation targets.
- Modify `internal/domain/workspace/workspace.go`: add `FavoriteInput` and `UserFavoriteWorkspace`.
- Modify `internal/domain/workspace/validation.go`: add normalization and validation for favorite input and favorite document models.
- Modify `internal/domain/workspace/validation_test.go`: add favorite validation tests.
- Modify `internal/workspace-service/transport/workspace_request.go`: add favorite request DTO decoding and mapping.
- Modify `internal/workspace-service/transport/workspace_request_test.go`: add favorite request decode and validation tests.
- Modify `internal/workspace-service/repositories/mongo_workspace_repository.go`: add `user_favorite_workspaces` collection, unique index, favorite document mapping, upsert, and delete methods.
- Modify `internal/workspace-service/repositories/mongo_workspace_repository_test.go`: add favorite index/filter/unit tests and Mongo integration tests.
- Modify `internal/workspace-service/services/workspace_service.go`: add favorite repository methods to the service persistence interface and implement `SetWorkspaceFavorite`.
- Modify `internal/workspace-service/services/workspace_service_test.go`: add favorite workflow tests.
- Modify `internal/workspace-service/handlers/workspace_handler.go`: register the favorite route, decode `X-User-Id`, and map favorite errors.
- Modify `internal/workspace-service/handlers/workspace_handler_test.go`: add favorite HTTP contract tests.
- Modify `examples/api/workspaces.http`: add executable favorite success and validation examples.

## API Contract

- Route: `POST /api/v1/workspaces/:workspace_id/favorite`
- Required header: `X-User-Id: <nt_account>`
- Body:

```json
{
  "favorite": true
}
```

- `favorite: true`: create or update one `user_favorite_workspaces` document for `nt_account + workspace_id`.
- `favorite: false`: delete the matching favorite document.
- Success status: `204 No Content` with no body.
- Clearing an absent favorite document: `204 No Content`.
- Missing workspace: `404 Not Found` with error code `workspace_not_found`.
- Validation failure: `400 Bad Request` with error code `validation_failed`.
- MongoDB favorite upsert/delete failure: `500 Internal Server Error` with error code `internal_error`.
- No HR lookup is performed by this endpoint.

## MongoDB Contract

Collection: `user_favorite_workspaces`

```ts
{
  "_id": string,
  "nt_account": string,
  "workspace_id": string,
  "created_at": Date,
  "updated_at": Date
}
```

Index:

```txt
unique { nt_account: 1, workspace_id: 1 }
```

Repeated `favorite: true` keeps the original `created_at` and refreshes `updated_at`. `favorite: false` hard-deletes the document and treats zero deleted rows as success.

### Task 1: Domain Favorite Contract

**Files:**
- Modify: `internal/domain/workspace/errors.go`
- Modify: `internal/domain/workspace/workspace.go`
- Modify: `internal/domain/workspace/validation.go`
- Modify: `internal/domain/workspace/validation_test.go`

- [ ] **Step 1: Write failing favorite domain tests**

Update the imports in `internal/domain/workspace/validation_test.go`:

```go
import (
	"errors"
	"testing"
	"time"
)
```

Add these tests to `internal/domain/workspace/validation_test.go`:

```go
func TestFavoriteInputNormalize(t *testing.T) {
	input := FavoriteInput{
		WorkspaceID: " workspace-1 ",
		NTAccount:   " user1 ",
		Favorite:    false,
	}

	normalized := input.Normalize()

	if normalized.WorkspaceID != "workspace-1" || normalized.NTAccount != "user1" {
		t.Fatalf("Normalize() = %+v, want trimmed workspace/user", normalized)
	}
	if normalized.Favorite {
		t.Fatal("Normalize().Favorite = true, want false preserved")
	}
}

func TestFavoriteInputValidateRejectsEmptyIdentity(t *testing.T) {
	tests := []FavoriteInput{
		{NTAccount: "user1", Favorite: true},
		{WorkspaceID: "workspace-1", Favorite: true},
		{WorkspaceID: "   ", NTAccount: "user1", Favorite: true},
		{WorkspaceID: "workspace-1", NTAccount: "   ", Favorite: true},
	}
	for _, input := range tests {
		if err := input.Normalize().Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate() error = %v, want ErrInvalidInput for input %+v", err, input)
		}
	}
}

func TestFavoriteInputValidateAcceptsFavoriteFalse(t *testing.T) {
	input := FavoriteInput{WorkspaceID: "workspace-1", NTAccount: "user1", Favorite: false}

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestUserFavoriteWorkspaceNormalize(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	model := UserFavoriteWorkspace{
		ID:          " favorite-1 ",
		NTAccount:   " user1 ",
		WorkspaceID: " workspace-1 ",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	normalized := model.Normalize()

	if normalized.ID != "favorite-1" || normalized.NTAccount != "user1" || normalized.WorkspaceID != "workspace-1" {
		t.Fatalf("Normalize() = %+v, want trimmed identity fields", normalized)
	}
}

func TestUserFavoriteWorkspaceValidateRejectsMissingFields(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	tests := []UserFavoriteWorkspace{
		{NTAccount: "user1", WorkspaceID: "workspace-1", CreatedAt: now, UpdatedAt: now},
		{ID: "favorite-1", WorkspaceID: "workspace-1", CreatedAt: now, UpdatedAt: now},
		{ID: "favorite-1", NTAccount: "user1", CreatedAt: now, UpdatedAt: now},
		{ID: "favorite-1", NTAccount: "user1", WorkspaceID: "workspace-1", UpdatedAt: now},
		{ID: "favorite-1", NTAccount: "user1", WorkspaceID: "workspace-1", CreatedAt: now},
	}
	for _, model := range tests {
		if err := model.Normalize().Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate() error = %v, want ErrInvalidInput for model %+v", err, model)
		}
	}
}
```

- [ ] **Step 2: Run domain tests and verify they fail**

Run:

```bash
go test ./internal/domain/workspace -run 'TestFavoriteInput|TestUserFavoriteWorkspace'
```

Expected: FAIL because `FavoriteInput` and `UserFavoriteWorkspace` are undefined.

- [ ] **Step 3: Add the missing workspace domain error**

Update `internal/domain/workspace/errors.go`:

```go
package workspace

import "errors"

var (
	ErrInvalidInput = errors.New("invalid workspace input")
	ErrNotFound     = errors.New("workspace not found")
)
```

- [ ] **Step 4: Add favorite domain models**

Add these types to `internal/domain/workspace/workspace.go` after `GetQuery`:

```go
type FavoriteInput struct {
	WorkspaceID string
	NTAccount   string
	Favorite    bool
}

type UserFavoriteWorkspace struct {
	ID          string
	NTAccount   string
	WorkspaceID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

- [ ] **Step 5: Add favorite normalization and validation**

Add these methods to `internal/domain/workspace/validation.go` before `invalidInput`:

```go
func (input FavoriteInput) Normalize() FavoriteInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.NTAccount = strings.TrimSpace(input.NTAccount)
	return input
}

func (input FavoriteInput) Validate() error {
	normalized := input.Normalize()
	if normalized.WorkspaceID == "" {
		return invalidInput("workspace id is required")
	}
	if normalized.NTAccount == "" {
		return invalidInput("nt account is required")
	}
	return nil
}

func (w UserFavoriteWorkspace) Normalize() UserFavoriteWorkspace {
	w.ID = strings.TrimSpace(w.ID)
	w.NTAccount = strings.TrimSpace(w.NTAccount)
	w.WorkspaceID = strings.TrimSpace(w.WorkspaceID)
	return w
}

func (w UserFavoriteWorkspace) Validate() error {
	normalized := w.Normalize()
	if normalized.ID == "" {
		return invalidInput("favorite id is required")
	}
	if normalized.NTAccount == "" {
		return invalidInput("nt account is required")
	}
	if normalized.WorkspaceID == "" {
		return invalidInput("workspace id is required")
	}
	if normalized.CreatedAt.IsZero() {
		return invalidInput("created at is required")
	}
	if normalized.UpdatedAt.IsZero() {
		return invalidInput("updated at is required")
	}
	return nil
}
```

- [ ] **Step 6: Format and run domain tests**

Run:

```bash
gofmt -w internal/domain/workspace/errors.go internal/domain/workspace/workspace.go internal/domain/workspace/validation.go internal/domain/workspace/validation_test.go
go test ./internal/domain/workspace
```

Expected: PASS.

- [ ] **Step 7: Commit the domain contract**

Run:

```bash
git add internal/domain/workspace/errors.go internal/domain/workspace/workspace.go internal/domain/workspace/validation.go internal/domain/workspace/validation_test.go
git commit -m "feat: add workspace favorite domain contract"
```

### Task 2: Favorite Request Transport DTO

**Files:**
- Modify: `internal/workspace-service/transport/workspace_request.go`
- Modify: `internal/workspace-service/transport/workspace_request_test.go`

- [ ] **Step 1: Write failing favorite request tests**

Add these tests to `internal/workspace-service/transport/workspace_request_test.go`:

```go
func TestDecodeWorkspaceFavoriteRequestTrue(t *testing.T) {
	request, err := DecodeWorkspaceFavoriteRequest(strings.NewReader(`{"favorite":true}`))
	if err != nil {
		t.Fatalf("DecodeWorkspaceFavoriteRequest() error = %v", err)
	}

	input, err := request.ToDomain(" workspace-1 ", " user1 ")
	if err != nil {
		t.Fatalf("ToDomain() error = %v", err)
	}
	if input.WorkspaceID != "workspace-1" || input.NTAccount != "user1" || !input.Favorite {
		t.Fatalf("input = %+v, want trimmed favorite true", input)
	}
}

func TestDecodeWorkspaceFavoriteRequestFalse(t *testing.T) {
	request, err := DecodeWorkspaceFavoriteRequest(strings.NewReader(`{"favorite":false}`))
	if err != nil {
		t.Fatalf("DecodeWorkspaceFavoriteRequest() error = %v", err)
	}

	input, err := request.ToDomain("workspace-1", "user1")
	if err != nil {
		t.Fatalf("ToDomain() error = %v", err)
	}
	if input.Favorite {
		t.Fatalf("Favorite = true, want false")
	}
}

func TestDecodeWorkspaceFavoriteRequestRejectsMalformedJSON(t *testing.T) {
	_, err := DecodeWorkspaceFavoriteRequest(strings.NewReader(`{"favorite":`))
	if err == nil {
		t.Fatal("DecodeWorkspaceFavoriteRequest() error = nil, want error")
	}
}

func TestDecodeWorkspaceFavoriteRequestRejectsNonBooleanFavorite(t *testing.T) {
	_, err := DecodeWorkspaceFavoriteRequest(strings.NewReader(`{"favorite":"yes"}`))
	if err == nil {
		t.Fatal("DecodeWorkspaceFavoriteRequest() error = nil, want error")
	}
}

func TestWorkspaceFavoriteRequestToDomainRejectsMissingFavorite(t *testing.T) {
	request, err := DecodeWorkspaceFavoriteRequest(strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("DecodeWorkspaceFavoriteRequest() error = %v", err)
	}
	if _, err := request.ToDomain("workspace-1", "user1"); err == nil {
		t.Fatal("ToDomain() error = nil, want error")
	}
}

func TestWorkspaceFavoriteRequestToDomainRejectsNullFavorite(t *testing.T) {
	request, err := DecodeWorkspaceFavoriteRequest(strings.NewReader(`{"favorite":null}`))
	if err != nil {
		t.Fatalf("DecodeWorkspaceFavoriteRequest() error = %v", err)
	}
	if _, err := request.ToDomain("workspace-1", "user1"); err == nil {
		t.Fatal("ToDomain() error = nil, want error")
	}
}

func TestWorkspaceFavoriteRequestToDomainRejectsMissingHeaderIdentity(t *testing.T) {
	request, err := DecodeWorkspaceFavoriteRequest(strings.NewReader(`{"favorite":true}`))
	if err != nil {
		t.Fatalf("DecodeWorkspaceFavoriteRequest() error = %v", err)
	}
	if _, err := request.ToDomain("workspace-1", " "); err == nil {
		t.Fatal("ToDomain() error = nil, want error")
	}
}
```

- [ ] **Step 2: Run transport tests and verify they fail**

Run:

```bash
go test ./internal/workspace-service/transport -run 'TestDecodeWorkspaceFavoriteRequest|TestWorkspaceFavoriteRequest'
```

Expected: FAIL because `DecodeWorkspaceFavoriteRequest` is undefined.

- [ ] **Step 3: Add favorite request DTO and mapper**

Add this type to `internal/workspace-service/transport/workspace_request.go` after `WorkspaceCreateRequest`:

```go
type WorkspaceFavoriteRequest struct {
	Favorite *bool `json:"favorite"`
}
```

Add these functions to `internal/workspace-service/transport/workspace_request.go` after `DecodeWorkspaceCreateRequest`:

```go
func DecodeWorkspaceFavoriteRequest(body io.Reader) (WorkspaceFavoriteRequest, error) {
	var request WorkspaceFavoriteRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return WorkspaceFavoriteRequest{}, fmt.Errorf("decode workspace favorite request: %w", err)
	}
	return request, nil
}

func (request WorkspaceFavoriteRequest) ToDomain(workspaceID string, ntAccount string) (workspace.FavoriteInput, error) {
	if request.Favorite == nil {
		return workspace.FavoriteInput{}, fmt.Errorf("%w: favorite is required", workspace.ErrInvalidInput)
	}
	input := workspace.FavoriteInput{
		WorkspaceID: workspaceID,
		NTAccount:   ntAccount,
		Favorite:    *request.Favorite,
	}.Normalize()
	if err := input.Validate(); err != nil {
		return workspace.FavoriteInput{}, err
	}
	return input, nil
}
```

- [ ] **Step 4: Format and run transport tests**

Run:

```bash
gofmt -w internal/workspace-service/transport/workspace_request.go internal/workspace-service/transport/workspace_request_test.go
go test ./internal/workspace-service/transport
```

Expected: PASS.

- [ ] **Step 5: Commit the transport DTO**

Run:

```bash
git add internal/workspace-service/transport/workspace_request.go internal/workspace-service/transport/workspace_request_test.go
git commit -m "feat: add workspace favorite request dto"
```

### Task 3: Favorite MongoDB Persistence

**Files:**
- Modify: `internal/workspace-service/repositories/mongo_workspace_repository.go`
- Modify: `internal/workspace-service/repositories/mongo_workspace_repository_test.go`

- [ ] **Step 1: Write failing favorite repository unit tests**

Add these tests to `internal/workspace-service/repositories/mongo_workspace_repository_test.go`:

```go
func TestUserFavoriteWorkspaceDocumentMapping(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	doc := userFavoriteWorkspaceDocument{
		ID:          "favorite-1",
		NTAccount:   "user1",
		WorkspaceID: "workspace-1",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	got := doc.toDomain()

	if got.ID != "favorite-1" || got.NTAccount != "user1" || got.WorkspaceID != "workspace-1" {
		t.Fatalf("toDomain() = %+v", got)
	}
}

func TestUserFavoriteWorkspaceFilter(t *testing.T) {
	filter := userFavoriteWorkspaceFilter(workspace.FavoriteInput{
		WorkspaceID: " workspace-1 ",
		NTAccount:   " user1 ",
	})

	if filter["workspace_id"] != "workspace-1" || filter["nt_account"] != "user1" {
		t.Fatalf("filter = %#v, want workspace/user", filter)
	}
}

func TestUserFavoriteWorkspaceUniqueIndexModel(t *testing.T) {
	index := userFavoriteWorkspaceUniqueIndexModel()
	keys, ok := index.Keys.(bson.D)
	if !ok {
		t.Fatalf("keys type = %T, want bson.D", index.Keys)
	}
	want := bson.D{
		{Key: "nt_account", Value: 1},
		{Key: "workspace_id", Value: 1},
	}
	if len(keys) != len(want) {
		t.Fatalf("keys = %#v, want %#v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys = %#v, want %#v", keys, want)
		}
	}
	if index.Options == nil {
		t.Fatal("Options = nil, want unique index options")
	}
}
```

- [ ] **Step 2: Write failing favorite repository integration tests**

Add these tests to `internal/workspace-service/repositories/mongo_workspace_repository_test.go`:

```go
func TestMongoWorkspaceRepositoryUpsertFavoriteInsertsIntegration(t *testing.T) {
	db := newIntegrationDatabase(t)
	repo := NewMongoWorkspaceRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)

	err := repo.UpsertFavorite(ctx, workspace.UserFavoriteWorkspace{
		ID:          "favorite-1",
		NTAccount:   " user1 ",
		WorkspaceID: " workspace-1 ",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		t.Fatalf("UpsertFavorite() error = %v, want nil", err)
	}

	var doc bson.M
	if err := db.Collection("user_favorite_workspaces").FindOne(ctx, bson.M{"nt_account": "user1", "workspace_id": "workspace-1"}).Decode(&doc); err != nil {
		t.Fatalf("find favorite: %v", err)
	}
	if doc["_id"] != "favorite-1" {
		t.Fatalf("_id = %v, want favorite-1", doc["_id"])
	}
}

func TestMongoWorkspaceRepositoryUpsertFavoriteUpdatesTimestampIntegration(t *testing.T) {
	db := newIntegrationDatabase(t)
	repo := NewMongoWorkspaceRepository(db)
	ctx := context.Background()
	createdAt := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	if err := repo.UpsertFavorite(ctx, workspace.UserFavoriteWorkspace{
		ID:          "favorite-1",
		NTAccount:   "user1",
		WorkspaceID: "workspace-1",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}); err != nil {
		t.Fatalf("initial UpsertFavorite() error = %v", err)
	}
	if err := repo.UpsertFavorite(ctx, workspace.UserFavoriteWorkspace{
		ID:          "favorite-2",
		NTAccount:   "user1",
		WorkspaceID: "workspace-1",
		CreatedAt:   updatedAt,
		UpdatedAt:   updatedAt,
	}); err != nil {
		t.Fatalf("second UpsertFavorite() error = %v", err)
	}

	var doc userFavoriteWorkspaceDocument
	if err := db.Collection("user_favorite_workspaces").FindOne(ctx, bson.M{"nt_account": "user1", "workspace_id": "workspace-1"}).Decode(&doc); err != nil {
		t.Fatalf("find favorite: %v", err)
	}
	if doc.ID != "favorite-1" {
		t.Fatalf("ID = %q, want original favorite-1", doc.ID)
	}
	if !doc.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt = %v, want %v", doc.CreatedAt, createdAt)
	}
	if !doc.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdatedAt = %v, want %v", doc.UpdatedAt, updatedAt)
	}
}

func TestMongoWorkspaceRepositoryDeleteFavoriteIntegration(t *testing.T) {
	db := newIntegrationDatabase(t)
	repo := NewMongoWorkspaceRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)

	if err := repo.UpsertFavorite(ctx, workspace.UserFavoriteWorkspace{
		ID:          "favorite-1",
		NTAccount:   "user1",
		WorkspaceID: "workspace-1",
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("UpsertFavorite() error = %v", err)
	}
	if err := repo.DeleteFavorite(ctx, workspace.FavoriteInput{WorkspaceID: " workspace-1 ", NTAccount: " user1 "}); err != nil {
		t.Fatalf("DeleteFavorite() error = %v, want nil", err)
	}

	count, err := db.Collection("user_favorite_workspaces").CountDocuments(ctx, bson.M{"nt_account": "user1", "workspace_id": "workspace-1"})
	if err != nil {
		t.Fatalf("count favorites: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
}

func TestMongoWorkspaceRepositoryDeleteFavoriteMissingIntegration(t *testing.T) {
	db := newIntegrationDatabase(t)
	repo := NewMongoWorkspaceRepository(db)

	err := repo.DeleteFavorite(context.Background(), workspace.FavoriteInput{WorkspaceID: "workspace-1", NTAccount: "user1"})
	if err != nil {
		t.Fatalf("DeleteFavorite() error = %v, want nil for missing favorite", err)
	}
}
```

- [ ] **Step 3: Run repository tests and verify they fail**

Run:

```bash
go test ./internal/workspace-service/repositories -run 'TestUserFavorite|TestMongoWorkspaceRepository.*Favorite'
```

Expected: FAIL because favorite repository types and methods are undefined. Integration tests skip if `WORKSPACE_SERVICE_MONGODB_TEST_URI` is not set; unit tests must still fail first.

- [ ] **Step 4: Update repository fields and constructor**

In `internal/workspace-service/repositories/mongo_workspace_repository.go`, replace the repository constant and struct section with:

```go
const (
	workspaceCollectionName             = "workspaces"
	userFavoriteWorkspaceCollectionName = "user_favorite_workspaces"
)

type MongoWorkspaceRepository struct {
	workspaces *mongo.Collection
	favorites  *mongo.Collection
}
```

Replace `NewMongoWorkspaceRepository` with:

```go
func NewMongoWorkspaceRepository(db *mongo.Database) *MongoWorkspaceRepository {
	return &MongoWorkspaceRepository{
		workspaces: db.Collection(workspaceCollectionName),
		favorites:  db.Collection(userFavoriteWorkspaceCollectionName),
	}
}
```

Replace existing `r.collection` references with `r.workspaces` in `EnsureIndexes`, `Create`, and `Get`.

- [ ] **Step 5: Add favorite document mapping and index helpers**

Add this document type after `workspaceDocument`:

```go
type userFavoriteWorkspaceDocument struct {
	ID          string    `bson:"_id"`
	NTAccount   string    `bson:"nt_account"`
	WorkspaceID string    `bson:"workspace_id"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}
```

Add these helpers near the existing repository helper functions:

```go
func userFavoriteWorkspaceFilter(input workspace.FavoriteInput) bson.M {
	input = input.Normalize()
	return bson.M{"nt_account": input.NTAccount, "workspace_id": input.WorkspaceID}
}

func userFavoriteWorkspaceUniqueIndexModel() mongo.IndexModel {
	return mongo.IndexModel{
		Keys: bson.D{
			{Key: "nt_account", Value: 1},
			{Key: "workspace_id", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
}

func newUserFavoriteWorkspaceDocument(input workspace.UserFavoriteWorkspace) userFavoriteWorkspaceDocument {
	input = input.Normalize()
	return userFavoriteWorkspaceDocument{
		ID:          input.ID,
		NTAccount:   input.NTAccount,
		WorkspaceID: input.WorkspaceID,
		CreatedAt:   input.CreatedAt,
		UpdatedAt:   input.UpdatedAt,
	}
}

func (d userFavoriteWorkspaceDocument) toDomain() workspace.UserFavoriteWorkspace {
	return workspace.UserFavoriteWorkspace{
		ID:          d.ID,
		NTAccount:   d.NTAccount,
		WorkspaceID: d.WorkspaceID,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}
```

Add `go.mongodb.org/mongo-driver/v2/mongo/options` to the repository imports.

- [ ] **Step 6: Create both repository indexes**

Replace `EnsureIndexes` in `internal/workspace-service/repositories/mongo_workspace_repository.go`:

```go
func (r *MongoWorkspaceRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.workspaces.Indexes().CreateOne(ctx, workspaceIndexModel()); err != nil {
		return fmt.Errorf("create workspaces index: %w", err)
	}
	if _, err := r.favorites.Indexes().CreateOne(ctx, userFavoriteWorkspaceUniqueIndexModel()); err != nil {
		return fmt.Errorf("create user_favorite_workspaces index: %w", err)
	}
	return nil
}
```

- [ ] **Step 7: Implement favorite upsert and delete**

Add these methods to `internal/workspace-service/repositories/mongo_workspace_repository.go` after `Get`:

```go
func (r *MongoWorkspaceRepository) UpsertFavorite(ctx context.Context, input workspace.UserFavoriteWorkspace) error {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}
	doc := newUserFavoriteWorkspaceDocument(input)
	filter := userFavoriteWorkspaceFilter(workspace.FavoriteInput{
		WorkspaceID: doc.WorkspaceID,
		NTAccount:   doc.NTAccount,
	})

	result, err := r.favorites.UpdateOne(ctx, filter, bson.M{
		"$set": bson.M{"updated_at": doc.UpdatedAt},
	})
	if err != nil {
		return fmt.Errorf("update workspace favorite: %w", err)
	}
	if result.MatchedCount > 0 {
		return nil
	}

	if _, err := r.favorites.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return r.updateFavoriteTimestamp(ctx, filter, doc.UpdatedAt)
		}
		return fmt.Errorf("insert workspace favorite: %w", err)
	}
	return nil
}

func (r *MongoWorkspaceRepository) updateFavoriteTimestamp(ctx context.Context, filter bson.M, updatedAt time.Time) error {
	result, err := r.favorites.UpdateOne(ctx, filter, bson.M{
		"$set": bson.M{"updated_at": updatedAt},
	})
	if err != nil {
		return fmt.Errorf("retry update workspace favorite: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("retry update workspace favorite: document not found after duplicate key")
	}
	return nil
}

func (r *MongoWorkspaceRepository) DeleteFavorite(ctx context.Context, input workspace.FavoriteInput) error {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}
	if _, err := r.favorites.DeleteOne(ctx, userFavoriteWorkspaceFilter(input)); err != nil {
		return fmt.Errorf("delete workspace favorite: %w", err)
	}
	return nil
}
```

- [ ] **Step 8: Format and run repository tests**

Run:

```bash
gofmt -w internal/workspace-service/repositories/mongo_workspace_repository.go internal/workspace-service/repositories/mongo_workspace_repository_test.go
go test ./internal/workspace-service/repositories
```

Expected: PASS for unit tests. Mongo integration tests skip when `WORKSPACE_SERVICE_MONGODB_TEST_URI` is not set.

- [ ] **Step 9: Commit the favorite repository**

Run:

```bash
git add internal/workspace-service/repositories/mongo_workspace_repository.go internal/workspace-service/repositories/mongo_workspace_repository_test.go
git commit -m "feat: persist workspace favorites"
```

### Task 4: Favorite Service Workflow

**Files:**
- Modify: `internal/workspace-service/services/workspace_service.go`
- Modify: `internal/workspace-service/services/workspace_service_test.go`

- [ ] **Step 1: Extend the fake repository for favorite tests**

Add these fields to `fakeWorkspaceRepository` in `internal/workspace-service/services/workspace_service_test.go`:

```go
favoriteInput workspace.UserFavoriteWorkspace
favoriteCalls int
favoriteErr   error
deleteInput   workspace.FavoriteInput
deleteCalls   int
deleteErr     error
```

Add these methods to the fake repository:

```go
func (f *fakeWorkspaceRepository) UpsertFavorite(_ context.Context, input workspace.UserFavoriteWorkspace) error {
	f.favoriteCalls++
	f.favoriteInput = input
	if f.favoriteErr != nil {
		return f.favoriteErr
	}
	return nil
}

func (f *fakeWorkspaceRepository) DeleteFavorite(_ context.Context, input workspace.FavoriteInput) error {
	f.deleteCalls++
	f.deleteInput = input
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}
```

- [ ] **Step 2: Write failing service workflow tests**

Add these tests to `internal/workspace-service/services/workspace_service_test.go`:

```go
func TestWorkspaceServiceSetWorkspaceFavoriteUpsertsWhenWorkspaceExists(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	repo := &fakeWorkspaceRepository{getFound: true}
	hrClient := &fakeHRClient{}
	service := NewWorkspaceService(repo, hrClient, nil,
		WithClock(func() time.Time { return now }),
		WithIDGenerator(sequenceIDs("favorite-1")),
	)

	err := service.SetWorkspaceFavorite(context.Background(), workspace.FavoriteInput{
		WorkspaceID: " workspace-1 ",
		NTAccount:   " user1 ",
		Favorite:    true,
	})
	if err != nil {
		t.Fatalf("SetWorkspaceFavorite() error = %v, want nil", err)
	}
	if repo.getCalls != 1 || repo.getQuery.ID != "workspace-1" {
		t.Fatalf("get calls=%d query=%+v, want workspace existence check", repo.getCalls, repo.getQuery)
	}
	if repo.favoriteCalls != 1 {
		t.Fatalf("favorite calls = %d, want 1", repo.favoriteCalls)
	}
	if repo.favoriteInput.ID != "favorite-1" || repo.favoriteInput.NTAccount != "user1" || repo.favoriteInput.WorkspaceID != "workspace-1" {
		t.Fatalf("favorite input = %+v", repo.favoriteInput)
	}
	if !repo.favoriteInput.CreatedAt.Equal(now) || !repo.favoriteInput.UpdatedAt.Equal(now) {
		t.Fatalf("favorite timestamps = %+v, want %v", repo.favoriteInput, now)
	}
	if hrClient.calls != 0 {
		t.Fatalf("hr calls = %d, want 0", hrClient.calls)
	}
}

func TestWorkspaceServiceSetWorkspaceFavoriteMissingWorkspace(t *testing.T) {
	repo := &fakeWorkspaceRepository{getFound: false}
	service := NewWorkspaceService(repo, &fakeHRClient{}, nil)

	err := service.SetWorkspaceFavorite(context.Background(), workspace.FavoriteInput{
		WorkspaceID: "missing-workspace",
		NTAccount:   "user1",
		Favorite:    true,
	})
	if !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("SetWorkspaceFavorite() error = %v, want ErrNotFound", err)
	}
	if repo.favoriteCalls != 0 || repo.deleteCalls != 0 {
		t.Fatalf("favorite calls=%d delete calls=%d, want no favorite write", repo.favoriteCalls, repo.deleteCalls)
	}
}

func TestWorkspaceServiceClearWorkspaceFavoriteDeletesWhenWorkspaceExists(t *testing.T) {
	repo := &fakeWorkspaceRepository{getFound: true}
	service := NewWorkspaceService(repo, &fakeHRClient{}, nil)

	err := service.SetWorkspaceFavorite(context.Background(), workspace.FavoriteInput{
		WorkspaceID: " workspace-1 ",
		NTAccount:   " user1 ",
		Favorite:    false,
	})
	if err != nil {
		t.Fatalf("SetWorkspaceFavorite() error = %v, want nil", err)
	}
	if repo.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", repo.deleteCalls)
	}
	if repo.deleteInput.WorkspaceID != "workspace-1" || repo.deleteInput.NTAccount != "user1" {
		t.Fatalf("delete input = %+v, want trimmed identity", repo.deleteInput)
	}
	if repo.favoriteCalls != 0 {
		t.Fatalf("favorite calls = %d, want 0", repo.favoriteCalls)
	}
}

func TestWorkspaceServiceFavoriteRepositoryFailure(t *testing.T) {
	repo := &fakeWorkspaceRepository{getFound: true, favoriteErr: errors.New("favorite write failed")}
	service := NewWorkspaceService(repo, &fakeHRClient{}, nil, WithIDGenerator(sequenceIDs("favorite-1")))

	err := service.SetWorkspaceFavorite(context.Background(), workspace.FavoriteInput{
		WorkspaceID: "workspace-1",
		NTAccount:   "user1",
		Favorite:    true,
	})
	if err == nil {
		t.Fatal("SetWorkspaceFavorite() error = nil, want error")
	}
}

func TestWorkspaceServiceClearFavoriteRepositoryFailure(t *testing.T) {
	repo := &fakeWorkspaceRepository{getFound: true, deleteErr: errors.New("favorite delete failed")}
	service := NewWorkspaceService(repo, &fakeHRClient{}, nil)

	err := service.SetWorkspaceFavorite(context.Background(), workspace.FavoriteInput{
		WorkspaceID: "workspace-1",
		NTAccount:   "user1",
		Favorite:    false,
	})
	if err == nil {
		t.Fatal("SetWorkspaceFavorite() error = nil, want error")
	}
}
```

- [ ] **Step 3: Run service favorite tests and verify they fail**

Run:

```bash
go test ./internal/workspace-service/services -run 'TestWorkspaceService.*Favorite'
```

Expected: FAIL because `SetWorkspaceFavorite` is undefined.

- [ ] **Step 4: Add favorite methods to the service repository interface**

Update `WorkspaceRepository` in `internal/workspace-service/services/workspace_service.go`:

```go
type WorkspaceRepository interface {
	Create(ctx context.Context, input workspace.Workspace) (workspace.Workspace, error)
	Get(ctx context.Context, query workspace.GetQuery) (workspace.Workspace, bool, error)
	UpsertFavorite(ctx context.Context, input workspace.UserFavoriteWorkspace) error
	DeleteFavorite(ctx context.Context, input workspace.FavoriteInput) error
}
```

- [ ] **Step 5: Implement favorite service workflow**

Add this method to `internal/workspace-service/services/workspace_service.go` after `GetWorkspace`:

```go
func (s *WorkspaceService) SetWorkspaceFavorite(ctx context.Context, input workspace.FavoriteInput) error {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}

	_, found, err := s.repository.Get(ctx, workspace.GetQuery{ID: input.WorkspaceID})
	if err != nil {
		return fmt.Errorf("get workspace for favorite: %w", err)
	}
	if !found {
		return workspace.ErrNotFound
	}

	if !input.Favorite {
		if err := s.repository.DeleteFavorite(ctx, input); err != nil {
			return fmt.Errorf("delete workspace favorite: %w", err)
		}
		return nil
	}

	now := s.clock().UTC()
	favorite := workspace.UserFavoriteWorkspace{
		ID:          s.idGenerator(),
		NTAccount:   input.NTAccount,
		WorkspaceID: input.WorkspaceID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repository.UpsertFavorite(ctx, favorite); err != nil {
		return fmt.Errorf("upsert workspace favorite: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: Format and run service tests**

Run:

```bash
gofmt -w internal/workspace-service/services/workspace_service.go internal/workspace-service/services/workspace_service_test.go
go test ./internal/workspace-service/services
```

Expected: PASS.

- [ ] **Step 7: Commit the favorite service workflow**

Run:

```bash
git add internal/workspace-service/services/workspace_service.go internal/workspace-service/services/workspace_service_test.go
git commit -m "feat: add workspace favorite service workflow"
```

### Task 5: Favorite HTTP Handler

**Files:**
- Modify: `internal/workspace-service/handlers/workspace_handler.go`
- Modify: `internal/workspace-service/handlers/workspace_handler_test.go`

- [ ] **Step 1: Extend the fake HTTP service**

Add these fields to `fakeHTTPWorkspaceService` in `internal/workspace-service/handlers/workspace_handler_test.go`:

```go
favoriteInput workspace.FavoriteInput
favoriteErr   error
favoriteCalls int
```

Add this method to the fake service:

```go
func (f *fakeHTTPWorkspaceService) SetWorkspaceFavorite(_ context.Context, input workspace.FavoriteInput) error {
	f.favoriteCalls++
	f.favoriteInput = input
	if f.favoriteErr != nil {
		return f.favoriteErr
	}
	return nil
}
```

- [ ] **Step 2: Write failing handler tests**

Add these tests to `internal/workspace-service/handlers/workspace_handler_test.go`:

```go
func TestWorkspaceHandlerSetWorkspaceFavorite(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := workspaceFavoriteRequest(`{"favorite":true}`)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if service.favoriteCalls != 1 {
		t.Fatalf("favorite calls = %d, want 1", service.favoriteCalls)
	}
	if service.favoriteInput.WorkspaceID != "workspace-1" || service.favoriteInput.NTAccount != "user1" || !service.favoriteInput.Favorite {
		t.Fatalf("favorite input = %+v, want workspace/user true", service.favoriteInput)
	}
}

func TestWorkspaceHandlerClearWorkspaceFavorite(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := workspaceFavoriteRequest(`{"favorite":false}`)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if service.favoriteInput.Favorite {
		t.Fatalf("favorite input = %+v, want favorite false", service.favoriteInput)
	}
}

func TestWorkspaceHandlerSetWorkspaceFavoriteRejectsMissingHeader(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/favorite", strings.NewReader(`{"favorite":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if service.favoriteCalls != 0 {
		t.Fatalf("favorite calls = %d, want 0", service.favoriteCalls)
	}
}

func TestWorkspaceHandlerSetWorkspaceFavoriteRejectsMissingFavorite(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := workspaceFavoriteRequest(`{}`)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if service.favoriteCalls != 0 {
		t.Fatalf("favorite calls = %d, want 0", service.favoriteCalls)
	}
}

func TestWorkspaceHandlerSetWorkspaceFavoriteMapsMissingWorkspace(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{favoriteErr: workspace.ErrNotFound}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := workspaceFavoriteRequest(`{"favorite":true}`)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"workspace_not_found"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceHandlerSetWorkspaceFavoriteMapsUnexpectedError(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{favoriteErr: errors.New("database down")}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := workspaceFavoriteRequest(`{"favorite":true}`)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func workspaceFavoriteRequest(body string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/favorite", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "user1")
	return req
}
```

- [ ] **Step 3: Run handler favorite tests and verify they fail**

Run:

```bash
go test ./internal/workspace-service/handlers -run 'TestWorkspaceHandler.*Favorite'
```

Expected: FAIL because the favorite route and service method are not registered in the handler interface.

- [ ] **Step 4: Add favorite method to the handler service interface**

Update `HTTPWorkspaceService` in `internal/workspace-service/handlers/workspace_handler.go`:

```go
type HTTPWorkspaceService interface {
	CreateWorkspace(ctx context.Context, input workspace.CreateInput) (services.CreateWorkspaceResult, error)
	GetWorkspace(ctx context.Context, input workspace.GetQuery) (services.GetWorkspaceResult, error)
	SetWorkspaceFavorite(ctx context.Context, input workspace.FavoriteInput) error
}
```

- [ ] **Step 5: Register the favorite route**

Update `RegisterRoutes` in `internal/workspace-service/handlers/workspace_handler.go`:

```go
func RegisterRoutes(e *echo.Echo, handler *WorkspaceHandler) {
	e.POST("/api/v1/workspaces", handler.CreateWorkspace)
	e.GET("/api/v1/workspaces/:workspace_id", handler.GetWorkspace)
	e.POST("/api/v1/workspaces/:workspace_id/favorite", handler.SetWorkspaceFavorite)
}
```

- [ ] **Step 6: Implement the favorite handler**

Add this method to `internal/workspace-service/handlers/workspace_handler.go` after `GetWorkspace`:

```go
func (h *WorkspaceHandler) SetWorkspaceFavorite(c *echo.Context) error {
	request, err := transport.DecodeWorkspaceFavoriteRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	input, err := request.ToDomain(c.Param("workspace_id"), c.Request().Header.Get("X-User-Id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	if err := h.service.SetWorkspaceFavorite(c.Request().Context(), input); err != nil {
		if errors.Is(err, workspace.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, workspace.ErrNotFound) {
			return c.JSON(http.StatusNotFound, workspaceNotFoundError())
		}
		h.logger.Warn("failed to set workspace favorite", "err", err, "workspace_id", input.WorkspaceID, "nt_account", input.NTAccount)
		return c.JSON(http.StatusInternalServerError, internalError())
	}
	return c.NoContent(http.StatusNoContent)
}
```

Add this helper near `internalError`:

```go
func workspaceNotFoundError() exception.ErrorResponse {
	return exception.WrapResponse(exception.New("workspace_not_found", "Workspace not found", exception.WithDetails(map[string]any{})))
}
```

- [ ] **Step 7: Format and run handler tests**

Run:

```bash
gofmt -w internal/workspace-service/handlers/workspace_handler.go internal/workspace-service/handlers/workspace_handler_test.go
go test ./internal/workspace-service/handlers
```

Expected: PASS.

- [ ] **Step 8: Commit the favorite HTTP route**

Run:

```bash
git add internal/workspace-service/handlers/workspace_handler.go internal/workspace-service/handlers/workspace_handler_test.go
git commit -m "feat: add workspace favorite api route"
```

### Task 6: REST Client Examples and Full Verification

**Files:**
- Modify: `examples/api/workspaces.http`

- [ ] **Step 1: Update REST Client examples**

Replace `examples/api/workspaces.http` with:

```http
@baseUrl = http://localhost:8083
@workspaceId = workspace-1
@userId = user1

### Create workspace without optional resources
POST {{baseUrl}}/api/v1/workspaces
Content-Type: application/json

{
  "name": "Planning Workspace",
  "description": "Workspace for planning",
  "owner": "user1"
}

### Create workspace with all optional resources
POST {{baseUrl}}/api/v1/workspaces
Content-Type: application/json

{
  "name": "Delivery Workspace",
  "description": "Workspace for delivery",
  "owner": "user1",
  "documents": {
    "resource_name": "Delivery documents"
  },
  "tasks": {
    "resource_name": "Delivery tasks"
  },
  "drive": {
    "resource_name": "Delivery drive"
  }
}

### Get workspace by ID
GET {{baseUrl}}/api/v1/workspaces/{{workspaceId}}

### Get missing workspace returns workspace null
GET {{baseUrl}}/api/v1/workspaces/missing-workspace

### Mark workspace as favorite
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/favorite
Content-Type: application/json
X-User-Id: {{userId}}

{
  "favorite": true
}

### Clear workspace favorite idempotently
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/favorite
Content-Type: application/json
X-User-Id: {{userId}}

{
  "favorite": false
}

### Missing favorite header returns 400
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/favorite
Content-Type: application/json

{
  "favorite": true
}

### Missing favorite field returns 400
POST {{baseUrl}}/api/v1/workspaces/{{workspaceId}}/favorite
Content-Type: application/json
X-User-Id: {{userId}}

{}

### Favorite missing workspace returns 404
POST {{baseUrl}}/api/v1/workspaces/missing-workspace/favorite
Content-Type: application/json
X-User-Id: {{userId}}

{
  "favorite": true
}

### Missing owner returns 400
POST {{baseUrl}}/api/v1/workspaces
Content-Type: application/json

{
  "name": "Missing Owner",
  "description": "Invalid workspace"
}

### Empty optional resource name returns 400
POST {{baseUrl}}/api/v1/workspaces
Content-Type: application/json

{
  "name": "Invalid Resource",
  "description": "Invalid workspace",
  "owner": "user1",
  "documents": {
    "resource_name": ""
  }
}
```

- [ ] **Step 2: Run focused package tests**

Run:

```bash
go test ./internal/domain/workspace ./internal/workspace-service/transport ./internal/workspace-service/repositories ./internal/workspace-service/services ./internal/workspace-service/handlers
```

Expected: PASS. Repository integration tests skip unless `WORKSPACE_SERVICE_MONGODB_TEST_URI` is set.

- [ ] **Step 3: Run repository-wide verification**

Run:

```bash
go test ./...
```

Expected: PASS. If MongoDB integration tests are skipped, confirm the skip output mentions `WORKSPACE_SERVICE_MONGODB_TEST_URI is not set`.

- [ ] **Step 4: Check formatting and stale references**

Run:

```bash
git diff --check
rg -n "workspace_not_found|X-User-Id|user_favorite_workspaces|favorite" docs/designs/workspace-service.md docs/designs/workspace-service-api-design.md docs/plans/active/2026-05-14-workspace-service-favorite.md examples/api/workspaces.http internal/domain/workspace internal/workspace-service
```

Expected: `git diff --check` produces no output, and `rg` shows the favorite API contract across docs, examples, domain, transport, handler, service, and repository files.

- [ ] **Step 5: Commit examples and final verification state**

Run:

```bash
git add examples/api/workspaces.http
git commit -m "docs: add workspace favorite api examples"
```

Do not move this plan out of `docs/plans/active/` until implementation is complete. After implementation is complete and verified, move the plan to `docs/plans/completed/` in a separate commit.

## Self-Review Checklist

- Spec coverage: Tasks cover the route, payload, `X-User-Id`, workspace existence check, `favorite: true` upsert, `favorite: false` delete, idempotent absent-delete `204`, missing workspace `404`, real repository errors `500`, `created_at`, `updated_at`, unique index, REST examples, and tests.
- Placeholder scan: This plan avoids placeholder markers and vague instructions; every code-changing step includes concrete snippets.
- Type consistency: Domain type names are `FavoriteInput` and `UserFavoriteWorkspace`; service method is `SetWorkspaceFavorite`; repository methods are `UpsertFavorite` and `DeleteFavorite`; handler route method is `SetWorkspaceFavorite`; transport DTO is `WorkspaceFavoriteRequest`.
- Policy alignment: Handler extracts body/path/header and maps errors, service orchestrates workflow and generated ID/time, repository owns MongoDB driver details, and transport owns JSON request decoding.
