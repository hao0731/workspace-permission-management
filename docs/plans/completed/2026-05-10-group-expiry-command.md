# Group Expiry Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add group-service grouping-rule expiry tasks and a JetStream CloudEvent command consumer that marks `groups.grouping_rule.expired_at` and clears the matching `group_expiry_task` transactionally.

**Architecture:** Keep the existing group-service layering. Domain owns command models, status values, and bucket date rules; transport owns CloudEvent parsing; handlers classify JetStream ack/retry/terminate outcomes; services generate IDs/time and orchestrate workflows through consumer-side repository interfaces; repositories own MongoDB documents, indexes, and transactions; `cmd/group-service` wires NATS and `internal/shared/eventbus`.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, NATS JetStream through `internal/shared/eventbus`, CloudEvents SDK for Go, `log/slog`, `viper`, standard `testing`.

---

## Source Designs and Policies

Read these before implementing:

- [Group Expiry Command Design](../../designs/group-service-group-expiry-command.md)
- [Group Service Design](../../designs/group-service.md)
- [Group API Design](../../designs/group-service-group.md)
- [Backend Architecture Principle](../../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- Backend work must keep domain and service code free of Echo, NATS, JetStream, MongoDB driver, and transport DTO types.
- Event subscribers must intentionally classify ack, retry, and poison-message outcomes, and command handling must be idempotent.
- MongoDB, CloudEvent, and config shapes are explicit contracts and need focused tests.
- This implementation plan lives under `docs/plans/active/` and links to its source designs.

## Working Tree Note

At plan-writing time, `lefthook.yml` has unrelated local modifications. Do not stage or revert that file while implementing this plan.

## File Structure

Create:

- `internal/group-service/transport/group_expiry_event.go`: parse the group expiry command CloudEvent envelope into a domain command.
- `internal/group-service/transport/group_expiry_event_test.go`: transport parser tests for valid and poison messages.
- `internal/group-service/handlers/group_expiry_event_handler.go`: JetStream handler that maps parse/service outcomes to `eventbus.HandleResult`.
- `internal/group-service/handlers/group_expiry_event_handler_test.go`: handler outcome and logging behavior tests.

Modify:

- `internal/domain/group/group.go`: add expiry task, command, status, and `GroupingRule.ExpiredAt`.
- `internal/domain/group/validation.go`: add command validation and bucket timezone helpers.
- `internal/domain/group/validation_test.go`: add command and bucket tests.
- `internal/group-service/config/config.go`: add NATS, expiry command consumer, and bucket timezone config.
- `internal/group-service/config/config_test.go`: cover required config, defaults, and invalid timezone values.
- `internal/group-service/services/group_service.go`: generate expiry tasks on create/update, add command service workflow, add bucket-location option.
- `internal/group-service/services/group_service_test.go`: cover task generation, update cleanup intent, command status preservation, and repository failures.
- `internal/group-service/repositories/mongo_group_repository.go`: add `group_expiry_task` collection, schema, indexes, write-path task maintenance, command transaction.
- `internal/group-service/repositories/mongo_group_repository_test.go`: cover schema/index mapping and repository workflows.
- `cmd/group-service/main.go`: wire NATS and JetStream consumer alongside HTTP server.
- `cmd/group-service/main_test.go`: cover config-to-runtime wiring where existing tests allow it.
- `.env.example`: add group-service NATS and expiry command settings.

No HTTP response DTO should expose `grouping_rule.expired_at` in this phase.

---

### Task 1: Domain Expiry Models and Bucket Rules

**Files:**
- Modify: `internal/domain/group/group.go`
- Modify: `internal/domain/group/validation.go`
- Modify: `internal/domain/group/validation_test.go`

- [x] **Step 1: Add failing domain tests**

Add tests to `internal/domain/group/validation_test.go`:

```go
func TestExpireGroupingRuleCommandValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		command   ExpireGroupingRuleCommand
		wantError string
	}{
		{
			name:      "empty task id",
			command:   ExpireGroupingRuleCommand{WorkspaceID: "workspace-1", GroupID: "group-1", ExpirationBucket: "2026-05-10"},
			wantError: "task id is required",
		},
		{
			name:      "empty workspace id",
			command:   ExpireGroupingRuleCommand{TaskID: "task-1", GroupID: "group-1", ExpirationBucket: "2026-05-10"},
			wantError: "workspace id is required",
		},
		{
			name:      "empty group id",
			command:   ExpireGroupingRuleCommand{TaskID: "task-1", WorkspaceID: "workspace-1", ExpirationBucket: "2026-05-10"},
			wantError: "group id is required",
		},
		{
			name:      "invalid bucket",
			command:   ExpireGroupingRuleCommand{TaskID: "task-1", WorkspaceID: "workspace-1", GroupID: "group-1", ExpirationBucket: "2026/05/10"},
			wantError: "expiration bucket must use yyyy-MM-dd",
		},
		{
			name:    "valid",
			command: ExpireGroupingRuleCommand{TaskID: " task-1 ", WorkspaceID: " workspace-1 ", GroupID: " group-1 ", ExpirationBucket: "2026-05-10"},
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
					command.WorkspaceID != strings.TrimSpace(tt.command.WorkspaceID) ||
					command.GroupID != strings.TrimSpace(tt.command.GroupID) {
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

func TestParseExpirationBucketLocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value          string
		wantName       string
		wantOffsetHour int
		wantErr        bool
	}{
		{value: "UTC", wantName: "UTC"},
		{value: " utc ", wantName: "UTC"},
		{value: "UTC+8", wantName: "UTC+08:00", wantOffsetHour: 8},
		{value: "UTC+08", wantName: "UTC+08:00", wantOffsetHour: 8},
		{value: "UTC+08:00", wantName: "UTC+08:00", wantOffsetHour: 8},
		{value: "UTC-5", wantName: "UTC-05:00", wantOffsetHour: -5},
		{value: "Asia/Taipei", wantErr: true},
		{value: "UTC+25", wantErr: true},
		{value: "UTC+08:61", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			location, err := ParseExpirationBucketLocation(tt.value)
			if tt.wantErr {
				if err == nil || !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("ParseExpirationBucketLocation() error = %v, want ErrInvalidInput", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseExpirationBucketLocation() error = %v, want nil", err)
			}
			if location.String() != tt.wantName {
				t.Fatalf("location name = %q, want %q", location.String(), tt.wantName)
			}
			_, offset := time.Date(2026, 5, 10, 0, 0, 0, 0, location).Zone()
			if offset != tt.wantOffsetHour*3600 {
				t.Fatalf("offset = %d, want %d", offset, tt.wantOffsetHour*3600)
			}
		})
	}
}

func TestExpirationBucketFor(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, 5, 10, 16, 30, 0, 0, time.UTC)
	utc, err := ParseExpirationBucketLocation("UTC")
	if err != nil {
		t.Fatal(err)
	}
	taipei, err := ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}

	if got := ExpirationBucketFor(expiration, utc); got != "2026-05-10" {
		t.Fatalf("UTC bucket = %q, want 2026-05-10", got)
	}
	if got := ExpirationBucketFor(expiration, taipei); got != "2026-05-11" {
		t.Fatalf("UTC+8 bucket = %q, want 2026-05-11", got)
	}
	if got := ExpirationBucketFor(expiration, nil); got != "2026-05-10" {
		t.Fatalf("nil location bucket = %q, want UTC fallback", got)
	}
}
```

Also add `errors`, `strings`, and `time` imports if they are not already present.

- [x] **Step 2: Run domain tests and verify failure**

Run:

```bash
go test ./internal/domain/group
```

Expected: FAIL because `ExpireGroupingRuleCommand`, `ParseExpirationBucketLocation`, and `ExpirationBucketFor` are undefined.

- [x] **Step 3: Add domain types**

In `internal/domain/group/group.go`, update `GroupingRule` and add expiry types:

```go
type Group struct {
	ID                string
	WorkspaceID       string
	Name              string
	NormalizedName    string
	Description       string
	GroupingRule      GroupingRule
	IndividualMembers []IndividualMember
	ExpiryTask        *ExpiryTask
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}
```

```go
type GroupingRule struct {
	Rules          []Rule
	ExpirationDate time.Time
	ExpiredAt      *time.Time
}

type ExpiryTask struct {
	ID               string
	WorkspaceID      string
	GroupID          string
	ExpirationBucket string
}

type ExpireGroupingRuleCommand struct {
	TaskID           string
	WorkspaceID      string
	GroupID          string
	ExpirationBucket string
}

type ExpireGroupingRuleStatus string

const (
	ExpireGroupingRuleStatusExpired        ExpireGroupingRuleStatus = "expired"
	ExpireGroupingRuleStatusStaleTask      ExpireGroupingRuleStatus = "stale_task"
	ExpireGroupingRuleStatusStaleGroup     ExpireGroupingRuleStatus = "stale_group"
	ExpireGroupingRuleStatusAlreadyExpired ExpireGroupingRuleStatus = "already_expired"
	ExpireGroupingRuleStatusStaleBucket    ExpireGroupingRuleStatus = "stale_bucket"
)
```

Add normalizers:

```go
func (task ExpiryTask) Normalize() ExpiryTask {
	task.ID = strings.TrimSpace(task.ID)
	task.WorkspaceID = strings.TrimSpace(task.WorkspaceID)
	task.GroupID = strings.TrimSpace(task.GroupID)
	task.ExpirationBucket = strings.TrimSpace(task.ExpirationBucket)
	return task
}

func (command ExpireGroupingRuleCommand) Normalize() ExpireGroupingRuleCommand {
	command.TaskID = strings.TrimSpace(command.TaskID)
	command.WorkspaceID = strings.TrimSpace(command.WorkspaceID)
	command.GroupID = strings.TrimSpace(command.GroupID)
	command.ExpirationBucket = strings.TrimSpace(command.ExpirationBucket)
	return command
}
```

- [x] **Step 4: Add bucket parser and validation**

In `internal/domain/group/validation.go`, add imports:

```go
import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)
```

If `validation.go` already imports any of these packages, merge the import list.

Add constants and regex:

```go
const expirationBucketLayout = "2006-01-02"

var expirationBucketOffsetPattern = regexp.MustCompile(`^UTC([+-])(\d{1,2})(?::?(\d{2}))?$`)
```

Add validation and helpers:

```go
func (command ExpireGroupingRuleCommand) Validate() error {
	if command.TaskID == "" {
		return invalidInput("task id is required")
	}
	if command.WorkspaceID == "" {
		return invalidInput("workspace id is required")
	}
	if command.GroupID == "" {
		return invalidInput("group id is required")
	}
	if !IsValidExpirationBucket(command.ExpirationBucket) {
		return invalidInput("expiration bucket must use yyyy-MM-dd")
	}
	return nil
}

func IsValidExpirationBucket(value string) bool {
	parsed, err := time.Parse(expirationBucketLayout, value)
	return err == nil && parsed.Format(expirationBucketLayout) == value
}

func ExpirationBucketFor(expiration time.Time, location *time.Location) string {
	if location == nil {
		location = time.UTC
	}
	return expiration.In(location).Format(expirationBucketLayout)
}

func ParseExpirationBucketLocation(value string) (*time.Location, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" || normalized == "UTC" {
		return time.UTC, nil
	}

	matches := expirationBucketOffsetPattern.FindStringSubmatch(normalized)
	if matches == nil {
		return nil, invalidInput("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE must be UTC or a fixed offset such as UTC+8")
	}

	hours, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid timezone hour offset", ErrInvalidInput)
	}
	minutes := 0
	if matches[3] != "" {
		minutes, err = strconv.Atoi(matches[3])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid timezone minute offset", ErrInvalidInput)
		}
	}
	if hours > 14 || minutes > 59 {
		return nil, invalidInput("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE offset is out of range")
	}

	sign := 1
	if matches[1] == "-" {
		sign = -1
	}
	totalSeconds := sign * ((hours * 60 * 60) + (minutes * 60))
	name := fmt.Sprintf("UTC%s%02d:%02d", matches[1], hours, minutes)
	return time.FixedZone(name, totalSeconds), nil
}
```

This helper intentionally accepts an empty string as UTC so callers can use the default when config has already applied defaults.

- [x] **Step 5: Run domain tests and verify pass**

Run:

```bash
go test ./internal/domain/group
```

Expected: PASS.

- [x] **Step 6: Commit domain changes**

```bash
git add internal/domain/group/group.go internal/domain/group/validation.go internal/domain/group/validation_test.go
git commit -m "feat: add group expiry domain models"
```

---

### Task 2: Group-Service Config for NATS and Expiry Commands

**Files:**
- Modify: `internal/group-service/config/config.go`
- Modify: `internal/group-service/config/config_test.go`
- Modify: `.env.example`

- [x] **Step 1: Add failing config tests**

In `internal/group-service/config/config_test.go`, update the existing happy-path setup helper or test setup to set:

```go
t.Setenv("GROUP_SERVICE_NATS_URL", "nats://example:4222")
t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM", "GROUP_EXPIRY")
t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE", "group-service-expiry")
t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT", "app.todo.group.expiry.process")
t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT", "25")
t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT", "7s")
t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE", "UTC+8")
```

Add assertions to the happy-path config test:

```go
if cfg.NATS.URL != "nats://example:4222" {
	t.Fatalf("NATS.URL = %q, want nats://example:4222", cfg.NATS.URL)
}
if cfg.GroupExpiryCommand.Stream != "GROUP_EXPIRY" {
	t.Fatalf("GroupExpiryCommand.Stream = %q, want GROUP_EXPIRY", cfg.GroupExpiryCommand.Stream)
}
if cfg.GroupExpiryCommand.Durable != "group-service-expiry" {
	t.Fatalf("GroupExpiryCommand.Durable = %q, want group-service-expiry", cfg.GroupExpiryCommand.Durable)
}
if cfg.GroupExpiryCommand.Subject != "app.todo.group.expiry.process" {
	t.Fatalf("GroupExpiryCommand.Subject = %q, want app.todo.group.expiry.process", cfg.GroupExpiryCommand.Subject)
}
if cfg.GroupExpiryCommand.FetchCount != 25 {
	t.Fatalf("GroupExpiryCommand.FetchCount = %d, want 25", cfg.GroupExpiryCommand.FetchCount)
}
if cfg.GroupExpiryCommand.MaxWait != 7*time.Second {
	t.Fatalf("GroupExpiryCommand.MaxWait = %s, want 7s", cfg.GroupExpiryCommand.MaxWait)
}
if cfg.GroupExpiryCommand.BucketTimezone != "UTC+8" {
	t.Fatalf("GroupExpiryCommand.BucketTimezone = %q, want UTC+8", cfg.GroupExpiryCommand.BucketTimezone)
}
if cfg.GroupExpiryCommand.BucketLocation == nil || cfg.GroupExpiryCommand.BucketLocation.String() != "UTC+08:00" {
	t.Fatalf("BucketLocation = %v, want UTC+08:00", cfg.GroupExpiryCommand.BucketLocation)
}
```

Add defaults test assertions:

```go
if cfg.GroupExpiryCommand.FetchCount != 20 {
	t.Fatalf("GroupExpiryCommand.FetchCount = %d, want 20", cfg.GroupExpiryCommand.FetchCount)
}
if cfg.GroupExpiryCommand.MaxWait != 5*time.Second {
	t.Fatalf("GroupExpiryCommand.MaxWait = %s, want 5s", cfg.GroupExpiryCommand.MaxWait)
}
if cfg.GroupExpiryCommand.BucketTimezone != "UTC" {
	t.Fatalf("GroupExpiryCommand.BucketTimezone = %q, want UTC", cfg.GroupExpiryCommand.BucketTimezone)
}
```

Add validation tests:

```go
func TestLoadRejectsMissingGroupExpiryCommandConfig(t *testing.T) {
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("GROUP_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM", "GROUP_EXPIRY")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE", "group-service-expiry")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT", " ")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT is required") {
		t.Fatalf("Load() error = %v, want missing subject", err)
	}
}

func TestLoadRejectsInvalidGroupExpiryBucketTimezone(t *testing.T) {
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("GROUP_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM", "GROUP_EXPIRY")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE", "group-service-expiry")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT", "app.todo.group.expiry.process")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE", "Asia/Taipei")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE") {
		t.Fatalf("Load() error = %v, want invalid timezone", err)
	}
}
```

Ensure the test file imports `strings` and `time`.

- [x] **Step 2: Run config tests and verify failure**

Run:

```bash
go test ./internal/group-service/config
```

Expected: FAIL because `NATS`, `GroupExpiryCommand`, and related config fields are undefined.

- [x] **Step 3: Add config structs and loading**

In `internal/group-service/config/config.go`, add the domain import:

```go
import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)
```

Update `Config`:

```go
type Config struct {
	Environment        environment.Environment
	HTTPAddr           string
	MongoDB            MongoDBConfig
	NATS               NATSConfig
	Validation         ValidationConfig
	GroupExpiryCommand GroupExpiryCommandConfig
	ShutdownTimeout    time.Duration
}

type NATSConfig struct {
	URL string
}

type GroupExpiryCommandConfig struct {
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
v.SetDefault("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT", 20)
v.SetDefault("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT", "5s")
v.SetDefault("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE", "UTC")
```

Parse the bucket location before building `cfg`:

```go
bucketTimezone := v.GetString("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE")
bucketLocation, err := group.ParseExpirationBucketLocation(bucketTimezone)
if err != nil {
	return Config{}, err
}
```

Add config fields:

```go
NATS: NATSConfig{
	URL: v.GetString("GROUP_SERVICE_NATS_URL"),
},
GroupExpiryCommand: GroupExpiryCommandConfig{
	Stream:         v.GetString("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM"),
	Durable:        v.GetString("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE"),
	Subject:        v.GetString("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT"),
	FetchCount:     v.GetInt("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT"),
	MaxWait:        v.GetDuration("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT"),
	BucketTimezone: bucketTimezone,
	BucketLocation: bucketLocation,
},
```

Add required keys to `Validate`:

```go
required := map[string]string{
	"GROUP_SERVICE_HTTP_ADDR":                           c.HTTPAddr,
	"GROUP_SERVICE_MONGODB_URI":                         c.MongoDB.URI,
	"GROUP_SERVICE_MONGODB_DATABASE":                    c.MongoDB.Database,
	"GROUP_SERVICE_NATS_URL":                            c.NATS.URL,
	"GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM":         c.GroupExpiryCommand.Stream,
	"GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE":        c.GroupExpiryCommand.Durable,
	"GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT":        c.GroupExpiryCommand.Subject,
	"GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE":        c.GroupExpiryCommand.BucketTimezone,
}
```

Add numeric validation:

```go
if c.GroupExpiryCommand.FetchCount <= 0 {
	return fmt.Errorf("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT must be greater than zero")
}
if c.GroupExpiryCommand.MaxWait <= 0 {
	return fmt.Errorf("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT must be positive")
}
if c.GroupExpiryCommand.BucketLocation == nil {
	return fmt.Errorf("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE must be valid")
}
```

- [x] **Step 4: Update `.env.example`**

Add under the group-service MongoDB or validation settings:

```env
# Group service NATS JetStream
GROUP_SERVICE_NATS_URL=nats://localhost:4222
GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM=GROUP_EXPIRY
GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE=group-service-expiry
GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT=app.todo.group.expiry.process
GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT=20
GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT=5s
GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE=UTC
```

- [x] **Step 5: Run config tests and verify pass**

Run:

```bash
go test ./internal/group-service/config
```

Expected: PASS.

- [x] **Step 6: Commit config changes**

```bash
git add internal/group-service/config/config.go internal/group-service/config/config_test.go .env.example
git commit -m "feat: add group expiry command config"
```

---

### Task 3: CloudEvent Transport Parser

**Files:**
- Create: `internal/group-service/transport/group_expiry_event.go`
- Create: `internal/group-service/transport/group_expiry_event_test.go`

- [x] **Step 1: Write failing CloudEvent parser tests**

Create `internal/group-service/transport/group_expiry_event_test.go`:

```go
package transport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func TestParseGroupExpiryCommandEvent(t *testing.T) {
	t.Parallel()

	event := newGroupExpiryCommandEvent(t)
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	input, err := ParseGroupExpiryCommandEvent(data, "app.todo.group.expiry.process")
	if err != nil {
		t.Fatalf("ParseGroupExpiryCommandEvent() error = %v, want nil", err)
	}
	if input.TaskID != "task-1" || input.WorkspaceID != "workspace-1" || input.GroupID != "group-1" || input.ExpirationBucket != "2026-05-10" {
		t.Fatalf("input = %+v", input)
	}
}

func TestParseGroupExpiryCommandEventRejectsInvalidEnvelope(t *testing.T) {
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
			name: "empty workspace",
			mutate: func(event *cloudevents.Event) {
				mustSetGroupExpiryData(t, event, groupExpiryCommandData{
					TaskID:           "task-1",
					WorkspaceID:      " ",
					GroupID:          "group-1",
					ExpirationBucket: "2026-05-10",
				})
			},
			wantErr: "empty required field",
		},
		{
			name: "invalid bucket",
			mutate: func(event *cloudevents.Event) {
				mustSetGroupExpiryData(t, event, groupExpiryCommandData{
					TaskID:           "task-1",
					WorkspaceID:      "workspace-1",
					GroupID:          "group-1",
					ExpirationBucket: "2026/05/10",
				})
			},
			wantErr: "expiration_bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := newGroupExpiryCommandEvent(t)
			tt.mutate(&event)
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ParseGroupExpiryCommandEvent(data, "app.todo.group.expiry.process")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseGroupExpiryCommandEvent() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseGroupExpiryCommandEventRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseGroupExpiryCommandEvent([]byte("{"), "app.todo.group.expiry.process")
	if err == nil || !strings.Contains(err.Error(), "parse cloudevent") {
		t.Fatalf("ParseGroupExpiryCommandEvent() error = %v, want parse error", err)
	}
}

func newGroupExpiryCommandEvent(t *testing.T) cloudevents.Event {
	t.Helper()

	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudEventSpecVersion)
	event.SetType("app.todo.group.expiry.process")
	event.SetSource("group-expiry-scheduler")
	event.SetSubject("task-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))
	mustSetGroupExpiryData(t, &event, groupExpiryCommandData{
		TaskID:           "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-10",
	})
	return event
}

func mustSetGroupExpiryData(t *testing.T, event *cloudevents.Event, payload groupExpiryCommandData) {
	t.Helper()

	if err := event.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		t.Fatal(err)
	}
}
```

- [x] **Step 2: Run transport tests and verify failure**

Run:

```bash
go test ./internal/group-service/transport
```

Expected: FAIL because `ParseGroupExpiryCommandEvent` and `groupExpiryCommandData` are undefined.

- [x] **Step 3: Implement CloudEvent parser**

Create `internal/group-service/transport/group_expiry_event.go`:

```go
package transport

import (
	"encoding/json"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type groupExpiryCommandData struct {
	TaskID           string `json:"task_id"`
	WorkspaceID      string `json:"workspace_id"`
	GroupID          string `json:"group_id"`
	ExpirationBucket string `json:"expiration_bucket"`
}

func ParseGroupExpiryCommandEvent(data []byte, expectedType string) (group.ExpireGroupingRuleCommand, error) {
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("parse cloudevent: %w", err)
	}
	if err := event.Validate(); err != nil {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("validate cloudevent: %w", err)
	}
	if event.SpecVersion() != cloudEventSpecVersion {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("unsupported cloudevent specversion %q", event.SpecVersion())
	}
	if event.Type() != expectedType {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("cloudevent type %q does not match expected %q", event.Type(), expectedType)
	}
	if event.DataContentType() != "application/json" {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("cloudevent datacontenttype must be application/json")
	}
	if event.Time().IsZero() {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("cloudevent time is required")
	}

	var payload groupExpiryCommandData
	if err := event.DataAs(&payload); err != nil {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("parse cloudevent data: %w", err)
	}
	if event.Subject() != payload.TaskID {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("cloudevent subject must match data.task_id")
	}
	if strings.TrimSpace(payload.TaskID) == "" ||
		strings.TrimSpace(payload.WorkspaceID) == "" ||
		strings.TrimSpace(payload.GroupID) == "" ||
		strings.TrimSpace(payload.ExpirationBucket) == "" {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("group expiry command data contains empty required field")
	}
	command := group.ExpireGroupingRuleCommand{
		TaskID:           payload.TaskID,
		WorkspaceID:      payload.WorkspaceID,
		GroupID:          payload.GroupID,
		ExpirationBucket: payload.ExpirationBucket,
	}.Normalize()
	if !group.IsValidExpirationBucket(command.ExpirationBucket) {
		return group.ExpireGroupingRuleCommand{}, fmt.Errorf("expiration_bucket must use yyyy-MM-dd")
	}
	return command, nil
}
```

- [x] **Step 4: Run transport tests and verify pass**

Run:

```bash
go test ./internal/group-service/transport
```

Expected: PASS.

- [x] **Step 5: Commit transport changes**

```bash
git add internal/group-service/transport/group_expiry_event.go internal/group-service/transport/group_expiry_event_test.go
git commit -m "feat: parse group expiry command events"
```

---

### Task 4: JetStream Command Handler

**Files:**
- Create: `internal/group-service/handlers/group_expiry_event_handler.go`
- Create: `internal/group-service/handlers/group_expiry_event_handler_test.go`

- [x] **Step 1: Write failing handler tests**

Create `internal/group-service/handlers/group_expiry_event_handler_test.go`:

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

func TestGroupExpiryEventHandlerHandle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		message    eventbus.Message
		service    *fakeGroupExpiryService
		wantResult eventbus.HandleResult
		wantCalls  int
	}{
		{
			name:       "invalid event terminates",
			message:    eventbus.Message{Subject: "app.todo.group.expiry.process", Data: []byte("{")},
			service:    &fakeGroupExpiryService{},
			wantResult: eventbus.HandleResultTerminate,
		},
		{
			name:       "service invalid input terminates",
			message:    validGroupExpiryMessage(t),
			service:    &fakeGroupExpiryService{err: group.ErrInvalidInput},
			wantResult: eventbus.HandleResultTerminate,
			wantCalls:  1,
		},
		{
			name:       "service failure retries",
			message:    validGroupExpiryMessage(t),
			service:    &fakeGroupExpiryService{err: errors.New("database unavailable")},
			wantResult: eventbus.HandleResultRetry,
			wantCalls:  1,
		},
		{
			name:       "expired status acks",
			message:    validGroupExpiryMessage(t),
			service:    &fakeGroupExpiryService{status: group.ExpireGroupingRuleStatusExpired},
			wantResult: eventbus.HandleResultAck,
			wantCalls:  1,
		},
		{
			name:       "stale task status acks",
			message:    validGroupExpiryMessage(t),
			service:    &fakeGroupExpiryService{status: group.ExpireGroupingRuleStatusStaleTask},
			wantResult: eventbus.HandleResultAck,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewGroupExpiryEventHandler(tt.service, "app.todo.group.expiry.process", testLogger())
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

type fakeGroupExpiryService struct {
	status group.ExpireGroupingRuleStatus
	err    error
	calls  int
	input  group.ExpireGroupingRuleCommand
}

func (s *fakeGroupExpiryService) ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand) (group.ExpireGroupingRuleStatus, error) {
	s.calls++
	s.input = input
	if s.status == "" {
		s.status = group.ExpireGroupingRuleStatusExpired
	}
	return s.status, s.err
}

func validGroupExpiryMessage(t *testing.T) eventbus.Message {
	t.Helper()

	event := cloudevents.NewEvent()
	event.SetSpecVersion("1.0")
	event.SetType("app.todo.group.expiry.process")
	event.SetSource("group-expiry-scheduler")
	event.SetSubject("task-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))
	if err := event.SetData(cloudevents.ApplicationJSON, map[string]string{
		"task_id":           "task-1",
		"workspace_id":      "workspace-1",
		"group_id":          "group-1",
		"expiration_bucket": "2026-05-10",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return eventbus.Message{Subject: "app.todo.group.expiry.process", Data: data}
}
```

- [x] **Step 2: Run handler tests and verify failure**

Run:

```bash
go test ./internal/group-service/handlers
```

Expected: FAIL because `NewGroupExpiryEventHandler` is undefined.

- [x] **Step 3: Implement command handler**

Create `internal/group-service/handlers/group_expiry_event_handler.go`:

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

type GroupExpiryService interface {
	ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand) (group.ExpireGroupingRuleStatus, error)
}

type GroupExpiryEventHandler struct {
	service      GroupExpiryService
	expectedType string
	logger       *slog.Logger
}

func NewGroupExpiryEventHandler(service GroupExpiryService, expectedType string, logger *slog.Logger) *GroupExpiryEventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &GroupExpiryEventHandler{service: service, expectedType: expectedType, logger: logger}
}

func (h *GroupExpiryEventHandler) Handle(ctx context.Context, msg eventbus.Message) eventbus.HandleResult {
	input, err := transport.ParseGroupExpiryCommandEvent(msg.Data, h.expectedType)
	if err != nil {
		h.logger.Warn("terminating invalid group expiry command", "err", err, "subject", msg.Subject)
		return eventbus.HandleResultTerminate
	}

	status, err := h.service.ExpireGroupingRule(ctx, input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			h.logger.Warn("terminating invalid group expiry command input",
				"err", err,
				"task_id", input.TaskID,
				"workspace_id", input.WorkspaceID,
				"group_id", input.GroupID,
				"expiration_bucket", input.ExpirationBucket,
			)
			return eventbus.HandleResultTerminate
		}
		h.logger.Warn("retrying group expiry command",
			"err", err,
			"task_id", input.TaskID,
			"workspace_id", input.WorkspaceID,
			"group_id", input.GroupID,
			"expiration_bucket", input.ExpirationBucket,
		)
		return eventbus.HandleResultRetry
	}

	h.logger.Info("handled group expiry command",
		"task_id", input.TaskID,
		"workspace_id", input.WorkspaceID,
		"group_id", input.GroupID,
		"expiration_bucket", input.ExpirationBucket,
		"status", status,
	)
	return eventbus.HandleResultAck
}
```

- [x] **Step 4: Run handler tests and verify pass**

Run:

```bash
go test ./internal/group-service/handlers
```

Expected: PASS.

- [x] **Step 5: Commit handler changes**

```bash
git add internal/group-service/handlers/group_expiry_event_handler.go internal/group-service/handlers/group_expiry_event_handler_test.go
git commit -m "feat: handle group expiry commands"
```

---

### Task 5: Group Service Expiry Workflows

**Files:**
- Modify: `internal/group-service/services/group_service.go`
- Modify: `internal/group-service/services/group_service_test.go`

- [x] **Step 1: Add failing service tests for create task generation**

In `internal/group-service/services/group_service_test.go`, add or update fake repository fields:

```go
expireStatus group.ExpireGroupingRuleStatus
expireErr    error
expiredAt    time.Time
expireInput  group.ExpireGroupingRuleCommand
expireLocation *time.Location
```

Add repository method:

```go
func (r *fakeGroupRepository) ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireGroupingRuleStatus, error) {
	r.expireInput = input
	r.expiredAt = expiredAt
	r.expireLocation = bucketLocation
	if r.expireStatus == "" {
		r.expireStatus = group.ExpireGroupingRuleStatusExpired
	}
	return r.expireStatus, r.expireErr
}
```

Add test:

```go
func TestGroupServiceCreateGroupCreatesExpiryTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 16, 30, 0, 0, time.UTC)
	location, err := group.ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}
	service := NewGroupService(repository,
		WithGroupIDGenerator(sequenceGenerator("group-1", "task-1")),
		WithGroupClock(func() time.Time { return now }),
		WithGroupExpiryBucketLocation(location),
	)

	_, err = service.CreateGroup(context.Background(), group.CreateInput{
		WorkspaceID: "workspace-1",
		Name:        "Reviewers",
		GroupingRule: group.GroupingRule{
			Rules:          []group.Rule{{AttributeKey: "department", Operator: group.OperatorEq, Value: "ABCD-123"}},
			ExpirationDate: now.Add(24 * time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	if repository.createdGroup.ExpiryTask == nil {
		t.Fatal("created group expiry task is nil")
	}
	if repository.createdGroup.ExpiryTask.ID != "task-1" {
		t.Fatalf("task id = %q, want task-1", repository.createdGroup.ExpiryTask.ID)
	}
	if repository.createdGroup.ExpiryTask.ExpirationBucket != "2026-05-12" {
		t.Fatalf("expiration bucket = %q, want 2026-05-12", repository.createdGroup.ExpiryTask.ExpirationBucket)
	}
	if repository.createdGroup.GroupingRule.ExpiredAt != nil {
		t.Fatalf("expired_at = %v, want nil", repository.createdGroup.GroupingRule.ExpiredAt)
	}
}
```

Add this helper if the test file does not already have an equivalent:

```go
func sequenceGenerator(values ...string) func() string {
	index := 0
	return func() string {
		if index >= len(values) {
			return values[len(values)-1]
		}
		value := values[index]
		index++
		return value
	}
}
```

- [x] **Step 2: Add failing service tests for update and command workflow**

Add tests:

```go
func TestGroupServiceUpdateGroupingRuleCreatesExpiryTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	service := NewGroupService(repository,
		WithGroupIDGenerator(sequenceGenerator("task-1")),
		WithGroupClock(func() time.Time { return now }),
	)

	err := service.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		Rules:          []group.Rule{{AttributeKey: "department", Operator: group.OperatorEq, Value: "ABCD-123"}},
		ExpirationDate: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("UpdateGroupingRule() error = %v", err)
	}
	if repository.updatedGroupingRule.ExpiryTask == nil {
		t.Fatal("updated grouping rule expiry task is nil")
	}
	if repository.updatedGroupingRule.ExpiryTask.ID != "task-1" {
		t.Fatalf("task id = %q, want task-1", repository.updatedGroupingRule.ExpiryTask.ID)
	}
}

func TestGroupServiceUpdateGroupingRuleWithoutRulesClearsExpiryTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	service := NewGroupService(repository, WithGroupClock(func() time.Time { return now }))

	err := service.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		Rules:          []group.Rule{},
		ExpirationDate: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("UpdateGroupingRule() error = %v", err)
	}
	if repository.updatedGroupingRule.ExpiryTask != nil {
		t.Fatalf("expiry task = %+v, want nil", repository.updatedGroupingRule.ExpiryTask)
	}
}

func TestGroupServiceExpireGroupingRule(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{expireStatus: group.ExpireGroupingRuleStatusStaleTask}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	location, err := group.ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}
	service := NewGroupService(repository,
		WithGroupClock(func() time.Time { return now }),
		WithGroupExpiryBucketLocation(location),
	)

	status, err := service.ExpireGroupingRule(context.Background(), group.ExpireGroupingRuleCommand{
		TaskID:           "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-10",
	})
	if err != nil {
		t.Fatalf("ExpireGroupingRule() error = %v", err)
	}
	if status != group.ExpireGroupingRuleStatusStaleTask {
		t.Fatalf("status = %s, want stale_task", status)
	}
	if !repository.expiredAt.Equal(now) {
		t.Fatalf("expiredAt = %s, want %s", repository.expiredAt, now)
	}
	if repository.expireLocation.String() != "UTC+08:00" {
		t.Fatalf("location = %s, want UTC+08:00", repository.expireLocation)
	}
}
```

- [x] **Step 3: Run service tests and verify failure**

Run:

```bash
go test ./internal/group-service/services
```

Expected: FAIL because service types and repository interface do not include expiry task behavior.

- [x] **Step 4: Implement service changes**

In `internal/group-service/services/group_service.go`, update the repository interface:

```go
type GroupRepository interface {
	Create(ctx context.Context, input group.Group) (group.Group, error)
	Get(ctx context.Context, query group.GetQuery) (*group.Group, error)
	Delete(ctx context.Context, input group.DeleteInput, deletedAt time.Time) error
	UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput, updatedAt time.Time) error
	ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error)
	AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error)
	UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput, updatedAt time.Time) error
	DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput, deletedAt time.Time) error
	ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireGroupingRuleStatus, error)
}
```

Add service option and field:

```go
func WithGroupExpiryBucketLocation(location *time.Location) GroupOption {
	return func(s *GroupService) {
		if location != nil {
			s.expiryBucketLocation = location
		}
	}
}
```

```go
type GroupService struct {
	repository           GroupRepository
	idGenerator          func() string
	now                  func() time.Time
	expiryBucketLocation *time.Location
	validateOptions      []group.ValidateOption
}
```

Set default in `NewGroupService`:

```go
expiryBucketLocation: time.UTC,
```

Add helper:

```go
func (s *GroupService) newExpiryTask(workspaceID string, groupID string, groupingRule group.GroupingRule) *group.ExpiryTask {
	if len(groupingRule.Rules) == 0 {
		return nil
	}
	return &group.ExpiryTask{
		ID:               s.idGenerator(),
		WorkspaceID:      workspaceID,
		GroupID:          groupID,
		ExpirationBucket: group.ExpirationBucketFor(groupingRule.ExpirationDate, s.expiryBucketLocation),
	}
}
```

In `CreateGroup`, after building `model.IndividualMembers`:

```go
model.GroupingRule.ExpiredAt = nil
model.ExpiryTask = s.newExpiryTask(model.WorkspaceID, model.ID, model.GroupingRule)
```

In `UpdateGroupingRule`, after validation:

```go
input.ExpiryTask = s.newExpiryTask(input.WorkspaceID, input.GroupID, group.GroupingRule{
	Rules:          input.Rules,
	ExpirationDate: input.ExpirationDate,
})
```

Add method:

```go
func (s *GroupService) ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand) (group.ExpireGroupingRuleStatus, error) {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return "", err
	}
	status, err := s.repository.ExpireGroupingRule(ctx, input, s.now().UTC(), s.expiryBucketLocation)
	if err != nil {
		return "", fmt.Errorf("expire grouping rule: %w", err)
	}
	return status, nil
}
```

Add `ExpiryTask *ExpiryTask` to `group.UpdateGroupingRuleInput` in `internal/domain/group/group.go` if Task 1 did not add it:

```go
type UpdateGroupingRuleInput struct {
	WorkspaceID    string
	GroupID        string
	Rules          []Rule
	ExpirationDate time.Time
	ExpiryTask     *ExpiryTask
}
```

- [x] **Step 5: Run service tests and verify pass**

Run:

```bash
go test ./internal/group-service/services
```

Expected: PASS.

- [x] **Step 6: Run domain tests after input struct change**

Run:

```bash
go test ./internal/domain/group
```

Expected: PASS.

- [x] **Step 7: Commit service changes**

```bash
git add internal/domain/group/group.go internal/group-service/services/group_service.go internal/group-service/services/group_service_test.go
git commit -m "feat: generate group expiry tasks in service"
```

---

### Task 6: Repository Schema, Indexes, and API Write-Path Task Maintenance

**Files:**
- Modify: `internal/group-service/repositories/mongo_group_repository.go`
- Modify: `internal/group-service/repositories/mongo_group_repository_test.go`

- [x] **Step 1: Add failing repository mapping and index tests**

In `internal/group-service/repositories/mongo_group_repository_test.go`, add tests near existing mapping/index tests:

```go
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

func TestGroupExpiryTaskIndexModels(t *testing.T) {
	t.Parallel()

	models := groupExpiryTaskIndexModels()
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
}
```

Add repository behavior tests using existing repository test helpers:

```go
func TestMongoGroupRepositoryCreateWritesExpiryTask(t *testing.T) {
	t.Parallel()

	repository, cleanup := newTestMongoGroupRepository(t)
	defer cleanup()

	now := repositoryTime()
	_, err := repository.Create(context.Background(), group.Group{
		ID:             "group-1",
		WorkspaceID:    "workspace-1",
		Name:           "Reviewers",
		NormalizedName: "Reviewers",
		GroupingRule: group.GroupingRule{
			Rules:          []group.Rule{{AttributeKey: "department", Operator: group.OperatorEq, Value: "ABCD-123"}},
			ExpirationDate: now.Add(24 * time.Hour),
		},
		ExpiryTask: &group.ExpiryTask{
			ID:               "task-1",
			WorkspaceID:      "workspace-1",
			GroupID:          "group-1",
			ExpirationBucket: "2026-05-11",
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var task expiryTaskDocument
	err = repository.expiryTasks.FindOne(context.Background(), bson.M{"_id": "task-1"}).Decode(&task)
	if err != nil {
		t.Fatalf("find expiry task: %v", err)
	}
	if task.ExpirationBucket != "2026-05-11" {
		t.Fatalf("expiration bucket = %q, want 2026-05-11", task.ExpirationBucket)
	}
}

func TestMongoGroupRepositoryUpdateGroupingRuleReplacesExpiryTask(t *testing.T) {
	t.Parallel()

	repository, cleanup := newTestMongoGroupRepository(t)
	defer cleanup()

	ctx := context.Background()
	now := repositoryTime()
	_, err := repository.groups.InsertOne(ctx, groupDocument{
		ID:             "group-1",
		WorkspaceID:    "workspace-1",
		Name:           "Reviewers",
		NormalizedName: "Reviewers",
		GroupingRule: groupingRuleDocument{
			Rules:          []ruleDocument{{AttributeKey: "department", Operator: group.OperatorEq, Value: "OLD"}},
			ExpirationDate: now.Add(24 * time.Hour),
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.expiryTasks.InsertOne(ctx, expiryTaskDocument{
		ID:               "task-old",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-11",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = repository.UpdateGroupingRule(ctx, group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		Rules:          []group.Rule{{AttributeKey: "department", Operator: group.OperatorEq, Value: "NEW"}},
		ExpirationDate: now.Add(48 * time.Hour),
		ExpiryTask: &group.ExpiryTask{
			ID:               "task-new",
			WorkspaceID:      "workspace-1",
			GroupID:          "group-1",
			ExpirationBucket: "2026-05-12",
		},
	}, now)
	if err != nil {
		t.Fatalf("UpdateGroupingRule() error = %v", err)
	}

	oldCount, err := repository.expiryTasks.CountDocuments(ctx, bson.M{"_id": "task-old"})
	if err != nil {
		t.Fatal(err)
	}
	if oldCount != 0 {
		t.Fatalf("old task count = %d, want 0", oldCount)
	}
	newCount, err := repository.expiryTasks.CountDocuments(ctx, bson.M{"_id": "task-new"})
	if err != nil {
		t.Fatal(err)
	}
	if newCount != 1 {
		t.Fatalf("new task count = %d, want 1", newCount)
	}
}
```

If repository integration helpers are gated behind environment variables, keep the new tests under the same gating pattern as existing MongoDB transaction tests.

- [x] **Step 2: Run repository tests and verify failure**

Run:

```bash
go test ./internal/group-service/repositories
```

Expected: FAIL because expiry task document and collection fields are undefined.

- [x] **Step 3: Add repository schema and collection**

In `internal/group-service/repositories/mongo_group_repository.go`, add constants:

```go
groupExpiryTaskCollectionName        = "group_expiry_task"
expiryTasksActiveGroupUniqueIndexName = "group_expiry_task_active_workspace_group_unique"
expiryTasksBucketIndexName            = "group_expiry_task_bucket_id"
```

Update repository struct and constructor:

```go
type MongoGroupRepository struct {
	client      *mongo.Client
	groups      *mongo.Collection
	members     *mongo.Collection
	expiryTasks *mongo.Collection
}
```

```go
return &MongoGroupRepository{
	client:      client,
	groups:      db.Collection(groupCollectionName),
	members:     db.Collection(groupIndividualMemberCollectionName),
	expiryTasks: db.Collection(groupExpiryTaskCollectionName),
}
```

Update grouping rule document and add task document:

```go
type groupingRuleDocument struct {
	Rules          []ruleDocument `bson:"rules"`
	ExpirationDate time.Time      `bson:"expiration_date"`
	ExpiredAt      *time.Time     `bson:"expired_at"`
}

type expiryTaskDocument struct {
	ID               string `bson:"_id"`
	WorkspaceID      string `bson:"workspace_id"`
	GroupID          string `bson:"group_id"`
	ExpirationBucket string `bson:"expiration_bucket"`
}
```

Update `EnsureIndexes`:

```go
if _, err := r.expiryTasks.Indexes().CreateMany(ctx, groupExpiryTaskIndexModels()); err != nil {
	return fmt.Errorf("create group expiry task indexes: %w", err)
}
```

Add index models:

```go
func groupExpiryTaskIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "workspace_id", Value: 1},
				{Key: "group_id", Value: 1},
			},
			Options: options.Index().
				SetName(expiryTasksActiveGroupUniqueIndexName).
				SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "expiration_bucket", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName(expiryTasksBucketIndexName),
		},
	}
}
```

Update mapping:

```go
func newGroupingRuleDocument(rule group.GroupingRule) groupingRuleDocument {
	rules := make([]ruleDocument, 0, len(rule.Rules))
	for _, item := range rule.Rules {
		rules = append(rules, ruleDocument{
			AttributeKey: item.AttributeKey,
			Operator:     item.Operator,
			Multi:        item.Multi,
			Value:        item.Value,
		})
	}
	return groupingRuleDocument{Rules: rules, ExpirationDate: rule.ExpirationDate, ExpiredAt: rule.ExpiredAt}
}

func newExpiryTaskDocument(task group.ExpiryTask) expiryTaskDocument {
	return expiryTaskDocument{
		ID:               task.ID,
		WorkspaceID:      task.WorkspaceID,
		GroupID:          task.GroupID,
		ExpirationBucket: task.ExpirationBucket,
	}
}
```

Update `groupDocument.toDomain` to set `ExpiredAt: d.GroupingRule.ExpiredAt`.

- [x] **Step 4: Maintain tasks in create, update, and delete transactions**

In `Create`, after member inserts:

```go
if input.ExpiryTask != nil {
	if _, err := r.expiryTasks.InsertOne(sessionCtx, newExpiryTaskDocument(*input.ExpiryTask)); err != nil {
		return nil, fmt.Errorf("insert group expiry task: %w", err)
	}
}
```

In `Delete`, after member soft-delete when the group matched:

```go
if _, deleteTaskErr := r.expiryTasks.DeleteMany(sessionCtx, bson.M{
	"workspace_id": input.WorkspaceID,
	"group_id":    input.GroupID,
}); deleteTaskErr != nil {
	return nil, fmt.Errorf("delete group expiry tasks: %w", deleteTaskErr)
}
```

In `UpdateGroupingRule`, include `expired_at: nil` in the group update:

```go
"grouping_rule": newGroupingRuleDocument(group.GroupingRule{Rules: input.Rules, ExpirationDate: input.ExpirationDate}),
"updated_at":    updatedAt,
```

The new `newGroupingRuleDocument` call produces `ExpiredAt: nil`.

After the empty-rules active member check passes, replace the task:

```go
if _, deleteTaskErr := r.expiryTasks.DeleteMany(sessionCtx, bson.M{
	"workspace_id": input.WorkspaceID,
	"group_id":    input.GroupID,
}); deleteTaskErr != nil {
	return nil, fmt.Errorf("delete group expiry tasks: %w", deleteTaskErr)
}
if input.ExpiryTask != nil {
	if _, insertTaskErr := r.expiryTasks.InsertOne(sessionCtx, newExpiryTaskDocument(*input.ExpiryTask)); insertTaskErr != nil {
		return nil, fmt.Errorf("insert group expiry task: %w", insertTaskErr)
	}
}
```

- [x] **Step 5: Run repository tests and verify pass**

Run:

```bash
go test ./internal/group-service/repositories
```

Expected: PASS. If local MongoDB is unavailable and existing tests skip integration cases, confirm the new integration cases follow the same skip behavior.

- [x] **Step 6: Commit repository write-path changes**

```bash
git add internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_group_repository_test.go
git commit -m "feat: persist group expiry tasks"
```

---

### Task 7: Repository Expire Command Transaction

**Files:**
- Modify: `internal/group-service/repositories/mongo_group_repository.go`
- Modify: `internal/group-service/repositories/mongo_group_repository_test.go`

- [x] **Step 1: Add failing command transaction tests**

In `internal/group-service/repositories/mongo_group_repository_test.go`, add tests using the existing Mongo test helper:

```go
func TestMongoGroupRepositoryExpireGroupingRuleExpiresAndDeletesTask(t *testing.T) {
	t.Parallel()

	repository, cleanup := newTestMongoGroupRepository(t)
	defer cleanup()

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
	if err := repository.groups.FindOne(ctx, bson.M{"_id": "group-1"}).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.GroupingRule.ExpiredAt == nil || !doc.GroupingRule.ExpiredAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expired_at = %v, want %s", doc.GroupingRule.ExpiredAt, now.Add(time.Hour))
	}
	taskCount, err := repository.expiryTasks.CountDocuments(ctx, bson.M{"_id": "task-1"})
	if err != nil {
		t.Fatal(err)
	}
	if taskCount != 0 {
		t.Fatalf("task count = %d, want 0", taskCount)
	}
}

func TestMongoGroupRepositoryExpireGroupingRuleStaleCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T, repository *MongoGroupRepository)
		command   group.ExpireGroupingRuleCommand
		want      group.ExpireGroupingRuleStatus
		wantTasks int64
	}{
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, cleanup := newTestMongoGroupRepository(t)
			defer cleanup()
			if tt.setup != nil {
				tt.setup(t, repository)
			}
			location, err := group.ParseExpirationBucketLocation("UTC+8")
			if err != nil {
				t.Fatal(err)
			}

			status, err := repository.ExpireGroupingRule(context.Background(), tt.command, repositoryTime(), location)
			if err != nil {
				t.Fatalf("ExpireGroupingRule() error = %v", err)
			}
			if status != tt.want {
				t.Fatalf("status = %s, want %s", status, tt.want)
			}
			if tt.wantTasks > 0 {
				count, err := repository.expiryTasks.CountDocuments(context.Background(), bson.M{"_id": tt.command.TaskID})
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

Add helpers:

```go
func insertGroupWithTask(t *testing.T, repository *MongoGroupRepository, groupDoc groupDocument, taskDoc expiryTaskDocument) {
	t.Helper()
	if _, err := repository.groups.InsertOne(context.Background(), groupDoc); err != nil {
		t.Fatal(err)
	}
	insertExpiryTask(t, repository, taskDoc)
}

func insertExpiryTask(t *testing.T, repository *MongoGroupRepository, taskDoc expiryTaskDocument) {
	t.Helper()
	if _, err := repository.expiryTasks.InsertOne(context.Background(), taskDoc); err != nil {
		t.Fatal(err)
	}
}
```

- [x] **Step 2: Run repository tests and verify failure**

Run:

```bash
go test ./internal/group-service/repositories
```

Expected: FAIL because `ExpireGroupingRule` is undefined or incomplete.

- [x] **Step 3: Implement repository command transaction**

In `internal/group-service/repositories/mongo_group_repository.go`, add:

```go
func (r *MongoGroupRepository) ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireGroupingRuleStatus, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return "", fmt.Errorf("start grouping rule expiry session: %w", err)
	}
	defer session.EndSession(ctx)

	var status group.ExpireGroupingRuleStatus
	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		task, taskErr := r.findExpiryTask(sessionCtx, input)
		if taskErr != nil {
			return nil, taskErr
		}
		if task == nil {
			status = group.ExpireGroupingRuleStatusStaleTask
			return nil, nil
		}

		var doc groupDocument
		err := r.groups.FindOne(sessionCtx, activeGroupFilter(group.GetQuery{
			WorkspaceID: input.WorkspaceID,
			GroupID:     input.GroupID,
		})).Decode(&doc)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				if deleteErr := r.deleteExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
					return nil, deleteErr
				}
				status = group.ExpireGroupingRuleStatusStaleGroup
				return nil, nil
			}
			return nil, fmt.Errorf("find group for expiry: %w", err)
		}

		if doc.GroupingRule.ExpiredAt != nil {
			if deleteErr := r.deleteExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
				return nil, deleteErr
			}
			status = group.ExpireGroupingRuleStatusAlreadyExpired
			return nil, nil
		}

		currentBucket := group.ExpirationBucketFor(doc.GroupingRule.ExpirationDate, bucketLocation)
		if currentBucket != input.ExpirationBucket {
			status = group.ExpireGroupingRuleStatusStaleBucket
			return nil, nil
		}

		result, updateErr := r.groups.UpdateOne(sessionCtx,
			activeGroupFilter(group.GetQuery{WorkspaceID: input.WorkspaceID, GroupID: input.GroupID}),
			bson.M{"$set": bson.M{
				"grouping_rule.expired_at": expiredAt,
				"updated_at":               expiredAt,
			}},
		)
		if updateErr != nil {
			return nil, fmt.Errorf("mark grouping rule expired: %w", updateErr)
		}
		if result.MatchedCount == 0 {
			status = group.ExpireGroupingRuleStatusStaleGroup
			return nil, nil
		}
		if deleteErr := r.deleteExpiryTaskByID(sessionCtx, input.TaskID); deleteErr != nil {
			return nil, deleteErr
		}
		status = group.ExpireGroupingRuleStatusExpired
		return nil, nil
	})
	if err != nil {
		return "", err
	}
	return status, nil
}
```

Add helpers:

```go
func (r *MongoGroupRepository) findExpiryTask(ctx context.Context, input group.ExpireGroupingRuleCommand) (*expiryTaskDocument, error) {
	var doc expiryTaskDocument
	err := r.expiryTasks.FindOne(ctx, bson.M{
		"_id":               input.TaskID,
		"workspace_id":      input.WorkspaceID,
		"group_id":          input.GroupID,
		"expiration_bucket": input.ExpirationBucket,
	}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("find group expiry task: %w", err)
	}
	return &doc, nil
}

