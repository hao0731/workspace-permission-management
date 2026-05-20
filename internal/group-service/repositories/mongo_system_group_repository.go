package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type systemGroupDocument struct {
	ID            string                    `bson:"_id"`
	SystemID      string                    `bson:"system_id"`
	Name          string                    `bson:"name"`
	GroupingRules []systemGroupRuleDocument `bson:"grouping_rules"`
	CreatedAt     time.Time                 `bson:"created_at"`
	UpdatedAt     time.Time                 `bson:"updated_at"`
}

type systemGroupRuleDocument struct {
	AttributeKey group.GroupAttributeKey `bson:"attribute_key"`
	Operator     group.Operator          `bson:"operator"`
	Multi        bool                    `bson:"multi"`
	Value        any                     `bson:"value"`
}

type systemGroupRelationshipDocument struct {
	SystemID      string                                `bson:"system_id"`
	GroupID       string                                `bson:"group_id"`
	Relationships []systemGroupRelationshipInfoDocument `bson:"relationship"`
	CreatedAt     time.Time                             `bson:"created_at"`
	UpdatedAt     time.Time                             `bson:"updated_at"`
}

type systemGroupRelationshipInfoDocument struct {
	Relationship any    `bson:"relationship"`
	Checksum     string `bson:"checksum"`
}

type systemGroupRelationshipValueDocument struct {
	Relation       string                                 `json:"relation" bson:"relation"`
	Resource       systemGroupRelationshipObjectDocument  `json:"resource" bson:"resource"`
	Subject        systemGroupRelationshipSubjectDocument `json:"subject" bson:"subject"`
	OptionalCaveat *systemGroupRelationshipCaveatDocument `json:"optionalCaveat,omitempty" bson:"optionalCaveat,omitempty"`
}

type systemGroupRelationshipObjectDocument struct {
	ObjectID   string `json:"object_id" bson:"object_id"`
	ObjectType string `json:"object_type" bson:"object_type"`
}

type systemGroupRelationshipSubjectDocument struct {
	Object           systemGroupRelationshipObjectDocument `json:"object" bson:"object"`
	OptionalRelation *string                               `json:"optionalRelation,omitempty" bson:"optionalRelation,omitempty"`
}

type systemGroupRelationshipCaveatDocument struct {
	CaveatName string         `json:"caveatName" bson:"caveatName"`
	Context    map[string]any `json:"context" bson:"context"`
}

func (r *MongoGroupRepository) CreateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return group.SystemGroup{}, fmt.Errorf("start system group create session: %w", err)
	}
	defer session.EndSession(ctx)

	if _, err := session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		if _, insertErr := r.systemGroups.InsertOne(sessionCtx, newSystemGroupDocument(model)); insertErr != nil {
			return nil, fmt.Errorf("insert system group: %w", insertErr)
		}
		if _, insertErr := r.systemGroupRelationships.InsertOne(sessionCtx, newSystemGroupRelationshipDocument(projection)); insertErr != nil {
			return nil, fmt.Errorf("insert system group relationships: %w", insertErr)
		}
		return nil, nil
	}); err != nil {
		return group.SystemGroup{}, err
	}
	return model, nil
}

func (r *MongoGroupRepository) ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error) {
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(query.Limit + 1))
	cursor, err := r.systemGroups.Find(ctx, buildSystemGroupListFilter(query), findOptions)
	if err != nil {
		return group.SystemGroupPage{}, fmt.Errorf("find system groups: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()
	var docs []systemGroupDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return group.SystemGroupPage{}, fmt.Errorf("decode system groups: %w", err)
	}
	return buildSystemGroupPage(docs, query.Limit), nil
}

func systemGroupIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{{
		Keys: bson.D{
			{Key: "system_id", Value: 1},
			{Key: "created_at", Value: -1},
			{Key: "_id", Value: -1},
		},
		Options: options.Index().SetName(systemGroupsSystemCreatedIndexName),
	}}
}

func systemGroupRelationshipIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{{
		Keys: bson.D{
			{Key: "system_id", Value: 1},
			{Key: "group_id", Value: 1},
		},
		Options: options.Index().SetName(systemGroupRelationshipsSystemGroupUniqueName).SetUnique(true),
	}}
}

func buildSystemGroupListFilter(query group.SystemGroupListQuery) bson.M {
	filter := bson.M{"system_id": query.SystemID}
	if query.Cursor != nil {
		filter["$or"] = bson.A{
			bson.M{"created_at": bson.M{"$lt": query.Cursor.CreatedAt}},
			bson.M{"created_at": query.Cursor.CreatedAt, "_id": bson.M{"$lt": query.Cursor.ID}},
		}
	}
	return filter
}

func newSystemGroupDocument(model group.SystemGroup) systemGroupDocument {
	rules := make([]systemGroupRuleDocument, 0, len(model.GroupingRules))
	for _, rule := range model.GroupingRules {
		rules = append(rules, systemGroupRuleDocument{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	return systemGroupDocument{
		ID:            model.ID,
		SystemID:      model.SystemID,
		Name:          model.Name,
		GroupingRules: rules,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}

func (d systemGroupDocument) toDomain() group.SystemGroup {
	rules := make([]group.SystemGroupRule, 0, len(d.GroupingRules))
	for _, rule := range d.GroupingRules {
		rules = append(rules, group.SystemGroupRule{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        systemGroupRuleValueToDomain(rule.Value),
		})
	}
	return group.SystemGroup{
		ID:            d.ID,
		SystemID:      d.SystemID,
		Name:          d.Name,
		GroupingRules: rules,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

func newSystemGroupRelationshipDocument(model group.SystemGroupRelationshipProjection) systemGroupRelationshipDocument {
	relationships := make([]systemGroupRelationshipInfoDocument, 0, len(model.Relationships))
	for _, relationship := range model.Relationships {
		relationships = append(relationships, systemGroupRelationshipInfoDocument{
			Relationship: newSystemGroupRelationshipValueDocument(relationship.Relationship),
			Checksum:     relationship.Checksum,
		})
	}
	return systemGroupRelationshipDocument{
		SystemID:      model.SystemID,
		GroupID:       model.GroupID,
		Relationships: relationships,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}

func newSystemGroupRelationshipValueDocument(relationship any) any {
	data, err := json.Marshal(relationship)
	if err != nil {
		return relationship
	}
	var document systemGroupRelationshipValueDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return relationship
	}
	return document
}

func buildSystemGroupPage(docs []systemGroupDocument, limit int) group.SystemGroupPage {
	hasNext := len(docs) > limit
	if hasNext {
		docs = docs[:limit]
	}
	groups := make([]group.SystemGroup, 0, len(docs))
	for _, doc := range docs {
		groups = append(groups, doc.toDomain())
	}
	var nextCursor *group.SystemGroupCursor
	if hasNext && len(groups) > 0 {
		last := groups[len(groups)-1]
		nextCursor = &group.SystemGroupCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return group.SystemGroupPage{Groups: groups, HasNextPage: hasNext, NextCursor: nextCursor}
}

func systemGroupRuleValueToDomain(value any) any {
	switch values := value.(type) {
	case bson.A:
		if out, ok := bsonArrayToStringSlice(values); ok {
			return out
		}
	case []any:
		if out, ok := anySliceToStringSlice(values); ok {
			return out
		}
	}
	return value
}

func bsonArrayToStringSlice(values bson.A) ([]string, bool) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		valueString, ok := value.(string)
		if !ok {
			return nil, false
		}
		out = append(out, valueString)
	}
	return out, true
}

func anySliceToStringSlice(values []any) ([]string, bool) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		valueString, ok := value.(string)
		if !ok {
			return nil, false
		}
		out = append(out, valueString)
	}
	return out, true
}
