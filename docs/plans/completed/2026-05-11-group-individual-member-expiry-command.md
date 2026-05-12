# Group Individual Member Expiry Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add individual-member expiry tasks and a JetStream CloudEvent command consumer that marks `group_individual_members.expired_at` and clears the matching `individual_member_expiry_task` transactionally.

**Architecture:** Keep the existing `group-service` layering. Domain owns command models, status values, and bucket validation; transport owns CloudEvent parsing; handlers classify JetStream ack/retry/terminate outcomes; services generate IDs/time and orchestrate workflows through consumer-side repository interfaces; repositories own MongoDB documents, indexes, and transactions; `cmd/group-service` wires the second JetStream consumer beside the existing grouping-rule expiry consumer.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, NATS JetStream through `internal/shared/eventbus`, CloudEvents SDK for Go, `log/slog`, `viper`, standard `testing`, Bash fixture scripts.

---

## Source Designs and Policies

Read these before implementing:

- [Group Individual Member Expiry Command Design](../../designs/group-service-individual-member-expiry-command.md)
- [Group Service Design](../../designs/group-service.md)
- [Group Individual Members API Design](../../designs/group-service-individual-members.md)
- [Group API Design](../../designs/group-service-group.md)
- [Group Expiry Command Design](../../designs/group-service-group-expiry-command.md)
- [Backend Architecture Principle](../../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend work must keep domain and service code free of Echo, NATS, JetStream, MongoDB driver, and transport DTO types.
- CloudEvent payloads, MongoDB schemas, indexes, and config keys are explicit contracts and need focused tests.
- Event subscribers must intentionally classify ack, retry, and poison-message outcomes, and command handling must be idempotent.
- MongoDB transaction behavior belongs in repositories; services pass domain inputs, timestamps, bucket locations, and generated task IDs.
- This implementation plan lives under `docs/plans/active/` and links to its source design documents.

## Working Tree Note

At plan-writing time, these design docs have local modifications from the source design work:

- `docs/designs/group-service.md`
- `docs/designs/group-service-group.md`
- `docs/designs/group-service-individual-members.md`
- `docs/designs/group-service-group-expiry-command.md`
- `docs/designs/group-service-individual-member-expiry-command.md`

There is also an unrelated local modification in `lefthook.yml`. Do not stage, edit, or revert `lefthook.yml` while implementing this plan unless the user explicitly asks.

## File Structure

Create:

- `internal/group-service/transport/individual_member_expiry_event.go`: parse the individual-member expiry command CloudEvent into a domain command.
- `internal/group-service/transport/individual_member_expiry_event_test.go`: parser tests for valid and poison-message CloudEvents.
- `internal/group-service/handlers/individual_member_expiry_event_handler.go`: JetStream handler that maps parse/service outcomes to `eventbus.HandleResult`.
- `internal/group-service/handlers/individual_member_expiry_event_handler_test.go`: handler tests for terminate, retry, and ack behavior.
- `examples/nats/fixtures/send_individual_member_expiry_event.sh`: manual fixture for publishing an individual-member expiry command CloudEvent.
- `examples/nats/fixtures/send_individual_member_expiry_event_test.sh`: shell test for the fixture payload and defaults.

Modify:

- `internal/domain/group/group.go`: add individual-member expiry task, command, status, `IndividualMember.ExpiredAt`, and per-member task metadata.
- `internal/domain/group/validation.go`: add individual-member command validation and make bucket timezone parser errors generic enough for both group and member config keys.
- `internal/domain/group/validation_test.go`: cover individual-member command validation and update bucket parser expectations.
- `internal/group-service/services/group_service.go`: generate individual-member expiry tasks on create/add/update, reset member `expired_at`, and add command service workflow.
- `internal/group-service/services/group_service_test.go`: cover task generation and command workflow.
- `internal/group-service/repositories/mongo_group_repository.go`: add collection, document, indexes, write-path task maintenance, `expired_at`, and command transaction.
- `internal/group-service/repositories/mongo_group_repository_test.go`: cover schema/index mapping, write-path task maintenance, unexpired-member counting, and command transaction behavior.
- `internal/group-service/config/config.go`: add individual-member command consumer config and bucket timezone parsing.
- `internal/group-service/config/config_test.go`: cover required values, defaults, and invalid individual-member bucket timezone.
- `cmd/group-service/main.go`: wire the individual-member expiry event handler and JetStream consumer.
- `cmd/group-service/main_test.go`: cover individual-member eventbus config mapping.
- `.env.example`: add local individual-member expiry command settings.
- `docker-compose.yml`: provision the individual-member expiry stream and durable consumer in `nats-init`.

No public HTTP response DTO should expose `expired_at` or task metadata in this phase.

---

### Task 1: Domain Models and Validation

**Files:**
- Modify: `internal/domain/group/group.go`
- Modify: `internal/domain/group/validation.go`
- Modify: `internal/domain/group/validation_test.go`

- [ ] **Step 1: Write failing command validation tests**

Add this test to `internal/domain/group/validation_test.go` near `TestExpireGroupingRuleCommandValidate`:

```go
func TestExpireIndividualMemberCommandValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		command   ExpireIndividualMemberCommand
		wantError string
	}{
		{
			name:      "empty task id",
			command:   ExpireIndividualMemberCommand{GroupID: "group-1", NTAccount: "user1", ExpirationBucket: "2026-05-10"},
			wantError: "task id is required",
		},
		{
			name:      "empty group id",
			command:   ExpireIndividualMemberCommand{TaskID: "task-1", NTAccount: "user1", ExpirationBucket: "2026-05-10"},
			wantError: "group id is required",
		},
		{
			name:      "empty nt account",
			command:   ExpireIndividualMemberCommand{TaskID: "task-1", GroupID: "group-1", ExpirationBucket: "2026-05-10"},
			wantError: "individual member nt account is required",
		},
		{
			name:      "invalid bucket",
			command:   ExpireIndividualMemberCommand{TaskID: "task-1", GroupID: "group-1", NTAccount: "user1", ExpirationBucket: "2026/05/10"},
			wantError: "expiration bucket must use yyyy-MM-dd",
		},
		{
			name:    "valid",
			command: ExpireIndividualMemberCommand{TaskID: " task-1 ", GroupID: " group-1 ", NTAccount: " user1 ", ExpirationBucket: "2026-05-10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := tt.command.Normalize()
			err := command.Validate()
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				if command.TaskID != strings.TrimSpace(tt.command.TaskID) ||
					command.GroupID != strings.TrimSpace(tt.command.GroupID) ||
					command.NTAccount != strings.TrimSpace(tt.command.NTAccount) {
					t.Fatalf("Normalize() command = %+v", command)
				}
				return
			}
			if err == nil || !errors.Is(err, ErrInvalidInput) || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Validate() error = %v, want ErrInvalidInput containing %q", err, tt.wantError)
			}
		})
	}
}
```

- [ ] **Step 2: Update bucket parser test expectations**

In `TestParseExpirationBucketLocation`, keep the existing table and assertions, but ensure invalid cases only assert `ErrInvalidInput`. Do not assert the group-specific environment variable name from `ParseExpirationBucketLocation`; config tests will assert the exact env key.

- [ ] **Step 3: Run domain tests to verify failure**

Run:

```bash
go test ./internal/domain/group
```

Expected: FAIL with undefined `ExpireIndividualMemberCommand`.

- [ ] **Step 4: Add domain types**

In `internal/domain/group/group.go`, update and add these definitions:

```go
type IndividualMember struct {
	ID             string
	GroupID        string
	NTAccount      string
	ExpirationDate time.Time
	ExpiredAt      *time.Time
	ExpiryTask     *IndividualMemberExpiryTask
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type IndividualMemberExpiryTask struct {
	ID               string
	GroupID          string
	NTAccount        string
	ExpirationBucket string
}

type ExpireIndividualMemberCommand struct {
	TaskID           string
	GroupID          string
	NTAccount        string
	ExpirationBucket string
}

type ExpireIndividualMemberStatus string

const (
	ExpireIndividualMemberStatusExpired        ExpireIndividualMemberStatus = "expired"
	ExpireIndividualMemberStatusStaleTask      ExpireIndividualMemberStatus = "stale_task"
	ExpireIndividualMemberStatusStaleMember    ExpireIndividualMemberStatus = "stale_member"
	ExpireIndividualMemberStatusAlreadyExpired ExpireIndividualMemberStatus = "already_expired"
	ExpireIndividualMemberStatusStaleBucket    ExpireIndividualMemberStatus = "stale_bucket"
)
```

Add normalize methods below the existing expiry command normalize method:

```go
func (task IndividualMemberExpiryTask) Normalize() IndividualMemberExpiryTask {
	task.ID = strings.TrimSpace(task.ID)
	task.GroupID = strings.TrimSpace(task.GroupID)
	task.NTAccount = strings.TrimSpace(task.NTAccount)
	task.ExpirationBucket = strings.TrimSpace(task.ExpirationBucket)
	return task
}

func (command ExpireIndividualMemberCommand) Normalize() ExpireIndividualMemberCommand {
	command.TaskID = strings.TrimSpace(command.TaskID)
	command.GroupID = strings.TrimSpace(command.GroupID)
	command.NTAccount = strings.TrimSpace(command.NTAccount)
	command.ExpirationBucket = strings.TrimSpace(command.ExpirationBucket)
	return command
}
```

- [ ] **Step 5: Add command validation and generic timezone errors**

In `internal/domain/group/validation.go`, add:

```go
func (command ExpireIndividualMemberCommand) Validate() error {
	if command.TaskID == "" {
		return invalidInput("task id is required")
	}
	if command.GroupID == "" {
		return invalidInput("group id is required")
	}
	if err := validateIndividualMemberAccount(command.NTAccount); err != nil {
		return err
	}
	if !IsValidExpirationBucket(command.ExpirationBucket) {
		return invalidInput("expiration bucket must use yyyy-MM-dd")
	}
	return nil
}
```

In `ParseExpirationBucketLocation`, replace group-specific messages:

```go
return nil, invalidInput("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE must be UTC or a fixed offset such as UTC+8")
```

with:

```go
return nil, invalidInput("expiration bucket timezone must be UTC or a fixed offset such as UTC+8")
```

and replace:

```go
return nil, invalidInput("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE offset is out of range")
```

with:

```go
return nil, invalidInput("expiration bucket timezone offset is out of range")
```

- [ ] **Step 6: Run domain tests**

Run:

```bash
go test ./internal/domain/group
```

Expected: PASS.

- [ ] **Step 7: Commit domain changes**

```bash
git add internal/domain/group/group.go internal/domain/group/validation.go internal/domain/group/validation_test.go
git commit -m "feat: add individual member expiry domain model"
```

---

### Task 2: Service Task Generation and Command Workflow

**Files:**
- Modify: `internal/group-service/services/group_service.go`
- Modify: `internal/group-service/services/group_service_test.go`

- [ ] **Step 1: Write failing service tests**

In `fakeGroupRepository`, add fields:

```go
expireMemberInput    group.ExpireIndividualMemberCommand
expireMemberStatus   group.ExpireIndividualMemberStatus
expireMemberErr      error
memberExpiredAt      time.Time
memberExpireLocation *time.Location
```

Add this method to `fakeGroupRepository`:

```go
func (f *fakeGroupRepository) ExpireIndividualMember(ctx context.Context, input group.ExpireIndividualMemberCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireIndividualMemberStatus, error) {
	f.expireMemberInput = input
	f.memberExpiredAt = expiredAt
	f.memberExpireLocation = bucketLocation
	if f.expireMemberStatus == "" {
		f.expireMemberStatus = group.ExpireIndividualMemberStatusExpired
	}
	return f.expireMemberStatus, f.expireMemberErr
}
```

Add these tests to `internal/group-service/services/group_service_test.go`:

```go
func TestGroupServiceCreateGroupCreatesIndividualMemberExpiryTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 16, 30, 0, 0, time.UTC)
	location, err := group.ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}
	service := NewGroupService(repository,
		WithGroupIDGenerator(sequenceGenerator("group-1", "member-1", "group-task-1", "member-task-1")),
		WithGroupClock(func() time.Time { return now }),
		WithIndividualMemberExpiryBucketLocation(location),
	)

	_, err = service.CreateGroup(context.Background(), validServiceCreateInput())
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	task := repository.input.IndividualMembers[0].ExpiryTask
	if task == nil {
		t.Fatal("individual member expiry task is nil")
	}
	if task.ID != "member-task-1" || task.GroupID != "group-1" || task.NTAccount != "user1" {
		t.Fatalf("task = %+v, want member-task-1/group-1/user1", task)
	}
	if task.ExpirationBucket != "2026-06-01" {
		t.Fatalf("expiration bucket = %q, want 2026-06-01", task.ExpirationBucket)
	}
}

func TestGroupServiceAddIndividualMembersCreatesExpiryTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	service := NewGroupService(repository,
		WithGroupIDGenerator(sequenceGenerator("member-2", "member-task-2")),
		WithGroupClock(func() time.Time { return now }),
	)

	_, err := service.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		IndividualMembers: []group.IndividualMember{{
			NTAccount:      "user2",
			ExpirationDate: now.Add(24 * time.Hour),
		}},
	})
	if err != nil {
		t.Fatalf("AddIndividualMembers() error = %v", err)
	}
	task := repository.addInput.IndividualMembers[0].ExpiryTask
	if task == nil {
		t.Fatal("individual member expiry task is nil")
	}
	if task.ID != "member-task-2" || task.GroupID != "group-1" || task.NTAccount != "user2" {
		t.Fatalf("task = %+v, want member-task-2/group-1/user2", task)
	}
	if task.ExpirationBucket != "2026-05-11" {
		t.Fatalf("expiration bucket = %q, want 2026-05-11", task.ExpirationBucket)
	}
}

func TestGroupServiceUpdateIndividualMemberExpirationCreatesReplacementTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	service := NewGroupService(repository,
		WithGroupIDGenerator(sequenceGenerator("member-task-new")),
		WithGroupClock(func() time.Time { return now }),
	)

	err := service.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		NTAccount:      "user2",
		ExpirationDate: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("UpdateIndividualMemberExpiration() error = %v", err)
	}
	task := repository.memberUpdate.ExpiryTask
	if task == nil {
		t.Fatal("replacement task is nil")
	}
	if task.ID != "member-task-new" || task.GroupID != "group-1" || task.NTAccount != "user2" {
		t.Fatalf("task = %+v, want member-task-new/group-1/user2", task)
	}
}

func TestGroupServiceExpireIndividualMember(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{expireMemberStatus: group.ExpireIndividualMemberStatusStaleTask}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	location, err := group.ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}
	service := NewGroupService(repository,
		WithGroupClock(func() time.Time { return now }),
		WithIndividualMemberExpiryBucketLocation(location),
	)

	status, err := service.ExpireIndividualMember(context.Background(), group.ExpireIndividualMemberCommand{
		TaskID:           "task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-05-10",
	})
	if err != nil {
		t.Fatalf("ExpireIndividualMember() error = %v", err)
	}
	if status != group.ExpireIndividualMemberStatusStaleTask {
		t.Fatalf("status = %s, want stale_task", status)
	}
	if !repository.memberExpiredAt.Equal(now) {
		t.Fatalf("expiredAt = %s, want %s", repository.memberExpiredAt, now)
	}
	if repository.memberExpireLocation.String() != "UTC+08:00" {
		t.Fatalf("location = %s, want UTC+08:00", repository.memberExpireLocation)
	}
}
```

- [ ] **Step 2: Run service tests to verify failure**

Run:

```bash
go test ./internal/group-service/services
```

Expected: FAIL with missing `WithIndividualMemberExpiryBucketLocation`, missing repository interface method, and missing `ExpiryTask` on update input.

- [ ] **Step 3: Update repository interface and service state**

In `internal/group-service/services/group_service.go`, extend `GroupRepository`:

```go
ExpireIndividualMember(ctx context.Context, input group.ExpireIndividualMemberCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireIndividualMemberStatus, error)
```

Add option and field:

```go
func WithIndividualMemberExpiryBucketLocation(location *time.Location) GroupOption {
	return func(s *GroupService) {
		if location != nil {
			s.individualMemberExpiryBucketLocation = location
		}
	}
}
```

```go
individualMemberExpiryBucketLocation *time.Location
```

Set the default in `NewGroupService`:

```go
individualMemberExpiryBucketLocation: time.UTC,
```

- [ ] **Step 4: Generate member expiry tasks in service workflows**

Update `newIndividualMembers`:

```go
func (s *GroupService) newIndividualMembers(groupID string, input []group.IndividualMember, now time.Time) []group.IndividualMember {
	members := make([]group.IndividualMember, 0, len(input))
	for _, member := range input {
		ntAccount := member.NTAccount
		members = append(members, group.IndividualMember{
			ID:             s.idGenerator(),
			GroupID:        groupID,
			NTAccount:      ntAccount,
			ExpirationDate: member.ExpirationDate,
			ExpiredAt:      nil,
			ExpiryTask:     s.newIndividualMemberExpiryTask(groupID, ntAccount, member.ExpirationDate),
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	return members
}
```

Add:

```go
func (s *GroupService) newIndividualMemberExpiryTask(groupID string, ntAccount string, expiration time.Time) *group.IndividualMemberExpiryTask {
	return &group.IndividualMemberExpiryTask{
		ID:               s.idGenerator(),
		GroupID:          groupID,
		NTAccount:        ntAccount,
		ExpirationBucket: group.ExpirationBucketFor(expiration, s.individualMemberExpiryBucketLocation),
	}
}
```

Update `UpdateIndividualMemberExpiration` after validation:

```go
input.ExpiryTask = s.newIndividualMemberExpiryTask(input.GroupID, input.NTAccount, input.ExpirationDate)
```

- [ ] **Step 5: Add command service workflow**

Add:

```go
func (s *GroupService) ExpireIndividualMember(ctx context.Context, input group.ExpireIndividualMemberCommand) (group.ExpireIndividualMemberStatus, error) {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return "", err
	}
	status, err := s.repository.ExpireIndividualMember(ctx, input, s.now().UTC(), s.individualMemberExpiryBucketLocation)
	if err != nil {
		return "", fmt.Errorf("expire individual member: %w", err)
	}
	return status, nil
}
```

- [ ] **Step 6: Run service tests**

Run:

```bash
go test ./internal/group-service/services
```

Expected: PASS.

- [ ] **Step 7: Commit service changes**

```bash
git add internal/group-service/services/group_service.go internal/group-service/services/group_service_test.go
git commit -m "feat: generate individual member expiry tasks"
```

---

### Task 3: Transport CloudEvent Parser

**Files:**
- Create: `internal/group-service/transport/individual_member_expiry_event.go`
- Create: `internal/group-service/transport/individual_member_expiry_event_test.go`

- [ ] **Step 1: Write failing transport tests**

Create `internal/group-service/transport/individual_member_expiry_event_test.go`:

```go
package transport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func TestParseIndividualMemberExpiryCommandEvent(t *testing.T) {
	t.Parallel()

	event := newIndividualMemberExpiryCommandEvent(t)
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	input, err := ParseIndividualMemberExpiryCommandEvent(data, "app.todo.group.individual-member.expiry.process")
	if err != nil {
		t.Fatalf("ParseIndividualMemberExpiryCommandEvent() error = %v, want nil", err)
	}
	if input.TaskID != "task-1" || input.GroupID != "group-1" || input.NTAccount != "user1" || input.ExpirationBucket != "2026-05-10" {
		t.Fatalf("input = %+v", input)
	}
}

func TestParseIndividualMemberExpiryCommandEventRejectsInvalidEnvelope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*cloudevents.Event)
		wantErr string
	}{
		{
			name: "wrong type",
			mutate: func(event *cloudevents.Event) {
				event.SetType("app.todo.other")
			},
			wantErr: "does not match expected",
		},
		{
			name: "non json content type",
			mutate: func(event *cloudevents.Event) {
				event.SetDataContentType("text/plain")
			},
			wantErr: "datacontenttype",
		},
		{
			name: "missing time",
			mutate: func(event *cloudevents.Event) {
				event.SetTime(time.Time{})
			},
			wantErr: "time is required",
		},
		{
			name: "subject mismatch",
			mutate: func(event *cloudevents.Event) {
				event.SetSubject("different-task")
			},
			wantErr: "subject must match data.task_id",
		},
		{
			name: "empty nt account",
			mutate: func(event *cloudevents.Event) {
				mustSetIndividualMemberExpiryData(t, event, individualMemberExpiryCommandData{
					TaskID:           "task-1",
					GroupID:          "group-1",
					NTAccount:        " ",
					ExpirationBucket: "2026-05-10",
				})
			},
			wantErr: "empty required field",
		},
		{
			name: "invalid bucket",
			mutate: func(event *cloudevents.Event) {
				mustSetIndividualMemberExpiryData(t, event, individualMemberExpiryCommandData{
					TaskID:           "task-1",
					GroupID:          "group-1",
					NTAccount:        "user1",
					ExpirationBucket: "2026/05/10",
				})
			},
			wantErr: "expiration_bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := newIndividualMemberExpiryCommandEvent(t)
			tt.mutate(&event)
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ParseIndividualMemberExpiryCommandEvent(data, "app.todo.group.individual-member.expiry.process")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseIndividualMemberExpiryCommandEvent() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseIndividualMemberExpiryCommandEventRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseIndividualMemberExpiryCommandEvent([]byte("{"), "app.todo.group.individual-member.expiry.process")
	if err == nil || !strings.Contains(err.Error(), "parse cloudevent") {
		t.Fatalf("ParseIndividualMemberExpiryCommandEvent() error = %v, want parse error", err)
	}
}

func newIndividualMemberExpiryCommandEvent(t *testing.T) cloudevents.Event {
	t.Helper()

	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudEventSpecVersion)
	event.SetType("app.todo.group.individual-member.expiry.process")
	event.SetSource("individual-member-expiry-scheduler")
	event.SetSubject("task-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))
	mustSetIndividualMemberExpiryData(t, &event, individualMemberExpiryCommandData{
		TaskID:           "task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-05-10",
	})
	return event
}

func mustSetIndividualMemberExpiryData(t *testing.T, event *cloudevents.Event, payload individualMemberExpiryCommandData) {
	t.Helper()

	if err := event.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run transport tests to verify failure**

Run:

```bash
go test ./internal/group-service/transport
```

Expected: FAIL with undefined parser and data type.

- [ ] **Step 3: Implement parser**

Create `internal/group-service/transport/individual_member_expiry_event.go`:

```go
package transport

import (
	"encoding/json"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type individualMemberExpiryCommandData struct {
	TaskID           string `json:"task_id"`
	GroupID          string `json:"group_id"`
	NTAccount        string `json:"nt_account"`
	ExpirationBucket string `json:"expiration_bucket"`
}

func ParseIndividualMemberExpiryCommandEvent(data []byte, expectedType string) (group.ExpireIndividualMemberCommand, error) {
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return group.ExpireIndividualMemberCommand{}, fmt.Errorf("parse cloudevent: %w", err)
	}
	if err := event.Validate(); err != nil {
		return group.ExpireIndividualMemberCommand{}, fmt.Errorf("validate cloudevent: %w", err)
	}
	if event.SpecVersion() != cloudEventSpecVersion {
		return group.ExpireIndividualMemberCommand{}, fmt.Errorf("unsupported cloudevent specversion %q", event.SpecVersion())
	}
	if event.Type() != expectedType {
		return group.ExpireIndividualMemberCommand{}, fmt.Errorf("cloudevent type %q does not match expected %q", event.Type(), expectedType)
	}
	if event.DataContentType() != "application/json" {
		return group.ExpireIndividualMemberCommand{}, fmt.Errorf("cloudevent datacontenttype must be application/json")
	}
	if event.Time().IsZero() {
		return group.ExpireIndividualMemberCommand{}, fmt.Errorf("cloudevent time is required")
	}

	var payload individualMemberExpiryCommandData
	if err := event.DataAs(&payload); err != nil {
		return group.ExpireIndividualMemberCommand{}, fmt.Errorf("parse cloudevent data: %w", err)
	}
	if event.Subject() != payload.TaskID {
		return group.ExpireIndividualMemberCommand{}, fmt.Errorf("cloudevent subject must match data.task_id")
	}
	if strings.TrimSpace(payload.TaskID) == "" ||
		strings.TrimSpace(payload.GroupID) == "" ||
		strings.TrimSpace(payload.NTAccount) == "" ||
		strings.TrimSpace(payload.ExpirationBucket) == "" {
		return group.ExpireIndividualMemberCommand{}, fmt.Errorf("individual member expiry command data contains empty required field")
	}
	command := group.ExpireIndividualMemberCommand{
		TaskID:           payload.TaskID,
		GroupID:          payload.GroupID,
		NTAccount:        payload.NTAccount,
		ExpirationBucket: payload.ExpirationBucket,
	}.Normalize()
	if !group.IsValidExpirationBucket(command.ExpirationBucket) {
		return group.ExpireIndividualMemberCommand{}, fmt.Errorf("expiration_bucket must use yyyy-MM-dd")
	}
	return command, nil
}
```

- [ ] **Step 4: Run transport tests**

Run:

```bash
go test ./internal/group-service/transport
```

Expected: PASS.

- [ ] **Step 5: Commit transport parser**

```bash
git add internal/group-service/transport/individual_member_expiry_event.go internal/group-service/transport/individual_member_expiry_event_test.go
git commit -m "feat: parse individual member expiry command events"
```

---

### Task 4: JetStream Handler

**Files:**
- Create: `internal/group-service/handlers/individual_member_expiry_event_handler.go`
- Create: `internal/group-service/handlers/individual_member_expiry_event_handler_test.go`

- [ ] **Step 1: Write failing handler tests**

Create `internal/group-service/handlers/individual_member_expiry_event_handler_test.go`:

```go
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

func TestIndividualMemberExpiryEventHandlerHandle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		message    eventbus.Message
		service    *fakeIndividualMemberExpiryService
		wantResult eventbus.HandleResult
		wantCalls  int
	}{
		{
			name:       "invalid event terminates",
			message:    eventbus.Message{Subject: "app.todo.group.individual-member.expiry.process", Data: []byte("{")},
			service:    &fakeIndividualMemberExpiryService{},
			wantResult: eventbus.HandleResultTerminate,
		},
		{
			name:       "service invalid input terminates",
			message:    validIndividualMemberExpiryMessage(t),
			service:    &fakeIndividualMemberExpiryService{err: group.ErrInvalidInput},
			wantResult: eventbus.HandleResultTerminate,
			wantCalls:  1,
		},
		{
			name:       "service failure retries",
			message:    validIndividualMemberExpiryMessage(t),
			service:    &fakeIndividualMemberExpiryService{err: errors.New("database unavailable")},
			wantResult: eventbus.HandleResultRetry,
			wantCalls:  1,
		},
		{
			name:       "expired status acks",
			message:    validIndividualMemberExpiryMessage(t),
			service:    &fakeIndividualMemberExpiryService{status: group.ExpireIndividualMemberStatusExpired},
			wantResult: eventbus.HandleResultAck,
			wantCalls:  1,
		},
		{
			name:       "stale task status acks",
			message:    validIndividualMemberExpiryMessage(t),
			service:    &fakeIndividualMemberExpiryService{status: group.ExpireIndividualMemberStatusStaleTask},
			wantResult: eventbus.HandleResultAck,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewIndividualMemberExpiryEventHandler(tt.service, "app.todo.group.individual-member.expiry.process", newTestLogger())
			got := handler.Handle(context.Background(), tt.message)
			if got != tt.wantResult {
				t.Fatalf("Handle() = %s, want %s", got, tt.wantResult)
			}
			if tt.service.calls != tt.wantCalls {
				t.Fatalf("service calls = %d, want %d", tt.service.calls, tt.wantCalls)
			}
		})
	}
}

