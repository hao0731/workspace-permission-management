package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	sharedexpiry "github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	groupCollectionName                           = "groups"
	groupIndividualMemberCollectionName           = "group_individual_members"
	systemGroupCollectionName                     = "system_groups"
	systemGroupRelationshipCollectionName         = "system_group_relationships"
	groupsActiveNameUniqueIndexName               = "groups_active_workspace_normalized_name_unique"
	groupsWorkspaceCreatedIndexName               = "groups_workspace_created_id"
	membersActiveGroupAccountUniqueIndexName      = "group_individual_members_active_group_account_unique"
	membersActiveUnexpiredGroupIndexName          = "group_individual_members_active_unexpired_group"
	membersGroupCreatedIndexName                  = "group_individual_members_group_created_id"
	systemGroupsSystemCreatedIndexName            = "system_groups_system_created_id"
	systemGroupRelationshipsSystemGroupUniqueName = "system_group_relationships_system_group_unique"
)

type MongoGroupRepository struct {
	client                   *mongo.Client
	groups                   *mongo.Collection
	members                  *mongo.Collection
	systemGroups             *mongo.Collection
	systemGroupRelationships *mongo.Collection
	expiryRepository         *sharedexpiry.MongoRepository
}

type groupDocument struct {
	ID             string               `bson:"_id"`
	WorkspaceID    string               `bson:"workspace_id"`
	Name           string               `bson:"name"`
	NormalizedName string               `bson:"normalized_name"`
	Description    string               `bson:"description"`
	GroupingRule   groupingRuleDocument `bson:"grouping_rule"`
	CreatedAt      time.Time            `bson:"created_at"`
	UpdatedAt      time.Time            `bson:"updated_at"`
	DeletedAt      *time.Time           `bson:"deleted_at"`
}

type groupingRuleDocument struct {
	Rules          []ruleDocument `bson:"rules"`
	ExpirationDate time.Time      `bson:"expiration_date"`
	ExpiredAt      *time.Time     `bson:"expired_at"`
}

type ruleDocument struct {
	AttributeKey string         `bson:"attribute_key"`
	Operator     group.Operator `bson:"operator"`
	Multi        bool           `bson:"multi"`
	Value        any            `bson:"value"`
}

type individualMemberDocument struct {
	ID             string     `bson:"_id"`
	GroupID        string     `bson:"group_id"`
	NTAccount      string     `bson:"nt_account"`
	ExpirationDate time.Time  `bson:"expiration_date"`
	ExpiredAt      *time.Time `bson:"expired_at"`
	CreatedAt      time.Time  `bson:"created_at"`
	UpdatedAt      time.Time  `bson:"updated_at"`
	DeletedAt      *time.Time `bson:"deleted_at"`
}

func NewMongoGroupRepository(client *mongo.Client, db *mongo.Database) *MongoGroupRepository {
	return &MongoGroupRepository{
		client:                   client,
		groups:                   db.Collection(groupCollectionName),
		members:                  db.Collection(groupIndividualMemberCollectionName),
		systemGroups:             db.Collection(systemGroupCollectionName),
		systemGroupRelationships: db.Collection(systemGroupRelationshipCollectionName),
		expiryRepository:         sharedexpiry.NewMongoRepository(db),
	}
}

func (r *MongoGroupRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.groups.Indexes().CreateMany(ctx, groupIndexModels()); err != nil {
		return fmt.Errorf("create group indexes: %w", err)
	}
	if _, err := r.members.Indexes().CreateMany(ctx, individualMemberIndexModels()); err != nil {
		return fmt.Errorf("create group individual member indexes: %w", err)
	}
	if _, err := r.systemGroups.Indexes().CreateMany(ctx, systemGroupIndexModels()); err != nil {
		return fmt.Errorf("create system group indexes: %w", err)
	}
	if _, err := r.systemGroupRelationships.Indexes().CreateMany(ctx, systemGroupRelationshipIndexModels()); err != nil {
		return fmt.Errorf("create system group relationship indexes: %w", err)
	}
	if err := r.expiryRepository.EnsureIndexes(ctx); err != nil {
		return err
	}
	return nil
}

