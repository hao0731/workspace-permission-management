package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	groupCollectionName                      = "groups"
	groupIndividualMemberCollectionName      = "group_individual_members"
	groupsActiveNameUniqueIndexName          = "groups_active_workspace_normalized_name_unique"
	groupsWorkspaceCreatedIndexName          = "groups_workspace_created_id"
	membersActiveGroupAccountUniqueIndexName = "group_individual_members_active_group_account_unique"
	membersGroupIDIndexName                  = "group_individual_members_group_id"
)

type MongoGroupRepository struct {
	client  *mongo.Client
	groups  *mongo.Collection
	members *mongo.Collection
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
	CreatedAt      time.Time  `bson:"created_at"`
	UpdatedAt      time.Time  `bson:"updated_at"`
	DeletedAt      *time.Time `bson:"deleted_at"`
}

func NewMongoGroupRepository(client *mongo.Client, db *mongo.Database) *MongoGroupRepository {
	return &MongoGroupRepository{
		client:  client,
		groups:  db.Collection(groupCollectionName),
		members: db.Collection(groupIndividualMemberCollectionName),
	}
}

func (r *MongoGroupRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.groups.Indexes().CreateMany(ctx, groupIndexModels()); err != nil {
		return fmt.Errorf("create group indexes: %w", err)
	}
	if _, err := r.members.Indexes().CreateMany(ctx, individualMemberIndexModels()); err != nil {
		return fmt.Errorf("create group individual member indexes: %w", err)
	}
	return nil
}

func (r *MongoGroupRepository) Create(ctx context.Context, input group.Group) (group.Group, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return group.Group{}, fmt.Errorf("start group create session: %w", err)
	}
	defer session.EndSession(ctx)

	if err := mongo.WithSession(ctx, session, func(sessionCtx context.Context) error {
		if err := session.StartTransaction(); err != nil {
			return fmt.Errorf("start group create transaction: %w", err)
		}
		if _, err := r.groups.InsertOne(sessionCtx, newGroupDocument(input)); err != nil {
			return r.abortTransaction(sessionCtx, session, mapGroupInsertError(err))
		}
		memberDocs := newIndividualMemberDocuments(input)
		if len(memberDocs) > 0 {
			docs := make([]any, 0, len(memberDocs))
			for _, doc := range memberDocs {
				docs = append(docs, doc)
			}
			if _, err := r.members.InsertMany(sessionCtx, docs); err != nil {
				return r.abortTransaction(sessionCtx, session, fmt.Errorf("insert group individual members: %w", err))
			}
		}
		if err := session.CommitTransaction(sessionCtx); err != nil {
			return fmt.Errorf("commit group create transaction: %w", err)
		}
		return nil
	}); err != nil {
		return group.Group{}, err
	}
	return input, nil
}

func (r *MongoGroupRepository) abortTransaction(ctx context.Context, session *mongo.Session, cause error) error {
	if err := session.AbortTransaction(ctx); err != nil {
		return fmt.Errorf("%w; abort group create transaction: %w", cause, err)
	}
	return cause
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
			Keys:    bson.D{{Key: "group_id", Value: 1}},
			Options: options.Index().SetName(membersGroupIDIndexName),
		},
	}
}

func mapGroupInsertError(err error) error {
	if isDuplicateIndex(err, groupsActiveNameUniqueIndexName) {
		return fmt.Errorf("%w: active group name already exists", group.ErrDuplicateName)
	}
	return fmt.Errorf("insert group: %w", err)
}

func isDuplicateIndex(err error, indexName string) bool {
	return mongo.IsDuplicateKeyError(err) && strings.Contains(err.Error(), indexName)
}

func newGroupDocument(model group.Group) groupDocument {
	rules := make([]ruleDocument, 0, len(model.GroupingRule.Rules))
	for _, rule := range model.GroupingRule.Rules {
		rules = append(rules, ruleDocument{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	return groupDocument{
		ID:             model.ID,
		WorkspaceID:    model.WorkspaceID,
		Name:           model.Name,
		NormalizedName: model.NormalizedName,
		Description:    model.Description,
		GroupingRule: groupingRuleDocument{
			Rules:          rules,
			ExpirationDate: model.GroupingRule.ExpirationDate,
		},
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		DeletedAt: model.DeletedAt,
	}
}

func newIndividualMemberDocuments(model group.Group) []individualMemberDocument {
	docs := make([]individualMemberDocument, 0, len(model.IndividualMembers))
	for _, member := range model.IndividualMembers {
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
	return docs
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
		},
		IndividualMembers: members,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
		DeletedAt:         d.DeletedAt,
	}
}
