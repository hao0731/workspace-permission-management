package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	workspaceCollectionName             = "workspaces"
	userFavoriteWorkspaceCollectionName = "user_favorite_workspaces"
)

type MongoWorkspaceRepository struct {
	workspaces *mongo.Collection
	favorites  *mongo.Collection
}

type workspaceDocument struct {
	ID             string    `bson:"_id"`
	Name           string    `bson:"name"`
	Description    string    `bson:"description"`
	OwnerNTAccount string    `bson:"owner_nt_account"`
	CreatedAt      time.Time `bson:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

type userFavoriteWorkspaceDocument struct {
	ID          string    `bson:"_id"`
	NTAccount   string    `bson:"nt_account"`
	WorkspaceID string    `bson:"workspace_id"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

func NewMongoWorkspaceRepository(db *mongo.Database) *MongoWorkspaceRepository {
	return &MongoWorkspaceRepository{
		workspaces: db.Collection(workspaceCollectionName),
		favorites:  db.Collection(userFavoriteWorkspaceCollectionName),
	}
}

func (r *MongoWorkspaceRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.workspaces.Indexes().CreateOne(ctx, workspaceIndexModel()); err != nil {
		return fmt.Errorf("create workspaces index: %w", err)
	}
	if _, err := r.favorites.Indexes().CreateOne(ctx, userFavoriteWorkspaceUniqueIndexModel()); err != nil {
		return fmt.Errorf("create user_favorite_workspaces index: %w", err)
	}
	return nil
}

func (r *MongoWorkspaceRepository) Create(ctx context.Context, input workspace.Workspace) (workspace.Workspace, error) {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return workspace.Workspace{}, err
	}
	doc := newWorkspaceDocument(input)
	if _, err := r.workspaces.InsertOne(ctx, doc); err != nil {
		return workspace.Workspace{}, fmt.Errorf("insert workspace: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *MongoWorkspaceRepository) Get(ctx context.Context, query workspace.GetQuery) (workspace.Workspace, bool, error) {
	query = query.Normalize()
	if err := query.Validate(); err != nil {
		return workspace.Workspace{}, false, err
	}

	var doc workspaceDocument
	if err := r.workspaces.FindOne(ctx, workspaceIDFilter(query)).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return workspace.Workspace{}, false, nil
		}
		return workspace.Workspace{}, false, fmt.Errorf("find workspace: %w", err)
	}
	return doc.toDomain(), true, nil
}

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

func workspaceIDFilter(query workspace.GetQuery) bson.M {
	query = query.Normalize()
	return bson.M{"_id": query.ID}
}

func userFavoriteWorkspaceFilter(input workspace.FavoriteInput) bson.M {
	input = input.Normalize()
	return bson.M{"nt_account": input.NTAccount, "workspace_id": input.WorkspaceID}
}

func workspaceIndexModel() mongo.IndexModel {
	return mongo.IndexModel{
		Keys: bson.D{
			{Key: "owner_nt_account", Value: 1},
			{Key: "created_at", Value: -1},
			{Key: "_id", Value: -1},
		},
	}
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

func newWorkspaceDocument(input workspace.Workspace) workspaceDocument {
	input = input.Normalize()
	return workspaceDocument{
		ID:             input.ID,
		Name:           input.Name,
		Description:    input.Description,
		OwnerNTAccount: input.OwnerNTAccount,
		CreatedAt:      input.CreatedAt,
		UpdatedAt:      input.UpdatedAt,
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

func (d workspaceDocument) toDomain() workspace.Workspace {
	return workspace.Workspace{
		ID:             d.ID,
		Name:           d.Name,
		Description:    d.Description,
		OwnerNTAccount: d.OwnerNTAccount,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
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