func (r *MongoGroupRepository) Create(ctx context.Context, input group.Group) (group.Group, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return group.Group{}, fmt.Errorf("start group create session: %w", err)
	}
	defer session.EndSession(ctx)

	if _, err := session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		if _, err := r.groups.InsertOne(sessionCtx, newGroupDocument(input)); err != nil {
			return nil, mapGroupInsertError(err)
		}
		memberDocs := newIndividualMemberDocuments(input)
		if len(memberDocs) > 0 {
			docs := make([]any, 0, len(memberDocs))
			for _, doc := range memberDocs {
				docs = append(docs, doc)
			}
			if _, err := r.members.InsertMany(sessionCtx, docs); err != nil {
				return nil, fmt.Errorf("insert group individual members: %w", err)
			}
		}
		if input.ExpiryTask != nil {
			if err := r.expiryRepository.InsertGroupTask(sessionCtx, newSharedGroupTask(*input.ExpiryTask)); err != nil {
				return nil, err
			}
		}
		if err := r.insertIndividualMemberExpiryTasks(sessionCtx, input.IndividualMembers); err != nil {
			return nil, err
		}
		return nil, nil
	}); err != nil {
		return group.Group{}, err
	}
	return input, nil
}

func (r *MongoGroupRepository) Get(ctx context.Context, query group.GetQuery) (*group.Group, error) {
	var doc groupDocument
	err := r.groups.FindOne(ctx, activeGroupFilter(query)).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("find group: %w", err)
	}
	model := doc.toDomain(nil)
	return &model, nil
}

