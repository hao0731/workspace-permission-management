package expiry

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoRepository struct {
	groupTasks            *mongo.Collection
	individualMemberTasks *mongo.Collection
}

type groupTaskDocument struct {
	ID               string `bson:"_id"`
	WorkspaceID      string `bson:"workspace_id"`
	GroupID          string `bson:"group_id"`
	ExpirationBucket string `bson:"expiration_bucket"`
}

type individualMemberTaskDocument struct {
	ID               string `bson:"_id"`
	GroupID          string `bson:"group_id"`
	NTAccount        string `bson:"nt_account"`
	ExpirationBucket string `bson:"expiration_bucket"`
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		groupTasks:            db.Collection(GroupTaskCollectionName),
		individualMemberTasks: db.Collection(IndividualMemberTaskCollectionName),
	}
}

func (r *MongoRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.groupTasks.Indexes().CreateMany(ctx, groupTaskIndexModels()); err != nil {
		return fmt.Errorf("create group expiry task indexes: %w", err)
	}
	if _, err := r.individualMemberTasks.Indexes().CreateMany(ctx, individualMemberTaskIndexModels()); err != nil {
		return fmt.Errorf("create individual member expiry task indexes: %w", err)
	}
	return nil
}

func (r *MongoRepository) InsertGroupTask(ctx context.Context, task GroupTask) error {
	if _, err := r.groupTasks.InsertOne(ctx, newGroupTaskDocument(task)); err != nil {
		return fmt.Errorf("insert group expiry task: %w", err)
	}
	return nil
}

func (r *MongoRepository) InsertIndividualMemberTasks(ctx context.Context, tasks []IndividualMemberTask) error {
	if len(tasks) == 0 {
		return nil
	}
	docs := make([]any, 0, len(tasks))
	for _, task := range tasks {
		docs = append(docs, newIndividualMemberTaskDocument(task))
	}
	if _, err := r.individualMemberTasks.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("insert individual member expiry tasks: %w", err)
	}
	return nil
}

func (r *MongoRepository) FindGroupTask(ctx context.Context, task GroupTask) (*GroupTask, error) {
	var doc groupTaskDocument
	err := r.groupTasks.FindOne(ctx, bson.M{
		"_id":               task.ID,
		"workspace_id":      task.WorkspaceID,
		"group_id":          task.GroupID,
		"expiration_bucket": task.ExpirationBucket,
	}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("find group expiry task: %w", err)
	}
	model := doc.toModel()
	return &model, nil
}

func (r *MongoRepository) FindIndividualMemberTask(ctx context.Context, task IndividualMemberTask) (*IndividualMemberTask, error) {
	var doc individualMemberTaskDocument
	err := r.individualMemberTasks.FindOne(ctx, bson.M{
		"_id":               task.ID,
		"group_id":          task.GroupID,
		"nt_account":        task.NTAccount,
		"expiration_bucket": task.ExpirationBucket,
	}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("find individual member expiry task: %w", err)
	}
	model := doc.toModel()
	return &model, nil
}

func (r *MongoRepository) DeleteGroupTasks(ctx context.Context, workspaceID string, groupID string) error {
	if _, err := r.groupTasks.DeleteMany(ctx, bson.M{"workspace_id": workspaceID, "group_id": groupID}); err != nil {
		return fmt.Errorf("delete group expiry tasks: %w", err)
	}
	return nil
}

func (r *MongoRepository) DeleteGroupTaskByID(ctx context.Context, taskID string) error {
	if _, err := r.groupTasks.DeleteOne(ctx, bson.M{"_id": taskID}); err != nil {
		return fmt.Errorf("delete group expiry task: %w", err)
	}
	return nil
}

func (r *MongoRepository) DeleteIndividualMemberTasksByGroup(ctx context.Context, groupID string) error {
	if _, err := r.individualMemberTasks.DeleteMany(ctx, bson.M{"group_id": groupID}); err != nil {
		return fmt.Errorf("delete individual member expiry tasks: %w", err)
	}
	return nil
}