func (r *MongoGroupRepository) deleteExpiryTaskByID(ctx context.Context, taskID string) error {
	if _, err := r.expiryTasks.DeleteOne(ctx, bson.M{"_id": taskID}); err != nil {
		return fmt.Errorf("delete group expiry task: %w", err)
	}
	return nil
}
```

- [x] **Step 4: Run repository tests and verify pass**

Run:

```bash
go test ./internal/group-service/repositories
```

Expected: PASS. If local MongoDB is unavailable and existing repository tests skip integration cases, record the skip output in the task notes.

- [x] **Step 5: Commit command transaction changes**

```bash
git add internal/group-service/repositories/mongo_group_repository.go internal/group-service/repositories/mongo_group_repository_test.go
git commit -m "feat: expire grouping rules transactionally"
```

---

### Task 8: Group-Service Runtime Wiring

**Files:**
- Modify: `cmd/group-service/main.go`
- Modify: `cmd/group-service/main_test.go`

- [x] **Step 1: Add failing main/config wiring tests**

Open `cmd/group-service/main_test.go`. If it already tests `run` failure modes by setting env vars, update required env setup with:

```go
t.Setenv("GROUP_SERVICE_NATS_URL", "nats://localhost:4222")
t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM", "GROUP_EXPIRY")
t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE", "group-service-expiry")
t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT", "app.todo.group.expiry.process")
```

If `main_test.go` only tests small helpers, add a config-loading smoke test in `internal/group-service/config/config_test.go` instead of creating brittle runtime tests. The runtime behavior is mostly covered by `internal/shared/eventbus` tests and handler tests.

- [x] **Step 2: Run group-service command tests and verify failure**

Run:

```bash
go test ./cmd/group-service ./internal/group-service/config
```

Expected: FAIL until `cmd/group-service` imports NATS/eventbus and passes the new config through service options.

- [x] **Step 3: Wire NATS, handler, and consumer**

In `cmd/group-service/main.go`, add imports:

```go
"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
"github.com/nats-io/nats.go"
```

After MongoDB repository setup, connect to NATS:

```go
nc, err := nats.Connect(cfg.NATS.URL)
if err != nil {
	return err
}
defer nc.Close()
```

When constructing `groupService`, pass the bucket location:

```go
groupService := services.NewGroupService(repository,
	services.WithGroupValidationLimits(
		cfg.Validation.MaxIndividualMembers,
		cfg.Validation.MaxGroupingRules,
	),
	services.WithGroupExpiryBucketLocation(cfg.GroupExpiryCommand.BucketLocation),
)
```

Create handler and consumer:

```go
eventHandler := handlers.NewGroupExpiryEventHandler(groupService, cfg.GroupExpiryCommand.Subject, logger)
consumer, err := eventbus.NewJetStreamConsumer(ctx, nc, eventbus.Config{
	Stream:    cfg.GroupExpiryCommand.Stream,
	Subjects:  []string{cfg.GroupExpiryCommand.Subject},
	Durable:   cfg.GroupExpiryCommand.Durable,
	BatchSize: cfg.GroupExpiryCommand.FetchCount,
	MaxWait:   cfg.GroupExpiryCommand.MaxWait,
}, eventHandler, logger)
if err != nil {
	return err
}
```

Change error channel capacity:

```go
errCh := make(chan error, 2)
```

Start the consumer next to HTTP:

```go
go func() {
	errCh <- consumer.Run(ctx)
}()
```

Keep the existing select logic that stops the context when any goroutine returns an error.

- [x] **Step 4: Run command package tests and verify pass**

Run:

```bash
go test ./cmd/group-service ./internal/group-service/config
```

Expected: PASS.

- [x] **Step 5: Commit runtime wiring**

```bash
git add cmd/group-service/main.go cmd/group-service/main_test.go internal/group-service/config/config_test.go
git commit -m "feat: wire group expiry command consumer"
```

---

### Task 9: Final Verification and Plan Completion

**Files:**
- Verify all files changed by Tasks 1-8.
- Do not stage `lefthook.yml` unless it was intentionally changed for this feature.

- [x] **Step 1: Run repository-wide tests**

Run:

```bash
go test ./...
```

Expected: PASS. If MongoDB-backed transaction tests require a local replica set and are skipped or fail because MongoDB is unavailable, record the exact output and run the largest non-integration subset that is meaningful:

```bash
go test ./internal/domain/group ./internal/group-service/config ./internal/group-service/transport ./internal/group-service/handlers ./internal/group-service/services ./internal/shared/eventbus ./cmd/group-service
```

Expected: PASS.

- [x] **Step 2: Run formatting**

Run:

```bash
gofmt -w internal/domain/group internal/group-service cmd/group-service
```

Expected: command exits 0 and produces no non-Go file changes.

- [x] **Step 3: Run tests again after formatting**

Run:

```bash
go test ./...
```

Expected: PASS or the same documented MongoDB availability skip behavior from Step 1.

- [x] **Step 4: Inspect final diff**

Run:

```bash
git diff --stat
git diff --check
```

Expected:

- `git diff --check` exits 0.
- Diff includes only group expiry command implementation files, `.env.example`, and this plan/design documentation if those were still uncommitted.
- Diff does not include unrelated `lefthook.yml` edits.

- [x] **Step 5: Commit final verification adjustments**

If formatting or final fixes changed files after the previous task commits:

```bash
git add internal/domain/group internal/group-service cmd/group-service .env.example
git commit -m "test: verify group expiry command workflow"
```

If no files changed after the previous task commits, do not create an empty commit.

---

## Self-Review Checklist

- [x] Source design coverage: plan covers `group_expiry_task`, `groups.grouping_rule.expired_at`, CloudEvent command parsing, JetStream consumer config, bucket timezone config, idempotent command handling, transaction cleanup, and tests.
- [x] Boundary coverage: domain has no Echo/NATS/MongoDB imports; service has no Echo/NATS/JetStream/MongoDB imports; transport owns CloudEvent parsing; repository owns MongoDB transactions.
- [x] Config coverage: required stream, durable, subject, NATS URL, fetch count, max wait, and bucket timezone are planned with validation and `.env.example`.
- [x] Verification coverage: each task includes targeted tests, commit steps, and final `go test ./...`.
- [x] Drift coverage: plan links back to the group-service design documents and does not change individual-member API behavior.
