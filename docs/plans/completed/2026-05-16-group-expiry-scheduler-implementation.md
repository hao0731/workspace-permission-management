# Group Expiry Scheduler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `group-expiry-scheduler`, an independent Go service that scans due group expiry tasks and individual-member expiry tasks, then publishes the existing JetStream command CloudEvents consumed by `group-service`.

**Architecture:** Extract expiry task collection ownership into `internal/shared/repositories/expiry`, then refactor `group-service` to use that shared repository without changing command behavior. Add a scheduler service with config, CloudEvent transport builders, an eventbus-backed publisher, cursor-based due scanning, a non-overlap job runner, liveness, gocron wiring, and local Docker Compose runtime.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, NATS JetStream through `internal/shared/eventbus`, CloudEvents SDK for Go, `github.com/go-co-op/gocron/v2`, `log/slog`, `viper`, standard `testing`, Docker Compose.

---

## Source Design and Policies

Read these before implementing:

- [Group Expiry Scheduler Design](../../designs/group-expiry-scheduler.md)
- [Group Service Design](../../designs/group-service.md)
- [Group Expiry Command Design](../../designs/group-service-group-expiry-command.md)
- [Group Individual Member Expiry Command Design](../../designs/group-service-individual-member-expiry-command.md)
- [Backend Architecture Principle](../../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend service logic must keep Echo, MongoDB driver, NATS, JetStream, gocron, and transport DTO types out of service packages.
- `internal/shared/repositories/expiry` may depend on MongoDB but must not import service-private packages.
- The scheduler service should depend on consumer-side repository and publisher interfaces.
- CloudEvent payloads, MongoDB schemas, indexes, config keys, and NATS subjects are contracts and need focused tests.
- Implementation plans belong in `docs/plans/active/` and must link back to their source design.

## Working Tree Note

At plan-writing time, the working tree is clean and the source design is committed as:

```txt
552943c docs: add group expiry scheduler design
```

Do not revert unrelated user changes if the working tree becomes dirty while executing this plan.

## File Structure

Create:

- `internal/shared/repositories/expiry/tasks.go`: shared task models, collection names, cursor type, and index name constants.
- `internal/shared/repositories/expiry/mongo_repository.go`: MongoDB repository for task inserts, deletes, identity lookups, index creation, and due cursor scans.
- `internal/shared/repositories/expiry/mongo_repository_test.go`: unit tests for document mapping, index models, due filters, and integration tests for cursor scans when MongoDB test URI is available.
- `internal/group-expiry-scheduler/config/config.go`: scheduler config loading and validation.
- `internal/group-expiry-scheduler/config/config_test.go`: scheduler config tests.
- `internal/group-expiry-scheduler/transport/expiry_event.go`: CloudEvent builders for group and individual-member expiry commands.
- `internal/group-expiry-scheduler/transport/expiry_event_test.go`: CloudEvent builder tests.
- `internal/group-expiry-scheduler/services/scheduler_service.go`: due scan workflow, publish orchestration, job stats, and logging.
- `internal/group-expiry-scheduler/services/scheduler_service_test.go`: scheduler workflow tests.
- `internal/group-expiry-scheduler/services/job_runner.go`: non-blocking in-process overlap guard.
- `internal/group-expiry-scheduler/services/job_runner_test.go`: overlap guard tests.
- `cmd/group-expiry-scheduler/main.go`: service entrypoint, Mongo/NATS/Echo/gocron wiring, health, and shutdown.
- `cmd/group-expiry-scheduler/main_test.go`: process indicator, liveness route, and gocron job wiring tests.
- `cmd/group-expiry-scheduler/expiry_command_publisher.go`: eventbus-backed command publisher.
- `cmd/group-expiry-scheduler/expiry_command_publisher_test.go`: publisher tests.

Modify:

- `go.mod`: add `github.com/go-co-op/gocron/v2`.
- `go.sum`: record gocron checksums.
- `internal/group-service/repositories/mongo_group_repository.go`: replace local expiry task collection code with shared expiry repository calls.
- `internal/group-service/repositories/mongo_group_repository_test.go`: update tests to use shared expiry collection names and shared repository expectations.
- `docker-compose.yml`: add local `group-expiry-scheduler` runtime service and keep NATS stream setup aligned with scheduler publish subjects.

No public HTTP response DTO should expose expiry task data.

---

### Task 1: Add gocron Dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add the dependency**

Run:

```bash
go get github.com/go-co-op/gocron/v2@v2.21.2
```

Expected: `go.mod` gains `github.com/go-co-op/gocron/v2 v2.21.2` and `go.sum` records its checksums. If Go also updates indirect checksums, keep them.

- [ ] **Step 2: Verify dependency resolution**

Run:

```bash
go test ./internal/shared/health
```

Expected: PASS. This is a fast smoke test proving module resolution still works before code changes begin.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add gocron dependency"
```

---

### Task 2: Shared Expiry Repository

**Files:**
- Create: `internal/shared/repositories/expiry/tasks.go`
- Create: `internal/shared/repositories/expiry/mongo_repository.go`
- Create: `internal/shared/repositories/expiry/mongo_repository_test.go`

- [ ] **Step 1: Write failing shared repository tests**

Create `internal/shared/repositories/expiry/mongo_repository_test.go` with tests that cover mapping, index models, due filters, and cursor scan integration. Use `EXPIRY_REPOSITORY_MONGODB_TEST_URI` for integration tests so they do not run unless explicitly enabled.

```go
package expiry

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
	_, db := newIntegrationDatabase(t)
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
}
```

- [ ] **Step 2: Run shared repository tests to verify failure**

Run:

```bash
go test ./internal/shared/repositories/expiry
```

Expected: FAIL because the package does not exist or the tested types/functions are undefined.

- [ ] **Step 3: Create shared task models**

Create `internal/shared/repositories/expiry/tasks.go`:

```go
package expiry

const (
	GroupTaskCollectionName            = "group_expiry_task"
	IndividualMemberTaskCollectionName = "individual_member_expiry_task"

	GroupTaskActiveGroupUniqueIndexName                 = "group_expiry_task_active_workspace_group_unique"
	GroupTaskBucketIndexName                            = "group_expiry_task_bucket_id"
	IndividualMemberTaskActiveMemberUniqueIndexName     = "individual_member_expiry_task_active_group_account_unique"
	IndividualMemberTaskBucketIndexName                 = "individual_member_expiry_task_bucket_id"
)

type GroupTask struct {
	ID               string
	WorkspaceID      string
	GroupID          string
	ExpirationBucket string
}

type IndividualMemberTask struct {
	ID               string
	GroupID          string
	NTAccount        string
	ExpirationBucket string
}

type Cursor struct {
	ExpirationBucket string
	ID               string
}
```

- [ ] **Step 4: Create Mongo repository implementation**

Create `internal/shared/repositories/expiry/mongo_repository.go`:

```go
package expiry

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoRepository struct {
	groupTasks            *mongo.Collection
	individualMemberTasks *mongo.Collection
}

type groupTaskDocument struct {
	ID               string `bson:"_id"`
	WorkspaceID      string `bson:"workspace_id"`
	GroupID          string `bson:"group_id"`
	ExpirationBucket string `bson:"expiration_bucket"`
}

type individualMemberTaskDocument struct {
	ID               string `bson:"_id"`
	GroupID          string `bson:"group_id"`
	NTAccount        string `bson:"nt_account"`
	ExpirationBucket string `bson:"expiration_bucket"`
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		groupTasks:            db.Collection(GroupTaskCollectionName),
		individualMemberTasks: db.Collection(IndividualMemberTaskCollectionName),
	}
}

func (r *MongoRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.groupTasks.Indexes().CreateMany(ctx, groupTaskIndexModels()); err != nil {
		return fmt.Errorf("create group expiry task indexes: %w", err)
	}
	if _, err := r.individualMemberTasks.Indexes().CreateMany(ctx, individualMemberTaskIndexModels()); err != nil {
		return fmt.Errorf("create individual member expiry task indexes: %w", err)
	}
	return nil
}

func (r *MongoRepository) InsertGroupTask(ctx context.Context, task GroupTask) error {
	if _, err := r.groupTasks.InsertOne(ctx, newGroupTaskDocument(task)); err != nil {
		return fmt.Errorf("insert group expiry task: %w", err)
	}
	return nil
}

func (r *MongoRepository) InsertIndividualMemberTasks(ctx context.Context, tasks []IndividualMemberTask) error {
	if len(tasks) == 0 {
		return nil
	}
	docs := make([]any, 0, len(tasks))
	for _, task := range tasks {
		docs = append(docs, newIndividualMemberTaskDocument(task))
	}
	if _, err := r.individualMemberTasks.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("insert individual member expiry tasks: %w", err)
	}
	return nil
}