type fakeIndividualMemberExpiryService struct {
	status group.ExpireIndividualMemberStatus
	err    error
	calls  int
	input  group.ExpireIndividualMemberCommand
}

func (s *fakeIndividualMemberExpiryService) ExpireIndividualMember(ctx context.Context, input group.ExpireIndividualMemberCommand) (group.ExpireIndividualMemberStatus, error) {
	s.calls++
	s.input = input
	if s.status == "" {
		s.status = group.ExpireIndividualMemberStatusExpired
	}
	return s.status, s.err
}

func validIndividualMemberExpiryMessage(t *testing.T) eventbus.Message {
	t.Helper()

	event := cloudevents.NewEvent()
	event.SetSpecVersion("1.0")
	event.SetType("app.todo.group.individual-member.expiry.process")
	event.SetSource("individual-member-expiry-scheduler")
	event.SetSubject("task-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))
	if err := event.SetData(cloudevents.ApplicationJSON, map[string]string{
		"task_id":           "task-1",
		"group_id":          "group-1",
		"nt_account":        "user1",
		"expiration_bucket": "2026-05-10",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return eventbus.Message{Subject: "app.todo.group.individual-member.expiry.process", Data: data}
}
```

- [ ] **Step 2: Run handler tests to verify failure**

Run:

```bash
go test ./internal/group-service/handlers
```

Expected: FAIL with undefined `NewIndividualMemberExpiryEventHandler`.

- [ ] **Step 3: Implement handler**

Create `internal/group-service/handlers/individual_member_expiry_event_handler.go`:

```go
package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/group-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type IndividualMemberExpiryService interface {
	ExpireIndividualMember(ctx context.Context, input group.ExpireIndividualMemberCommand) (group.ExpireIndividualMemberStatus, error)
}

type IndividualMemberExpiryEventHandler struct {
	service      IndividualMemberExpiryService
	expectedType string
	logger       *slog.Logger
}

func NewIndividualMemberExpiryEventHandler(service IndividualMemberExpiryService, expectedType string, logger *slog.Logger) *IndividualMemberExpiryEventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &IndividualMemberExpiryEventHandler{service: service, expectedType: expectedType, logger: logger}
}

func (h *IndividualMemberExpiryEventHandler) Handle(ctx context.Context, msg eventbus.Message) eventbus.HandleResult {
	input, err := transport.ParseIndividualMemberExpiryCommandEvent(msg.Data, h.expectedType)
	if err != nil {
		h.logger.Warn("terminating invalid individual member expiry command", "err", err, "subject", msg.Subject)
		return eventbus.HandleResultTerminate
	}

	status, err := h.service.ExpireIndividualMember(ctx, input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			h.logger.Warn("terminating invalid individual member expiry command input",
				"err", err,
				"task_id", input.TaskID,
				"group_id", input.GroupID,
				"nt_account", input.NTAccount,
				"expiration_bucket", input.ExpirationBucket,
			)
			return eventbus.HandleResultTerminate
		}
		h.logger.Warn("retrying individual member expiry command",
			"err", err,
			"task_id", input.TaskID,
			"group_id", input.GroupID,
			"nt_account", input.NTAccount,
			"expiration_bucket", input.ExpirationBucket,
		)
		return eventbus.HandleResultRetry
	}

	h.logger.Info("handled individual member expiry command",
		"task_id", input.TaskID,
		"group_id", input.GroupID,
		"nt_account", input.NTAccount,
		"expiration_bucket", input.ExpirationBucket,
		"status", status,
	)
	return eventbus.HandleResultAck
}
```

- [ ] **Step 4: Run handler tests**

Run:

```bash
go test ./internal/group-service/handlers
```

Expected: PASS.

- [ ] **Step 5: Commit handler**

```bash
git add internal/group-service/handlers/individual_member_expiry_event_handler.go internal/group-service/handlers/individual_member_expiry_event_handler_test.go
git commit -m "feat: handle individual member expiry commands"
```

---

### Task 5: Repository Schema, Indexes, and Mappers

**Files:**
- Modify: `internal/group-service/repositories/mongo_group_repository.go`
- Modify: `internal/group-service/repositories/mongo_group_repository_test.go`

- [ ] **Step 1: Write failing mapper and index tests**

Add to `internal/group-service/repositories/mongo_group_repository_test.go`:

```go
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
```

Update `TestIndexModels` to assert `individualMemberExpiryTaskIndexModels()`:

```go
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
```

Add:

```go
func TestActiveUnexpiredIndividualMemberFilter(t *testing.T) {
	filter := activeUnexpiredIndividualMemberFilter("group-1")
	want := bson.M{"group_id": "group-1", "deleted_at": nil, "expired_at": nil}

	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}
```

- [ ] **Step 2: Run repository tests to verify failure**

Run:

```bash
go test ./internal/group-service/repositories -run 'Test(NewIndividualMemberDocumentsIncludesExpiredAtAndTask|NewIndividualMemberExpiryTaskDocument|IndexModels|ActiveUnexpiredIndividualMemberFilter)'
```

Expected: FAIL with undefined document, constants, index model, and helper.

- [ ] **Step 3: Add collection constants, repository field, and document structs**

In `internal/group-service/repositories/mongo_group_repository.go`, add constants:

```go
individualMemberExpiryTaskCollectionName          = "individual_member_expiry_task"
individualMemberExpiryTasksActiveMemberUniqueIndexName = "individual_member_expiry_task_active_group_account_unique"
individualMemberExpiryTasksBucketIndexName             = "individual_member_expiry_task_bucket_id"
```

Add repository field:

```go
memberExpiryTasks *mongo.Collection
```

Initialize it in `NewMongoGroupRepository`:

```go
memberExpiryTasks: db.Collection(individualMemberExpiryTaskCollectionName),
```

Update `individualMemberDocument`:

```go
ExpiredAt *time.Time `bson:"expired_at"`
```

Add:

```go
type individualMemberExpiryTaskDocument struct {
	ID               string `bson:"_id"`
	GroupID          string `bson:"group_id"`
	NTAccount        string `bson:"nt_account"`
	ExpirationBucket string `bson:"expiration_bucket"`
}
```

- [ ] **Step 4: Add indexes**

In `EnsureIndexes`, add:

```go
if _, err := r.memberExpiryTasks.Indexes().CreateMany(ctx, individualMemberExpiryTaskIndexModels()); err != nil {
	return fmt.Errorf("create individual member expiry task indexes: %w", err)
}
```

Add:

```go
func individualMemberExpiryTaskIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "group_id", Value: 1},
				{Key: "nt_account", Value: 1},
			},
			Options: options.Index().
				SetName(individualMemberExpiryTasksActiveMemberUniqueIndexName).
				SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "expiration_bucket", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName(individualMemberExpiryTasksBucketIndexName),
		},
	}
}
```

- [ ] **Step 5: Add mappers and filters**

Update `newIndividualMemberDocuments` and `individualMemberDocument.toDomain` to copy `ExpiredAt`.

Add:

```go
func newIndividualMemberExpiryTaskDocument(task group.IndividualMemberExpiryTask) individualMemberExpiryTaskDocument {
	return individualMemberExpiryTaskDocument{
		ID:               task.ID,
		GroupID:          task.GroupID,
		NTAccount:        task.NTAccount,
		ExpirationBucket: task.ExpirationBucket,
	}
}

