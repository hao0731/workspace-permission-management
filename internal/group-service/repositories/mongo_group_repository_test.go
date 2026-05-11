package repositories

import (
	"context"
	"errors"
	"hash/fnv"
	"os"
	"reflect"
	"strconv"
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

const (
	maxMongoDatabaseNameLength    = 63
	integrationDatabaseNamePrefix = "wpm_group_test_"
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

func TestNewGroupDocumentIncludesExpiredAt(t *testing.T) {
	t.Parallel()

	expiredAt := repositoryTime()
	doc := newGroupDocument(group.Group{
		ID:          "group-1",
		WorkspaceID: "workspace-1",
		Name:        "Reviewers",
		GroupingRule: group.GroupingRule{
			Rules:          []group.Rule{{AttributeKey: "department", Operator: group.OperatorEq, Value: "ABCD-123"}},
			ExpirationDate: expiredAt.Add(24 * time.Hour),
			ExpiredAt:      &expiredAt,
		},
	})

	if doc.GroupingRule.ExpiredAt == nil || !doc.GroupingRule.ExpiredAt.Equal(expiredAt) {
		t.Fatalf("ExpiredAt = %v, want %s", doc.GroupingRule.ExpiredAt, expiredAt)
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

func TestNewIndividualMemberDocumentsIncludesExpiredAtAndTask(t *testing.T) {
	expiredAt := repositoryTime()
	model := repositoryGroup()
	model.IndividualMembers[0].ExpiredAt = &expiredAt
	model.IndividualMembers[0].ExpiryTask = &group.IndividualMemberExpiryTask{
		ID:               "member-task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-06-01",
	}

	docs := newIndividualMemberDocuments(model)
	if docs[0].ExpiredAt == nil || !docs[0].ExpiredAt.Equal(expiredAt) {
		t.Fatalf("ExpiredAt = %v, want %s", docs[0].ExpiredAt, expiredAt)
	}

	taskDocs := newIndividualMemberExpiryTaskDocuments(model.IndividualMembers)
	if len(taskDocs) != 1 {
		t.Fatalf("task docs len = %d, want 1", len(taskDocs))
	}
	if taskDocs[0].ID != "member-task-1" || taskDocs[0].GroupID != "group-1" || taskDocs[0].NTAccount != "user1" {
		t.Fatalf("task doc = %+v", taskDocs[0])
	}
}

func TestNewExpiryTaskDocument(t *testing.T) {
	t.Parallel()

	doc := newExpiryTaskDocument(group.ExpiryTask{
		ID:               "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-10",
	})

	if doc.ID != "task-1" || doc.WorkspaceID != "workspace-1" || doc.GroupID != "group-1" || doc.ExpirationBucket != "2026-05-10" {
		t.Fatalf("doc = %+v", doc)
	}
}

func TestNewIndividualMemberExpiryTaskDocument(t *testing.T) {
	t.Parallel()

	doc := newIndividualMemberExpiryTaskDocument(group.IndividualMemberExpiryTask{
		ID:               "member-task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-05-10",
	})

	if doc.ID != "member-task-1" || doc.GroupID != "group-1" || doc.NTAccount != "user1" || doc.ExpirationBucket != "2026-05-10" {
		t.Fatalf("doc = %+v", doc)
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

	expiryTaskIndexes := groupExpiryTaskIndexModels()
	if len(expiryTaskIndexes) != 2 {
		t.Fatalf("expiry task indexes len = %d, want 2", len(expiryTaskIndexes))
	}
	expiryTaskUniqueOptions := indexOptions(t, expiryTaskIndexes[0])
	if *expiryTaskUniqueOptions.Name != expiryTasksActiveGroupUniqueIndexName {
		t.Fatalf("expiry task unique index name = %q, want %q", *expiryTaskUniqueOptions.Name, expiryTasksActiveGroupUniqueIndexName)
	}
	if expiryTaskUniqueOptions.Unique == nil || !*expiryTaskUniqueOptions.Unique {
		t.Fatal("expiry task unique index Unique = false, want true")
	}

	memberExpiryTaskIndexes := individualMemberExpiryTaskIndexModels()
	if len(memberExpiryTaskIndexes) != 2 {
		t.Fatalf("individual member expiry task indexes len = %d, want 2", len(memberExpiryTaskIndexes))
	}
	memberExpiryTaskUniqueOptions := indexOptions(t, memberExpiryTaskIndexes[0])
	if *memberExpiryTaskUniqueOptions.Name != individualMemberExpiryTasksActiveMemberUniqueIndexName {
		t.Fatalf("individual member expiry task unique index name = %q, want %q", *memberExpiryTaskUniqueOptions.Name, individualMemberExpiryTasksActiveMemberUniqueIndexName)
	}
	if memberExpiryTaskUniqueOptions.Unique == nil || !*memberExpiryTaskUniqueOptions.Unique {
		t.Fatal("individual member expiry task unique index Unique = false, want true")
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

func TestActiveIndividualMemberFilter(t *testing.T) {
	filter := activeIndividualMemberFilter("group-1", "user2")
	want := bson.M{"group_id": "group-1", "nt_account": "user2", "deleted_at": nil}

	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestActiveUnexpiredIndividualMemberFilter(t *testing.T) {
	filter := activeUnexpiredIndividualMemberFilter("group-1")
	want := bson.M{"group_id": "group-1", "deleted_at": nil, "expired_at": nil}

	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestIntegrationDatabaseNameFitsMongoLimit(t *testing.T) {
	name := integrationDatabaseName("TestMongoGroupRepositoryUpdateIndividualMemberExpirationMissingMemberIntegration")
	other := integrationDatabaseName("TestMongoGroupRepositoryDeleteIndividualMemberMissingTargetIntegration")

	if len(name) > 63 {
		t.Fatalf("database name length = %d, want <= 63: %q", len(name), name)
	}
	if name == other {
		t.Fatalf("database names should remain distinct, got %q", name)
	}
	if !strings.HasPrefix(name, "wpm_group_test_") {
		t.Fatalf("database name = %q, want wpm_group_test_ prefix", name)
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

func TestMongoGroupRepositoryActiveGroupExistsUsesIDProjection(t *testing.T) {
	var projection bson.Raw
	monitor := &event.CommandMonitor{
		Started: func(_ context.Context, evt *event.CommandStartedEvent) {
			if evt.CommandName != "find" {
				return
			}
			var ok bool
			projection, ok = evt.Command.Lookup("projection").DocumentOK()
			if !ok {
				t.Fatal("find command projection is missing")
			}
		},
	}
	deployment := drivertest.NewMockDeployment(mockFindGroupIDResponse())
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

	exists, err := repository.activeGroupExists(context.Background(), group.GetQuery{WorkspaceID: "workspace-1", GroupID: "group-1"})
	if err != nil {
		t.Fatalf("activeGroupExists error = %v, want nil", err)
	}
	if !exists {
		t.Fatal("activeGroupExists = false, want true")
	}
	idValue := projection.Lookup("_id")
	if idValue.Type != bson.TypeInt32 || idValue.Int32() != 1 {
		t.Fatalf("projection _id = %v, want int32(1)", idValue)
	}
	elements, err := projection.Elements()
	if err != nil {
		t.Fatalf("read projection elements: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("projection = %s, want only _id", projection)
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

func mockFindGroupIDResponse() bson.D {
	return bson.D{
		{Key: "ok", Value: 1},
		{Key: "cursor", Value: bson.D{
			{Key: "id", Value: int64(0)},
			{Key: "ns", Value: "test.groups"},
			{Key: "firstBatch", Value: bson.A{bson.D{{Key: "_id", Value: "group-1"}}}},
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

func TestMapMemberInsertError(t *testing.T) {
	err := mongo.WriteException{
		WriteErrors: []mongo.WriteError{{
			Code:    11000,
			Message: "E11000 duplicate key error collection: group_individual_members index: " + membersActiveGroupAccountUniqueIndexName + " dup key",
		}},
	}

	mapped := mapMemberInsertError(err)
	if !errors.Is(mapped, group.ErrDuplicateMember) {
		t.Fatalf("mapped error = %v, want ErrDuplicateMember", mapped)
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

func TestMongoGroupRepositoryCreateWritesExpiryTaskIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	model := repositoryGroup()
	model.ExpiryTask = &group.ExpiryTask{
		ID:               "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-06-01",
	}
	if _, err := repository.Create(context.Background(), model); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	var task expiryTaskDocument
	err := db.Collection(groupExpiryTaskCollectionName).FindOne(context.Background(), bson.M{"_id": "task-1"}).Decode(&task)
	if err != nil {
		t.Fatalf("find expiry task: %v", err)
	}
	if task.ExpirationBucket != "2026-06-01" {
		t.Fatalf("expiration bucket = %q, want 2026-06-01", task.ExpirationBucket)
	}
}

func TestMongoGroupRepositoryCreateWritesIndividualMemberExpiryTasksIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	model := repositoryGroup()
	model.IndividualMembers[0].ExpiryTask = &group.IndividualMemberExpiryTask{
		ID:               "member-task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-06-01",
	}
	if _, err := repository.Create(context.Background(), model); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	var task individualMemberExpiryTaskDocument
	err := db.Collection(individualMemberExpiryTaskCollectionName).FindOne(context.Background(), bson.M{"_id": "member-task-1"}).Decode(&task)
	if err != nil {
		t.Fatalf("find individual member expiry task: %v", err)
	}
	if task.GroupID != "group-1" || task.NTAccount != "user1" || task.ExpirationBucket != "2026-06-01" {
		t.Fatalf("task = %+v", task)
	}
}

func TestMongoGroupRepositoryCreateWithoutExpiryTaskIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	model := repositoryGroup()
	model.ExpiryTask = nil
	if _, err := repository.Create(context.Background(), model); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}
	count, err := db.Collection(groupExpiryTaskCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1"})
	if err != nil {
		t.Fatalf("count expiry tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("expiry task count = %d, want 0", count)
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
	if _, err := db.Collection(groupExpiryTaskCollectionName).InsertOne(context.Background(), expiryTaskDocument{
		ID:               "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-06-01",
	}); err != nil {
		t.Fatalf("insert expiry task: %v", err)
	}
	insertIndividualMemberExpiryTask(t, repository, individualMemberExpiryTaskDocument{
		ID:               "member-task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-06-01",
	})

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
	taskCount, err := db.Collection(groupExpiryTaskCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1"})
	if err != nil {
		t.Fatalf("count expiry tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expiry task count = %d, want 0", taskCount)
	}
	memberTaskCount, err := db.Collection(individualMemberExpiryTaskCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1"})
	if err != nil {
		t.Fatalf("count individual member expiry tasks: %v", err)
	}
	if memberTaskCount != 0 {
		t.Fatalf("individual member expiry task count = %d, want 0", memberTaskCount)
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
	if _, err := db.Collection(groupExpiryTaskCollectionName).InsertOne(context.Background(), expiryTaskDocument{
		ID:               "task-old",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-06-01",
	}); err != nil {
		t.Fatalf("insert expiry task: %v", err)
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
		ExpiryTask: &group.ExpiryTask{
			ID:               "task-new",
			WorkspaceID:      "workspace-1",
			GroupID:          "group-1",
			ExpirationBucket: "2026-05-11",
		},
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
	oldCount, err := db.Collection(groupExpiryTaskCollectionName).CountDocuments(context.Background(), bson.M{"_id": "task-old"})
	if err != nil {
		t.Fatalf("count old expiry tasks: %v", err)
	}
	if oldCount != 0 {
		t.Fatalf("old expiry task count = %d, want 0", oldCount)
	}
	newCount, err := db.Collection(groupExpiryTaskCollectionName).CountDocuments(context.Background(), bson.M{"_id": "task-new"})
	if err != nil {
		t.Fatalf("count new expiry tasks: %v", err)
	}
	if newCount != 1 {
		t.Fatalf("new expiry task count = %d, want 1", newCount)
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

func TestMongoGroupRepositoryUpdateGroupingRuleRejectsEmptyRulesWithOnlyExpiredMembersIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	model := repositoryGroup()
	expiredAt := repositoryTime()
	model.IndividualMembers[0].ExpiredAt = &expiredAt
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

func TestMongoGroupRepositoryUpdateGroupingRuleWithoutRulesDeletesExpiryTaskIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}
	if _, err := db.Collection(groupExpiryTaskCollectionName).InsertOne(context.Background(), expiryTaskDocument{
		ID:               "task-old",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-06-01",
	}); err != nil {
		t.Fatalf("insert expiry task: %v", err)
	}

	err := repository.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		ExpirationDate: repositoryTime().Add(48 * time.Hour),
		Rules:          nil,
	}, repositoryTime().Add(time.Hour))
	if err != nil {
		t.Fatalf("UpdateGroupingRule error = %v, want nil", err)
	}
	count, err := db.Collection(groupExpiryTaskCollectionName).CountDocuments(context.Background(), bson.M{"_id": "task-old"})
	if err != nil {
		t.Fatalf("count expiry tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("expiry task count = %d, want 0", count)
	}
}

func TestMongoGroupRepositoryExpireGroupingRuleExpiresAndDeletesTaskIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	ctx := context.Background()
	now := repositoryTime()
	expiration := time.Date(2026, 5, 10, 16, 30, 0, 0, time.UTC)
	insertGroupWithTask(t, repository, groupDocument{
		ID:          "group-1",
		WorkspaceID: "workspace-1",
		GroupingRule: groupingRuleDocument{
			Rules:          []ruleDocument{{AttributeKey: "department", Operator: group.OperatorEq, Value: "ABCD-123"}},
			ExpirationDate: expiration,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}, expiryTaskDocument{
		ID:               "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-11",
	})
	location, err := group.ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}

	status, err := repository.ExpireGroupingRule(ctx, group.ExpireGroupingRuleCommand{
		TaskID:           "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-11",
	}, now.Add(time.Hour), location)
	if err != nil {
		t.Fatalf("ExpireGroupingRule() error = %v", err)
	}
	if status != group.ExpireGroupingRuleStatusExpired {
		t.Fatalf("status = %s, want expired", status)
	}

	var doc groupDocument
	err = db.Collection(groupCollectionName).FindOne(ctx, bson.M{"_id": "group-1"}).Decode(&doc)
	if err != nil {
		t.Fatal(err)
	}
	if doc.GroupingRule.ExpiredAt == nil || !doc.GroupingRule.ExpiredAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expired_at = %v, want %s", doc.GroupingRule.ExpiredAt, now.Add(time.Hour))
	}
	taskCount, err := db.Collection(groupExpiryTaskCollectionName).CountDocuments(ctx, bson.M{"_id": "task-1"})
	if err != nil {
		t.Fatal(err)
	}
	if taskCount != 0 {
		t.Fatalf("task count = %d, want 0", taskCount)
	}
}

func TestMongoGroupRepositoryExpireGroupingRuleStaleCasesIntegration(t *testing.T) {
	tests := []repositoryExpiryStaleCase[group.ExpireGroupingRuleCommand, group.ExpireGroupingRuleStatus]{
		{
			name: "missing task",
			command: group.ExpireGroupingRuleCommand{
				TaskID:           "task-missing",
				WorkspaceID:      "workspace-1",
				GroupID:          "group-1",
				ExpirationBucket: "2026-05-10",
			},
			want: group.ExpireGroupingRuleStatusStaleTask,
		},
		{
			name: "missing group deletes task",
			setup: func(t *testing.T, repository *MongoGroupRepository) {
				insertExpiryTask(t, repository, expiryTaskDocument{
					ID:               "task-1",
					WorkspaceID:      "workspace-1",
					GroupID:          "group-1",
					ExpirationBucket: "2026-05-10",
				})
			},
			command: group.ExpireGroupingRuleCommand{
				TaskID:           "task-1",
				WorkspaceID:      "workspace-1",
				GroupID:          "group-1",
				ExpirationBucket: "2026-05-10",
			},
			want: group.ExpireGroupingRuleStatusStaleGroup,
		},
		{
			name: "already expired deletes task",
			setup: func(t *testing.T, repository *MongoGroupRepository) {
				now := repositoryTime()
				insertGroupWithTask(t, repository, groupDocument{
					ID:          "group-1",
					WorkspaceID: "workspace-1",
					GroupingRule: groupingRuleDocument{
						Rules:          []ruleDocument{{AttributeKey: "department", Operator: group.OperatorEq, Value: "ABCD-123"}},
						ExpirationDate: now,
						ExpiredAt:      &now,
					},
					CreatedAt: now,
					UpdatedAt: now,
				}, expiryTaskDocument{
					ID:               "task-1",
					WorkspaceID:      "workspace-1",
					GroupID:          "group-1",
					ExpirationBucket: "2026-05-10",
				})
			},
			command: group.ExpireGroupingRuleCommand{
				TaskID:           "task-1",
				WorkspaceID:      "workspace-1",
				GroupID:          "group-1",
				ExpirationBucket: "2026-05-10",
			},
			want: group.ExpireGroupingRuleStatusAlreadyExpired,
		},
		{
			name: "bucket mismatch keeps task",
			setup: func(t *testing.T, repository *MongoGroupRepository) {
				now := time.Date(2026, 5, 10, 16, 30, 0, 0, time.UTC)
				insertGroupWithTask(t, repository, groupDocument{
					ID:          "group-1",
					WorkspaceID: "workspace-1",
					GroupingRule: groupingRuleDocument{
						Rules:          []ruleDocument{{AttributeKey: "department", Operator: group.OperatorEq, Value: "ABCD-123"}},
						ExpirationDate: now,
					},
					CreatedAt: now,
					UpdatedAt: now,
				}, expiryTaskDocument{
					ID:               "task-1",
					WorkspaceID:      "workspace-1",
					GroupID:          "group-1",
					ExpirationBucket: "2026-05-10",
				})
			},
			command: group.ExpireGroupingRuleCommand{
				TaskID:           "task-1",
				WorkspaceID:      "workspace-1",
				GroupID:          "group-1",
				ExpirationBucket: "2026-05-10",
			},
			want:      group.ExpireGroupingRuleStatusStaleBucket,
			wantTasks: 1,
		},
	}

	runRepositoryExpiryStaleCases(t, tests,
		func(ctx context.Context, repository *MongoGroupRepository, command group.ExpireGroupingRuleCommand, expiredAt time.Time, location *time.Location) (group.ExpireGroupingRuleStatus, error) {
			return repository.ExpireGroupingRule(ctx, command, expiredAt, location)
		},
		func(ctx context.Context, repository *MongoGroupRepository, command group.ExpireGroupingRuleCommand) (int64, error) {
			return repository.expiryTasks.CountDocuments(ctx, bson.M{"_id": command.TaskID})
		},
	)
}

func TestMongoGroupRepositoryExpireIndividualMemberExpiresAndDeletesTaskIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	ctx := context.Background()
	now := repositoryTime()
	expiration := time.Date(2026, 5, 10, 16, 30, 0, 0, time.UTC)
	if _, err := repository.Create(ctx, group.Group{
		ID:             "group-1",
		WorkspaceID:    "workspace-1",
		Name:           "Reviewers",
		NormalizedName: "Reviewers",
		GroupingRule: group.GroupingRule{
			ExpirationDate: now.Add(24 * time.Hour),
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
	}); err != nil {
		t.Fatalf("Create error = %v", err)
	}
	insertIndividualMemberExpiryTask(t, repository, individualMemberExpiryTaskDocument{
		ID:               "member-task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-05-11",
	})
	location, err := group.ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}

	status, err := repository.ExpireIndividualMember(ctx, group.ExpireIndividualMemberCommand{
		TaskID:           "member-task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-05-11",
	}, now.Add(time.Hour), location)
	if err != nil {
		t.Fatalf("ExpireIndividualMember() error = %v", err)
	}
	if status != group.ExpireIndividualMemberStatusExpired {
		t.Fatalf("status = %s, want expired", status)
	}

	var doc individualMemberDocument
	err = db.Collection(groupIndividualMemberCollectionName).FindOne(ctx, bson.M{"group_id": "group-1", "nt_account": "user1"}).Decode(&doc)
	if err != nil {
		t.Fatal(err)
	}
	if doc.ExpiredAt == nil || !doc.ExpiredAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expired_at = %v, want %s", doc.ExpiredAt, now.Add(time.Hour))
	}
	taskCount, err := db.Collection(individualMemberExpiryTaskCollectionName).CountDocuments(ctx, bson.M{"_id": "member-task-1"})
	if err != nil {
		t.Fatal(err)
	}
	if taskCount != 0 {
		t.Fatalf("task count = %d, want 0", taskCount)
	}
}

func TestMongoGroupRepositoryExpireIndividualMemberStaleCasesIntegration(t *testing.T) {
	tests := []repositoryExpiryStaleCase[group.ExpireIndividualMemberCommand, group.ExpireIndividualMemberStatus]{
		{
			name: "missing task",
			command: group.ExpireIndividualMemberCommand{
				TaskID:           "task-missing",
				GroupID:          "group-1",
				NTAccount:        "user1",
				ExpirationBucket: "2026-05-10",
			},
			want: group.ExpireIndividualMemberStatusStaleTask,
		},
		{
			name: "missing member deletes task",
			setup: func(t *testing.T, repository *MongoGroupRepository) {
				insertIndividualMemberExpiryTask(t, repository, individualMemberExpiryTaskDocument{
					ID:               "member-task-1",
					GroupID:          "group-1",
					NTAccount:        "user1",
					ExpirationBucket: "2026-05-10",
				})
			},
			command: group.ExpireIndividualMemberCommand{
				TaskID:           "member-task-1",
				GroupID:          "group-1",
				NTAccount:        "user1",
				ExpirationBucket: "2026-05-10",
			},
			want: group.ExpireIndividualMemberStatusStaleMember,
		},
		{
			name: "already expired deletes task",
			setup: func(t *testing.T, repository *MongoGroupRepository) {
				now := repositoryTime()
				expiredAt := now.Add(time.Minute)
				insertGroupWithMember(t, repository, groupDocument{
					ID:          "group-1",
					WorkspaceID: "workspace-1",
					GroupingRule: groupingRuleDocument{
						ExpirationDate: now.Add(24 * time.Hour),
					},
					CreatedAt: now,
					UpdatedAt: now,
				}, individualMemberDocument{
					ID:             "member-1",
					GroupID:        "group-1",
					NTAccount:      "user1",
					ExpirationDate: now,
					ExpiredAt:      &expiredAt,
					CreatedAt:      now,
					UpdatedAt:      now,
				})
				insertIndividualMemberExpiryTask(t, repository, individualMemberExpiryTaskDocument{
					ID:               "member-task-1",
					GroupID:          "group-1",
					NTAccount:        "user1",
					ExpirationBucket: "2026-05-10",
				})
			},
			command: group.ExpireIndividualMemberCommand{
				TaskID:           "member-task-1",
				GroupID:          "group-1",
				NTAccount:        "user1",
				ExpirationBucket: "2026-05-10",
			},
			want: group.ExpireIndividualMemberStatusAlreadyExpired,
		},
		{
			name: "bucket mismatch keeps task",
			setup: func(t *testing.T, repository *MongoGroupRepository) {
				now := time.Date(2026, 5, 10, 16, 30, 0, 0, time.UTC)
				insertGroupWithMember(t, repository, groupDocument{
					ID:          "group-1",
					WorkspaceID: "workspace-1",
					GroupingRule: groupingRuleDocument{
						ExpirationDate: now.Add(24 * time.Hour),
					},
					CreatedAt: now,
					UpdatedAt: now,
				}, individualMemberDocument{
					ID:             "member-1",
					GroupID:        "group-1",
					NTAccount:      "user1",
					ExpirationDate: now,
					CreatedAt:      now,
					UpdatedAt:      now,
				})
				insertIndividualMemberExpiryTask(t, repository, individualMemberExpiryTaskDocument{
					ID:               "member-task-1",
					GroupID:          "group-1",
					NTAccount:        "user1",
					ExpirationBucket: "2026-05-10",
				})
			},
			command: group.ExpireIndividualMemberCommand{
				TaskID:           "member-task-1",
				GroupID:          "group-1",
				NTAccount:        "user1",
				ExpirationBucket: "2026-05-10",
			},
			want:      group.ExpireIndividualMemberStatusStaleBucket,
			wantTasks: 1,
		},
	}

	runRepositoryExpiryStaleCases(t, tests,
		func(ctx context.Context, repository *MongoGroupRepository, command group.ExpireIndividualMemberCommand, expiredAt time.Time, location *time.Location) (group.ExpireIndividualMemberStatus, error) {
			return repository.ExpireIndividualMember(ctx, command, expiredAt, location)
		},
		func(ctx context.Context, repository *MongoGroupRepository, command group.ExpireIndividualMemberCommand) (int64, error) {
			return repository.memberExpiryTasks.CountDocuments(ctx, bson.M{"_id": command.TaskID})
		},
	)
}

type repositoryExpiryStaleCase[C any, S comparable] struct {
	name      string
	setup     func(t *testing.T, repository *MongoGroupRepository)
	command   C
	want      S
	wantTasks int64
}

func runRepositoryExpiryStaleCases[C any, S comparable](
	t *testing.T,
	tests []repositoryExpiryStaleCase[C, S],
	expire func(context.Context, *MongoGroupRepository, C, time.Time, *time.Location) (S, error),
	countTasks func(context.Context, *MongoGroupRepository, C) (int64, error),
) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newIntegrationDatabase(t)
			repository := NewMongoGroupRepository(client, client.Database(integrationDatabaseName(t.Name())))
			if err := repository.EnsureIndexes(context.Background()); err != nil {
				t.Fatalf("EnsureIndexes error = %v, want nil", err)
			}
			if tt.setup != nil {
				tt.setup(t, repository)
			}
			location, err := group.ParseExpirationBucketLocation("UTC+8")
			if err != nil {
				t.Fatal(err)
			}

			status, err := expire(context.Background(), repository, tt.command, repositoryTime(), location)
			if err != nil {
				t.Fatalf("expire command error = %v", err)
			}
			if status != tt.want {
				t.Fatalf("status = %v, want %v", status, tt.want)
			}
			if tt.wantTasks > 0 {
				count, err := countTasks(context.Background(), repository, tt.command)
				if err != nil {
					t.Fatal(err)
				}
				if count != tt.wantTasks {
					t.Fatalf("task count = %d, want %d", count, tt.wantTasks)
				}
			}
		})
	}
}

func insertGroupWithTask(t *testing.T, repository *MongoGroupRepository, groupDoc groupDocument, taskDoc expiryTaskDocument) {
	t.Helper()
	if _, err := repository.groups.InsertOne(context.Background(), groupDoc); err != nil {
		t.Fatal(err)
	}
	insertExpiryTask(t, repository, taskDoc)
}

func insertGroupWithMember(t *testing.T, repository *MongoGroupRepository, groupDoc groupDocument, memberDoc individualMemberDocument) {
	t.Helper()
	if _, err := repository.groups.InsertOne(context.Background(), groupDoc); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.members.InsertOne(context.Background(), memberDoc); err != nil {
		t.Fatal(err)
	}
}

func insertExpiryTask(t *testing.T, repository *MongoGroupRepository, taskDoc expiryTaskDocument) {
	t.Helper()
	if _, err := repository.expiryTasks.InsertOne(context.Background(), taskDoc); err != nil {
		t.Fatal(err)
	}
}

func insertIndividualMemberExpiryTask(t *testing.T, repository *MongoGroupRepository, taskDoc individualMemberExpiryTaskDocument) {
	t.Helper()
	if _, err := repository.memberExpiryTasks.InsertOne(context.Background(), taskDoc); err != nil {
		t.Fatal(err)
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

func TestMongoGroupRepositoryAddIndividualMembersIntegration(t *testing.T) {
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

	member := group.IndividualMember{
		ID:             "member-2",
		GroupID:        "group-1",
		NTAccount:      "user2",
		ExpirationDate: repositoryTime().Add(24 * time.Hour),
		CreatedAt:      repositoryTime(),
		UpdatedAt:      repositoryTime(),
	}
	members, err := repository.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID:       "workspace-1",
		GroupID:           "group-1",
		IndividualMembers: []group.IndividualMember{member},
	})
	if err != nil {
		t.Fatalf("AddIndividualMembers error = %v, want nil", err)
	}
	if len(members) != 1 || members[0].NTAccount != "user2" {
		t.Fatalf("members = %+v, want user2", members)
	}
	count, err := db.Collection(groupIndividualMemberCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1", "nt_account": "user2", "deleted_at": nil})
	if err != nil {
		t.Fatalf("count members: %v", err)
	}
	if count != 1 {
		t.Fatalf("active member count = %d, want 1", count)
	}
}

func TestMongoGroupRepositoryAddIndividualMembersWritesExpiryTaskIntegration(t *testing.T) {
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

	member := group.IndividualMember{
		ID:             "member-2",
		GroupID:        "group-1",
		NTAccount:      "user2",
		ExpirationDate: repositoryTime().Add(24 * time.Hour),
		CreatedAt:      repositoryTime(),
		UpdatedAt:      repositoryTime(),
		ExpiryTask: &group.IndividualMemberExpiryTask{
			ID:               "member-task-2",
			GroupID:          "group-1",
			NTAccount:        "user2",
			ExpirationBucket: "2026-05-10",
		},
	}
	if _, err := repository.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID:       "workspace-1",
		GroupID:           "group-1",
		IndividualMembers: []group.IndividualMember{member},
	}); err != nil {
		t.Fatalf("AddIndividualMembers error = %v, want nil", err)
	}

	count, err := db.Collection(individualMemberExpiryTaskCollectionName).CountDocuments(context.Background(), bson.M{"_id": "member-task-2"})
	if err != nil {
		t.Fatalf("count individual member expiry tasks: %v", err)
	}
	if count != 1 {
		t.Fatalf("task count = %d, want 1", count)
	}
}

func TestMongoGroupRepositoryAddIndividualMembersMissingGroupIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	_, err := repository.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID: "workspace-1",
		GroupID:     "missing",
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-2",
			GroupID:        "missing",
			NTAccount:      "user2",
			ExpirationDate: repositoryTime().Add(24 * time.Hour),
			CreatedAt:      repositoryTime(),
			UpdatedAt:      repositoryTime(),
		}},
	})
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("AddIndividualMembers error = %v, want ErrNotFound", err)
	}
}

func TestMongoGroupRepositoryAddIndividualMembersDuplicateActiveMemberIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	_, err := repository.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-2",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: repositoryTime().Add(24 * time.Hour),
			CreatedAt:      repositoryTime(),
			UpdatedAt:      repositoryTime(),
		}},
	})
	if !errors.Is(err, group.ErrDuplicateMember) {
		t.Fatalf("AddIndividualMembers error = %v, want ErrDuplicateMember", err)
	}
}

func TestMongoGroupRepositoryUpdateIndividualMemberExpirationIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	expiration := repositoryTime().Add(48 * time.Hour)
	updatedAt := repositoryTime().Add(time.Hour)
	err := repository.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		NTAccount:      "user1",
		ExpirationDate: expiration,
	}, updatedAt)
	if err != nil {
		t.Fatalf("UpdateIndividualMemberExpiration error = %v, want nil", err)
	}
	var doc individualMemberDocument
	err = db.Collection(groupIndividualMemberCollectionName).FindOne(context.Background(), bson.M{"group_id": "group-1", "nt_account": "user1"}).Decode(&doc)
	if err != nil {
		t.Fatalf("find member: %v", err)
	}
	if !doc.ExpirationDate.Equal(expiration) || !doc.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("member doc = %+v, want updated expiration and updated_at", doc)
	}
}