func (r *MongoRepository) FindGroupTask(ctx context.Context, task GroupTask) (*GroupTask, error) {
	var doc groupTaskDocument
	err := r.groupTasks.FindOne(ctx, bson.M{
		"_id":               task.ID,
		"workspace_id":      task.WorkspaceID,
		"group_id":          task.GroupID,
		"expiration_bucket": task.ExpirationBucket,
	}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("find group expiry task: %w", err)
	}
	model := doc.toModel()
	return &model, nil
}

func (r *MongoRepository) FindIndividualMemberTask(ctx context.Context, task IndividualMemberTask) (*IndividualMemberTask, error) {
	var doc individualMemberTaskDocument
	err := r.individualMemberTasks.FindOne(ctx, bson.M{
		"_id":               task.ID,
		"group_id":          task.GroupID,
		"nt_account":        task.NTAccount,
		"expiration_bucket": task.ExpirationBucket,
	}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("find individual member expiry task: %w", err)
	}
	model := doc.toModel()
	return &model, nil
}

func (r *MongoRepository) DeleteGroupTasks(ctx context.Context, workspaceID string, groupID string) error {
	if _, err := r.groupTasks.DeleteMany(ctx, bson.M{"workspace_id": workspaceID, "group_id": groupID}); err != nil {
		return fmt.Errorf("delete group expiry tasks: %w", err)
	}
	return nil
}

func (r *MongoRepository) DeleteGroupTaskByID(ctx context.Context, taskID string) error {
	if _, err := r.groupTasks.DeleteOne(ctx, bson.M{"_id": taskID}); err != nil {
		return fmt.Errorf("delete group expiry task: %w", err)
	}
	return nil
}

func (r *MongoRepository) DeleteIndividualMemberTasksByGroup(ctx context.Context, groupID string) error {
	if _, err := r.individualMemberTasks.DeleteMany(ctx, bson.M{"group_id": groupID}); err != nil {
		return fmt.Errorf("delete individual member expiry tasks: %w", err)
	}
	return nil
}

func (r *MongoRepository) DeleteIndividualMemberTask(ctx context.Context, groupID string, ntAccount string) error {
	if _, err := r.individualMemberTasks.DeleteOne(ctx, bson.M{"group_id": groupID, "nt_account": ntAccount}); err != nil {
		return fmt.Errorf("delete individual member expiry task: %w", err)
	}
	return nil
}

func (r *MongoRepository) DeleteIndividualMemberTaskByID(ctx context.Context, taskID string) error {
	if _, err := r.individualMemberTasks.DeleteOne(ctx, bson.M{"_id": taskID}); err != nil {
		return fmt.Errorf("delete individual member expiry task: %w", err)
	}
	return nil
}

func (r *MongoRepository) ListDueGroupTasks(ctx context.Context, dueBucket string, cursor *Cursor, limit int) ([]GroupTask, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}
	findOptions := options.Find().
		SetSort(bson.D{{Key: "expiration_bucket", Value: 1}, {Key: "_id", Value: 1}}).
		SetLimit(int64(limit))
	mongoCursor, err := r.groupTasks.Find(ctx, dueTaskFilter(dueBucket, cursor), findOptions)
	if err != nil {
		return nil, fmt.Errorf("find due group expiry tasks: %w", err)
	}
	defer func() {
		_ = mongoCursor.Close(ctx)
	}()
	var docs []groupTaskDocument
	if err := mongoCursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode due group expiry tasks: %w", err)
	}
	tasks := make([]GroupTask, 0, len(docs))
	for _, doc := range docs {
		tasks = append(tasks, doc.toModel())
	}
	return tasks, nil
}

func (r *MongoRepository) ListDueIndividualMemberTasks(ctx context.Context, dueBucket string, cursor *Cursor, limit int) ([]IndividualMemberTask, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}
	findOptions := options.Find().
		SetSort(bson.D{{Key: "expiration_bucket", Value: 1}, {Key: "_id", Value: 1}}).
		SetLimit(int64(limit))
	mongoCursor, err := r.individualMemberTasks.Find(ctx, dueTaskFilter(dueBucket, cursor), findOptions)
	if err != nil {
		return nil, fmt.Errorf("find due individual member expiry tasks: %w", err)
	}
	defer func() {
		_ = mongoCursor.Close(ctx)
	}()
	var docs []individualMemberTaskDocument
	if err := mongoCursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode due individual member expiry tasks: %w", err)
	}
	tasks := make([]IndividualMemberTask, 0, len(docs))
	for _, doc := range docs {
		tasks = append(tasks, doc.toModel())
	}
	return tasks, nil
}

func groupTaskIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "workspace_id", Value: 1}, {Key: "group_id", Value: 1}},
			Options: options.Index().
				SetName(GroupTaskActiveGroupUniqueIndexName).
				SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "expiration_bucket", Value: 1}, {Key: "_id", Value: 1}},
			Options: options.Index().SetName(GroupTaskBucketIndexName),
		},
	}
}

func individualMemberTaskIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "group_id", Value: 1}, {Key: "nt_account", Value: 1}},
			Options: options.Index().
				SetName(IndividualMemberTaskActiveMemberUniqueIndexName).
				SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "expiration_bucket", Value: 1}, {Key: "_id", Value: 1}},
			Options: options.Index().SetName(IndividualMemberTaskBucketIndexName),
		},
	}
}

func dueTaskFilter(dueBucket string, cursor *Cursor) bson.M {
	filter := bson.M{"expiration_bucket": bson.M{"$lte": dueBucket}}
	if cursor == nil {
		return filter
	}
	filter["$or"] = bson.A{
		bson.M{"expiration_bucket": bson.M{"$gt": cursor.ExpirationBucket}},
		bson.M{"expiration_bucket": cursor.ExpirationBucket, "_id": bson.M{"$gt": cursor.ID}},
	}
	return filter
}

func newGroupTaskDocument(task GroupTask) groupTaskDocument {
	return groupTaskDocument{
		ID:               task.ID,
		WorkspaceID:      task.WorkspaceID,
		GroupID:          task.GroupID,
		ExpirationBucket: task.ExpirationBucket,
	}
}

func (doc groupTaskDocument) toModel() GroupTask {
	return GroupTask{
		ID:               doc.ID,
		WorkspaceID:      doc.WorkspaceID,
		GroupID:          doc.GroupID,
		ExpirationBucket: doc.ExpirationBucket,
	}
}

func newIndividualMemberTaskDocument(task IndividualMemberTask) individualMemberTaskDocument {
	return individualMemberTaskDocument{
		ID:               task.ID,
		GroupID:          task.GroupID,
		NTAccount:        task.NTAccount,
		ExpirationBucket: task.ExpirationBucket,
	}
}

func (doc individualMemberTaskDocument) toModel() IndividualMemberTask {
	return IndividualMemberTask{
		ID:               doc.ID,
		GroupID:          doc.GroupID,
		NTAccount:        doc.NTAccount,
		ExpirationBucket: doc.ExpirationBucket,
	}
}
```

- [ ] **Step 5: Add missing test helpers**

Complete `mongo_repository_test.go` with these helpers:

```go
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
```

Add integration helpers by copying the existing `newIntegrationDatabase`, `integrationDatabaseName`, and database-name hash helpers from `internal/group-service/repositories/mongo_group_repository_test.go`, changing the env var to `EXPIRY_REPOSITORY_MONGODB_TEST_URI` and the database prefix to `wpm_expiry_test_`.

- [ ] **Step 6: Run shared repository tests**

Run:

```bash
go test ./internal/shared/repositories/expiry
```

Expected: PASS. If `EXPIRY_REPOSITORY_MONGODB_TEST_URI` is not set, integration tests should skip and unit tests should pass.

- [ ] **Step 7: Commit**

```bash
git add internal/shared/repositories/expiry
git commit -m "feat: add shared expiry task repository"
```

---

### Task 3: Refactor Group Service to Use Shared Expiry Repository

**Files:**
- Modify: `internal/group-service/repositories/mongo_group_repository.go`
- Modify: `internal/group-service/repositories/mongo_group_repository_test.go`

- [ ] **Step 1: Update repository tests to reference shared collection names**

In `internal/group-service/repositories/mongo_group_repository_test.go`, import:

```go
sharedexpiry "github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
```

Replace collection-name references used only by tests:

```go
db.Collection(groupExpiryTaskCollectionName)
db.Collection(individualMemberExpiryTaskCollectionName)
```

with:

```go
db.Collection(sharedexpiry.GroupTaskCollectionName)
db.Collection(sharedexpiry.IndividualMemberTaskCollectionName)
```

Replace test-only task document construction with shared models where direct inserts are needed:

```go
sharedexpiry.GroupTask{
	ID:               "task-old",
	WorkspaceID:      "workspace-1",
	GroupID:          "group-1",
	ExpirationBucket: "2026-06-01",
}
```

and:

```go
sharedexpiry.IndividualMemberTask{
	ID:               "member-task-1",
	GroupID:          "group-1",
	NTAccount:        "user1",
	ExpirationBucket: "2026-06-01",
}
```

Replace the local task document mapping assertions because those document types move to the shared package:

```go
func TestNewSharedGroupTask(t *testing.T) {
	t.Parallel()

	task := newSharedGroupTask(group.ExpiryTask{
		ID:               "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-10",
	})

	if task.ID != "task-1" || task.WorkspaceID != "workspace-1" || task.GroupID != "group-1" || task.ExpirationBucket != "2026-05-10" {
		t.Fatalf("task = %+v", task)
	}
}

