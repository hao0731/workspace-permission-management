package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const permissionCollectionName = "function_resource_permissions"

type MongoPermissionRepository struct {
	collection *mongo.Collection
}

type permissionDocument struct {
	ID               string                    `bson:"_id"`
	WorkspaceID      string                    `bson:"workspace_id"`
	FunctionKey      string                    `bson:"function_key"`
	OfficePermission permissionSectionDocument `bson:"office_permission"`
	RemotePermission permissionSectionDocument `bson:"remote_permission"`
}

type permissionSectionDocument struct {
	BaselineRule baselineRuleDocument `bson:"baseline_rule"`
	ExtraRules   []extraRuleDocument  `bson:"extra_rules"`
}

type baselineRuleDocument struct {
	ActionID     string   `bson:"action_id"`
	ResourceTags []string `bson:"resource_tags"`
	Enabled      bool     `bson:"enabled"`
}

type extraRuleDocument struct {
	RuleID         string    `bson:"rule_id"`
	GroupIDs       []string  `bson:"group_ids"`
	ActionID       string    `bson:"action_id"`
	ResourceTags   []string  `bson:"resource_tags"`
	ExpirationDate time.Time `bson:"expiration_date"`
}

func NewMongoPermissionRepository(db *mongo.Database) *MongoPermissionRepository {
	return &MongoPermissionRepository{collection: db.Collection(permissionCollectionName)}
}

func (r *MongoPermissionRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.collection.Indexes().CreateOne(ctx, permissionUniqueIndexModel()); err != nil {
		return fmt.Errorf("create function_resource_permissions index: %w", err)
	}
	return nil
}

func (r *MongoPermissionRepository) Save(ctx context.Context, input permission.Permission) (permission.Permission, error) {
	doc := newPermissionDocument(input)
	filter := buildPermissionFilter(input.WorkspaceID, input.FunctionKey)

	result, err := r.collection.UpdateOne(ctx, filter, buildPermissionUpdate(doc))
	if err != nil {
		return permission.Permission{}, fmt.Errorf("update permissions: %w", err)
	}
	if result.MatchedCount > 0 {
		return r.findByWorkspaceFunction(ctx, input.WorkspaceID, input.FunctionKey)
	}

	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return r.retryPermissionUpdate(ctx, doc)
		}
		return permission.Permission{}, fmt.Errorf("insert permissions: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *MongoPermissionRepository) retryPermissionUpdate(ctx context.Context, doc permissionDocument) (permission.Permission, error) {
	result, err := r.collection.UpdateOne(ctx, buildPermissionFilter(doc.WorkspaceID, doc.FunctionKey), buildPermissionUpdate(doc))
	if err != nil {
		return permission.Permission{}, fmt.Errorf("retry update permissions: %w", err)
	}
	if result.MatchedCount == 0 {
		return permission.Permission{}, fmt.Errorf("retry update permissions: document not found after duplicate key")
	}
	return r.findByWorkspaceFunction(ctx, doc.WorkspaceID, doc.FunctionKey)
}

func (r *MongoPermissionRepository) findByWorkspaceFunction(ctx context.Context, workspaceID, functionKey string) (permission.Permission, error) {
	var doc permissionDocument
	if err := r.collection.FindOne(ctx, buildPermissionFilter(workspaceID, functionKey)).Decode(&doc); err != nil {
		return permission.Permission{}, fmt.Errorf("find permissions: %w", err)
	}
	return doc.toDomain(), nil
}

func permissionUniqueIndexModel() mongo.IndexModel {
	return mongo.IndexModel{
		Keys: bson.D{
			{Key: "workspace_id", Value: 1},
			{Key: "function_key", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
}

func buildPermissionFilter(workspaceID, functionKey string) bson.M {
	return bson.M{
		"workspace_id": workspaceID,
		"function_key": functionKey,
	}
}

func buildPermissionUpdate(doc permissionDocument) bson.M {
	return bson.M{
		"$set": bson.M{
			"office_permission": doc.OfficePermission,
			"remote_permission": doc.RemotePermission,
		},
	}
}

func newPermissionDocument(model permission.Permission) permissionDocument {
	return permissionDocument{
		ID:               model.ID,
		WorkspaceID:      model.WorkspaceID,
		FunctionKey:      model.FunctionKey,
		OfficePermission: newPermissionSectionDocument(model.OfficePermission),
		RemotePermission: newPermissionSectionDocument(model.RemotePermission),
	}
}

func newPermissionSectionDocument(section permission.PermissionSection) permissionSectionDocument {
	extraRules := make([]extraRuleDocument, 0, len(section.ExtraRules))
	for _, rule := range section.ExtraRules {
		extraRules = append(extraRules, extraRuleDocument{
			RuleID:         rule.RuleID,
			GroupIDs:       append([]string(nil), rule.GroupIDs...),
			ActionID:       rule.ActionID,
			ResourceTags:   append([]string(nil), rule.ResourceTags...),
			ExpirationDate: rule.ExpirationDate,
		})
	}
	return permissionSectionDocument{
		BaselineRule: baselineRuleDocument{
			ActionID:     section.BaselineRule.ActionID,
			ResourceTags: append([]string(nil), section.BaselineRule.ResourceTags...),
			Enabled:      section.BaselineRule.Enabled,
		},
		ExtraRules: extraRules,
	}
}

func (d permissionDocument) toDomain() permission.Permission {
	return permission.Permission{
		ID:               d.ID,
		WorkspaceID:      d.WorkspaceID,
		FunctionKey:      d.FunctionKey,
		OfficePermission: d.OfficePermission.toDomain(),
		RemotePermission: d.RemotePermission.toDomain(),
	}
}

func (d permissionSectionDocument) toDomain() permission.PermissionSection {
	extraRules := make([]permission.ExtraRule, 0, len(d.ExtraRules))
	for _, rule := range d.ExtraRules {
		extraRules = append(extraRules, permission.ExtraRule{
			RuleID:         rule.RuleID,
			GroupIDs:       append([]string(nil), rule.GroupIDs...),
			ActionID:       rule.ActionID,
			ResourceTags:   append([]string(nil), rule.ResourceTags...),
			ExpirationDate: rule.ExpirationDate,
		})
	}
	return permission.PermissionSection{
		BaselineRule: permission.BaselineRule{
			ActionID:     d.BaselineRule.ActionID,
			ResourceTags: append([]string(nil), d.BaselineRule.ResourceTags...),
			Enabled:      d.BaselineRule.Enabled,
		},
		ExtraRules: extraRules,
	}
}