func newIndividualMemberExpiryTaskDocuments(members []group.IndividualMember) []individualMemberExpiryTaskDocument {
	docs := make([]individualMemberExpiryTaskDocument, 0, len(members))
	for _, member := range members {
		if member.ExpiryTask == nil {
			continue
		}
		docs = append(docs, newIndividualMemberExpiryTaskDocument(*member.ExpiryTask))
	}
	return docs
}

func activeUnexpiredIndividualMemberFilter(groupID string) bson.M {
	return bson.M{
		"group_id":   groupID,
		"deleted_at": nil,
		"expired_at": nil,
	}
}
```

- [ ] **Step 6: Run repository mapper/index tests**

Run:

```bash
go test ./internal/group-service/repositories -run 'Test(NewIndividualMemberDocumentsIncludesExpiredAtAndTask|NewIndividualMemberExpiryTaskDocument|IndexModels|ActiveUnexpiredIndividualMemberFilter)'
```

Expected: PASS.

- [ ] **Step 7: Commit repository schema foundation**

```bash
git add internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_group_repository_test.go
git commit -m "feat: add individual member expiry task schema"
```

---

### Task 6: Repository Write-Path Task Maintenance

**Files:**
- Modify: `internal/group-service/repositories/mongo_group_repository.go`
- Modify: `internal/group-service/repositories/mongo_group_repository_test.go`

- [ ] **Step 1: Write failing integration tests for create/add/update/delete**

Add tests:

```go
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
```

Add helper:

```go
func insertIndividualMemberExpiryTask(t *testing.T, repository *MongoGroupRepository, taskDoc individualMemberExpiryTaskDocument) {
	t.Helper()
	if _, err := repository.memberExpiryTasks.InsertOne(context.Background(), taskDoc); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Write failing delete-group and unexpired-count tests**

Extend `TestMongoGroupRepositoryDeleteIntegration` by inserting an `individualMemberExpiryTaskDocument` before delete and asserting zero remaining tasks for `group_id`.

Add:

```go
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
```

- [ ] **Step 3: Run targeted integration tests to verify failure**

Run:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -run 'IndividualMemberExpiryTask|DeleteIntegration|OnlyExpiredMembers' -count=1
```

Expected: FAIL if local MongoDB replica set is available. If `GROUP_SERVICE_MONGODB_TEST_URI` is not available, the tests skip; continue implementing and run unit tests in Step 6.

- [ ] **Step 4: Implement write-path task inserts and deletes**

In `Create`, after inserting `group_expiry_task`, insert member task docs:

```go
memberExpiryTaskDocs := newIndividualMemberExpiryTaskDocuments(input.IndividualMembers)
if len(memberExpiryTaskDocs) > 0 {
	docs := make([]any, 0, len(memberExpiryTaskDocs))
	for _, doc := range memberExpiryTaskDocs {
		docs = append(docs, doc)
	}
	if _, err := r.memberExpiryTasks.InsertMany(sessionCtx, docs); err != nil {
		return nil, fmt.Errorf("insert individual member expiry tasks: %w", err)
	}
}
```

In `AddIndividualMembers`, after `InsertMany` for members, insert `newIndividualMemberExpiryTaskDocuments(input.IndividualMembers)` using the same conversion and error message.

In `UpdateIndividualMemberExpiration`, update with `expired_at: nil`:

```go
bson.M{"$set": bson.M{"expiration_date": input.ExpirationDate, "updated_at": updatedAt, "expired_at": nil}}
```

After matched-count validation, delete and insert replacement task:

```go
if _, deleteTaskErr := r.memberExpiryTasks.DeleteOne(sessionCtx, bson.M{"group_id": input.GroupID, "nt_account": input.NTAccount}); deleteTaskErr != nil {
	return nil, fmt.Errorf("delete individual member expiry task: %w", deleteTaskErr)
}
if input.ExpiryTask != nil {
	if _, insertTaskErr := r.memberExpiryTasks.InsertOne(sessionCtx, newIndividualMemberExpiryTaskDocument(*input.ExpiryTask)); insertTaskErr != nil {
		return nil, fmt.Errorf("insert individual member expiry task: %w", insertTaskErr)
	}
}
```

In `DeleteIndividualMember`, delete matching task after the active group check:

```go
if _, deleteTaskErr := r.memberExpiryTasks.DeleteOne(sessionCtx, bson.M{"group_id": input.GroupID, "nt_account": input.NTAccount}); deleteTaskErr != nil {
	return nil, fmt.Errorf("delete individual member expiry task: %w", deleteTaskErr)
}
```

In `Delete`, delete all member expiry tasks for the group when the group matched:

```go
if _, deleteMemberTasksErr := r.memberExpiryTasks.DeleteMany(sessionCtx, bson.M{"group_id": input.GroupID}); deleteMemberTasksErr != nil {
	return nil, fmt.Errorf("delete individual member expiry tasks: %w", deleteMemberTasksErr)
}
```

In `UpdateGroupingRule`, change empty-rule member count to:

```go
memberCount, memberCountErr := r.members.CountDocuments(sessionCtx, activeUnexpiredIndividualMemberFilter(input.GroupID))
```

- [ ] **Step 5: Run targeted integration tests**

Run:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -run 'IndividualMemberExpiryTask|DeleteIntegration|OnlyExpiredMembers' -count=1
```

Expected: PASS if local MongoDB replica set is available. If skipped, note the skip and run Step 6.

- [ ] **Step 6: Run repository unit tests**

Run:

```bash
go test ./internal/group-service/repositories
```

Expected: PASS, with integration tests skipped when `GROUP_SERVICE_MONGODB_TEST_URI` is unset.

- [ ] **Step 7: Commit write-path repository changes**

```bash
git add internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_group_repository_test.go
git commit -m "feat: maintain individual member expiry tasks"
```

---

### Task 7: Repository Command Transaction

**Files:**
- Modify: `internal/group-service/repositories/mongo_group_repository.go`
- Modify: `internal/group-service/repositories/mongo_group_repository_test.go`

- [ ] **Step 1: Write failing command transaction tests**

Add:

```go
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
	tests := []struct {
		name      string
		setup     func(t *testing.T, repository *MongoGroupRepository)
		command   group.ExpireIndividualMemberCommand
		want      group.ExpireIndividualMemberStatus
		wantTasks int64
	}{
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

			status, err := repository.ExpireIndividualMember(context.Background(), tt.command, repositoryTime(), location)
			if err != nil {
				t.Fatalf("ExpireIndividualMember() error = %v", err)
			}
			if status != tt.want {
				t.Fatalf("status = %s, want %s", status, tt.want)
			}
			if tt.wantTasks > 0 {
				count, err := repository.memberExpiryTasks.CountDocuments(context.Background(), bson.M{"_id": tt.command.TaskID})
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
```

Add helper:

```go
func insertGroupWithMember(t *testing.T, repository *MongoGroupRepository, groupDoc groupDocument, memberDoc individualMemberDocument) {
	t.Helper()
	if _, err := repository.groups.InsertOne(context.Background(), groupDoc); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.members.InsertOne(context.Background(), memberDoc); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run command tests to verify failure**

Run:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -run 'ExpireIndividualMember' -count=1
```

Expected: FAIL with undefined `ExpireIndividualMember` if MongoDB is available; otherwise tests skip.

- [ ] **Step 3: Implement command transaction**

Add helper methods:

```go
func (r *MongoGroupRepository) findIndividualMemberExpiryTask(ctx context.Context, input group.ExpireIndividualMemberCommand) (*individualMemberExpiryTaskDocument, error) {
	var doc individualMemberExpiryTaskDocument
	err := r.memberExpiryTasks.FindOne(ctx, bson.M{
		"_id":               input.TaskID,
		"group_id":          input.GroupID,
		"nt_account":        input.NTAccount,
		"expiration_bucket": input.ExpirationBucket,
	}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("find individual member expiry task: %w", err)
	}
	return &doc, nil
}

func (r *MongoGroupRepository) deleteIndividualMemberExpiryTaskByID(ctx context.Context, taskID string) error {
	if _, err := r.memberExpiryTasks.DeleteOne(ctx, bson.M{"_id": taskID}); err != nil {
		return fmt.Errorf("delete individual member expiry task: %w", err)
	}
	return nil
}
```

Add method:

```go
func (r *MongoGroupRepository) ExpireIndividualMember(ctx context.Context, input group.ExpireIndividualMemberCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireIndividualMemberStatus, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return "", fmt.Errorf("start individual member expiry session: %w", err)
	}
	defer session.EndSession(ctx)

	var status group.ExpireIndividualMemberStatus
	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		task, taskErr := r.findIndividualMemberExpiryTask(sessionCtx, input)
		if taskErr != nil {
			return nil, taskErr
		}
		if task == nil {
			status = group.ExpireIndividualMemberStatusStaleTask
			return nil, nil
		}

		var doc individualMemberDocument
		findMemberErr := r.members.FindOne(sessionCtx, activeIndividualMemberFilter(input.GroupID, input.NTAccount)).Decode(&doc)
		if findMemberErr != nil {
			if errors.Is(findMemberErr, mongo.ErrNoDocuments) {
				if deleteErr := r.deleteIndividualMemberExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
					return nil, deleteErr
				}
				status = group.ExpireIndividualMemberStatusStaleMember
				return nil, nil
			}
			return nil, fmt.Errorf("find individual member for expiry: %w", findMemberErr)
		}

		if doc.ExpiredAt != nil {
			if deleteErr := r.deleteIndividualMemberExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
				return nil, deleteErr
			}
			status = group.ExpireIndividualMemberStatusAlreadyExpired
			return nil, nil
		}

		currentBucket := group.ExpirationBucketFor(doc.ExpirationDate, bucketLocation)
		if currentBucket != input.ExpirationBucket {
			status = group.ExpireIndividualMemberStatusStaleBucket
			return nil, nil
		}

		result, updateErr := r.members.UpdateOne(sessionCtx,
			activeIndividualMemberFilter(input.GroupID, input.NTAccount),
			bson.M{"$set": bson.M{
				"expired_at": expiredAt,
				"updated_at": expiredAt,
			}},
		)
		if updateErr != nil {
			return nil, fmt.Errorf("mark individual member expired: %w", updateErr)
		}
		if result.MatchedCount == 0 {
			status = group.ExpireIndividualMemberStatusStaleMember
			return nil, nil
		}
		if deleteErr := r.deleteIndividualMemberExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
			return nil, deleteErr
		}
		status = group.ExpireIndividualMemberStatusExpired
		return nil, nil
	})
	if err != nil {
		return "", err
	}
	return status, nil
}
```

- [ ] **Step 4: Run command transaction tests**

Run:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -run 'ExpireIndividualMember' -count=1
```

Expected: PASS if local MongoDB replica set is available. If skipped, note the skip.

- [ ] **Step 5: Run repository tests**

Run:

```bash
go test ./internal/group-service/repositories
```

Expected: PASS, with integration tests skipped when `GROUP_SERVICE_MONGODB_TEST_URI` is unset.

- [ ] **Step 6: Commit command transaction**

```bash
git add internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_group_repository_test.go
git commit -m "feat: expire individual members transactionally"
```

---

### Task 8: Configuration, Environment, and Main Wiring

**Files:**
- Modify: `internal/group-service/config/config.go`
- Modify: `internal/group-service/config/config_test.go`
- Modify: `cmd/group-service/main.go`
- Modify: `cmd/group-service/main_test.go`
- Modify: `.env.example`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Write failing config tests**

In `TestLoadReadsRequiredEnvironment`, set:

```go
t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM", "INDIVIDUAL_MEMBER_EXPIRY")
t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE", "group-service-individual-member-expiry")
t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT", "app.todo.group.individual-member.expiry.process")
t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT", "30")
t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT", "9s")
t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", "UTC+8")
```

Add assertions:

```go
if cfg.IndividualMemberExpiryCommand.Stream != "INDIVIDUAL_MEMBER_EXPIRY" {
	t.Fatalf("IndividualMemberExpiryCommand.Stream = %q, want INDIVIDUAL_MEMBER_EXPIRY", cfg.IndividualMemberExpiryCommand.Stream)
}
if cfg.IndividualMemberExpiryCommand.Durable != "group-service-individual-member-expiry" {
	t.Fatalf("IndividualMemberExpiryCommand.Durable = %q, want group-service-individual-member-expiry", cfg.IndividualMemberExpiryCommand.Durable)
}
if cfg.IndividualMemberExpiryCommand.Subject != "app.todo.group.individual-member.expiry.process" {
	t.Fatalf("IndividualMemberExpiryCommand.Subject = %q, want app.todo.group.individual-member.expiry.process", cfg.IndividualMemberExpiryCommand.Subject)
}
if cfg.IndividualMemberExpiryCommand.FetchCount != 30 {
	t.Fatalf("IndividualMemberExpiryCommand.FetchCount = %d, want 30", cfg.IndividualMemberExpiryCommand.FetchCount)
}
if cfg.IndividualMemberExpiryCommand.MaxWait != 9*time.Second {
	t.Fatalf("IndividualMemberExpiryCommand.MaxWait = %s, want 9s", cfg.IndividualMemberExpiryCommand.MaxWait)
}
if cfg.IndividualMemberExpiryCommand.BucketTimezone != "UTC+8" {
	t.Fatalf("IndividualMemberExpiryCommand.BucketTimezone = %q, want UTC+8", cfg.IndividualMemberExpiryCommand.BucketTimezone)
}
if cfg.IndividualMemberExpiryCommand.BucketLocation == nil || cfg.IndividualMemberExpiryCommand.BucketLocation.String() != "UTC+08:00" {
	t.Fatalf("IndividualMemberExpiryCommand.BucketLocation = %v, want UTC+08:00", cfg.IndividualMemberExpiryCommand.BucketLocation)
}
```

In `TestLoadAppliesOptionalDefaults`, assert individual-member defaults:

```go
if cfg.IndividualMemberExpiryCommand.FetchCount != 20 {
	t.Fatalf("IndividualMemberExpiryCommand.FetchCount = %d, want 20", cfg.IndividualMemberExpiryCommand.FetchCount)
}
if cfg.IndividualMemberExpiryCommand.MaxWait != 5*time.Second {
	t.Fatalf("IndividualMemberExpiryCommand.MaxWait = %s, want 5s", cfg.IndividualMemberExpiryCommand.MaxWait)
}
if cfg.IndividualMemberExpiryCommand.BucketTimezone != "UTC" {
	t.Fatalf("IndividualMemberExpiryCommand.BucketTimezone = %q, want UTC", cfg.IndividualMemberExpiryCommand.BucketTimezone)
}
```

Add tests:

```go
func TestLoadRejectsMissingIndividualMemberExpiryCommandConfig(t *testing.T) {
	setRequiredGroupServiceConfig(t)
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT", " ")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT is required") {
		t.Fatalf("Load() error = %v, want missing subject", err)
	}
}