func TestNewSharedIndividualMemberTask(t *testing.T) {
	t.Parallel()

	task := newSharedIndividualMemberTask(group.IndividualMemberExpiryTask{
		ID:               "member-task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-05-10",
	})

	if task.ID != "member-task-1" || task.GroupID != "group-1" || task.NTAccount != "user1" || task.ExpirationBucket != "2026-05-10" {
		t.Fatalf("task = %+v", task)
	}
}
```

In `TestNewIndividualMemberDocumentsIncludesExpiredAtAndTask`, replace:

```go
taskDocs := newIndividualMemberExpiryTaskDocuments(model.IndividualMembers)
```

with:

```go
taskDocs := newSharedIndividualMemberTasks(model.IndividualMembers)
```

In `TestIndexModels`, remove the sections that assert `groupExpiryTaskIndexModels()` and `individualMemberExpiryTaskIndexModels()`. Those index assertions now live in `internal/shared/repositories/expiry/mongo_repository_test.go`.

- [ ] **Step 2: Run group repository tests to verify failure**

Run:

```bash
go test ./internal/group-service/repositories
```

Expected: FAIL because production code still defines and uses local expiry task types and constants, and tests now expect shared repository behavior.

- [ ] **Step 3: Refactor repository construction**

In `internal/group-service/repositories/mongo_group_repository.go`, import:

```go
sharedexpiry "github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
```

Change `MongoGroupRepository` to hold the shared repository:

```go
type MongoGroupRepository struct {
	client           *mongo.Client
	groups           *mongo.Collection
	members          *mongo.Collection
	expiryRepository *sharedexpiry.MongoRepository
}
```

Update `NewMongoGroupRepository`:

```go
func NewMongoGroupRepository(client *mongo.Client, db *mongo.Database) *MongoGroupRepository {
	return &MongoGroupRepository{
		client:           client,
		groups:           db.Collection(groupCollectionName),
		members:          db.Collection(groupIndividualMemberCollectionName),
		expiryRepository: sharedexpiry.NewMongoRepository(db),
	}
}
```

Update `EnsureIndexes`:

```go
func (r *MongoGroupRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.groups.Indexes().CreateMany(ctx, groupIndexModels()); err != nil {
		return fmt.Errorf("create group indexes: %w", err)
	}
	if _, err := r.members.Indexes().CreateMany(ctx, individualMemberIndexModels()); err != nil {
		return fmt.Errorf("create group individual member indexes: %w", err)
	}
	if err := r.expiryRepository.EnsureIndexes(ctx); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 4: Replace local task writes with shared repository calls**

Add conversion helpers near the existing document mapping helpers:

```go
func newSharedGroupTask(task group.ExpiryTask) sharedexpiry.GroupTask {
	return sharedexpiry.GroupTask{
		ID:               task.ID,
		WorkspaceID:      task.WorkspaceID,
		GroupID:          task.GroupID,
		ExpirationBucket: task.ExpirationBucket,
	}
}

func newSharedIndividualMemberTask(task group.IndividualMemberExpiryTask) sharedexpiry.IndividualMemberTask {
	return sharedexpiry.IndividualMemberTask{
		ID:               task.ID,
		GroupID:          task.GroupID,
		NTAccount:        task.NTAccount,
		ExpirationBucket: task.ExpirationBucket,
	}
}

func newSharedIndividualMemberTasks(members []group.IndividualMember) []sharedexpiry.IndividualMemberTask {
	tasks := make([]sharedexpiry.IndividualMemberTask, 0, len(members))
	for _, member := range members {
		if member.ExpiryTask == nil {
			continue
		}
		tasks = append(tasks, newSharedIndividualMemberTask(*member.ExpiryTask))
	}
	return tasks
}
```

Replace calls:

```go
r.expiryTasks.InsertOne(sessionCtx, newExpiryTaskDocument(*input.ExpiryTask))
r.memberExpiryTasks.InsertMany(sessionCtx, docs)
```

with:

```go
r.expiryRepository.InsertGroupTask(sessionCtx, newSharedGroupTask(*input.ExpiryTask))
r.expiryRepository.InsertIndividualMemberTasks(sessionCtx, newSharedIndividualMemberTasks(input.IndividualMembers))
```

Replace deletes:

```go
r.expiryTasks.DeleteMany(sessionCtx, bson.M{"workspace_id": input.WorkspaceID, "group_id": input.GroupID})
r.memberExpiryTasks.DeleteMany(sessionCtx, bson.M{"group_id": input.GroupID})
r.memberExpiryTasks.DeleteOne(sessionCtx, bson.M{"group_id": input.GroupID, "nt_account": input.NTAccount})
```

with:

```go
r.expiryRepository.DeleteGroupTasks(sessionCtx, input.WorkspaceID, input.GroupID)
r.expiryRepository.DeleteIndividualMemberTasksByGroup(sessionCtx, input.GroupID)
r.expiryRepository.DeleteIndividualMemberTask(sessionCtx, input.GroupID, input.NTAccount)
```

- [ ] **Step 5: Replace command lookup and delete helpers**

Update `findExpiryTask` and `findIndividualMemberExpiryTask` to return shared task models:

```go
func (r *MongoGroupRepository) findExpiryTask(ctx context.Context, input group.ExpireGroupingRuleCommand) (*sharedexpiry.GroupTask, error) {
	return r.expiryRepository.FindGroupTask(ctx, sharedexpiry.GroupTask{
		ID:               input.TaskID,
		WorkspaceID:      input.WorkspaceID,
		GroupID:          input.GroupID,
		ExpirationBucket: input.ExpirationBucket,
	})
}

func (r *MongoGroupRepository) deleteExpiryTaskByID(ctx context.Context, taskID string) error {
	return r.expiryRepository.DeleteGroupTaskByID(ctx, taskID)
}

func (r *MongoGroupRepository) findIndividualMemberExpiryTask(ctx context.Context, input group.ExpireIndividualMemberCommand) (*sharedexpiry.IndividualMemberTask, error) {
	return r.expiryRepository.FindIndividualMemberTask(ctx, sharedexpiry.IndividualMemberTask{
		ID:               input.TaskID,
		GroupID:          input.GroupID,
		NTAccount:        input.NTAccount,
		ExpirationBucket: input.ExpirationBucket,
	})
}

func (r *MongoGroupRepository) deleteIndividualMemberExpiryTaskByID(ctx context.Context, taskID string) error {
	return r.expiryRepository.DeleteIndividualMemberTaskByID(ctx, taskID)
}
```

Remove local task collection constants, local task document structs, task index model functions, and local task document mapping helpers from `mongo_group_repository.go`.

- [ ] **Step 6: Run focused tests**

Run:

```bash
go test ./internal/group-service/repositories
```

Expected: PASS.

- [ ] **Step 7: Run group-service package tests**

Run:

```bash
go test ./internal/group-service/...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_group_repository_test.go
git commit -m "refactor: share expiry task repository"
```

---

### Task 4: Scheduler Configuration

**Files:**
- Create: `internal/group-expiry-scheduler/config/config.go`
- Create: `internal/group-expiry-scheduler/config/config_test.go`

- [ ] **Step 1: Write failing config tests**

Create `internal/group-expiry-scheduler/config/config_test.go`:

```go
package config

import (
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

func TestLoadReadsRequiredEnvironment(t *testing.T) {
	t.Setenv("GROUP_EXPIRY_SCHEDULER_ENV", "production")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_HTTP_ADDR", ":9094")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_MONGODB_URI", "mongodb://example:27017")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE", "wpm")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_NATS_URL", "nats://example:4222")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT", "app.todo.group.expiry.process")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT", "app.todo.group.individual-member.expiry.process")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION", "*/5 * * * * *")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_CRON_WITH_SECONDS", "true")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE", "Asia/Taipei")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE", "UTC+8")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", "UTC+8")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_BATCH_SIZE", "25")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT", "9s")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT", "15s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Production {
		t.Fatalf("Environment = %q, want production", cfg.Environment)
	}
	if cfg.HTTPAddr != ":9094" || cfg.MongoDB.URI != "mongodb://example:27017" || cfg.NATS.URL != "nats://example:4222" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.Schedule.Expression != "*/5 * * * * *" || !cfg.Schedule.WithSeconds {
		t.Fatalf("Schedule = %+v", cfg.Schedule)
	}
	if cfg.Schedule.Location.String() != "Asia/Taipei" {
		t.Fatalf("Schedule.Location = %v, want Asia/Taipei", cfg.Schedule.Location)
	}
	if cfg.BatchSize != 25 || cfg.PublishTimeout != 9*time.Second || cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("timeouts/batch = %+v", cfg)
	}
	if cfg.GroupExpiry.BucketLocation.String() != "UTC+08:00" {
		t.Fatalf("GroupExpiry.BucketLocation = %v, want UTC+08:00", cfg.GroupExpiry.BucketLocation)
	}
	if cfg.IndividualMemberExpiry.BucketLocation.String() != "UTC+08:00" {
		t.Fatalf("IndividualMemberExpiry.BucketLocation = %v, want UTC+08:00", cfg.IndividualMemberExpiry.BucketLocation)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	setRequiredConfig(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Development {
		t.Fatalf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.BatchSize != 20 {
		t.Fatalf("BatchSize = %d, want 20", cfg.BatchSize)
	}
	if cfg.PublishTimeout != 15*time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("timeouts = %s/%s", cfg.PublishTimeout, cfg.ShutdownTimeout)
	}
	if cfg.Schedule.WithSeconds {
		t.Fatal("Schedule.WithSeconds = true, want false")
	}
	if cfg.Schedule.Location.String() != "UTC" {
		t.Fatalf("Schedule.Location = %v, want UTC", cfg.Schedule.Location)
	}
}

func TestLoadRejectsMissingRequiredValue(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION", " ")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION is required") {
		t.Fatalf("Load error = %v, want missing cron expression", err)
	}
}

func TestLoadRejectsInvalidCronExpression(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION", "not cron")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION") {
		t.Fatalf("Load error = %v, want invalid cron expression", err)
	}
}

func TestLoadRejectsInvalidSchedulerTimezone(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE", "No/SuchZone")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE") {
		t.Fatalf("Load error = %v, want invalid scheduler timezone", err)
	}
}

func TestLoadRejectsInvalidBucketTimezone(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE", "Asia/Taipei")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE") {
		t.Fatalf("Load error = %v, want invalid bucket timezone", err)
	}
}

func TestLoadRejectsNonPositiveValues(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "batch", key: "GROUP_EXPIRY_SCHEDULER_BATCH_SIZE"},
		{name: "publish timeout", key: "GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT"},
		{name: "shutdown timeout", key: "GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredConfig(t)
			t.Setenv(tt.key, "0")
			if _, err := Load(); err == nil {
				t.Fatal("Load error = nil, want error")
			}
		})
	}
}

func setRequiredConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GROUP_EXPIRY_SCHEDULER_HTTP_ADDR", ":8084")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_NATS_URL", "nats://localhost:4222")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT", "app.todo.group.expiry.process")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT", "app.todo.group.individual-member.expiry.process")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION", "* * * * *")
}
```

- [ ] **Step 2: Run config tests to verify failure**

Run:

```bash
go test ./internal/group-expiry-scheduler/config
```

Expected: FAIL because the config package does not exist.

- [ ] **Step 3: Implement config loading**

Create `internal/group-expiry-scheduler/config/config.go` with `Config`, nested config structs, `Load`, `Validate`, `parseScheduleLocation`, and `parseBucketLocation`. Use viper defaults and `gocron.NewDefaultCron`.

```go
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/spf13/viper"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

type Config struct {
	Environment            environment.Environment
	HTTPAddr               string
	MongoDB                MongoDBConfig
	NATS                   NATSConfig
	GroupExpiry            ExpiryCommandConfig
	IndividualMemberExpiry ExpiryCommandConfig
	Schedule               ScheduleConfig
	BatchSize              int
	PublishTimeout         time.Duration
	ShutdownTimeout        time.Duration
}

type MongoDBConfig struct {
	URI      string
	Database string
}

type NATSConfig struct {
	URL string
}

type ExpiryCommandConfig struct {
	Subject        string
	BucketTimezone string
	BucketLocation *time.Location
}

type ScheduleConfig struct {
	Expression   string
	WithSeconds  bool
	Timezone     string
	Location     *time.Location
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("GROUP_EXPIRY_SCHEDULER_ENV", string(environment.Development))
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT", "10s")
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_BATCH_SIZE", 20)
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT", "15s")
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_CRON_WITH_SECONDS", false)
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE", "UTC")
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE", "UTC")
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", "UTC")

	scheduleLocation, err := parseScheduleLocation(v.GetString("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE"))
	if err != nil {
		return Config{}, err
	}
	groupBucketLocation, err := parseBucketLocation("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE", v.GetString("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE"))
	if err != nil {
		return Config{}, err
	}
	memberBucketLocation, err := parseBucketLocation("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", v.GetString("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE"))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment: environment.Environment(v.GetString("GROUP_EXPIRY_SCHEDULER_ENV")),
		HTTPAddr:    v.GetString("GROUP_EXPIRY_SCHEDULER_HTTP_ADDR"),
		MongoDB: MongoDBConfig{
			URI:      v.GetString("GROUP_EXPIRY_SCHEDULER_MONGODB_URI"),
			Database: v.GetString("GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE"),
		},
		NATS: NATSConfig{URL: v.GetString("GROUP_EXPIRY_SCHEDULER_NATS_URL")},
		GroupExpiry: ExpiryCommandConfig{
			Subject:        v.GetString("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT"),
			BucketTimezone: v.GetString("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE"),
			BucketLocation: groupBucketLocation,
		},
		IndividualMemberExpiry: ExpiryCommandConfig{
			Subject:        v.GetString("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT"),
			BucketTimezone: v.GetString("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE"),
			BucketLocation: memberBucketLocation,
		},
		Schedule: ScheduleConfig{
			Expression:  v.GetString("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION"),
			WithSeconds: v.GetBool("GROUP_EXPIRY_SCHEDULER_CRON_WITH_SECONDS"),
			Timezone:    v.GetString("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE"),
			Location:    scheduleLocation,
		},
		BatchSize:       v.GetInt("GROUP_EXPIRY_SCHEDULER_BATCH_SIZE"),
		PublishTimeout:  v.GetDuration("GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT"),
		ShutdownTimeout: v.GetDuration("GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !environment.IsValidEnvironment(c.Environment) {
		return fmt.Errorf("%w: GROUP_EXPIRY_SCHEDULER_ENV must be %q or %q", environment.ErrInvalidEnv, environment.Development, environment.Production)
	}
	required := map[string]string{
		"GROUP_EXPIRY_SCHEDULER_HTTP_ADDR":                                c.HTTPAddr,
		"GROUP_EXPIRY_SCHEDULER_MONGODB_URI":                              c.MongoDB.URI,
		"GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE":                         c.MongoDB.Database,
		"GROUP_EXPIRY_SCHEDULER_NATS_URL":                                 c.NATS.URL,
		"GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT":             c.GroupExpiry.Subject,
		"GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT": c.IndividualMemberExpiry.Subject,
		"GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION":                          c.Schedule.Expression,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if c.BatchSize <= 0 {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_BATCH_SIZE must be greater than zero")
	}
	if c.PublishTimeout <= 0 {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT must be positive")
	}
	if c.Schedule.Location == nil {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE must be valid")
	}
	cron := gocron.NewDefaultCron(c.Schedule.WithSeconds)
	if err := cron.IsValid(c.Schedule.Expression, c.Schedule.Location, time.Now().In(c.Schedule.Location)); err != nil {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION must be valid: %w", err)
	}
	if c.GroupExpiry.BucketLocation == nil {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE must be valid")
	}
	if c.IndividualMemberExpiry.BucketLocation == nil {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE must be valid")
	}
	return nil
}

func parseScheduleLocation(value string) (*time.Location, error) {
	location, err := time.LoadLocation(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE must be valid: %w", err)
	}
	return location, nil
}

func parseBucketLocation(key string, value string) (*time.Location, error) {
	location, err := group.ParseExpirationBucketLocation(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be valid: %w", key, err)
	}
	return location, nil
}
```

- [ ] **Step 4: Run config tests**

Run:

```bash
go test ./internal/group-expiry-scheduler/config
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/group-expiry-scheduler/config
git commit -m "feat: add group expiry scheduler config"
```

---

### Task 5: Scheduler CloudEvent Transport and Publisher

**Files:**
- Create: `internal/group-expiry-scheduler/transport/expiry_event.go`
- Create: `internal/group-expiry-scheduler/transport/expiry_event_test.go`
- Create: `cmd/group-expiry-scheduler/expiry_command_publisher.go`
- Create: `cmd/group-expiry-scheduler/expiry_command_publisher_test.go`

- [ ] **Step 1: Write failing CloudEvent builder tests**

Create `internal/group-expiry-scheduler/transport/expiry_event_test.go`:

```go
package transport

import (
	"encoding/json"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

func TestNewGroupExpiryCommandEvent(t *testing.T) {
	data, err := NewGroupExpiryCommandEvent(expiry.GroupTask{
		ID:               "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-16",
	}, "app.todo.group.expiry.process", "event-1", time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewGroupExpiryCommandEvent error = %v, want nil", err)
	}
	event := parseEvent(t, data)
	if event.Type() != "app.todo.group.expiry.process" || event.Source() != "group-expiry-scheduler" || event.Subject() != "task-1" || event.ID() != "event-1" {
		t.Fatalf("event metadata = type:%q source:%q subject:%q id:%q", event.Type(), event.Source(), event.Subject(), event.ID())
	}
	var payload groupExpiryCommandData
	if err := event.DataAs(&payload); err != nil {
		t.Fatalf("DataAs error = %v", err)
	}
	if payload.TaskID != "task-1" || payload.WorkspaceID != "workspace-1" || payload.GroupID != "group-1" || payload.ExpirationBucket != "2026-05-16" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestNewIndividualMemberExpiryCommandEvent(t *testing.T) {
	data, err := NewIndividualMemberExpiryCommandEvent(expiry.IndividualMemberTask{
		ID:               "task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-05-16",
	}, "app.todo.group.individual-member.expiry.process", "event-1", time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewIndividualMemberExpiryCommandEvent error = %v, want nil", err)
	}
	event := parseEvent(t, data)
	if event.Type() != "app.todo.group.individual-member.expiry.process" || event.Source() != "group-expiry-scheduler" || event.Subject() != "task-1" || event.ID() != "event-1" {
		t.Fatalf("event metadata = type:%q source:%q subject:%q id:%q", event.Type(), event.Source(), event.Subject(), event.ID())
	}
	var payload individualMemberExpiryCommandData
	if err := event.DataAs(&payload); err != nil {
		t.Fatalf("DataAs error = %v", err)
	}
	if payload.TaskID != "task-1" || payload.GroupID != "group-1" || payload.NTAccount != "user1" || payload.ExpirationBucket != "2026-05-16" {
		t.Fatalf("payload = %+v", payload)
	}
}

func parseEvent(t *testing.T, data []byte) cloudevents.Event {
	t.Helper()
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	return event
}
```

- [ ] **Step 2: Run transport tests to verify failure**

Run:

```bash
go test ./internal/group-expiry-scheduler/transport
```

Expected: FAIL because the transport package does not exist.

- [ ] **Step 3: Implement CloudEvent builders**

Create `internal/group-expiry-scheduler/transport/expiry_event.go`:

```go
package transport

import (
	"encoding/json"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

const eventSource = "group-expiry-scheduler"

type groupExpiryCommandData struct {
	TaskID           string `json:"task_id"`
	WorkspaceID      string `json:"workspace_id"`
	GroupID          string `json:"group_id"`
	ExpirationBucket string `json:"expiration_bucket"`
}

type individualMemberExpiryCommandData struct {
	TaskID           string `json:"task_id"`
	GroupID          string `json:"group_id"`
	NTAccount        string `json:"nt_account"`
	ExpirationBucket string `json:"expiration_bucket"`
}

func NewGroupExpiryCommandEvent(task expiry.GroupTask, eventType string, eventID string, eventTime time.Time) ([]byte, error) {
	event := newCommandEvent(task.ID, eventType, eventID, eventTime)
	if err := event.SetData(cloudevents.ApplicationJSON, groupExpiryCommandData{
		TaskID:           task.ID,
		WorkspaceID:      task.WorkspaceID,
		GroupID:          task.GroupID,
		ExpirationBucket: task.ExpirationBucket,
	}); err != nil {
		return nil, fmt.Errorf("set group expiry command data: %w", err)
	}
	return marshalEvent(event)
}

func NewIndividualMemberExpiryCommandEvent(task expiry.IndividualMemberTask, eventType string, eventID string, eventTime time.Time) ([]byte, error) {
	event := newCommandEvent(task.ID, eventType, eventID, eventTime)
	if err := event.SetData(cloudevents.ApplicationJSON, individualMemberExpiryCommandData{
		TaskID:           task.ID,
		GroupID:          task.GroupID,
		NTAccount:        task.NTAccount,
		ExpirationBucket: task.ExpirationBucket,
	}); err != nil {
		return nil, fmt.Errorf("set individual member expiry command data: %w", err)
	}
	return marshalEvent(event)
}

func newCommandEvent(taskID string, eventType string, eventID string, eventTime time.Time) cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetSubject(taskID)
	event.SetID(eventID)
	event.SetTime(eventTime)
	return event
}

func marshalEvent(event cloudevents.Event) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal expiry command event: %w", err)
	}
	return data, nil
}
```

- [ ] **Step 4: Write failing publisher tests**

Create `cmd/group-expiry-scheduler/expiry_command_publisher_test.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

type fakeMessagePublisher struct {
	subject string
	data    []byte
	err     error
	opts    []eventbus.PublishOption
}

func (f *fakeMessagePublisher) Publish(_ context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error {
	f.subject = subject
	f.data = append([]byte(nil), data...)
	f.opts = append([]eventbus.PublishOption(nil), opts...)
	return f.err
}

func TestExpiryCommandPublisherPublishesGroupCommand(t *testing.T) {
	fake := &fakeMessagePublisher{}
	publisher := newExpiryCommandPublisher(fake, "app.todo.group.expiry.process", "app.todo.group.individual-member.expiry.process",
		withPublisherClock(func() time.Time { return time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC) }),
		withPublisherIDGenerator(func() string { return "event-1" }),
	)

	err := publisher.PublishGroupExpiryCommand(context.Background(), expiry.GroupTask{ID: "task-1", WorkspaceID: "workspace-1", GroupID: "group-1", ExpirationBucket: "2026-05-16"})
	if err != nil {
		t.Fatalf("PublishGroupExpiryCommand error = %v, want nil", err)
	}
	if fake.subject != "app.todo.group.expiry.process" {
		t.Fatalf("subject = %q, want group subject", fake.subject)
	}
	event := parsePublisherEvent(t, fake.data)
	if event.ID() != "event-1" || event.Subject() != "task-1" {
		t.Fatalf("event id/subject = %q/%q", event.ID(), event.Subject())
	}
}

func TestExpiryCommandPublisherPublishesIndividualMemberCommand(t *testing.T) {
	fake := &fakeMessagePublisher{}
	publisher := newExpiryCommandPublisher(fake, "app.todo.group.expiry.process", "app.todo.group.individual-member.expiry.process",
		withPublisherClock(func() time.Time { return time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC) }),
		withPublisherIDGenerator(func() string { return "event-1" }),
	)

	err := publisher.PublishIndividualMemberExpiryCommand(context.Background(), expiry.IndividualMemberTask{ID: "task-1", GroupID: "group-1", NTAccount: "user1", ExpirationBucket: "2026-05-16"})
	if err != nil {
		t.Fatalf("PublishIndividualMemberExpiryCommand error = %v, want nil", err)
	}
	if fake.subject != "app.todo.group.individual-member.expiry.process" {
		t.Fatalf("subject = %q, want member subject", fake.subject)
	}
}

func TestExpiryCommandPublisherReturnsPublishError(t *testing.T) {
	fake := &fakeMessagePublisher{err: errors.New("nats unavailable")}
	publisher := newExpiryCommandPublisher(fake, "group.subject", "member.subject")

	err := publisher.PublishGroupExpiryCommand(context.Background(), expiry.GroupTask{ID: "task-1"})
	if err == nil {
		t.Fatal("PublishGroupExpiryCommand error = nil, want error")
	}
}

func parsePublisherEvent(t *testing.T, data []byte) cloudevents.Event {
	t.Helper()
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}
	return event
}
```

- [ ] **Step 5: Implement publisher**

Create `cmd/group-expiry-scheduler/expiry_command_publisher.go`:

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/group-expiry-scheduler/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

type messagePublisher interface {
	Publish(ctx context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error
}

type expiryCommandPublisher struct {
	publisher               messagePublisher
	groupSubject            string
	individualMemberSubject string
	now                     func() time.Time
	idGenerator             func() string
	opts                    []eventbus.PublishOption
}

type publisherOption func(*expiryCommandPublisher)

func withPublisherClock(clock func() time.Time) publisherOption {
	return func(p *expiryCommandPublisher) {
		if clock != nil {
			p.now = clock
		}
	}
}

func withPublisherIDGenerator(generator func() string) publisherOption {
	return func(p *expiryCommandPublisher) {
		if generator != nil {
			p.idGenerator = generator
		}
	}
}

func withPublisherPublishOptions(opts ...eventbus.PublishOption) publisherOption {
	return func(p *expiryCommandPublisher) {
		p.opts = append([]eventbus.PublishOption(nil), opts...)
	}
}

func newExpiryCommandPublisher(publisher messagePublisher, groupSubject string, individualMemberSubject string, opts ...publisherOption) expiryCommandPublisher {
	p := expiryCommandPublisher{
		publisher:               publisher,
		groupSubject:            groupSubject,
		individualMemberSubject: individualMemberSubject,
		now: func() time.Time {
			return time.Now().UTC()
		},
		idGenerator: uuid.NewString,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&p)
		}
	}
	return p
}