func TestMongoGroupRepositoryUpdateIndividualMemberExpirationReplacesExpiryTaskIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}
	expiredAt := repositoryTime().Add(30 * time.Minute)
	if _, err := repository.members.UpdateOne(context.Background(),
		bson.M{"group_id": "group-1", "nt_account": "user1"},
		bson.M{"$set": bson.M{"expired_at": expiredAt}},
	); err != nil {
		t.Fatalf("seed expired_at: %v", err)
	}
	insertIndividualMemberExpiryTask(t, repository, individualMemberExpiryTaskDocument{
		ID:               "member-task-old",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-06-01",
	})

	expiration := repositoryTime().Add(48 * time.Hour)
	updatedAt := repositoryTime().Add(time.Hour)
	err := repository.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		NTAccount:      "user1",
		ExpirationDate: expiration,
		ExpiryTask: &group.IndividualMemberExpiryTask{
			ID:               "member-task-new",
			GroupID:          "group-1",
			NTAccount:        "user1",
			ExpirationBucket: "2026-05-11",
		},
	}, updatedAt)
	if err != nil {
		t.Fatalf("UpdateIndividualMemberExpiration error = %v, want nil", err)
	}

	var doc individualMemberDocument
	err = db.Collection(groupIndividualMemberCollectionName).FindOne(context.Background(), bson.M{"group_id": "group-1", "nt_account": "user1"}).Decode(&doc)
	if err != nil {
		t.Fatalf("find member: %v", err)
	}
	if doc.ExpiredAt != nil {
		t.Fatalf("ExpiredAt = %v, want nil", doc.ExpiredAt)
	}
	oldCount, err := db.Collection(individualMemberExpiryTaskCollectionName).CountDocuments(context.Background(), bson.M{"_id": "member-task-old"})
	if err != nil {
		t.Fatal(err)
	}
	newCount, err := db.Collection(individualMemberExpiryTaskCollectionName).CountDocuments(context.Background(), bson.M{"_id": "member-task-new"})
	if err != nil {
		t.Fatal(err)
	}
	if oldCount != 0 || newCount != 1 {
		t.Fatalf("old/new task counts = %d/%d, want 0/1", oldCount, newCount)
	}
}