func TestLoadRejectsInvalidIndividualMemberExpiryBucketTimezone(t *testing.T) {
	setRequiredGroupServiceConfig(t)
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", "Asia/Taipei")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE") {
		t.Fatalf("Load() error = %v, want invalid timezone", err)
	}
}
```

Update `setRequiredGroupServiceConfig`:

```go
t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM", "INDIVIDUAL_MEMBER_EXPIRY")
t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE", "group-service-individual-member-expiry")
t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT", "app.todo.group.individual-member.expiry.process")
```

- [ ] **Step 2: Write failing main config test**

Add to `cmd/group-service/main_test.go`:

```go
func TestNewIndividualMemberExpiryEventbusConfig(t *testing.T) {
	cfg := config.Config{
		IndividualMemberExpiryCommand: config.IndividualMemberExpiryCommandConfig{
			Stream:     "INDIVIDUAL_MEMBER_EXPIRY",
			Durable:    "group-service-individual-member-expiry",
			Subject:    "app.todo.group.individual-member.expiry.process",
			FetchCount: 30,
			MaxWait:    9 * time.Second,
		},
	}

	got := newIndividualMemberExpiryEventbusConfig(cfg)

	if got.Stream != "INDIVIDUAL_MEMBER_EXPIRY" {
		t.Fatalf("Stream = %q, want INDIVIDUAL_MEMBER_EXPIRY", got.Stream)
	}
	if got.Durable != "group-service-individual-member-expiry" {
		t.Fatalf("Durable = %q, want group-service-individual-member-expiry", got.Durable)
	}
	if len(got.Subjects) != 1 || got.Subjects[0] != "app.todo.group.individual-member.expiry.process" {
		t.Fatalf("Subjects = %v, want [app.todo.group.individual-member.expiry.process]", got.Subjects)
	}
	if got.BatchSize != 30 {
		t.Fatalf("BatchSize = %d, want 30", got.BatchSize)
	}
	if got.MaxWait != 9*time.Second {
		t.Fatalf("MaxWait = %s, want 9s", got.MaxWait)
	}
}
```

- [ ] **Step 3: Run config and main tests to verify failure**

Run:

```bash
go test ./internal/group-service/config ./cmd/group-service
```

Expected: FAIL with missing config struct/field and missing eventbus config helper.

- [ ] **Step 4: Implement config loading**

In `internal/group-service/config/config.go`, add to `Config`:

```go
IndividualMemberExpiryCommand IndividualMemberExpiryCommandConfig
```

Add:

```go
type IndividualMemberExpiryCommandConfig struct {
	Stream         string
	Durable        string
	Subject        string
	FetchCount     int
	MaxWait        time.Duration
	BucketTimezone string
	BucketLocation *time.Location
}
```

Add defaults:

```go
v.SetDefault("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT", 20)
v.SetDefault("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT", "5s")
v.SetDefault("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", "UTC")
```

Parse the bucket:

```go
memberBucketTimezone := v.GetString("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE")
memberBucketLocation, err := parseExpirationBucketLocationConfig("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", memberBucketTimezone)
if err != nil {
	return Config{}, err
}
```

Also change the group bucket parse to use:

```go
bucketLocation, err := parseExpirationBucketLocationConfig("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE", bucketTimezone)
```

Add helper:

```go
func parseExpirationBucketLocationConfig(key string, value string) (*time.Location, error) {
	location, err := group.ParseExpirationBucketLocation(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be valid: %w", key, err)
	}
	return location, nil
}
```

Populate config:

```go
IndividualMemberExpiryCommand: IndividualMemberExpiryCommandConfig{
	Stream:         v.GetString("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM"),
	Durable:        v.GetString("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE"),
	Subject:        v.GetString("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT"),
	FetchCount:     v.GetInt("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT"),
	MaxWait:        v.GetDuration("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT"),
	BucketTimezone: memberBucketTimezone,
	BucketLocation: memberBucketLocation,
},
```

Add required values:

```go
"GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM":  c.IndividualMemberExpiryCommand.Stream,
"GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE": c.IndividualMemberExpiryCommand.Durable,
"GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT": c.IndividualMemberExpiryCommand.Subject,
"GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE": c.IndividualMemberExpiryCommand.BucketTimezone,
```

Add validation:

```go
if c.IndividualMemberExpiryCommand.FetchCount <= 0 {
	return fmt.Errorf("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT must be greater than zero")
}
if c.IndividualMemberExpiryCommand.MaxWait <= 0 {
	return fmt.Errorf("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT must be positive")
}
if c.IndividualMemberExpiryCommand.BucketLocation == nil {
	return fmt.Errorf("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE must be valid")
}
```

- [ ] **Step 5: Wire main**

In `cmd/group-service/main.go`, construct service with both bucket locations:

```go
services.WithIndividualMemberExpiryBucketLocation(cfg.IndividualMemberExpiryCommand.BucketLocation),
```

Create handler and consumer:

```go
individualMemberExpiryEventHandler := handlers.NewIndividualMemberExpiryEventHandler(groupService, cfg.IndividualMemberExpiryCommand.Subject, logger)
individualMemberExpiryConsumer, err := eventbus.NewJetStreamConsumer(ctx, nc, newIndividualMemberExpiryEventbusConfig(cfg), individualMemberExpiryEventHandler, logger)
if err != nil {
	return err
}
```

Increase channel buffer:

```go
errCh := make(chan error, 3)
```

Start the consumer:

```go
go func() {
	errCh <- individualMemberExpiryConsumer.Run(ctx)
}()
```

Add helper:

```go
func newIndividualMemberExpiryEventbusConfig(cfg config.Config) eventbus.Config {
	return eventbus.Config{
		Stream:    cfg.IndividualMemberExpiryCommand.Stream,
		Subjects:  []string{cfg.IndividualMemberExpiryCommand.Subject},
		Durable:   cfg.IndividualMemberExpiryCommand.Durable,
		BatchSize: cfg.IndividualMemberExpiryCommand.FetchCount,
		MaxWait:   cfg.IndividualMemberExpiryCommand.MaxWait,
	}
}
```

- [ ] **Step 6: Update `.env.example`**

Add under group-service NATS settings:

```dotenv
GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM=INDIVIDUAL_MEMBER_EXPIRY
GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE=group-service-individual-member-expiry
GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT=app.todo.group.individual-member.expiry.process
GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT=20
GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT=5s
GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE=UTC
```

- [ ] **Step 7: Update `docker-compose.yml` NATS provisioning**

In the `nats-init` environment, add:

```yaml
      GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM: ${GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM:-INDIVIDUAL_MEMBER_EXPIRY}
      GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE: ${GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE:-group-service-individual-member-expiry}
      GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT: ${GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT:-app.todo.group.individual-member.expiry.process}