func (r *MongoGroupRepository) Delete(ctx context.Context, input group.DeleteInput, deletedAt time.Time) error {
	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf("start group delete session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		result, updateErr := r.groups.UpdateOne(sessionCtx,
			activeGroupFilter(group.GetQuery(input)),
			bson.M{"$set": bson.M{"deleted_at": deletedAt, "updated_at": deletedAt}},
		)
		if updateErr != nil {
			return nil, fmt.Errorf("soft delete group: %w", updateErr)
		}
		if result.MatchedCount == 0 {
			return nil, nil
		}
		if _, updateMembersErr := r.members.UpdateMany(sessionCtx,
			bson.M{"group_id": input.GroupID, "deleted_at": nil},
			bson.M{"$set": bson.M{"deleted_at": deletedAt, "updated_at": deletedAt}},
		); updateMembersErr != nil {
			return nil, fmt.Errorf("soft delete group individual members: %w", updateMembersErr)
		}
		if deleteTaskErr := r.expiryRepository.DeleteGroupTasks(sessionCtx, input.WorkspaceID, input.GroupID); deleteTaskErr != nil {
			return nil, deleteTaskErr
		}
		if deleteMemberTasksErr := r.expiryRepository.DeleteIndividualMemberTasksByGroup(sessionCtx, input.GroupID); deleteMemberTasksErr != nil {
			return nil, deleteMemberTasksErr
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *MongoGroupRepository) UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput, updatedAt time.Time) error {
	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf("start grouping rule update session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		result, updateErr := r.groups.UpdateOne(sessionCtx,
			activeGroupFilter(group.GetQuery{WorkspaceID: input.WorkspaceID, GroupID: input.GroupID}),
			bson.M{"$set": bson.M{
				"grouping_rule": newGroupingRuleDocument(group.GroupingRule{Rules: input.Rules, ExpirationDate: input.ExpirationDate}),
				"updated_at":    updatedAt,
			}},
		)
		if updateErr != nil {
			return nil, fmt.Errorf("update grouping rule: %w", updateErr)
		}
		if result.MatchedCount == 0 {
			return nil, group.ErrNotFound
		}
		if len(input.Rules) == 0 {
			memberExists, memberExistsErr := r.activeUnexpiredIndividualMemberExists(sessionCtx, input.GroupID)
			if memberExistsErr != nil {
				return nil, memberExistsErr
			}
			if !memberExists {
				return nil, fmt.Errorf("%w: at least one membership source is required", group.ErrInvalidInput)
			}
		}
		if deleteTaskErr := r.expiryRepository.DeleteGroupTasks(sessionCtx, input.WorkspaceID, input.GroupID); deleteTaskErr != nil {
			return nil, deleteTaskErr
		}
		if input.ExpiryTask != nil {
			if insertTaskErr := r.expiryRepository.InsertGroupTask(sessionCtx, newSharedGroupTask(*input.ExpiryTask)); insertTaskErr != nil {
				return nil, insertTaskErr
			}
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *MongoGroupRepository) ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error) {
	groupDoc, err := r.Get(ctx, group.GetQuery{WorkspaceID: query.WorkspaceID, GroupID: query.GroupID})
	if err != nil {
		return group.IndividualMemberPage{}, err
	}
	if groupDoc == nil {
		return group.IndividualMemberPage{Members: []group.IndividualMember{}}, nil
	}
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(query.Limit + 1))
	cursor, err := r.members.Find(ctx, buildIndividualMemberListFilter(query), findOptions)
	if err != nil {
		return group.IndividualMemberPage{}, fmt.Errorf("find group individual members: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var docs []individualMemberDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return group.IndividualMemberPage{}, fmt.Errorf("decode group individual members: %w", err)
	}
	return buildIndividualMemberPage(docs, query.Limit), nil
}

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
		memberDocs := newIndividualMemberDocuments(group.Group{IndividualMembers: input.IndividualMembers})
		docs := make([]any, 0, len(memberDocs))
		for _, doc := range memberDocs {
			docs = append(docs, doc)
		}
		if _, insertErr := r.members.InsertMany(sessionCtx, docs); insertErr != nil {
			return nil, mapMemberInsertError(insertErr)
		}
		if taskErr := r.insertIndividualMemberExpiryTasks(sessionCtx, input.IndividualMembers); taskErr != nil {
			return nil, taskErr
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
			bson.M{"$set": bson.M{"expiration_date": input.ExpirationDate, "updated_at": updatedAt, "expired_at": nil}},
		)
		if updateErr != nil {
			return nil, fmt.Errorf("update group individual member expiration: %w", updateErr)
		}
		if result.MatchedCount == 0 {
			return nil, group.ErrNotFound
		}
		if deleteTaskErr := r.expiryRepository.DeleteIndividualMemberTask(sessionCtx, input.GroupID, input.NTAccount); deleteTaskErr != nil {
			return nil, deleteTaskErr
		}
		if input.ExpiryTask != nil {
			if insertTaskErr := r.expiryRepository.InsertIndividualMemberTasks(sessionCtx, []sharedexpiry.IndividualMemberTask{newSharedIndividualMemberTask(*input.ExpiryTask)}); insertTaskErr != nil {
				return nil, insertTaskErr
			}
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
		if deleteTaskErr := r.expiryRepository.DeleteIndividualMemberTask(sessionCtx, input.GroupID, input.NTAccount); deleteTaskErr != nil {
			return nil, deleteTaskErr
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

func (r *MongoGroupRepository) ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireGroupingRuleStatus, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return "", fmt.Errorf("start grouping rule expiry session: %w", err)
	}
	defer session.EndSession(ctx)

	var status group.ExpireGroupingRuleStatus
	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		task, taskErr := r.findExpiryTask(sessionCtx, input)
		if taskErr != nil {
			return nil, taskErr
		}
		if task == nil {
			status = group.ExpireGroupingRuleStatusStaleTask
			return nil, nil
		}

		var doc groupDocument
		findGroupErr := r.groups.FindOne(sessionCtx, activeGroupFilter(group.GetQuery{
			WorkspaceID: input.WorkspaceID,
			GroupID:     input.GroupID,
		})).Decode(&doc)
		if findGroupErr != nil {
			if errors.Is(findGroupErr, mongo.ErrNoDocuments) {
				if deleteErr := r.deleteExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
					return nil, deleteErr
				}
				status = group.ExpireGroupingRuleStatusStaleGroup
				return nil, nil
			}
			return nil, fmt.Errorf("find group for expiry: %w", findGroupErr)
		}

		if doc.GroupingRule.ExpiredAt != nil {
			if deleteErr := r.deleteExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
				return nil, deleteErr
			}
			status = group.ExpireGroupingRuleStatusAlreadyExpired
			return nil, nil
		}

		currentBucket := group.ExpirationBucketFor(doc.GroupingRule.ExpirationDate, bucketLocation)
		if currentBucket != input.ExpirationBucket {
			status = group.ExpireGroupingRuleStatusStaleBucket
			return nil, nil
		}

		result, updateErr := r.groups.UpdateOne(sessionCtx,
			activeGroupFilter(group.GetQuery{WorkspaceID: input.WorkspaceID, GroupID: input.GroupID}),
			bson.M{"$set": bson.M{
				"grouping_rule.expired_at": expiredAt,
				"updated_at":               expiredAt,
			}},
		)
		if updateErr != nil {
			return nil, fmt.Errorf("mark grouping rule expired: %w", updateErr)
		}
		if result.MatchedCount == 0 {
			status = group.ExpireGroupingRuleStatusStaleGroup
			return nil, nil
		}
		if deleteErr := r.deleteExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
			return nil, deleteErr
		}
		status = group.ExpireGroupingRuleStatusExpired
		return nil, nil
	})
	if err != nil {
		return "", err
	}
	return status, nil
}

