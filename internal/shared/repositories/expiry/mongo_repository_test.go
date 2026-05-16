package expiry

import (
	"context"
	"hash/fnv"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	maxMongoDatabaseNameLength    = 63
	integrationDatabaseNamePrefix = "wpm_expiry_test_"
)

func TestGroupTaskDocumentMapping(t *testing.T) {
	task := GroupTask{ID: "task-1", WorkspaceID: "workspace-1", GroupID: "group-1", ExpirationBucket: "2026-05-16"}
	doc := newGroupTaskDocument(task)
	if doc.ID != task.ID || doc.WorkspaceID != task.WorkspaceID || doc.GroupID != task.GroupID || doc.ExpirationBucket != task.ExpirationBucket {
		t.Fatalf("doc = %+v, want %+v", doc, task)
	}
	if got := doc.toModel(); got != task {
		t.Fatalf("toModel() = %+v, want %+v", got, task)
	}
}

func TestIndividualMemberTaskDocumentMapping(t *testing.T) {
	task := IndividualMemberTask{ID: "task-1", GroupID: "group-1", NTAccount: "user1", ExpirationBucket: "2026-05-16"}
	doc := newIndividualMemberTaskDocument(task)
	if doc.ID != task.ID || doc.GroupID != task.GroupID || doc.NTAccount != task.NTAccount || doc.ExpirationBucket != task.ExpirationBucket {
		t.Fatalf("doc = %+v, want %+v", doc, task)
	}
	if got := doc.toModel(); got != task {
		t.Fatalf("toModel() = %+v, want %+v", got, task)
	}
}

func TestIndexModels(t *testing.T) {
	groupIndexes := groupTaskIndexModels()
	if len(groupIndexes) != 2 {
		t.Fatalf("group indexes len = %d, want 2", len(groupIndexes))
	}
	if !reflect.DeepEqual(groupIndexes[1].Keys, bson.D{{Key: "expiration_bucket", Value: 1}, {Key: "_id", Value: 1}}) {
		t.Fatalf("group due index keys = %#v", groupIndexes[1].Keys)
	}
	groupUnique := indexOptions(t, groupIndexes[0])
	if groupUnique.Name == nil || *groupUnique.Name != GroupTaskActiveGroupUniqueIndexName {
		t.Fatalf("group unique index name = %v, want %s", groupUnique.Name, GroupTaskActiveGroupUniqueIndexName)
	}
	if groupUnique.Unique == nil || !*groupUnique.Unique {
		t.Fatal("group unique index Unique = false, want true")
	}

	memberIndexes := individualMemberTaskIndexModels()
	if len(memberIndexes) != 2 {
		t.Fatalf("member indexes len = %d, want 2", len(memberIndexes))
	}
	if !reflect.DeepEqual(memberIndexes[1].Keys, bson.D{{Key: "expiration_bucket", Value: 1}, {Key: "_id", Value: 1}}) {
		t.Fatalf("member due index keys = %#v", memberIndexes[1].Keys)
	}
	memberUnique := indexOptions(t, memberIndexes[0])
	if memberUnique.Name == nil || *memberUnique.Name != IndividualMemberTaskActiveMemberUniqueIndexName {
		t.Fatalf("member unique index name = %v, want %s", memberUnique.Name, IndividualMemberTaskActiveMemberUniqueIndexName)
	}
	if memberUnique.Unique == nil || !*memberUnique.Unique {
		t.Fatal("member unique index Unique = false, want true")
	}
}

