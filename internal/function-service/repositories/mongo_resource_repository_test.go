package repositories

import (
	"reflect"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestBuildListFilterWithoutCursor(t *testing.T) {
	got := buildListFilter(resource.ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       20,
	})
	want := bson.M{
		"workspace_id": "workspace-1",
		"function_key": "todo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestBuildListFilterWithCursor(t *testing.T) {
	cursorTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	got := buildListFilter(resource.ListQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		Limit:       20,
		Cursor:      &resource.Cursor{CreatedAt: cursorTime, ID: "resource-9"},
	})
	want := bson.M{
		"workspace_id": "workspace-1",
		"function_key": "todo",
		"$or": bson.A{
			bson.M{"created_at": bson.M{"$lt": cursorTime}},
			bson.M{"created_at": cursorTime, "_id": bson.M{"$lt": "resource-9"}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestBuildPageUsesLimitPlusOne(t *testing.T) {
	items := []resource.Resource{
		{ID: "resource-1", CreatedAt: time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)},
		{ID: "resource-2", CreatedAt: time.Date(2026, 5, 5, 7, 30, 0, 0, time.UTC)},
		{ID: "resource-3", CreatedAt: time.Date(2026, 5, 5, 7, 29, 0, 0, time.UTC)},
	}

	page := buildPage(items, 2)
	if len(page.Resources) != 2 {
		t.Fatalf("resources len = %d, want 2", len(page.Resources))
	}
	if !page.HasNextPage {
		t.Fatal("HasNextPage = false, want true")
	}
	if page.NextCursor == nil || page.NextCursor.ID != "resource-2" {
		t.Fatalf("NextCursor = %+v, want resource-2", page.NextCursor)
	}
}

func TestResourceDocumentMapping(t *testing.T) {
	now := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	doc := resourceDocument{
		ID:           "resource-1",
		WorkspaceID:  "workspace-1",
		FunctionKey:  "todo",
		DisplayName:  "Spec",
		ResourceType: "document",
		ResourceTags: []string{"section_1"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	got := doc.toDomain()
	if got.ID != "resource-1" || got.Type != "document" {
		t.Fatalf("domain resource = %+v, want resource-1/document", got)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "section_1" {
		t.Fatalf("tags = %#v, want [section_1]", got.Tags)
	}
}