```

After the group expiry consumer provisioning block, add stream and consumer creation commands for `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_*` using the same flags as the group expiry stream and consumer:

```sh
        if ! nats --server "$$GROUP_SERVICE_NATS_URL" stream info "$$GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM" >/dev/null 2>&1; then
          nats --server "$$GROUP_SERVICE_NATS_URL" stream add "$$GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM" \
            --subjects "$$GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT" \
            --storage file \
            --retention limits \
            --discard old \
            --replicas 1 \
            --defaults
        fi

        if ! nats --server "$$GROUP_SERVICE_NATS_URL" consumer info "$$GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM" "$$GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE" >/dev/null 2>&1; then
          nats --server "$$GROUP_SERVICE_NATS_URL" consumer add "$$GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM" "$$GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE" \
            --filter "$$GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT" \
            --pull \
            --ack explicit \
            --deliver all \
            --replay instant \
            --max-deliver=-1 \
            --defaults
        fi
```

- [ ] **Step 8: Run config and main tests**

Run:

```bash
go test ./internal/group-service/config ./cmd/group-service
```

Expected: PASS.

- [ ] **Step 9: Commit config and wiring**

```bash
git add internal/group-service/config/config.go internal/group-service/config/config_test.go cmd/group-service/main.go cmd/group-service/main_test.go .env.example docker-compose.yml
git commit -m "feat: wire individual member expiry consumer"
```

---

### Task 9: NATS Fixture Script

**Files:**
- Create: `examples/nats/fixtures/send_individual_member_expiry_event.sh`
- Create: `examples/nats/fixtures/send_individual_member_expiry_event_test.sh`

- [ ] **Step 1: Write fixture script**

Create `examples/nats/fixtures/send_individual_member_expiry_event.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

