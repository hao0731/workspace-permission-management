package repositories

import (
	"reflect"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestSystemResourceFilter(t *testing.T) {
	got := buildSystemResourceFilter("todo", resource.ResourceDefinitionKindAction, "can_edit")
	want := bson.M{"system_id": "todo", "type": resource.ResourceDefinitionKindAction, "key": "can_edit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestSystemResourceUpdateSetsDescriptionWhenPresent(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	doc := systemResourceDocument{
		ID:          "definition-1",
		SystemID:    "todo",
		Type:        resource.ResourceDefinitionKindAction,
		Label:       "Can Edit",
		Key:         "can_edit",
		Description: "Allows editing.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	update := buildSystemResourceUpdate(doc)
	set := update["$set"].(bson.M)
	if set["description"] != "Allows editing." {
		t.Fatalf("description = %#v, want Allows editing.", set["description"])
	}
	if _, ok := update["$unset"]; ok {
		t.Fatalf("update contains unset: %#v", update)
	}
}

func TestSystemResourceUpdateClearsDescriptionWhenEmpty(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	doc := systemResourceDocument{
		ID:        "definition-1",
		SystemID:  "todo",
		Type:      resource.ResourceDefinitionKindAction,
		Label:     "Can Edit",
		Key:       "can_edit",
		CreatedAt: now,
		UpdatedAt: now,
	}
	update := buildSystemResourceUpdate(doc)
	unset := update["$unset"].(bson.M)
	if _, ok := unset["description"]; !ok {
		t.Fatalf("unset = %#v, want description unset", unset)
	}
}

func TestSystemResourceBulkWriteModels(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	docs := []systemResourceDocument{
		{ID: "definition-1", SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit", CreatedAt: now, UpdatedAt: now},
		{ID: "definition-2", SystemID: "todo", Type: resource.ResourceDefinitionKindTag, Label: "Private", Key: "private", CreatedAt: now, UpdatedAt: now},
	}

	models := buildSystemResourceBulkWriteModels(docs)
	if len(models) != 2 {
		t.Fatalf("models len = %d, want 2", len(models))
	}
	model, ok := models[0].(*mongo.UpdateOneModel)
	if !ok {
		t.Fatalf("model type = %T, want *mongo.UpdateOneModel", models[0])
	}
	wantFilter := buildSystemResourceFilter("todo", resource.ResourceDefinitionKindAction, "can_edit")
	if !reflect.DeepEqual(model.Filter, wantFilter) {
		t.Fatalf("filter = %#v, want %#v", model.Filter, wantFilter)
	}
	if model.Upsert == nil || !*model.Upsert {
		t.Fatalf("upsert = %v, want true", model.Upsert)
	}
	update := model.Update.(bson.M)
	set := update["$set"].(bson.M)
	if set["label"] != "Can Edit" {
		t.Fatalf("label = %#v, want Can Edit", set["label"])
	}
}

func TestSystemResourceReadbackFilter(t *testing.T) {
	filter := buildSystemResourceReadbackFilter([]resource.ResourceDefinition{
		{SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Key: "can_edit"},
		{SystemID: "todo", Type: resource.ResourceDefinitionKindTag, Key: "private"},
	})
	want := bson.M{"$or": bson.A{
		bson.M{"system_id": "todo", "type": resource.ResourceDefinitionKindAction, "key": "can_edit"},
		bson.M{"system_id": "todo", "type": resource.ResourceDefinitionKindTag, "key": "private"},
	}}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestOrderSystemResourceDefinitionsByRequest(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	requested := []resource.ResourceDefinition{
		{SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Key: "can_edit"},
		{SystemID: "todo", Type: resource.ResourceDefinitionKindType, Key: "repo"},
	}
	persisted := []systemResourceDocument{
		{ID: "definition-2", SystemID: "todo", Type: resource.ResourceDefinitionKindType, Label: "Repository", Key: "repo", CreatedAt: now, UpdatedAt: now},
		{ID: "definition-1", SystemID: "todo", Type: resource.ResourceDefinitionKindAction, Label: "Can Edit", Key: "can_edit", CreatedAt: now, UpdatedAt: now},
	}

	got, err := orderSystemResourceDefinitionsByRequest(requested, persisted)
	if err != nil {
		t.Fatalf("orderSystemResourceDefinitionsByRequest error = %v, want nil", err)
	}
	if len(got) != 2 || got[0].Key != "can_edit" || got[1].Key != "repo" {
		t.Fatalf("ordered = %+v, want request order can_edit/repo", got)
	}
}

func TestSystemResourceIndexModelsAreUnique(t *testing.T) {
	models := systemResourceIndexModels()
	if len(models) != 2 {
		t.Fatalf("models len = %d, want 2", len(models))
	}
	for _, model := range models {
		opts := &options.IndexOptions{}
		for _, setter := range model.Options.List() {
			if err := setter(opts); err != nil {
				t.Fatalf("apply index option: %v", err)
			}
		}
		if opts.Unique == nil || !*opts.Unique {
			t.Fatalf("model %#v unique = %v, want true", model.Keys, opts.Unique)
		}
	}
}

func TestSystemResourceDocumentMapping(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	doc := systemResourceDocument{
		ID:          "definition-1",
		SystemID:    "todo",
		Type:        resource.ResourceDefinitionKindAction,
		Label:       "Can Edit",
		Key:         "can_edit",
		Description: "Allows editing.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	got := doc.toDomain()
	if got.ID != "definition-1" || got.SystemID != "todo" || got.Type != resource.ResourceDefinitionKindAction || got.Key != "can_edit" {
		t.Fatalf("domain = %+v, want definition-1/todo/action/can_edit", got)
	}
}

func TestSystemResourceAttributesDocumentMapping(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	doc := systemResourceAttributesDocument{
		ID:                 "attributes-1",
		SystemID:           "todo",
		ResourceAttributes: []string{"can_edit_private_repo"},
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	got := doc.toDomain()
	if got.Values[0] != resource.ResourceAttribute("can_edit_private_repo") {
		t.Fatalf("attributes = %#v, want can_edit_private_repo", got.Values)
	}
}