func (r *MongoGroupRepository) ExpireIndividualMember(ctx context.Context, input group.ExpireIndividualMemberCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireIndividualMemberStatus, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return "", fmt.Errorf("start individual member expiry session: %w", err)
	}
	defer session.EndSession(ctx)

	var status group.ExpireIndividualMemberStatus
	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		task, taskErr := r.findIndividualMemberExpiryTask(sessionCtx, input)
		if taskErr != nil {
			return nil, taskErr
		}
		if task == nil {
			status = group.ExpireIndividualMemberStatusStaleTask
			return nil, nil
		}

		var doc individualMemberDocument
		findMemberErr := r.members.FindOne(sessionCtx, activeIndividualMemberFilter(input.GroupID, input.NTAccount)).Decode(&doc)
		if findMemberErr != nil {
			if errors.Is(findMemberErr, mongo.ErrNoDocuments) {
				if deleteErr := r.deleteIndividualMemberExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
					return nil, deleteErr
				}
				status = group.ExpireIndividualMemberStatusStaleMember
				return nil, nil
			}
			return nil, fmt.Errorf("find individual member for expiry: %w", findMemberErr)
		}

		if doc.ExpiredAt != nil {
			if deleteErr := r.deleteIndividualMemberExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
				return nil, deleteErr
			}
			status = group.ExpireIndividualMemberStatusAlreadyExpired
			return nil, nil
		}

		currentBucket := group.ExpirationBucketFor(doc.ExpirationDate, bucketLocation)
		if currentBucket != input.ExpirationBucket {
			status = group.ExpireIndividualMemberStatusStaleBucket
			return nil, nil
		}

		result, updateErr := r.members.UpdateOne(sessionCtx,
			activeIndividualMemberFilter(input.GroupID, input.NTAccount),
			bson.M{"$set": bson.M{
				"expired_at": expiredAt,
				"updated_at": expiredAt,
			}},
		)
		if updateErr != nil {
			return nil, fmt.Errorf("mark individual member expired: %w", updateErr)
		}
		if result.MatchedCount == 0 {
			status = group.ExpireIndividualMemberStatusStaleMember
			return nil, nil
		}
		if deleteErr := r.deleteIndividualMemberExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
			return nil, deleteErr
		}
		status = group.ExpireIndividualMemberStatusExpired
		return nil, nil
	})
	if err != nil {
		return "", err
	}
	return status, nil
}

func groupIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "workspace_id", Value: 1},
				{Key: "normalized_name", Value: 1},
			},
			Options: options.Index().
				SetName(groupsActiveNameUniqueIndexName).
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"deleted_at": nil}),
		},
		{
			Keys: bson.D{
				{Key: "workspace_id", Value: 1},
				{Key: "created_at", Value: -1},
				{Key: "_id", Value: -1},
			},
			Options: options.Index().SetName(groupsWorkspaceCreatedIndexName),
		},
	}
}

func individualMemberIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "group_id", Value: 1},
				{Key: "nt_account", Value: 1},
			},
			Options: options.Index().
				SetName(membersActiveGroupAccountUniqueIndexName).
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"deleted_at": nil}),
		},
		{
			Keys: bson.D{
				{Key: "group_id", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().
				SetName(membersActiveUnexpiredGroupIndexName).
				SetPartialFilterExpression(bson.M{
					"deleted_at": nil,
					"expired_at": nil,
				}),
		},
		{
			Keys: bson.D{
				{Key: "group_id", Value: 1},
				{Key: "created_at", Value: -1},
				{Key: "_id", Value: -1},
			},
			Options: options.Index().SetName(membersGroupCreatedIndexName),
		},
	}
}

