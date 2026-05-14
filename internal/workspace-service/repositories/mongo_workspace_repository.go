package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const workspaceCollectionName = "workspaces"

type MongoWorkspaceRepository struct {
	collection *mongo.Collection
}

type workspaceDocument struct {
	ID             string    `bson:"_id"`
	Name           string    `bson:"name"`
	Description    string    `bson:"description"`
	OwnerNTAccount string    `bson:"owner_nt_account"`
	CreatedAt      time.Time `bson:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

func NewMongoWorkspaceRepository(db *mongo.Database) *MongoWorkspaceRepository {
	return &MongoWorkspaceRepository{collection: db.Collection(workspaceCollectionName)}
}

func (r *MongoWorkspaceRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.collection.Indexes().CreateOne(ctx, workspaceIndexModel()); err != nil {
		return fmt.Errorf("create workspaces index: %w", err)
	}
	return nil
}

func (r *MongoWorkspaceRepository) Create(ctx context.Context, input workspace.Workspace) (workspace.Workspace, error) {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return workspace.Workspace{}, err
	}
	doc := newWorkspaceDocument(input)
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
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
	if err := r.collection.FindOne(ctx, workspaceIDFilter(query)).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return workspace.Workspace{}, false, nil
		}
		return workspace.Workspace{}, false, fmt.Errorf("find workspace: %w", err)
	}
	return doc.toDomain(), true, nil
}

func workspaceIDFilter(query workspace.GetQuery) bson.M {
	query = query.Normalize()
	return bson.M{"_id": query.ID}
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