func TestMongoGroupRepositoryUpdateIndividualMemberExpirationMissingMemberIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	err := repository.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		NTAccount:      "missing",
		ExpirationDate: repositoryTime().Add(48 * time.Hour),
	}, repositoryTime().Add(time.Hour))
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("UpdateIndividualMemberExpiration error = %v, want ErrNotFound", err)
	}
}

func TestMongoGroupRepositoryDeleteIndividualMemberIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}

	deletedAt := repositoryTime().Add(time.Hour)
	err := repository.DeleteIndividualMember(context.Background(), group.DeleteIndividualMemberInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		NTAccount:   "user1",
	}, deletedAt)
	if err != nil {
		t.Fatalf("DeleteIndividualMember error = %v, want nil", err)
	}
	count, err := db.Collection(groupIndividualMemberCollectionName).CountDocuments(context.Background(), bson.M{"group_id": "group-1", "nt_account": "user1", "deleted_at": deletedAt})
	if err != nil {
		t.Fatalf("count deleted member: %v", err)
	}
	if count != 1 {
		t.Fatalf("deleted member count = %d, want 1", count)
	}
}

func TestMongoGroupRepositoryDeleteIndividualMemberDeletesExpiryTaskIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	if _, err := repository.Create(context.Background(), repositoryGroup()); err != nil {
		t.Fatalf("Create error = %v, want nil", err)
	}
	insertIndividualMemberExpiryTask(t, repository, individualMemberExpiryTaskDocument{
		ID:               "member-task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-06-01",
	})

	err := repository.DeleteIndividualMember(context.Background(), group.DeleteIndividualMemberInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		NTAccount:   "user1",
	}, repositoryTime().Add(time.Hour))
	if err != nil {
		t.Fatalf("DeleteIndividualMember error = %v, want nil", err)
	}
	count, err := db.Collection(individualMemberExpiryTaskCollectionName).CountDocuments(context.Background(), bson.M{"_id": "member-task-1"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("task count = %d, want 0", count)
	}
}