func (r *MongoRepository) DeleteIndividualMemberTask(ctx context.Context, groupID string, ntAccount string) error {
	if _, err := r.individualMemberTasks.DeleteOne(ctx, bson.M{"group_id": groupID, "nt_account": ntAccount}); err != nil {
		return fmt.Errorf("delete individual member expiry task: %w", err)
	}
	return nil
}

func (r *MongoRepository) DeleteIndividualMemberTaskByID(ctx context.Context, taskID string) error {
	if _, err := r.individualMemberTasks.DeleteOne(ctx, bson.M{"_id": taskID}); err != nil {
		return fmt.Errorf("delete individual member expiry task: %w", err)
	}
	return nil
}

func (r *MongoRepository) ListDueGroupTasks(ctx context.Context, dueBucket string, cursor *Cursor, limit int) ([]GroupTask, error) {
	return listDueTasks[groupTaskDocument, GroupTask](ctx, r.groupTasks, dueBucket, cursor, limit, "group expiry")
}

func (r *MongoRepository) ListDueIndividualMemberTasks(ctx context.Context, dueBucket string, cursor *Cursor, limit int) ([]IndividualMemberTask, error) {
	return listDueTasks[individualMemberTaskDocument, IndividualMemberTask](ctx, r.individualMemberTasks, dueBucket, cursor, limit, "individual member expiry")
}

type modelDocument[T any] interface {
	toModel() T
}

func listDueTasks[D modelDocument[T], T any](ctx context.Context, collection *mongo.Collection, dueBucket string, cursor *Cursor, limit int, taskName string) ([]T, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}
	findOptions := options.Find().
		SetSort(bson.D{{Key: "expiration_bucket", Value: 1}, {Key: "_id", Value: 1}}).
		SetLimit(int64(limit))
	mongoCursor, err := collection.Find(ctx, dueTaskFilter(dueBucket, cursor), findOptions)
	if err != nil {
		return nil, fmt.Errorf("find due %s tasks: %w", taskName, err)
	}
	defer func() {
		_ = mongoCursor.Close(ctx)
	}()

	var docs []D
	if err := mongoCursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode due %s tasks: %w", taskName, err)
	}
	tasks := make([]T, 0, len(docs))
	for _, doc := range docs {
		tasks = append(tasks, doc.toModel())
	}
	return tasks, nil
}

func groupTaskIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "workspace_id", Value: 1},
				{Key: "group_id", Value: 1},
			},
			Options: options.Index().
				SetName(GroupTaskActiveGroupUniqueIndexName).
				SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "expiration_bucket", Value: 1}, {Key: "_id", Value: 1}},
			Options: options.Index().SetName(GroupTaskBucketIndexName),
		},
	}
}

func individualMemberTaskIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "group_id", Value: 1},
				{Key: "nt_account", Value: 1},
			},
			Options: options.Index().
				SetName(IndividualMemberTaskActiveMemberUniqueIndexName).
				SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "expiration_bucket", Value: 1}, {Key: "_id", Value: 1}},
			Options: options.Index().SetName(IndividualMemberTaskBucketIndexName),
		},
	}
}

func dueTaskFilter(dueBucket string, cursor *Cursor) bson.M {
	filter := bson.M{"expiration_bucket": bson.M{"$lte": dueBucket}}
	if cursor == nil {
		return filter
	}
	filter["$or"] = bson.A{
		bson.M{"expiration_bucket": bson.M{"$gt": cursor.ExpirationBucket}},
		bson.M{"expiration_bucket": cursor.ExpirationBucket, "_id": bson.M{"$gt": cursor.ID}},
	}
	return filter
}

func newGroupTaskDocument(task GroupTask) groupTaskDocument {
	return groupTaskDocument(task)
}

func (doc groupTaskDocument) toModel() GroupTask {
	return GroupTask(doc)
}

func newIndividualMemberTaskDocument(task IndividualMemberTask) individualMemberTaskDocument {
	return individualMemberTaskDocument(task)
}

func (doc individualMemberTaskDocument) toModel() IndividualMemberTask {
	return IndividualMemberTask(doc)
}
