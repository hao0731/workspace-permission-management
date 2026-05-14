package repositories

import (
	"context"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	maxMongoDatabaseNameLength    = 63
	integrationDatabaseNamePrefix = "wpm_workspace_test_"
)

func TestWorkspaceDocumentMapping(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	doc := workspaceDocument{
		ID:             "workspace-1",
		Name:           "Planning",
		Description:    "Planning workspace",
		OwnerNTAccount: "user1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	got := doc.toDomain()
	if got.ID != "workspace-1" || got.OwnerNTAccount != "user1" {
		t.Fatalf("toDomain() = %+v", got)
	}
}

func TestWorkspaceIndexModel(t *testing.T) {
	index := workspaceIndexModel()
	keys, ok := index.Keys.(bson.D)
	if !ok {
		t.Fatalf("keys type = %T, want bson.D", index.Keys)
	}
	want := bson.D{
		{Key: "owner_nt_account", Value: 1},
		{Key: "created_at", Value: -1},
		{Key: "_id", Value: -1},
	}
	if len(keys) != len(want) {
		t.Fatalf("keys = %#v, want %#v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys = %#v, want %#v", keys, want)
		}
	}
}

func TestWorkspaceIDFilter(t *testing.T) {
	filter := workspaceIDFilter(workspace.GetQuery{ID: "workspace-1"})

	if filter["_id"] != "workspace-1" {
		t.Fatalf("filter = %#v, want _id workspace-1", filter)
	}
}

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

func TestMongoWorkspaceRepositoryCreateIntegration(t *testing.T) {
	db := newIntegrationDatabase(t)
	repo := NewMongoWorkspaceRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	created, err := repo.Create(ctx, workspace.Workspace{
		ID:             "workspace-1",
		Name:           "Planning",
		Description:    "Planning workspace",
		OwnerNTAccount: "user1",
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.OwnerNTAccount != "user1" {
		t.Fatalf("OwnerNTAccount = %q", created.OwnerNTAccount)
	}

	var doc bson.M
	if err := db.Collection("workspaces").FindOne(ctx, bson.M{"_id": "workspace-1"}).Decode(&doc); err != nil {
		t.Fatalf("find workspace: %v", err)
	}
	if _, ok := doc["display_name"]; ok {
		t.Fatal("display_name was persisted, want omitted")
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

func TestMongoWorkspaceRepositoryEnsureIndexesIntegration(t *testing.T) {
	db := newIntegrationDatabase(t)
	repo := NewMongoWorkspaceRepository(db)
	if err := repo.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes() error = %v", err)
	}
}

func newIntegrationDatabase(t *testing.T) *mongo.Database {
	t.Helper()
	uri := os.Getenv("WORKSPACE_SERVICE_MONGODB_TEST_URI")
	if strings.TrimSpace(uri) == "" {
		t.Skip("WORKSPACE_SERVICE_MONGODB_TEST_URI is not set")
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
	db := client.Database(integrationDatabaseName(t.Name()))
	t.Cleanup(func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Fatalf("drop database: %v", err)
		}
	})
	return db
}

func integrationDatabaseName(testName string) string {
	sanitized := strings.NewReplacer("/", "_", "\\", "_", ".", "_", "\"", "_", "$", "_", "*", "_", "<", "_", ">", "_", ":", "_", "|", "_", "?", "_").Replace(testName)
	name := integrationDatabaseNamePrefix + sanitized
	if len(name) <= maxMongoDatabaseNameLength {
		return name
	}
	suffix := "_" + integrationDatabaseNameHash(sanitized)
	availableNameLength := maxMongoDatabaseNameLength - len(integrationDatabaseNamePrefix) - len(suffix)
	return integrationDatabaseNamePrefix + sanitized[:availableNameLength] + suffix
}

func integrationDatabaseNameHash(value string) string {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(value))
	return strings.ToLower(strconv.FormatUint(hasher.Sum64(), 16))
}
