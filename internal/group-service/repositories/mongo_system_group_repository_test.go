package repositories

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func repositorySystemGroup() group.SystemGroup {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	return group.SystemGroup{
		ID:       "group-1",
		SystemID: "system-a",
		Name:     "System Admins",
		GroupingRules: []group.SystemGroupRule{{
			AttributeKey: group.GroupAttributeOrganization,
			Operator:     group.OperatorEq,
			Multi:        true,
			Value:        []string{"ORG-100", "ORG-200"},
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func repositorySystemGroupProjection() group.SystemGroupRelationshipProjection {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	return group.SystemGroupRelationshipProjection{
		SystemID: "system-a",
		GroupID:  "group-1",
		Relationships: []group.RelationshipInfo{{
			Relationship: map[string]any{"relation": "hr_member"},
			Checksum:     "checksum-1",
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestNewSystemGroupDocumentMapping(t *testing.T) {
	doc := newSystemGroupDocument(repositorySystemGroup())
	if doc.ID != "group-1" || doc.SystemID != "system-a" || doc.Name != "System Admins" {
		t.Fatalf("doc = %+v, want identity/name copied", doc)
	}
	model := doc.toDomain()
	if model.ID != "group-1" || model.GroupingRules[0].AttributeKey != group.GroupAttributeOrganization {
		t.Fatalf("model = %+v, want domain mapping", model)
	}
}

func TestSystemGroupDocumentToDomainConvertsBSONArrays(t *testing.T) {
	doc := newSystemGroupDocument(repositorySystemGroup())
	doc.GroupingRules[0].Value = bson.A{"ORG-100", "ORG-200"}

	model := doc.toDomain()
	values, ok := model.GroupingRules[0].Value.([]string)
	if !ok {
		t.Fatalf("value type = %T, want []string", model.GroupingRules[0].Value)
	}
	if !reflect.DeepEqual(values, []string{"ORG-100", "ORG-200"}) {
		t.Fatalf("values = %#v, want ORG-100/ORG-200", values)
	}
}

func TestNewSystemGroupRelationshipDocumentMapping(t *testing.T) {
	doc, err := newSystemGroupRelationshipDocument(repositorySystemGroupProjection())
	if err != nil {
		t.Fatalf("newSystemGroupRelationshipDocument error = %v, want nil", err)
	}
	if doc.SystemID != "system-a" || doc.GroupID != "group-1" {
		t.Fatalf("doc = %+v, want projection identity", doc)
	}
	if len(doc.Relationships) != 1 || doc.Relationships[0].Checksum != "checksum-1" {
		t.Fatalf("relationships = %+v, want checksum", doc.Relationships)
	}
}

func TestSystemGroupRelationshipDocumentToDomain(t *testing.T) {
	projection := repositorySystemGroupProjection()
	doc, err := newSystemGroupRelationshipDocument(projection)
	if err != nil {
		t.Fatalf("newSystemGroupRelationshipDocument error = %v, want nil", err)
	}

	model := doc.toDomain()
	if model.SystemID != "system-a" || model.GroupID != "group-1" {
		t.Fatalf("model = %+v, want projection identity", model)
	}
	if len(model.Relationships) != 1 || model.Relationships[0].Checksum != "checksum-1" {
		t.Fatalf("relationships = %+v, want checksum-1", model.Relationships)
	}
}

func TestNewSystemGroupRelationshipInfoDocuments(t *testing.T) {
	docs, err := newSystemGroupRelationshipInfoDocuments(repositorySystemGroupProjection().Relationships)
	if err != nil {
		t.Fatalf("newSystemGroupRelationshipInfoDocuments error = %v, want nil", err)
	}
	if len(docs) != 1 || docs[0].Checksum != "checksum-1" {
		t.Fatalf("docs = %+v, want checksum-1", docs)
	}
}

func TestNewSystemGroupRelationshipDocumentRejectsRelationshipMappingFailure(t *testing.T) {
	projection := repositorySystemGroupProjection()
	projection.Relationships[0].Relationship = map[string]any{
		"relation": func() {},
	}

	_, err := newSystemGroupRelationshipDocument(projection)
	if err == nil {
		t.Fatal("newSystemGroupRelationshipDocument error = nil, want mapping error")
	}
	if !strings.Contains(err.Error(), "marshal system group relationship") {
		t.Fatalf("error = %q, want marshal system group relationship", err.Error())
	}
}

func TestSystemGroupRelationshipDocumentUsesPermissionJSONKeys(t *testing.T) {
	subjectRelation := "internal_member"
	projection := repositorySystemGroupProjection()
	projection.Relationships[0].Relationship = systemGroupTestRelationship{
		Relation: "checked_member",
		Resource: systemGroupTestRelationshipObject{
			ObjectID:   "group-1",
			ObjectType: "group",
		},
		Subject: systemGroupTestRelationshipSubject{
			Object: systemGroupTestRelationshipObject{
				ObjectID:   "group-1",
				ObjectType: "group",
			},
			Relation: &subjectRelation,
		},
		Caveat: &systemGroupTestRelationshipCaveat{
			Name: "static_attributes_check",
			Context: systemGroupTestRelationshipStaticAttributesContext{
				AllowedTypes:       []string{"DL"},
				AllowedLevels:      []string{"M2"},
				IsContainSecretary: true,
			},
		},
	}

	doc, err := newSystemGroupRelationshipDocument(projection)
	if err != nil {
		t.Fatalf("newSystemGroupRelationshipDocument error = %v, want nil", err)
	}
	data, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var raw bson.M
	if err := bson.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}
	relationships := raw["relationship"].(bson.A)
	info := bsonDocumentToMap(t, relationships[0])
	relationshipBody := bsonDocumentToMap(t, info["relationship"])
	resource := bsonDocumentToMap(t, relationshipBody["resource"])
	if resource["object_id"] != "group-1" || resource["object_type"] != "group" {
		t.Fatalf("resource = %#v, want JSON keys object_id/object_type", resource)
	}
	if _, ok := resource["objectid"]; ok {
		t.Fatalf("resource has objectid key: %#v", resource)
	}
	subject := bsonDocumentToMap(t, relationshipBody["subject"])
	if subject["optionalRelation"] != "internal_member" {
		t.Fatalf("subject = %#v, want optionalRelation", subject)
	}
	caveat := bsonDocumentToMap(t, relationshipBody["optionalCaveat"])
	if caveat["caveatName"] != "static_attributes_check" {
		t.Fatalf("caveat = %#v, want caveatName", caveat)
	}
	context := bsonDocumentToMap(t, caveat["context"])
	if _, ok := context["allowedtypes"]; ok {
		t.Fatalf("context has allowedtypes key: %#v", context)
	}
	if _, ok := context["allowed_types"]; !ok {
		t.Fatalf("context = %#v, want allowed_types", context)
	}
	if context["is_contain_secretary"] != true {
		t.Fatalf("context = %#v, want is_contain_secretary true", context)
	}
}

func TestSystemGroupIndexModels(t *testing.T) {
	groupIndexes := systemGroupIndexModels()
	if len(groupIndexes) != 1 {
		t.Fatalf("system group indexes len = %d, want 1", len(groupIndexes))
	}
	if *indexOptions(t, groupIndexes[0]).Name != systemGroupsSystemCreatedIndexName {
		t.Fatalf("index name = %q, want %q", *indexOptions(t, groupIndexes[0]).Name, systemGroupsSystemCreatedIndexName)
	}

	relationshipIndexes := systemGroupRelationshipIndexModels()
	if len(relationshipIndexes) != 1 {
		t.Fatalf("relationship indexes len = %d, want 1", len(relationshipIndexes))
	}
	options := indexOptions(t, relationshipIndexes[0])
	if options.Unique == nil || !*options.Unique {
		t.Fatal("relationship unique index Unique = false, want true")
	}
}

func TestBuildSystemGroupIdentityFilter(t *testing.T) {
	filter := buildSystemGroupIdentityFilter("system-a", "group-1")
	want := bson.M{"system_id": "system-a", "_id": "group-1"}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestBuildSystemGroupRelationshipIdentityFilter(t *testing.T) {
	filter := buildSystemGroupRelationshipIdentityFilter("system-a", "group-1")
	want := bson.M{"system_id": "system-a", "group_id": "group-1"}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestBuildSystemGroupListFilter(t *testing.T) {
	cursorTime := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	filter := buildSystemGroupListFilter(group.SystemGroupListQuery{
		SystemID: "system-a",
		Cursor:   &group.SystemGroupCursor{CreatedAt: cursorTime, ID: "group-9"},
	})
	want := bson.M{
		"system_id": "system-a",
		"$or": bson.A{
			bson.M{"created_at": bson.M{"$lt": cursorTime}},
			bson.M{"created_at": cursorTime, "_id": bson.M{"$lt": "group-9"}},
		},
	}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestBuildSystemGroupPage(t *testing.T) {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	page := buildSystemGroupPage([]systemGroupDocument{
		{ID: "group-1", SystemID: "system-a", Name: "One", CreatedAt: now, UpdatedAt: now},
		{ID: "group-2", SystemID: "system-a", Name: "Two", CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}, 1)
	if !page.HasNextPage || len(page.Groups) != 1 {
		t.Fatalf("page = %+v, want one item with next page", page)
	}
	if page.NextCursor == nil || page.NextCursor.ID != "group-1" {
		t.Fatalf("next cursor = %+v, want group-1", page.NextCursor)
	}
}

func TestMongoGroupRepositoryCreateAndListSystemGroupsIntegration(t *testing.T) {
	ctx := context.Background()
	repository := newIntegrationRepository(t)
	if err := repository.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes error = %v", err)
	}

	model := repositorySystemGroup()
	projection := repositorySystemGroupProjection()
	saved, err := repository.CreateSystemGroup(ctx, model, projection)
	if err != nil {
		t.Fatalf("CreateSystemGroup error = %v", err)
	}
	if saved.ID != model.ID {
		t.Fatalf("saved ID = %q, want %q", saved.ID, model.ID)
	}
	page, err := repository.ListSystemGroups(ctx, group.SystemGroupListQuery{SystemID: "system-a", Limit: 20})
	if err != nil {
		t.Fatalf("ListSystemGroups error = %v", err)
	}
	if len(page.Groups) != 1 || page.Groups[0].ID != "group-1" {
		t.Fatalf("page = %+v, want group-1", page)
	}
	count, err := repository.systemGroupRelationships.CountDocuments(ctx, bson.M{"system_id": "system-a", "group_id": "group-1"})
	if err != nil {
		t.Fatalf("count relationships: %v", err)
	}
	if count != 1 {
		t.Fatalf("relationship docs = %d, want 1", count)
	}
}

func TestMongoGroupRepositoryGetAndUpdateSystemGroupIntegration(t *testing.T) {
	ctx := context.Background()
	repository := newIntegrationRepository(t)
	if err := repository.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes error = %v", err)
	}

	created := repositorySystemGroup()
	projection := repositorySystemGroupProjection()
	if _, err := repository.CreateSystemGroup(ctx, created, projection); err != nil {
		t.Fatalf("CreateSystemGroup error = %v", err)
	}

	gotGroup, gotProjection, err := repository.GetSystemGroupWithRelationships(ctx, "system-a", "group-1")
	if err != nil {
		t.Fatalf("GetSystemGroupWithRelationships error = %v", err)
	}
	if gotGroup.ID != "group-1" || gotProjection.GroupID != "group-1" {
		t.Fatalf("got group/projection = %+v/%+v, want group-1", gotGroup, gotProjection)
	}

	updated := gotGroup
	updated.Name = "Updated Admins"
	updated.GroupingRules = []group.SystemGroupRule{{
		AttributeKey: group.GroupAttributeJobType,
		Operator:     group.OperatorEq,
		Multi:        false,
		Value:        group.SystemGroupJobTypeIDL,
	}}
	updated.UpdatedAt = gotGroup.UpdatedAt.Add(time.Hour)
	updatedProjection := gotProjection
	updatedProjection.Relationships = []group.RelationshipInfo{{
		Relationship: map[string]any{"relation": "checked_member"},
		Checksum:     "checksum-2",
	}}
	updatedProjection.UpdatedAt = updated.UpdatedAt

	saved, err := repository.UpdateSystemGroup(ctx, updated, updatedProjection)
	if err != nil {
		t.Fatalf("UpdateSystemGroup error = %v", err)
	}
	if saved.Name != "Updated Admins" || !saved.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("saved = %+v, want updated name and preserved created_at", saved)
	}

	gotGroup, gotProjection, err = repository.GetSystemGroupWithRelationships(ctx, "system-a", "group-1")
	if err != nil {
		t.Fatalf("Get after update error = %v", err)
	}
	if gotGroup.Name != "Updated Admins" {
		t.Fatalf("group name = %q, want Updated Admins", gotGroup.Name)
	}
	if len(gotProjection.Relationships) != 1 || gotProjection.Relationships[0].Checksum != "checksum-2" {
		t.Fatalf("projection = %+v, want checksum-2", gotProjection)
	}
}

func TestMongoGroupRepositoryGetSystemGroupWithRelationshipsNotFound(t *testing.T) {
	ctx := context.Background()
	repository := newIntegrationRepository(t)

	_, _, err := repository.GetSystemGroupWithRelationships(ctx, "system-a", "missing")
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("GetSystemGroupWithRelationships error = %v, want ErrNotFound", err)
	}
}

func newIntegrationRepository(t *testing.T) *MongoGroupRepository {
	t.Helper()
	client, db := newIntegrationDatabase(t)
	return NewMongoGroupRepository(client, db)
}

func bsonDocumentToMap(t *testing.T, value any) bson.M {
	t.Helper()
	switch typed := value.(type) {
	case bson.M:
		return typed
	case bson.D:
		out := bson.M{}
		for _, element := range typed {
			out[element.Key] = element.Value
		}
		return out
	default:
		t.Fatalf("value = %#v (%T), want BSON document", value, value)
		return nil
	}
}

type systemGroupTestRelationship struct {
	Relation string                             `json:"relation"`
	Resource systemGroupTestRelationshipObject  `json:"resource"`
	Subject  systemGroupTestRelationshipSubject `json:"subject"`
	Caveat   *systemGroupTestRelationshipCaveat `json:"optionalCaveat,omitempty"`
}

type systemGroupTestRelationshipObject struct {
	ObjectID   string `json:"object_id"`
	ObjectType string `json:"object_type"`
}

type systemGroupTestRelationshipSubject struct {
	Object   systemGroupTestRelationshipObject `json:"object"`
	Relation *string                           `json:"optionalRelation,omitempty"`
}

type systemGroupTestRelationshipCaveat struct {
	Name    string                                             `json:"caveatName"`
	Context systemGroupTestRelationshipStaticAttributesContext `json:"context"`
}

type systemGroupTestRelationshipStaticAttributesContext struct {
	AllowedTypes       []string `json:"allowed_types"`
	AllowedLevels      []string `json:"allowed_levels"`
	IsContainSecretary bool     `json:"is_contain_secretary"`
}