func (p expiryCommandPublisher) PublishGroupExpiryCommand(ctx context.Context, task expiry.GroupTask) error {
	data, err := transport.NewGroupExpiryCommandEvent(task, p.groupSubject, p.idGenerator(), p.now().UTC())
	if err != nil {
		return fmt.Errorf("build group expiry command event: %w", err)
	}
	if err := p.publisher.Publish(ctx, p.groupSubject, data, p.opts...); err != nil {
		return fmt.Errorf("publish group expiry command event: %w", err)
	}
	return nil
}

func (p expiryCommandPublisher) PublishIndividualMemberExpiryCommand(ctx context.Context, task expiry.IndividualMemberTask) error {
	data, err := transport.NewIndividualMemberExpiryCommandEvent(task, p.individualMemberSubject, p.idGenerator(), p.now().UTC())
	if err != nil {
		return fmt.Errorf("build individual member expiry command event: %w", err)
	}
	if err := p.publisher.Publish(ctx, p.individualMemberSubject, data, p.opts...); err != nil {
		return fmt.Errorf("publish individual member expiry command event: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: Run transport and publisher tests**

Run:

```bash
go test ./internal/group-expiry-scheduler/transport ./cmd/group-expiry-scheduler
```

Expected: PASS. A Go `package main` test package can compile without a `main.go` file as long as the publisher files and tests compile.

- [ ] **Step 7: Commit**

```bash
git add internal/group-expiry-scheduler/transport cmd/group-expiry-scheduler/expiry_command_publisher.go cmd/group-expiry-scheduler/expiry_command_publisher_test.go
git commit -m "feat: add expiry scheduler command publisher"
```

---

### Task 6: Scheduler Service and Overlap Guard

**Files:**
- Create: `internal/group-expiry-scheduler/services/scheduler_service.go`
- Create: `internal/group-expiry-scheduler/services/scheduler_service_test.go`
- Create: `internal/group-expiry-scheduler/services/job_runner.go`
- Create: `internal/group-expiry-scheduler/services/job_runner_test.go`

- [ ] **Step 1: Write failing scheduler service tests**

Create `internal/group-expiry-scheduler/services/scheduler_service_test.go` with fakes for repository and publisher:

```go
package services

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

type fakeTaskRepository struct {
	groupBatches  [][]expiry.GroupTask
	memberBatches [][]expiry.IndividualMemberTask
	groupErr      error
	memberErr     error
	groupCalls    int
	memberCalls   int
}

func (f *fakeTaskRepository) ListDueGroupTasks(_ context.Context, _ string, _ *expiry.Cursor, _ int) ([]expiry.GroupTask, error) {
	if f.groupErr != nil {
		return nil, f.groupErr
	}
	if f.groupCalls >= len(f.groupBatches) {
		return nil, nil
	}
	batch := f.groupBatches[f.groupCalls]
	f.groupCalls++
	return batch, nil
}

func (f *fakeTaskRepository) ListDueIndividualMemberTasks(_ context.Context, _ string, _ *expiry.Cursor, _ int) ([]expiry.IndividualMemberTask, error) {
	if f.memberErr != nil {
		return nil, f.memberErr
	}
	if f.memberCalls >= len(f.memberBatches) {
		return nil, nil
	}
	batch := f.memberBatches[f.memberCalls]
	f.memberCalls++
	return batch, nil
}

type fakeCommandPublisher struct {
	groupPublished  []string
	memberPublished []string
	groupErrByID    map[string]error
	memberErrByID   map[string]error
}

func (f *fakeCommandPublisher) PublishGroupExpiryCommand(_ context.Context, task expiry.GroupTask) error {
	f.groupPublished = append(f.groupPublished, task.ID)
	return f.groupErrByID[task.ID]
}

func (f *fakeCommandPublisher) PublishIndividualMemberExpiryCommand(_ context.Context, task expiry.IndividualMemberTask) error {
	f.memberPublished = append(f.memberPublished, task.ID)
	return f.memberErrByID[task.ID]
}

func TestSchedulerServiceRunScansUntilEmpty(t *testing.T) {
	repository := &fakeTaskRepository{
		groupBatches: [][]expiry.GroupTask{
			{{ID: "group-task-1", ExpirationBucket: "2026-05-15"}, {ID: "group-task-2", ExpirationBucket: "2026-05-16"}},
			{{ID: "group-task-3", ExpirationBucket: "2026-05-16"}},
		},
		memberBatches: [][]expiry.IndividualMemberTask{
			{{ID: "member-task-1", ExpirationBucket: "2026-05-16"}},
		},
	}
	publisher := &fakeCommandPublisher{}
	service := NewSchedulerService(repository, publisher,
		WithLogger(slog.Default()),
		WithBatchSize(2),
		WithClock(func() time.Time { return time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC) }),
		WithRunIDGenerator(func() string { return "run-1" }),
	)

	stats, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	if stats.GroupScanned != 3 || stats.GroupPublished != 3 || stats.IndividualMemberScanned != 1 || stats.IndividualMemberPublished != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	if repository.groupCalls != 2 || repository.memberCalls != 1 {
		t.Fatalf("calls = group:%d member:%d", repository.groupCalls, repository.memberCalls)
	}
}

func TestSchedulerServiceRunContinuesAfterPublishFailure(t *testing.T) {
	repository := &fakeTaskRepository{
		groupBatches: [][]expiry.GroupTask{{{ID: "group-task-1"}, {ID: "group-task-2"}}},
		memberBatches: [][]expiry.IndividualMemberTask{{{ID: "member-task-1"}, {ID: "member-task-2"}}},
	}
	publisher := &fakeCommandPublisher{
		groupErrByID:  map[string]error{"group-task-1": errors.New("publish failed")},
		memberErrByID: map[string]error{"member-task-1": errors.New("publish failed")},
	}
	service := NewSchedulerService(repository, publisher, WithLogger(slog.Default()), WithRunIDGenerator(func() string { return "run-1" }))

	stats, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	if stats.GroupFailed != 1 || stats.GroupPublished != 1 || stats.IndividualMemberFailed != 1 || stats.IndividualMemberPublished != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestSchedulerServiceRunStopsOnQueryFailure(t *testing.T) {
	repository := &fakeTaskRepository{groupErr: errors.New("mongo unavailable")}
	publisher := &fakeCommandPublisher{}
	service := NewSchedulerService(repository, publisher, WithLogger(slog.Default()), WithRunIDGenerator(func() string { return "run-1" }))

	_, err := service.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want error")
	}
}
```

- [ ] **Step 2: Run scheduler service tests to verify failure**

Run:

```bash
go test ./internal/group-expiry-scheduler/services
```

Expected: FAIL because the services package does not exist.

- [ ] **Step 3: Implement scheduler service**

Create `internal/group-expiry-scheduler/services/scheduler_service.go` with repository/publisher interfaces, stats, options, and run logic:

```go
package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

type TaskRepository interface {
	ListDueGroupTasks(ctx context.Context, dueBucket string, cursor *expiry.Cursor, limit int) ([]expiry.GroupTask, error)
	ListDueIndividualMemberTasks(ctx context.Context, dueBucket string, cursor *expiry.Cursor, limit int) ([]expiry.IndividualMemberTask, error)
}

type CommandPublisher interface {
	PublishGroupExpiryCommand(ctx context.Context, task expiry.GroupTask) error
	PublishIndividualMemberExpiryCommand(ctx context.Context, task expiry.IndividualMemberTask) error
}

type DispatchStats struct {
	RunID                     string
	GroupDueBucket            string
	IndividualMemberDueBucket string
	GroupScanned              int
	GroupPublished            int
	GroupFailed               int
	IndividualMemberScanned   int
	IndividualMemberPublished int
	IndividualMemberFailed    int
	Duration                  time.Duration
}

type SchedulerService struct {
	repository                           TaskRepository
	publisher                            CommandPublisher
	logger                               *slog.Logger
	now                                  func() time.Time
	runIDGenerator                       func() string
	groupBucketLocation                  *time.Location
	individualMemberBucketLocation       *time.Location
	batchSize                            int
}

type Option func(*SchedulerService)

func NewSchedulerService(repository TaskRepository, publisher CommandPublisher, opts ...Option) *SchedulerService {
	service := &SchedulerService{
		repository:                     repository,
		publisher:                      publisher,
		logger:                         slog.Default(),
		now:                            func() time.Time { return time.Now().UTC() },
		runIDGenerator:                 uuid.NewString,
		groupBucketLocation:            time.UTC,
		individualMemberBucketLocation: time.UTC,
		batchSize:                      20,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *SchedulerService) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func WithClock(clock func() time.Time) Option {
	return func(s *SchedulerService) {
		if clock != nil {
			s.now = clock
		}
	}
}

func WithRunIDGenerator(generator func() string) Option {
	return func(s *SchedulerService) {
		if generator != nil {
			s.runIDGenerator = generator
		}
	}
}

func WithBucketLocations(groupLocation *time.Location, individualMemberLocation *time.Location) Option {
	return func(s *SchedulerService) {
		if groupLocation != nil {
			s.groupBucketLocation = groupLocation
		}
		if individualMemberLocation != nil {
			s.individualMemberBucketLocation = individualMemberLocation
		}
	}
}

func WithBatchSize(batchSize int) Option {
	return func(s *SchedulerService) {
		if batchSize > 0 {
			s.batchSize = batchSize
		}
	}
}

func (s *SchedulerService) Run(ctx context.Context) (DispatchStats, error) {
	startedAt := s.now().UTC()
	stats := DispatchStats{
		RunID:                     s.runIDGenerator(),
		GroupDueBucket:            group.ExpirationBucketFor(startedAt, s.groupBucketLocation),
		IndividualMemberDueBucket: group.ExpirationBucketFor(startedAt, s.individualMemberBucketLocation),
	}
	s.logger.Info("group expiry scheduler job started",
		"run_id", stats.RunID,
		"group_due_bucket", stats.GroupDueBucket,
		"individual_member_due_bucket", stats.IndividualMemberDueBucket,
	)

	if err := s.dispatchGroupTasks(ctx, &stats); err != nil {
		stats.Duration = s.now().UTC().Sub(startedAt)
		s.logger.Error("group expiry scheduler job failed", "err", err, "run_id", stats.RunID, "duration", stats.Duration)
		return stats, err
	}
	if err := s.dispatchIndividualMemberTasks(ctx, &stats); err != nil {
		stats.Duration = s.now().UTC().Sub(startedAt)
		s.logger.Error("group expiry scheduler job failed", "err", err, "run_id", stats.RunID, "duration", stats.Duration)
		return stats, err
	}

	stats.Duration = s.now().UTC().Sub(startedAt)
	s.logger.Info("group expiry scheduler job finished",
		"run_id", stats.RunID,
		"group_scanned", stats.GroupScanned,
		"group_published", stats.GroupPublished,
		"group_failed", stats.GroupFailed,
		"individual_member_scanned", stats.IndividualMemberScanned,
		"individual_member_published", stats.IndividualMemberPublished,
		"individual_member_failed", stats.IndividualMemberFailed,
		"duration", stats.Duration,
	)
	return stats, nil
}

func (s *SchedulerService) dispatchGroupTasks(ctx context.Context, stats *DispatchStats) error {
	var cursor *expiry.Cursor
	for {
		tasks, err := s.repository.ListDueGroupTasks(ctx, stats.GroupDueBucket, cursor, s.batchSize)
		if err != nil {
			return fmt.Errorf("list due group expiry tasks: %w", err)
		}
		if len(tasks) == 0 {
			return nil
		}
		for _, task := range tasks {
			stats.GroupScanned++
			if err := s.publisher.PublishGroupExpiryCommand(ctx, task); err != nil {
				stats.GroupFailed++
				s.logger.Warn("failed to publish group expiry command", "err", err, "run_id", stats.RunID, "task_type", "group", "task_id", task.ID, "workspace_id", task.WorkspaceID, "group_id", task.GroupID, "expiration_bucket", task.ExpirationBucket)
				continue
			}
			stats.GroupPublished++
		}
		last := tasks[len(tasks)-1]
		cursor = &expiry.Cursor{ExpirationBucket: last.ExpirationBucket, ID: last.ID}
	}
}

func (s *SchedulerService) dispatchIndividualMemberTasks(ctx context.Context, stats *DispatchStats) error {
	var cursor *expiry.Cursor
	for {
		tasks, err := s.repository.ListDueIndividualMemberTasks(ctx, stats.IndividualMemberDueBucket, cursor, s.batchSize)
		if err != nil {
			return fmt.Errorf("list due individual member expiry tasks: %w", err)
		}
		if len(tasks) == 0 {
			return nil
		}
		for _, task := range tasks {
			stats.IndividualMemberScanned++
			if err := s.publisher.PublishIndividualMemberExpiryCommand(ctx, task); err != nil {
				stats.IndividualMemberFailed++
				s.logger.Warn("failed to publish individual member expiry command", "err", err, "run_id", stats.RunID, "task_type", "individual_member", "task_id", task.ID, "group_id", task.GroupID, "nt_account", task.NTAccount, "expiration_bucket", task.ExpirationBucket)
				continue
			}
			stats.IndividualMemberPublished++
		}
		last := tasks[len(tasks)-1]
		cursor = &expiry.Cursor{ExpirationBucket: last.ExpirationBucket, ID: last.ID}
	}
}
```

- [ ] **Step 4: Write failing overlap guard tests**

Create `internal/group-expiry-scheduler/services/job_runner_test.go`:

```go
package services

import (
	"context"
	"log/slog"
	"sync"
	"testing"
)

func TestJobRunnerSkipsOverlap(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	runner := NewJobRunner(func(ctx context.Context) {
		calls++
		if calls == 1 {
			close(started)
			<-release
		}
	}, slog.Default())

	done := make(chan struct{})
	go func() {
		runner.Run(context.Background())
		close(done)
	}()
	<-started
	runner.Run(context.Background())
	close(release)
	<-done

	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestJobRunnerAllowsSequentialRuns(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	runner := NewJobRunner(func(ctx context.Context) {
		mu.Lock()
		defer mu.Unlock()
		calls++
	}, slog.Default())

	runner.Run(context.Background())
	runner.Run(context.Background())

	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}
```

- [ ] **Step 5: Implement overlap guard**

Create `internal/group-expiry-scheduler/services/job_runner.go`:

```go
package services

import (
	"context"
	"log/slog"
	"sync/atomic"
)

type JobRunner struct {
	run    func(context.Context)
	logger *slog.Logger
	active atomic.Bool
}

func NewJobRunner(run func(context.Context), logger *slog.Logger) *JobRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &JobRunner{run: run, logger: logger}
}

func (r *JobRunner) Run(ctx context.Context) {
	if !r.active.CompareAndSwap(false, true) {
		r.logger.Warn("group expiry scheduler job skipped because previous run is still active")
		return
	}
	defer r.active.Store(false)
	r.run(ctx)
}
```

- [ ] **Step 6: Run service tests**

Run:

```bash
go test ./internal/group-expiry-scheduler/services
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/group-expiry-scheduler/services
git commit -m "feat: add expiry scheduler service"
```

---

### Task 7: Scheduler Main Wiring and Health

**Files:**
- Create: `cmd/group-expiry-scheduler/main.go`
- Create: `cmd/group-expiry-scheduler/main_test.go`
- Modify: `cmd/group-expiry-scheduler/expiry_command_publisher_test.go`

- [ ] **Step 1: Write failing main tests**

Create `cmd/group-expiry-scheduler/main_test.go`:

```go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	schedulerconfig "github.com/hao0731/workspace-permission-management/internal/group-expiry-scheduler/config"
	"github.com/labstack/echo/v5"
)

func TestProcessIndicator(t *testing.T) {
	indicator := processIndicator{}
	if indicator.Name() != "process" {
		t.Fatalf("Name = %q, want process", indicator.Name())
	}
	if !indicator.IsHealthy(context.Background()) {
		t.Fatal("IsHealthy = false, want true")
	}
}

func TestRegisterHealthRoutes(t *testing.T) {
	e := echo.New()
	registerHealthRoutes(e)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/liveness", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestNewGocronScheduler(t *testing.T) {
	cfg := schedulerconfig.Config{
		Schedule: schedulerconfig.ScheduleConfig{
			Expression:  "* * * * *",
			WithSeconds: false,
			Location:    time.UTC,
		},
		ShutdownTimeout: time.Second,
	}

	scheduler, err := newGocronScheduler(cfg, func(context.Context) {})
	if err != nil {
		t.Fatalf("newGocronScheduler error = %v, want nil", err)
	}
	if err := scheduler.Shutdown(); err != nil {
		t.Fatalf("Shutdown error = %v", err)
	}
}
```

- [ ] **Step 2: Run command package tests to verify failure**

Run:

```bash
go test ./cmd/group-expiry-scheduler
```

Expected: FAIL because `main.go`, `processIndicator`, `registerHealthRoutes`, or `newGocronScheduler` is undefined.

- [ ] **Step 3: Implement main wiring**

Create `cmd/group-expiry-scheduler/main.go`:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-co-op/gocron/v2"
	schedulerconfig "github.com/hao0731/workspace-permission-management/internal/group-expiry-scheduler/config"
	schedulerservices "github.com/hao0731/workspace-permission-management/internal/group-expiry-scheduler/services"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
	sharedexpiry "github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
	"github.com/labstack/echo/v5"
	"github.com/nats-io/nats.go"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type processIndicator struct{}

func (processIndicator) Name() string {
	return "process"
}

func (processIndicator) IsHealthy(context.Context) bool {
	return true
}

func main() {
	if err := run(); err != nil {
		slog.Error("group expiry scheduler stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := schedulerconfig.Load()
	if err != nil {
		return err
	}
	logger := sharedlogger.New(cfg.Environment)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoDB.URI))
	if err != nil {
		return err
	}
	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if disconnectErr := mongoClient.Disconnect(disconnectCtx); disconnectErr != nil {
			logger.Warn("failed to disconnect mongodb", "err", disconnectErr)
		}
	}()

	repository := sharedexpiry.NewMongoRepository(mongoClient.Database(cfg.MongoDB.Database))
	if err := repository.EnsureIndexes(ctx); err != nil {
		return err
	}

	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return err
	}
	defer nc.Close()

	producer, err := eventbus.NewJetStreamProducer(ctx, nc, logger)
	if err != nil {
		return err
	}
	publisher := newExpiryCommandPublisher(
		producer,
		cfg.GroupExpiry.Subject,
		cfg.IndividualMemberExpiry.Subject,
		withPublisherPublishOptions(eventbus.WithPublishTimeout(cfg.PublishTimeout)),
	)
	service := schedulerservices.NewSchedulerService(
		repository,
		publisher,
		schedulerservices.WithLogger(logger),
		schedulerservices.WithBatchSize(cfg.BatchSize),
		schedulerservices.WithBucketLocations(cfg.GroupExpiry.BucketLocation, cfg.IndividualMemberExpiry.BucketLocation),
	)
	runner := schedulerservices.NewJobRunner(func(jobCtx context.Context) {
		_, _ = service.Run(jobCtx)
	}, logger)

	scheduler, err := newGocronScheduler(cfg, runner.Run)
	if err != nil {
		return err
	}
	scheduler.Start()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if shutdownErr := scheduler.ShutdownWithContext(shutdownCtx); shutdownErr != nil {
			logger.Warn("failed to shutdown scheduler", "err", shutdownErr)
		}
	}()

	e := echo.New()
	registerHealthRoutes(e)

	errCh := make(chan error, 1)
	go func() {
		startConfig := echo.StartConfig{
			Address:         cfg.HTTPAddr,
			GracefulTimeout: cfg.ShutdownTimeout,
		}
		if err := startConfig.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if err != nil {
			stop()
			return err
		}
		return nil
	}
}