func activeGroupFilter(query group.GetQuery) bson.M {
	return bson.M{
		"_id":          query.GroupID,
		"workspace_id": query.WorkspaceID,
		"deleted_at":   nil,
	}
}

func (r *MongoGroupRepository) activeGroupExists(ctx context.Context, query group.GetQuery) (bool, error) {
	var doc groupDocument
	err := r.groups.FindOne(ctx, activeGroupFilter(query), options.FindOne().SetProjection(bson.M{"_id": 1})).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, fmt.Errorf("find active group: %w", err)
	}
	return true, nil
}

func (r *MongoGroupRepository) activeUnexpiredIndividualMemberExists(ctx context.Context, groupID string) (bool, error) {
	var doc individualMemberDocument
	err := r.members.FindOne(ctx,
		activeUnexpiredIndividualMemberFilter(groupID),
		options.FindOne().SetProjection(bson.M{"_id": 1}),
	).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, fmt.Errorf("find active unexpired individual member: %w", err)
	}
	return true, nil
}

func (r *MongoGroupRepository) findExpiryTask(ctx context.Context, input group.ExpireGroupingRuleCommand) (*sharedexpiry.GroupTask, error) {
	return r.expiryRepository.FindGroupTask(ctx, sharedexpiry.GroupTask{
		ID:               input.TaskID,
		WorkspaceID:      input.WorkspaceID,
		GroupID:          input.GroupID,
		ExpirationBucket: input.ExpirationBucket,
	})
}

func (r *MongoGroupRepository) deleteExpiryTaskByID(ctx context.Context, taskID string) error {
	return r.expiryRepository.DeleteGroupTaskByID(ctx, taskID)
}

func (r *MongoGroupRepository) findIndividualMemberExpiryTask(ctx context.Context, input group.ExpireIndividualMemberCommand) (*sharedexpiry.IndividualMemberTask, error) {
	return r.expiryRepository.FindIndividualMemberTask(ctx, sharedexpiry.IndividualMemberTask{
		ID:               input.TaskID,
		GroupID:          input.GroupID,
		NTAccount:        input.NTAccount,
		ExpirationBucket: input.ExpirationBucket,
	})
}

func (r *MongoGroupRepository) deleteIndividualMemberExpiryTaskByID(ctx context.Context, taskID string) error {
	return r.expiryRepository.DeleteIndividualMemberTaskByID(ctx, taskID)
}

func (r *MongoGroupRepository) insertIndividualMemberExpiryTasks(ctx context.Context, members []group.IndividualMember) error {
	return r.expiryRepository.InsertIndividualMemberTasks(ctx, newSharedIndividualMemberTasks(members))
}

func activeIndividualMemberFilter(groupID string, ntAccount string) bson.M {
	return bson.M{
		"group_id":   groupID,
		"nt_account": ntAccount,
		"deleted_at": nil,
	}
}

func activeUnexpiredIndividualMemberFilter(groupID string) bson.M {
	return bson.M{
		"group_id":   groupID,
		"deleted_at": nil,
		"expired_at": nil,
	}
}

func buildIndividualMemberListFilter(query group.ListIndividualMembersQuery) bson.M {
	filter := bson.M{
		"group_id":   query.GroupID,
		"deleted_at": nil,
	}
	if query.Cursor != nil {
		filter["$or"] = bson.A{
			bson.M{"created_at": bson.M{"$lt": query.Cursor.CreatedAt}},
			bson.M{"created_at": query.Cursor.CreatedAt, "_id": bson.M{"$lt": query.Cursor.ID}},
		}
	}
	return filter
}

func mapGroupInsertError(err error) error {
	if isDuplicateIndex(err, groupsActiveNameUniqueIndexName) {
		return fmt.Errorf("%w: active group name already exists", group.ErrDuplicateName)
	}
	return fmt.Errorf("insert group: %w", err)
}

func mapMemberInsertError(err error) error {
	if isDuplicateIndex(err, membersActiveGroupAccountUniqueIndexName) {
		return fmt.Errorf("%w: active individual member already exists", group.ErrDuplicateMember)
	}
	return fmt.Errorf("insert group individual members: %w", err)
}

func isDuplicateIndex(err error, indexName string) bool {
	return mongo.IsDuplicateKeyError(err) && strings.Contains(err.Error(), indexName)
}