DEFAULT_GROUP_ID="group-1"
DEFAULT_NT_ACCOUNT="user1"
DEFAULT_NATS_SUBJECT="app.todo.group.individual-member.expiry.process"
DEFAULT_NATS_SERVER="nats://workspace-permission-management-nats:4222"
DEFAULT_DOCKER_NETWORK="workspace_permission_management"
DEFAULT_NATS_BOX_IMAGE="natsio/nats-box:0.19.3"

usage() {
  cat <<'USAGE'
Usage:
  send_individual_member_expiry_event.sh [group_id] [nt_account] [expiration_bucket] [task_id]

Arguments:
  group_id            Defaults to group-1
  nt_account          Defaults to user1
  expiration_bucket   Defaults to the current UTC date in yyyy-MM-dd
  task_id             Defaults to a generated UUID

Environment overrides:
  GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT  NATS subject and CloudEvent type
  GROUP_SERVICE_NATS_URL                                  NATS server URL reachable from the nats-box container
  DOCKER_NETWORK                                          Docker network for the nats-box container
  NATS_BOX_IMAGE                                          nats-box image to run
USAGE
}

require_command() {
  local name="$1"

  if ! command -v "${name}" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "${name}" >&2
    exit 1
  fi
}

new_uuid() {
  uuidgen | tr '[:upper:]' '[:lower:]'
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "$#" -gt 4 ]]; then
  usage >&2
  exit 1
