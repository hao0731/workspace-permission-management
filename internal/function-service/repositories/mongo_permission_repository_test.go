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

func TestBuildPermissionUpdate(t *testing.T) {
	doc := permissionDocument{
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

	got := buildPermissionUpdate(doc)
	set, ok := got["$set"].(bson.M)
	if !ok {
		t.Fatalf("update = %#v, want $set", got)
	}
	if _, ok := set["office_permission"]; !ok {
		t.Fatalf("update set = %#v, want office_permission", set)
	}
	if _, ok := set["remote_permission"]; !ok {
		t.Fatalf("update set = %#v, want remote_permission", set)
	}
}

func TestPermissionDocumentMapping(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	model := permission.Permission{
		ID:          "permission-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
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
