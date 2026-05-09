package repositories

import (
	"reflect"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestBuildPermissionFilter(t *testing.T) {
	got := buildPermissionFilter("workspace-1", "todo")
	want := bson.M{
		"workspace_id": "workspace-1",
		"function_key": "todo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestBuildPermissionUpdateSetsPermissionBodyAndTimestamps(t *testing.T) {
	updatedAt := time.Date(2026, 5, 9, 2, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 5, 9, 1, 0, 0, 0, time.UTC)
	doc := permissionDocument{
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		OfficePermission: permissionSectionDocument{
			BaselineRule: baselineRuleDocument{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
		},
		RemotePermission: permissionSectionDocument{
			BaselineRule: baselineRuleDocument{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	}

	pipeline := buildPermissionUpdate(doc)
	if len(pipeline) != 1 {
		t.Fatalf("pipeline length = %d, want 1", len(pipeline))
	}
	stage := pipeline[0]
	if len(stage) != 1 || stage[0].Key != "$set" {
		t.Fatalf("pipeline stage = %#v, want $set stage", stage)
	}
	set, ok := stage[0].Value.(bson.D)
	if !ok {
		t.Fatalf("$set = %#v, want bson.D", stage[0].Value)
	}
	if _, officeOK := bsonDValue(set, "office_permission").(permissionSectionDocument); !officeOK {
		t.Fatalf("$set office_permission missing or wrong type: %#v", set)
	}
	if _, remoteOK := bsonDValue(set, "remote_permission").(permissionSectionDocument); !remoteOK {
		t.Fatalf("$set remote_permission missing or wrong type: %#v", set)
	}
	if got, timeOK := bsonDValue(set, "updated_at").(time.Time); !timeOK || !got.Equal(updatedAt) {
		t.Fatalf("$set updated_at = %#v, want %s", bsonDValue(set, "updated_at"), updatedAt)
	}
	createdAtExpr, ok := bsonDValue(set, "created_at").(bson.D)
	if !ok {
		t.Fatalf("$set created_at = %#v, want $ifNull expression", bsonDValue(set, "created_at"))
	}
	if len(createdAtExpr) != 1 || createdAtExpr[0].Key != "$ifNull" {
		t.Fatalf("created_at expression = %#v, want $ifNull", createdAtExpr)
	}
}

func bsonDValue(doc bson.D, key string) any {
	for _, element := range doc {
		if element.Key == key {
			return element.Value
		}
	}
	return nil
}

func TestPermissionDocumentMapping(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 5, 9, 1, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 9, 2, 0, 0, 0, time.UTC)
	model := permission.Permission{
		ID:          "permission-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		OfficePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []permission.ExtraRule{{
				RuleID:         "rule-1",
				GroupIDs:       []string{"group-1"},
				ActionID:       "edit",
				ResourceTags:   []string{"section_1"},
				ExpirationDate: expiration,
			}},
		},
		RemotePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	}

	doc := newPermissionDocument(model)
	got := doc.toDomain()

	if got.ID != "permission-1" || got.WorkspaceID != "workspace-1" || got.FunctionKey != "todo" {
		t.Fatalf("identity = %+v, want permission-1/workspace-1/todo", got)
	}
	if !got.CreatedAt.Equal(createdAt) || !got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("timestamps = %s/%s, want %s/%s", got.CreatedAt, got.UpdatedAt, createdAt, updatedAt)
	}
	if !got.OfficePermission.BaselineRule.Enabled {
		t.Fatal("office enabled = false, want true")
	}
	if got.RemotePermission.BaselineRule.Enabled {
		t.Fatal("remote enabled = true, want false")
	}
	if got.OfficePermission.ExtraRules[0].RuleID != "rule-1" {
		t.Fatalf("rule id = %q, want rule-1", got.OfficePermission.ExtraRules[0].RuleID)
	}
}

func TestPermissionUniqueIndexModel(t *testing.T) {
	model := permissionUniqueIndexModel()
	if model.Options == nil {
		t.Fatal("index options = nil, want unique option")
	}
	opts := &options.IndexOptions{}
	for _, setter := range model.Options.List() {
		if err := setter(opts); err != nil {
			t.Fatalf("apply index option: %v", err)
		}
	}
	if opts.Unique == nil || !*opts.Unique {
		t.Fatalf("unique option = %v, want true", opts.Unique)
	}
}