fi

require_command docker
require_command jq
require_command uuidgen
require_command date

group_id="${1:-${DEFAULT_GROUP_ID}}"
nt_account="${2:-${DEFAULT_NT_ACCOUNT}}"
expiration_bucket="${3:-$(date -u +"%Y-%m-%d")}"
task_id="${4:-$(new_uuid)}"
nats_subject="${GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT:-${DEFAULT_NATS_SUBJECT}}"
nats_server="${GROUP_SERVICE_NATS_URL:-${DEFAULT_NATS_SERVER}}"
docker_network="${DOCKER_NETWORK:-${DEFAULT_DOCKER_NETWORK}}"
nats_box_image="${NATS_BOX_IMAGE:-${DEFAULT_NATS_BOX_IMAGE}}"

event_id="$(new_uuid)"
event_time="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

payload="$(
  jq -n \
    --arg event_type "${nats_subject}" \
    --arg event_source "individual-member-expiry-fixture" \
    --arg task_id "${task_id}" \
    --arg event_id "${event_id}" \
    --arg event_time "${event_time}" \
    --arg group_id "${group_id}" \
    --arg nt_account "${nt_account}" \
    --arg expiration_bucket "${expiration_bucket}" \
    '{
      specversion: "1.0",
      type: $event_type,
      source: $event_source,
      subject: $task_id,
      id: $event_id,
      time: $event_time,
      datacontenttype: "application/json",
      data: {
        task_id: $task_id,
        group_id: $group_id,
        nt_account: $nt_account,
        expiration_bucket: $expiration_bucket
      }
    }'
)"

