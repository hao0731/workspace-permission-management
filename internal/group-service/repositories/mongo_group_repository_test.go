package repositories

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/drivertest"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/xoptions"
)

func repositoryTime() time.Time {
	return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
}

func repositoryGroup() group.Group {
	now := repositoryTime()
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return group.Group{
		ID:             "group-1",
		WorkspaceID:    "workspace-1",
		Name:           "Design Reviewers",
		NormalizedName: "Design Reviewers",
		Description:    "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: "department",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: expiration,
		},
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-1",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: expiration,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestNewGroupDocumentMapping(t *testing.T) {
	model := repositoryGroup()
	doc := newGroupDocument(model)

	if doc.ID != "group-1" || doc.WorkspaceID != "workspace-1" {
		t.Fatalf("doc identity = %+v, want group-1/workspace-1", doc)
	}
	if doc.DeletedAt != nil {
		t.Fatal("DeletedAt != nil, want nil")
	}
	if len(doc.GroupingRule.Rules) != 1 {
		t.Fatalf("rules len = %d, want 1", len(doc.GroupingRule.Rules))
	}

	got := doc.toDomain(model.IndividualMembers)
	if got.ID != model.ID || got.Name != model.Name {
		t.Fatalf("domain = %+v, want ID/name copied", got)
	}
}

func TestNewIndividualMemberDocuments(t *testing.T) {
	model := repositoryGroup()
	docs := newIndividualMemberDocuments(model)

	if len(docs) != 1 {
		t.Fatalf("docs len = %d, want 1", len(docs))
	}
	if docs[0].ID != "member-1" || docs[0].GroupID != "group-1" {
		t.Fatalf("member doc = %+v, want member-1/group-1", docs[0])
	}
	if docs[0].DeletedAt != nil {
		t.Fatal("DeletedAt != nil, want nil")
	}
}

func TestIndexModels(t *testing.T) {
	groupIndexes := groupIndexModels()
	if len(groupIndexes) != 2 {
		t.Fatalf("group indexes len = %d, want 2", len(groupIndexes))
	}
	groupUniqueOptions := indexOptions(t, groupIndexes[0])
	if *groupUniqueOptions.Name != groupsActiveNameUniqueIndexName {
		t.Fatalf("group unique index name = %q, want %q", *groupUniqueOptions.Name, groupsActiveNameUniqueIndexName)
	}
	if groupUniqueOptions.Unique == nil || !*groupUniqueOptions.Unique {
		t.Fatal("group unique index Unique = false, want true")
	}

	memberIndexes := individualMemberIndexModels()
	if len(memberIndexes) != 2 {
		t.Fatalf("member indexes len = %d, want 2", len(memberIndexes))
	}
	memberUniqueOptions := indexOptions(t, memberIndexes[0])
	if *memberUniqueOptions.Name != membersActiveGroupAccountUniqueIndexName {
		t.Fatalf("member unique index name = %q, want %q", *memberUniqueOptions.Name, membersActiveGroupAccountUniqueIndexName)
	}
}