func newGocronScheduler(cfg schedulerconfig.Config, run func(context.Context)) (gocron.Scheduler, error) {
	scheduler, err := gocron.NewScheduler(
		gocron.WithLocation(cfg.Schedule.Location),
		gocron.WithStopTimeout(cfg.ShutdownTimeout),
	)
	if err != nil {
		return nil, err
	}
	if _, err := scheduler.NewJob(
		gocron.CronJob(cfg.Schedule.Expression, cfg.Schedule.WithSeconds),
		gocron.NewTask(func(ctx context.Context) {
			run(ctx)
		}),
	); err != nil {
		_ = scheduler.Shutdown()
		return nil, err
	}
	return scheduler, nil
}

func registerHealthRoutes(e *echo.Echo) {
	health.NewHealthManager(processIndicator{}).RegisterRoutes(e)
}
```

- [ ] **Step 4: Run command package tests**

Run:

```bash
go test ./cmd/group-expiry-scheduler
```

Expected: PASS.

- [ ] **Step 5: Run scheduler packages**

Run:

```bash
go test ./internal/group-expiry-scheduler/... ./cmd/group-expiry-scheduler
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/group-expiry-scheduler internal/group-expiry-scheduler
git commit -m "feat: wire group expiry scheduler service"
```

---

### Task 8: Docker Compose Runtime

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Update Docker Compose**

Add this service after `nats-init`:

```yaml
  group-expiry-scheduler:
    image: golang:1.25
    container_name: workspace-permission-management-group-expiry-scheduler
    working_dir: /workspace
    command: ["go", "run", "./cmd/group-expiry-scheduler"]
    volumes:
      - .:/workspace
      - go_mod_cache:/go/pkg/mod
    environment:
      GROUP_EXPIRY_SCHEDULER_ENV: development
      GROUP_EXPIRY_SCHEDULER_HTTP_ADDR: :8084
      GROUP_EXPIRY_SCHEDULER_MONGODB_URI: mongodb://mongodb:27017/?replicaSet=rs0
      GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE: workspace_permission_management
      GROUP_EXPIRY_SCHEDULER_NATS_URL: nats://nats:4222
      GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT: ${GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT:-app.todo.group.expiry.process}
      GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT: ${GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT:-app.todo.group.individual-member.expiry.process}
      GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION: "${GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION:-* * * * *}"
      GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE: ${GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE:-UTC}
      GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE: ${GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE:-UTC}
    ports:
      - "8084:8084"
    depends_on:
      mongo-init:
        condition: service_completed_successfully
      nats-init:
        condition: service_completed_successfully
    networks:
      - workspace_permission_management