printf '%s' "${payload}" | docker run --rm -i \
  --network "${docker_network}" \
  "${nats_box_image}" \
  nats --server "${nats_server}" pub --jetstream --force-stdin "${nats_subject}"

printf 'published individual member expiry event: subject=%s group_id=%s nt_account=%s expiration_bucket=%s task_id=%s event_id=%s\n' \
  "${nats_subject}" \
  "${group_id}" \
  "${nt_account}" \
  "${expiration_bucket}" \
  "${task_id}" \
  "${event_id}"
```

- [ ] **Step 2: Write fixture test**

Create `examples/nats/fixtures/send_individual_member_expiry_event_test.sh` by following the existing `send_group_expiry_event_test.sh` structure with these exact assertions for the custom run:

```bash
run_script custom group-9 user9 2026-06-01 task-9

assert_docker_args "${TMP_DIR}/custom.args"

jq -e '
  .specversion == "1.0" and
  .type == "app.todo.group.individual-member.expiry.process" and
  .source == "individual-member-expiry-fixture" and
  .subject == "task-9" and
  .id == "11111111-1111-4111-8111-111111111111" and
  .time == "2026-05-10T00:00:00Z" and
  .datacontenttype == "application/json" and
  .data.task_id == "task-9" and
  .data.group_id == "group-9" and
  .data.nt_account == "user9" and
  .data.expiration_bucket == "2026-06-01"
' "${TMP_DIR}/custom.stdin" >/dev/null
```

Use this expected docker args string:

```bash
local expected_args="run --rm -i --network workspace_permission_management natsio/nats-box:0.19.3 nats --server nats://workspace-permission-management-nats:4222 pub --jetstream --force-stdin app.todo.group.individual-member.expiry.process"
```

For default runs, assert:

```jq
.data.group_id == "group-1" and
.data.nt_account == "user1" and
.data.expiration_bucket == "2026-05-10"
```

- [ ] **Step 3: Make scripts executable**

Run:

```bash
chmod +x examples/nats/fixtures/send_individual_member_expiry_event.sh examples/nats/fixtures/send_individual_member_expiry_event_test.sh
```

- [ ] **Step 4: Run fixture test**

Run:

```bash
examples/nats/fixtures/send_individual_member_expiry_event_test.sh
```

Expected: PASS with no output.

- [ ] **Step 5: Commit fixture**

```bash
git add examples/nats/fixtures/send_individual_member_expiry_event.sh examples/nats/fixtures/send_individual_member_expiry_event_test.sh
git commit -m "chore: add individual member expiry event fixture"
```

---

### Task 10: Full Verification and Plan Finalization

**Files:**
- Modify after implementation completion: `docs/plans/active/2026-05-11-group-individual-member-expiry-command.md`
- Move after implementation completion: `docs/plans/active/2026-05-11-group-individual-member-expiry-command.md` to `docs/plans/completed/2026-05-11-group-individual-member-expiry-command.md`

- [ ] **Step 1: Run package tests**

Run:

```bash
go test ./internal/domain/group ./internal/group-service/transport ./internal/group-service/handlers ./internal/group-service/services ./internal/group-service/config ./cmd/group-service
```

Expected: PASS.

- [ ] **Step 2: Run repository tests**

Run:

```bash
go test ./internal/group-service/repositories
```

Expected: PASS, with MongoDB integration tests skipped when `GROUP_SERVICE_MONGODB_TEST_URI` is unset.

- [ ] **Step 3: Run repository integration tests when MongoDB replica set is available**

Run:

```bash
GROUP_SERVICE_MONGODB_TEST_URI=mongodb://localhost:27017 go test ./internal/group-service/repositories -count=1
```

Expected: PASS. If MongoDB is unavailable, record the exact skip or connection failure in the final response.

- [ ] **Step 4: Run fixture scripts**

Run:

```bash
examples/nats/fixtures/send_group_expiry_event_test.sh
examples/nats/fixtures/send_individual_member_expiry_event_test.sh
```

Expected: PASS with no output.

- [ ] **Step 5: Run repository-wide tests**

Run:

```bash
go test ./...
```

Expected: PASS, with integration tests skipped when their environment variables are unset.

- [ ] **Step 6: Check diffs and formatting**

Run:

```bash
gofmt -w internal/domain/group/group.go internal/domain/group/validation.go internal/domain/group/validation_test.go internal/group-service/services/group_service.go internal/group-service/services/group_service_test.go internal/group-service/transport/individual_member_expiry_event.go internal/group-service/transport/individual_member_expiry_event_test.go internal/group-service/handlers/individual_member_expiry_event_handler.go internal/group-service/handlers/individual_member_expiry_event_handler_test.go internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_group_repository_test.go internal/group-service/config/config.go internal/group-service/config/config_test.go cmd/group-service/main.go cmd/group-service/main_test.go
git diff --check
git status --short
```

Expected: no `gofmt` changes after the command, `git diff --check` exits 0, and `git status --short` shows only intended files plus any pre-existing unrelated files.

- [ ] **Step 7: Move completed plan**

After all implementation verification is complete, move this plan:

```bash
mv docs/plans/active/2026-05-11-group-individual-member-expiry-command.md docs/plans/completed/2026-05-11-group-individual-member-expiry-command.md
```

- [ ] **Step 8: Commit final plan transition**

```bash
git add docs/plans/completed/2026-05-11-group-individual-member-expiry-command.md
git add -u docs/plans/active/2026-05-11-group-individual-member-expiry-command.md
git commit -m "docs: complete individual member expiry command plan"
```

## Self-Review Results

- Spec coverage: tasks cover domain command/status, CloudEvent parser, handler ack/retry/terminate mapping, config, JetStream consumer wiring, MongoDB collection/indexes, create/add/update/delete task maintenance, command transaction idempotency, fixture support, and verification.
- Placeholder scan: no incomplete placeholder instructions or open-ended implementation notes remain.
- Type consistency: command name is `ExpireIndividualMemberCommand`, task name is `IndividualMemberExpiryTask`, status name is `ExpireIndividualMemberStatus`, config field is `IndividualMemberExpiryCommand`, collection is `individual_member_expiry_task`, and subject default is `app.todo.group.individual-member.expiry.process`.
