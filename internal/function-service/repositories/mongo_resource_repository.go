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

const resourceCollectionName = "function_resources"

type MongoResourceRepository struct {
	collection *mongo.Collection
}

type resourceDocument struct {
	ID           string    `bson:"_id"`
	WorkspaceID  string    `bson:"workspace_id"`
	FunctionKey  string    `bson:"function_key"`
	DisplayName  string    `bson:"display_name"`
	ResourceType string    `bson:"resource_type"`
	ResourceTags []string  `bson:"resource_tags"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

func NewMongoResourceRepository(db *mongo.Database) *MongoResourceRepository {
	return &MongoResourceRepository{collection: db.Collection(resourceCollectionName)}
}

func (r *MongoResourceRepository) EnsureIndexes(ctx context.Context) error {
	model := mongo.IndexModel{
		Keys: bson.D{
			{Key: "workspace_id", Value: 1},
			{Key: "function_key", Value: 1},
			{Key: "created_at", Value: -1},
			{Key: "_id", Value: -1},
		},
	}
	if _, err := r.collection.Indexes().CreateOne(ctx, model); err != nil {
		return fmt.Errorf("create function_resources index: %w", err)
	}
	return nil
}

func (r *MongoResourceRepository) Upsert(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	update := bson.M{
		"$set": bson.M{
			"workspace_id":  input.WorkspaceID,
			"function_key":  input.FunctionKey,
			"display_name":  input.DisplayName,
			"resource_type": input.Type,
			"resource_tags": append([]string(nil), input.Tags...),
			"updated_at":    input.EventTime,
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{
		"_id":        input.ID,
		"updated_at": bson.M{"$lte": input.EventTime},
	}, update)
	if err != nil {
		return "", fmt.Errorf("update current resource: %w", err)
	}
	if result.MatchedCount > 0 {
		return resource.UpsertStatusUpdated, nil
	}

	doc := resourceDocument{
		ID:           input.ID,
		WorkspaceID:  input.WorkspaceID,
		FunctionKey:  input.FunctionKey,
		DisplayName:  input.DisplayName,
		ResourceType: input.Type,
		ResourceTags: append([]string(nil), input.Tags...),
		CreatedAt:    input.EventTime,
		UpdatedAt:    input.EventTime,
	}
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			status, retryErr := r.retryUpdateAfterDuplicate(ctx, input)
			if retryErr != nil {
				return "", retryErr
			}
			return status, nil
		}
		return "", fmt.Errorf("insert resource: %w", err)
	}
	return resource.UpsertStatusInserted, nil
}

func (r *MongoResourceRepository) retryUpdateAfterDuplicate(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error) {
	result, err := r.collection.UpdateOne(ctx, bson.M{
		"_id":        input.ID,
		"updated_at": bson.M{"$lte": input.EventTime},
	}, bson.M{
		"$set": bson.M{
			"workspace_id":  input.WorkspaceID,
			"function_key":  input.FunctionKey,
			"display_name":  input.DisplayName,
			"resource_type": input.Type,
			"resource_tags": append([]string(nil), input.Tags...),
			"updated_at":    input.EventTime,
		},
	})
	if err != nil {
		return "", fmt.Errorf("retry update resource: %w", err)
	}
	if result.MatchedCount == 0 {
		return resource.UpsertStatusIgnored, nil
	}
	return resource.UpsertStatusUpdated, nil
}

func (r *MongoResourceRepository) List(ctx context.Context, query resource.ListQuery) (resource.Page, error) {
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(query.Limit + 1))

	cursor, err := r.collection.Find(ctx, buildListFilter(query), findOptions)
	if err != nil {
		return resource.Page{}, fmt.Errorf("find resources: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var docs []resourceDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return resource.Page{}, fmt.Errorf("decode resources: %w", err)
	}

	resources := make([]resource.Resource, 0, len(docs))
	for _, doc := range docs {
		resources = append(resources, doc.toDomain())
	}
	return buildPage(resources, query.Limit), nil
}

func buildListFilter(query resource.ListQuery) bson.M {
	filter := bson.M{
		"workspace_id": query.WorkspaceID,
		"function_key": query.FunctionKey,
	}
	if query.Cursor != nil {
		filter["$or"] = bson.A{
			bson.M{"created_at": bson.M{"$lt": query.Cursor.CreatedAt}},
			bson.M{"created_at": query.Cursor.CreatedAt, "_id": bson.M{"$lt": query.Cursor.ID}},
		}
	}
	return filter
}

func buildPage(items []resource.Resource, limit int) resource.Page {
	if len(items) <= limit {
		return resource.Page{Resources: items, HasNextPage: false}
	}
	pageItems := append([]resource.Resource(nil), items[:limit]...)
	last := pageItems[len(pageItems)-1]
	return resource.Page{
		Resources:   pageItems,
		HasNextPage: true,
		NextCursor:  &resource.Cursor{CreatedAt: last.CreatedAt, ID: last.ID},
	}
}

func (d resourceDocument) toDomain() resource.Resource {
	return resource.Resource{
		ID:          d.ID,
		WorkspaceID: d.WorkspaceID,
		FunctionKey: d.FunctionKey,
		DisplayName: d.DisplayName,
		Type:        d.ResourceType,
		Tags:        append([]string(nil), d.ResourceTags...),
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func IsTransientMongoError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}