func newGroupDocument(model group.Group) groupDocument {
	return groupDocument{
		ID:             model.ID,
		WorkspaceID:    model.WorkspaceID,
		Name:           model.Name,
		NormalizedName: model.NormalizedName,
		Description:    model.Description,
		GroupingRule:   newGroupingRuleDocument(model.GroupingRule),
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
		DeletedAt:      model.DeletedAt,
	}
}

func newGroupingRuleDocument(rule group.GroupingRule) groupingRuleDocument {
	rules := make([]ruleDocument, 0, len(rule.Rules))
	for _, item := range rule.Rules {
		rules = append(rules, ruleDocument{
			AttributeKey: item.AttributeKey,
			Operator:     item.Operator,
			Multi:        item.Multi,
			Value:        item.Value,
		})
	}
	return groupingRuleDocument{Rules: rules, ExpirationDate: rule.ExpirationDate, ExpiredAt: rule.ExpiredAt}
}

func newSharedGroupTask(task group.ExpiryTask) sharedexpiry.GroupTask {
	return sharedexpiry.GroupTask{
		ID:               task.ID,
		WorkspaceID:      task.WorkspaceID,
		GroupID:          task.GroupID,
		ExpirationBucket: task.ExpirationBucket,
	}
}

func newSharedIndividualMemberTask(task group.IndividualMemberExpiryTask) sharedexpiry.IndividualMemberTask {
	return sharedexpiry.IndividualMemberTask{
		ID:               task.ID,
		GroupID:          task.GroupID,
		NTAccount:        task.NTAccount,
		ExpirationBucket: task.ExpirationBucket,
	}
}

func newSharedIndividualMemberTasks(members []group.IndividualMember) []sharedexpiry.IndividualMemberTask {
	tasks := make([]sharedexpiry.IndividualMemberTask, 0, len(members))
	for _, member := range members {
		if member.ExpiryTask == nil {
			continue
		}
		tasks = append(tasks, newSharedIndividualMemberTask(*member.ExpiryTask))
	}
	return tasks
}

func newIndividualMemberDocuments(model group.Group) []individualMemberDocument {
	docs := make([]individualMemberDocument, 0, len(model.IndividualMembers))
	for _, member := range model.IndividualMembers {
		docs = append(docs, individualMemberDocument{
			ID:             member.ID,
			GroupID:        member.GroupID,
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
			ExpiredAt:      member.ExpiredAt,
			CreatedAt:      member.CreatedAt,
			UpdatedAt:      member.UpdatedAt,
			DeletedAt:      member.DeletedAt,
		})
	}
	return docs
}

func (d individualMemberDocument) toDomain() group.IndividualMember {
	return group.IndividualMember{
		ID:             d.ID,
		GroupID:        d.GroupID,
		NTAccount:      d.NTAccount,
		ExpirationDate: d.ExpirationDate,
		ExpiredAt:      d.ExpiredAt,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
		DeletedAt:      d.DeletedAt,
	}
}

func buildIndividualMemberPage(docs []individualMemberDocument, limit int) group.IndividualMemberPage {
	hasNext := len(docs) > limit
	if hasNext {
		docs = docs[:limit]
	}
	members := make([]group.IndividualMember, 0, len(docs))
	for _, doc := range docs {
		members = append(members, doc.toDomain())
	}
	var nextCursor *group.IndividualMemberCursor
	if hasNext && len(members) > 0 {
		last := members[len(members)-1]
		nextCursor = &group.IndividualMemberCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return group.IndividualMemberPage{Members: members, HasNextPage: hasNext, NextCursor: nextCursor}
}

func (d groupDocument) toDomain(members []group.IndividualMember) group.Group {
	rules := make([]group.Rule, 0, len(d.GroupingRule.Rules))
	for _, rule := range d.GroupingRule.Rules {
		rules = append(rules, group.Rule{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	return group.Group{
		ID:             d.ID,
		WorkspaceID:    d.WorkspaceID,
		Name:           d.Name,
		NormalizedName: d.NormalizedName,
		Description:    d.Description,
		GroupingRule: group.GroupingRule{
			Rules:          rules,
			ExpirationDate: d.GroupingRule.ExpirationDate,
			ExpiredAt:      d.GroupingRule.ExpiredAt,
		},
		IndividualMembers: members,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
		DeletedAt:         d.DeletedAt,
	}
}