func TestActiveGroupFilter(t *testing.T) {
	filter := activeGroupFilter(group.GetQuery{WorkspaceID: "workspace-1", GroupID: "group-1"})
	want := bson.M{"_id": "group-1", "workspace_id": "workspace-1", "deleted_at": nil}

	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestBuildIndividualMemberListFilter(t *testing.T) {
	cursorTime := time.Date(2026, 5, 9, 7, 31, 0, 0, time.UTC)
	filter := buildIndividualMemberListFilter(group.ListIndividualMembersQuery{
		GroupID: "group-1",
		Cursor:  &group.IndividualMemberCursor{CreatedAt: cursorTime, ID: "member-9"},
	})

	want := bson.M{
		"group_id":   "group-1",
		"deleted_at": nil,
		"$or": bson.A{
			bson.M{"created_at": bson.M{"$lt": cursorTime}},
			bson.M{"created_at": cursorTime, "_id": bson.M{"$lt": "member-9"}},
		},
	}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestMongoGroupRepositoryUpdateGroupingRuleUsesUpdateResultForExistence(t *testing.T) {
	var commands []string
	monitor := &event.CommandMonitor{
		Started: func(_ context.Context, evt *event.CommandStartedEvent) {
			commands = append(commands, evt.CommandName)
		},
	}
	deployment := drivertest.NewMockDeployment(
		mockMatchedUpdateResponse(),
		mockMatchedUpdateResponse(),
		bson.D{{Key: "ok", Value: 1}},
	)
	clientOptions := options.Client().SetMonitor(monitor)
	if err := xoptions.SetInternalClientOptions(clientOptions, "deployment", deployment); err != nil {
		t.Fatalf("set mock deployment: %v", err)
	}
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		t.Fatalf("connect mock mongodb: %v", err)
	}
	t.Cleanup(func() {
		if disconnectErr := client.Disconnect(context.Background()); disconnectErr != nil {
			t.Fatalf("disconnect mock mongodb: %v", disconnectErr)
		}
	})
	repository := NewMongoGroupRepository(client, client.Database("test"))

	err = repository.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		ExpirationDate: repositoryTime().Add(48 * time.Hour),
		Rules: []group.Rule{{
			AttributeKey: "department",
			Operator:     group.OperatorEq,
			Multi:        false,
			Value:        "ABCD-123",
		}},
	}, repositoryTime().Add(time.Hour))
	if err != nil {
		t.Fatalf("UpdateGroupingRule error = %v, want nil", err)
	}

	if !containsCommand(commands, "update") {
		t.Fatalf("commands = %v, want update command", commands)
	}
	if containsCommand(commands, "aggregate") || containsCommand(commands, "count") {
		t.Fatalf("commands = %v, want existence determined by update matched count without count command", commands)
	}
}

func TestIndividualMemberPaginationIndex(t *testing.T) {
	memberIndexes := individualMemberIndexModels()
	if len(memberIndexes) != 2 {
		t.Fatalf("member indexes len = %d, want 2", len(memberIndexes))
	}
	keys := memberIndexes[1].Keys
	want := bson.D{{Key: "group_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("member pagination index keys = %#v, want %#v", keys, want)
	}
}

func indexOptions(t *testing.T, model mongo.IndexModel) options.IndexOptions {
	t.Helper()
	var out options.IndexOptions
	for _, setter := range model.Options.List() {
		if err := setter(&out); err != nil {
			t.Fatalf("apply index option: %v", err)
		}
	}
	return out
}

func mockMatchedUpdateResponse() bson.D {
	return bson.D{
		{Key: "ok", Value: 1},
		{Key: "n", Value: 1},
		{Key: "nModified", Value: 1},
		{Key: "cursor", Value: bson.D{
			{Key: "id", Value: int64(0)},
			{Key: "ns", Value: "test.groups"},
			{Key: "firstBatch", Value: bson.A{bson.D{{Key: "n", Value: 1}}}},
		}},
	}
}

func containsCommand(commands []string, name string) bool {
	for _, command := range commands {
		if command == name {
			return true
		}
	}
	return false
}

func TestIsDuplicateIndex(t *testing.T) {
	err := mongo.WriteException{
		WriteErrors: []mongo.WriteError{{
			Code:    11000,
			Message: "E11000 duplicate key error collection: groups index: " + groupsActiveNameUniqueIndexName + " dup key",
		}},
	}

	if !isDuplicateIndex(err, groupsActiveNameUniqueIndexName) {
		t.Fatal("isDuplicateIndex = false, want true")
	}
	if isDuplicateIndex(err, membersActiveGroupAccountUniqueIndexName) {
		t.Fatal("isDuplicateIndex for member index = true, want false")
	}
}

func TestGroupDocumentBSONKeys(t *testing.T) {
	doc := newGroupDocument(repositoryGroup())
	data, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var raw bson.M
	if err := bson.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}

	for _, key := range []string{"_id", "workspace_id", "name", "normalized_name", "description", "grouping_rule", "created_at", "updated_at", "deleted_at"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("BSON key %q missing from %#v", key, raw)
		}
	}
}

func TestMongoGroupRepositoryCreateIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	model := repositoryGroup()
	got, err := repository.Create(context.Background(), model)
	if err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, model) {
		t.Fatalf("created group = %#v, want %#v", got, model)
	}

	groupCount, err := db.Collection(groupCollectionName).CountDocuments(context.Background(), bson.M{"_id": "group-1"})
	if err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if groupCount != 1 {
		t.Fatalf("group count = %d, want 1", groupCount)
	}
	memberCount, err := db.Collection(groupIndividualMemberCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1"})
	if err != nil {
		t.Fatalf("count members: %v", err)
	}
	if memberCount != 1 {
		t.Fatalf("member count = %d, want 1", memberCount)
	}
}

func TestMongoGroupRepositoryDuplicateActiveNameIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	first := repositoryGroup()
	if _, err := repository.Create(context.Background(), first); err != nil {
		t.Fatalf("first Create error = %v, want nil", err)
	}
	second := repositoryGroup()
	second.ID = "group-2"
	second.IndividualMembers = nil

	_, err := repository.Create(context.Background(), second)
	if !errors.Is(err, group.ErrDuplicateName) {
		t.Fatalf("second Create error = %v, want ErrDuplicateName", err)
	}
}

func TestMongoGroupRepositorySameNameDifferentWorkspaceIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	first := repositoryGroup()
	second := repositoryGroup()
	second.ID = "group-2"
	second.WorkspaceID = "workspace-2"
	second.IndividualMembers = nil

	if _, err := repository.Create(context.Background(), first); err != nil {
		t.Fatalf("first Create error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), second); err != nil {
		t.Fatalf("second Create error = %v, want nil", err)
	}
}

func TestMongoGroupRepositoryRollsBackMemberInsertFailureIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	model := repositoryGroup()
	model.IndividualMembers = []group.IndividualMember{
		{ID: "member-duplicate", GroupID: "group-1", NTAccount: "user1", ExpirationDate: repositoryTime().Add(time.Hour), CreatedAt: repositoryTime(), UpdatedAt: repositoryTime()},
		{ID: "member-duplicate", GroupID: "group-1", NTAccount: "user2", ExpirationDate: repositoryTime().Add(time.Hour), CreatedAt: repositoryTime(), UpdatedAt: repositoryTime()},
	}

	if _, err := repository.Create(context.Background(), model); err == nil {
		t.Fatal("Create error = nil, want duplicate _id error")
	}
	groupCount, err := db.Collection(groupCollectionName).CountDocuments(context.Background(), bson.M{"_id": "group-1"})
	if err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if groupCount != 0 {
		t.Fatalf("group count = %d, want rollback to 0", groupCount)
	}
}

func TestMongoGroupRepositoryGetIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	model := repositoryGroup()
	if _, err := repository.Create(context.Background(), model); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	got, err := repository.Get(context.Background(), group.GetQuery{WorkspaceID: "workspace-1", GroupID: "group-1"})
	if err != nil {
		t.Fatalf("Get error = %v, want nil", err)
	}
	if got == nil || got.ID != "group-1" || len(got.IndividualMembers) != 0 {
		t.Fatalf("Get = %+v, want group without embedded individual members", got)
	}
}

func TestMongoGroupRepositoryGetMissingIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)

	got, err := repository.Get(context.Background(), group.GetQuery{WorkspaceID: "workspace-1", GroupID: "missing"})
	if err != nil {
		t.Fatalf("Get error = %v, want nil", err)
	}
	if got != nil {
		t.Fatalf("Get = %+v, want nil", got)
	}
}

func TestMongoGroupRepositoryDeleteIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	deletedAt := repositoryTime().Add(time.Hour)
	if err := repository.Delete(context.Background(), group.DeleteInput{WorkspaceID: "workspace-1", GroupID: "group-1"}, deletedAt); err != nil {
		t.Fatalf("Delete error = %v, want nil", err)
	}

	groupCount, err := db.Collection(groupCollectionName).CountDocuments(context.Background(), bson.M{"_id": "group-1", "deleted_at": deletedAt})
	if err != nil {
		t.Fatalf("count deleted groups: %v", err)
	}
	if groupCount != 1 {
		t.Fatalf("deleted group count = %d, want 1", groupCount)
	}
	memberCount, err := db.Collection(groupIndividualMemberCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1", "deleted_at": deletedAt})
	if err != nil {
		t.Fatalf("count deleted members: %v", err)
	}
	if memberCount != 1 {
		t.Fatalf("deleted member count = %d, want 1", memberCount)
	}
}

func TestMongoGroupRepositoryUpdateGroupingRuleIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	expiration := repositoryTime().Add(48 * time.Hour)
	err := repository.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		ExpirationDate: expiration,
		Rules: []group.Rule{{
			AttributeKey: "level",
			Operator:     group.OperatorGte,
			Multi:        false,
			Value:        int32(5),
		}},
	}, repositoryTime().Add(time.Hour))
	if err != nil {
		t.Fatalf("UpdateGroupingRule error = %v, want nil", err)
	}

	got, err := repository.Get(context.Background(), group.GetQuery{WorkspaceID: "workspace-1", GroupID: "group-1"})
	if err != nil {
		t.Fatalf("Get error = %v, want nil", err)
	}
	if len(got.GroupingRule.Rules) != 1 || got.GroupingRule.Rules[0].AttributeKey != "level" {
		t.Fatalf("rules = %+v, want level rule", got.GroupingRule.Rules)
	}
}

func TestMongoGroupRepositoryUpdateGroupingRuleRejectsEmptyRulesWithoutMembersIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	model := repositoryGroup()
	model.IndividualMembers = nil
	if _, err := repository.Create(context.Background(), model); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	err := repository.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		ExpirationDate: repositoryTime().Add(48 * time.Hour),
		Rules:          nil,
	}, repositoryTime().Add(time.Hour))
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("UpdateGroupingRule error = %v, want ErrInvalidInput", err)
	}
}

func TestMongoGroupRepositoryListIndividualMembersIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	model := repositoryGroup()
	model.IndividualMembers = []group.IndividualMember{
		{ID: "member-1", GroupID: "group-1", NTAccount: "user1", ExpirationDate: repositoryTime().Add(24 * time.Hour), CreatedAt: repositoryTime().Add(1 * time.Minute), UpdatedAt: repositoryTime()},
		{ID: "member-2", GroupID: "group-1", NTAccount: "user2", ExpirationDate: repositoryTime().Add(24 * time.Hour), CreatedAt: repositoryTime().Add(2 * time.Minute), UpdatedAt: repositoryTime()},
	}
	if _, err := repository.Create(context.Background(), model); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	page, err := repository.ListIndividualMembers(context.Background(), group.ListIndividualMembersQuery{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("ListIndividualMembers error = %v, want nil", err)
	}
	if len(page.Members) != 1 || page.Members[0].ID != "member-2" {
		t.Fatalf("members = %+v, want newest member-2", page.Members)
	}
	if !page.HasNextPage || page.NextCursor == nil || page.NextCursor.ID != "member-2" {
		t.Fatalf("page = %+v, want next cursor for member-2", page)
	}
}

func newIntegrationDatabase(t *testing.T) (*mongo.Client, *mongo.Database) {
	t.Helper()
	uri := os.Getenv("GROUP_SERVICE_MONGODB_TEST_URI")
	if strings.TrimSpace(uri) == "" {
		t.Skip("GROUP_SERVICE_MONGODB_TEST_URI is not set")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect mongodb: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Fatalf("disconnect mongodb: %v", err)
		}
	})
	db := client.Database("workspace_permission_management_group_service_test_" + strings.ReplaceAll(t.Name(), "/", "_"))
	t.Cleanup(func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Fatalf("drop database: %v", err)
		}
	})
	return client, db
}