func TestMongoGroupRepositoryDeleteIndividualMemberMissingTargetIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoGroupRepository(client, db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}

	err := repository.DeleteIndividualMember(context.Background(), group.DeleteIndividualMemberInput{
		WorkspaceID: "workspace-1",
		GroupID:     "missing",
		NTAccount:   "user1",
	}, repositoryTime().Add(time.Hour))
	if err != nil {
		t.Fatalf("DeleteIndividualMember missing group error = %v, want nil", err)
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
	db := client.Database(integrationDatabaseName(t.Name()))
	t.Cleanup(func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Fatalf("drop database: %v", err)
		}
	})
	return client, db
}

func integrationDatabaseName(testName string) string {
	sanitized := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		".", "_",
		"\"", "_",
		"$", "_",
		"*", "_",
		"<", "_",
		">", "_",
		":", "_",
		"|", "_",
		"?", "_",
	).Replace(testName)
	name := integrationDatabaseNamePrefix + sanitized
	if len(name) <= maxMongoDatabaseNameLength {
		return name
	}

	suffix := "_" + integrationDatabaseNameHash(sanitized)
	availableNameLength := maxMongoDatabaseNameLength - len(integrationDatabaseNamePrefix) - len(suffix)
	return integrationDatabaseNamePrefix + sanitized[:availableNameLength] + suffix
}

func integrationDatabaseNameHash(value string) string {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(value))
	return strconv.FormatUint(hasher.Sum64(), 36)
}