func TestDueTaskFilter(t *testing.T) {
	got := dueTaskFilter("2026-05-16", &Cursor{ExpirationBucket: "2026-05-15", ID: "task-9"})
	want := bson.M{
		"expiration_bucket": bson.M{"$lte": "2026-05-16"},
		"$or": bson.A{
			bson.M{"expiration_bucket": bson.M{"$gt": "2026-05-15"}},
			bson.M{"expiration_bucket": "2026-05-15", "_id": bson.M{"$gt": "task-9"}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestListDueGroupTasksIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoRepository(db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	tasks := []GroupTask{
		{ID: "task-1", WorkspaceID: "workspace-1", GroupID: "group-1", ExpirationBucket: "2026-05-14"},
		{ID: "task-2", WorkspaceID: "workspace-1", GroupID: "group-2", ExpirationBucket: "2026-05-15"},
		{ID: "task-3", WorkspaceID: "workspace-1", GroupID: "group-3", ExpirationBucket: "2026-05-16"},
		{ID: "task-4", WorkspaceID: "workspace-1", GroupID: "group-4", ExpirationBucket: "2026-05-17"},
	}
	for _, task := range tasks {
		if err := repository.InsertGroupTask(context.Background(), task); err != nil {
			t.Fatalf("InsertGroupTask(%s) error = %v", task.ID, err)
		}
	}

	first, err := repository.ListDueGroupTasks(context.Background(), "2026-05-16", nil, 2)
	if err != nil {
		t.Fatalf("ListDueGroupTasks first error = %v, want nil", err)
	}
	if gotIDs(first) != "task-1,task-2" {
		t.Fatalf("first IDs = %s, want task-1,task-2", gotIDs(first))
	}

	second, err := repository.ListDueGroupTasks(context.Background(), "2026-05-16", &Cursor{ExpirationBucket: first[1].ExpirationBucket, ID: first[1].ID}, 2)
	if err != nil {
		t.Fatalf("ListDueGroupTasks second error = %v, want nil", err)
	}
	if gotIDs(second) != "task-3" {
		t.Fatalf("second IDs = %s, want task-3", gotIDs(second))
	}

	if err := client.Database(db.Name()).Drop(context.Background()); err != nil {
		t.Fatalf("drop database: %v", err)
	}
}

func TestListDueIndividualMemberTasksIntegration(t *testing.T) {
	client, db := newIntegrationDatabase(t)
	repository := NewMongoRepository(db)
	if err := repository.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes error = %v, want nil", err)
	}
	tasks := []IndividualMemberTask{
		{ID: "task-1", GroupID: "group-1", NTAccount: "user1", ExpirationBucket: "2026-05-14"},
		{ID: "task-2", GroupID: "group-1", NTAccount: "user2", ExpirationBucket: "2026-05-15"},
		{ID: "task-3", GroupID: "group-1", NTAccount: "user3", ExpirationBucket: "2026-05-16"},
		{ID: "task-4", GroupID: "group-1", NTAccount: "user4", ExpirationBucket: "2026-05-17"},
	}
	if err := repository.InsertIndividualMemberTasks(context.Background(), tasks); err != nil {
		t.Fatalf("InsertIndividualMemberTasks error = %v", err)
	}

	first, err := repository.ListDueIndividualMemberTasks(context.Background(), "2026-05-16", nil, 2)
	if err != nil {
		t.Fatalf("ListDueIndividualMemberTasks first error = %v, want nil", err)
	}
	if gotMemberIDs(first) != "task-1,task-2" {
		t.Fatalf("first IDs = %s, want task-1,task-2", gotMemberIDs(first))
	}

	second, err := repository.ListDueIndividualMemberTasks(context.Background(), "2026-05-16", &Cursor{ExpirationBucket: first[1].ExpirationBucket, ID: first[1].ID}, 2)
	if err != nil {
		t.Fatalf("ListDueIndividualMemberTasks second error = %v, want nil", err)
	}
	if gotMemberIDs(second) != "task-3" {
		t.Fatalf("second IDs = %s, want task-3", gotMemberIDs(second))
	}

	if err := client.Database(db.Name()).Drop(context.Background()); err != nil {
		t.Fatalf("drop database: %v", err)
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

func gotIDs(tasks []GroupTask) string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return strings.Join(ids, ",")
}

func gotMemberIDs(tasks []IndividualMemberTask) string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return strings.Join(ids, ",")
}

func newIntegrationDatabase(t *testing.T) (*mongo.Client, *mongo.Database) {
	t.Helper()
	uri := os.Getenv("EXPIRY_REPOSITORY_MONGODB_TEST_URI")
	if uri == "" {
		t.Skip("EXPIRY_REPOSITORY_MONGODB_TEST_URI is not set")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect mongodb: %v", err)
	}
	t.Cleanup(func() {
		if disconnectErr := client.Disconnect(context.Background()); disconnectErr != nil {
			t.Fatalf("disconnect mongodb: %v", disconnectErr)
		}
	})
	db := client.Database(integrationDatabaseName(t.Name()))
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
	})
	return client, db
}

func integrationDatabaseName(testName string) string {
	sanitized := strings.NewReplacer(
		"/", "_",
		" ", "_",
		"(", "_",
		")", "_",
		"-", "_",
	).Replace(testName)
	if len(integrationDatabaseNamePrefix+sanitized) <= maxMongoDatabaseNameLength {
		return integrationDatabaseNamePrefix + sanitized
	}
	suffix := "_" + integrationDatabaseNameHash(sanitized)
	availableNameLength := maxMongoDatabaseNameLength - len(integrationDatabaseNamePrefix) - len(suffix)
	return integrationDatabaseNamePrefix + sanitized[:availableNameLength] + suffix
}

func integrationDatabaseNameHash(value string) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(value))
	return strconv.FormatUint(uint64(hash.Sum32()), 16)
}