```

Add the volume:

```yaml
  go_mod_cache:
```

The existing `nats-init` stream setup already creates both command streams and subjects. Keep those subjects aligned with:

```txt
app.todo.group.expiry.process
app.todo.group.individual-member.expiry.process
```

- [ ] **Step 2: Validate compose syntax**

Run:

```bash
docker compose config
```

Expected: PASS and output includes `group-expiry-scheduler`.

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml
git commit -m "chore: add group expiry scheduler compose service"
```

---

### Task 9: Full Verification and Plan Finalization

**Files:**
- Verify all changed implementation files.
- Move this plan after implementation completes.

- [ ] **Step 1: Run repository-wide tests**

Run:

```bash
go test ./...
```

Expected: PASS. If Mongo integration env vars are not set, integration tests that require them should skip.

- [ ] **Step 2: Run compose config check**

Run:

```bash
docker compose config
```

Expected: PASS.

- [ ] **Step 3: Inspect git status**

Run:

```bash
git status --short
```

Expected: no unstaged or uncommitted implementation changes.

- [ ] **Step 4: Move the completed plan**

Move this plan from:

```txt
docs/plans/active/2026-05-16-group-expiry-scheduler-implementation.md
```

to:

```txt
docs/plans/completed/2026-05-16-group-expiry-scheduler-implementation.md
```

- [ ] **Step 5: Commit the plan status transition**

```bash
git add docs/plans/active/2026-05-16-group-expiry-scheduler-implementation.md docs/plans/completed/2026-05-16-group-expiry-scheduler-implementation.md
git commit -m "docs: complete group expiry scheduler implementation plan"
```

---

## Self-Review Result

- [x] Plan links to the source design and required policies.
- [x] Plan starts in `docs/plans/active/`.
- [x] Shared expiry repository extraction is implemented before scheduler reads the task collections.
- [x] `group-service` keeps transaction ownership while delegating task collection operations to the shared package.
- [x] Scheduler uses cursor scans with `expiration_bucket <= today_bucket`.
- [x] Group and individual-member bucket timezones are configured separately.
- [x] CloudEvent `source` is `group-expiry-scheduler` for both event types.
- [x] Publish failure logs and continues.
- [x] Query failure stops the current job.
- [x] The scheduler never deletes task documents after publish.
- [x] Liveness uses `internal/shared/health`.
- [x] gocron uses cron expression config.
- [x] Overlap skip is guarded and logged in-process.
- [x] Docker Compose contains a runnable local scheduler service.
- [x] Final verification includes `go test ./...` and `docker compose config`.
