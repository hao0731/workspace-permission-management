---
doc_id: design.group-expiry-scheduler
doc_type: design
title: Group expiry scheduler design
status: implemented

tags:
  - group
  - expiry
  - scheduler

code_paths:
  - cmd/group-expiry-scheduler/**
  - internal/group-expiry-scheduler/**
  - internal/shared/repositories/expiry/**

related:
  designs:
    - design.group-service
    - design.group-service-group-expiry-command
    - design.group-service-individual-member-expiry-command
  adrs: []

last_updated_at: 2026-05-30

summary: >
  Read this when changing the group-expiry-scheduler service, expiry task
  scanning, or scheduled publication of group expiry commands.
---

# Group Expiry Scheduler Design

## Background

`group-service` already owns the command handlers that expire dynamic grouping rules and explicit individual members. Those handlers consume JetStream CloudEvents and remove matching documents from `group_expiry_task` and `individual_member_expiry_task` after successful processing.

This design adds a new backend service named `group-expiry-scheduler`. The scheduler scans due expiry task collections, publishes the existing command CloudEvents to JetStream, and leaves command processing, stale-task detection, idempotency, and task deletion to `group-service`.

Related designs:

- [Group Service Design](group-service.md)
- [Group Expiry Command Design](group-service-group-expiry-command.md)
- [Group Individual Member Expiry Command Design](group-service-individual-member-expiry-command.md)

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- MongoDB task schemas, indexes, cursor pagination, NATS subjects, and CloudEvent payloads are explicit contracts.
- `group-expiry-scheduler` service logic depends on consumer-side repository and publisher interfaces, not MongoDB, NATS, JetStream, Echo, or gocron types.
- CloudEvent construction stays in the scheduler transport package.
- MongoDB task collection details move to `internal/shared/repositories/expiry`, which must not import `internal/group-service/*` or `internal/group-expiry-scheduler/*`.
- The scheduler is a separate composition root under `cmd/group-expiry-scheduler`.
- This design is stored under `docs/designs/`.

## Goals

- Add an independent `group-expiry-scheduler` service.
- Use `github.com/go-co-op/gocron/v2` to run one configurable cron-expression job.
- Scan `group_expiry_task` and `individual_member_expiry_task` in batches of 20 by default.
- Use cursor pagination over `{ expiration_bucket: 1, _id: 1 }` to scan each due task collection until empty.
- Treat due tasks as `expiration_bucket <= today_bucket`, so missed past buckets are recovered.
- Keep group and individual-member bucket timezones separately configurable.
- Publish the existing group-service expiry command CloudEvents to JetStream.
- Continue the job when a single task publish fails, and log the failure.
- Log job start, skipped overlap, finish, and job-level failure.
- Expose liveness through `internal/shared/health`.
- Move expiry task collection schema, index, and persistence helpers into `internal/shared/repositories/expiry`.
- Include local `docker-compose.yml` runtime and NATS stream initialization in implementation scope.

## Non-Goals

- Do not move expiry command handling out of `group-service`.
- Do not delete task documents from the scheduler after publish.
- Do not add immediate per-task publish retries inside one job run.
- Do not add a readiness endpoint in this phase.
- Do not design distributed leader election or a cross-pod distributed lock in this phase.
- Do not change the existing NATS subjects or CloudEvent data fields consumed by `group-service`.
- Do not add frontend changes.

## Architecture

The new service follows the backend layout:

```plaintext
cmd/group-expiry-scheduler
internal/group-expiry-scheduler/config
internal/group-expiry-scheduler/services
internal/group-expiry-scheduler/transport
internal/shared/repositories/expiry
```

Responsibilities:

- `cmd/group-expiry-scheduler`: composition root. It loads configuration, creates the logger, connects to MongoDB and NATS, creates the shared expiry repository, ensures expiry task indexes, creates a JetStream producer, registers liveness, wires gocron, starts HTTP and scheduler runtimes, and handles graceful shutdown.
- `internal/group-expiry-scheduler/config`: environment-based configuration and validation for HTTP, MongoDB, NATS, cron, timezones, batch size, publish timeout, and shutdown timeout.
- `internal/group-expiry-scheduler/services`: job workflow. It computes due buckets, scans both task collections to exhaustion, invokes a publisher interface per task, aggregates statistics, and returns job-level query failures.
- `internal/group-expiry-scheduler/transport`: CloudEvent builders for the two existing expiry command contracts.
- `internal/shared/repositories/expiry`: MongoDB documents, indexes, insert/delete/find helpers, and due-task cursor queries for `group_expiry_task` and `individual_member_expiry_task`.

`group-service` should be refactored to use `internal/shared/repositories/expiry` for expiry task collection operations. Its `MongoGroupRepository` still owns transactions that span `groups`, `group_individual_members`, and task collections; shared repository methods must accept the caller's `context.Context` and must not start their own sessions, so they participate in an existing `mongo.SessionContext` when called inside a transaction.

Rationale:

- A shared expiry repository keeps task schema, indexes, and query behavior in one place.
- Keeping transaction ownership in `group-service` preserves existing atomic group/member/task writes.
- Keeping scheduler publishing separate from group-service command handling avoids a service-private import boundary violation and keeps scheduler behavior narrow.

## Shared Expiry Repository

Package:

```txt
internal/shared/repositories/expiry
```

Collection names:

```txt
group_expiry_task
individual_member_expiry_task
```

Group task document:

```ts
{
  "_id": string,
  "workspace_id": string,
  "group_id": string,
  "expiration_bucket": string
}
```

Individual-member task document:

```ts
{
  "_id": string,
  "group_id": string,
  "nt_account": string,
  "expiration_bucket": string
}
```

Indexes:

```txt
group_expiry_task:
unique { workspace_id: 1, group_id: 1 }
{ expiration_bucket: 1, _id: 1 }

individual_member_expiry_task:
unique { group_id: 1, nt_account: 1 }
{ expiration_bucket: 1, _id: 1 }
```

The shared package should expose persistence-oriented task models, not service-specific domain types:

```go
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

Expected repository capability:

- Ensure indexes for both task collections.
- Insert group and individual-member tasks.
- Find a task by full command identity for group-service command handling.
- Delete tasks by ID, group, workspace/group, or individual member identity as needed by existing group-service workflows.
- List due group tasks by `due_bucket`, `cursor`, and `limit`.
- List due individual-member tasks by `due_bucket`, `cursor`, and `limit`.

The package may depend on the MongoDB driver because it is an infrastructure package. It must not depend on any service-specific package.

## Due Scan

Each cron trigger runs one dispatch cycle. The cycle scans group tasks first, then individual-member tasks.

Due condition:

```txt
expiration_bucket <= today_bucket
```

Sort:

```txt
expiration_bucket ASC, _id ASC
```

First batch filter:

```txt
expiration_bucket <= today_bucket
```

Next batch filter:

```txt
expiration_bucket <= today_bucket
AND (
  expiration_bucket > last_expiration_bucket
  OR (expiration_bucket == last_expiration_bucket AND _id > last_id)
)
```

The default batch size is `20`, configurable through `GROUP_EXPIRY_SCHEDULER_BATCH_SIZE`.

Workflow:

1. Compute the group task `today_bucket` from the scheduler clock and configured group expiry bucket timezone.
2. Fetch a due group task batch.
3. Publish one group expiry command CloudEvent for each fetched task.
4. Log and continue when a single publish fails.
5. Advance the cursor to the last fetched task, regardless of publish success for individual tasks.
6. Repeat until no group tasks are returned.
7. Compute the individual-member task `today_bucket` from the scheduler clock and configured individual-member expiry bucket timezone.
8. Repeat the same cursor loop for individual-member tasks.

Trade-offs:

- A task inserted behind the current cursor during a running job may not be seen until the next cron trigger. This is acceptable because the job is periodic and due scanning includes past buckets.
- A successfully published task remains in MongoDB until `group-service` consumes the command and deletes the task. If a later cron runs before the task is deleted, the scheduler may publish a duplicate command. Existing group-service command handling is idempotent and treats stale, already-expired, and missing targets safely.
- A task whose publish fails is not retried inside the same job after the cursor advances. The next cron trigger will see it again because the scheduler never deletes tasks.

## CloudEvent Contracts

The scheduler publishes to the existing group-service command subjects. The configured subject is also used as the CloudEvent `type`, matching current group-service parser behavior.

Default subjects:

```txt
app.todo.group.expiry.process
app.todo.group.individual-member.expiry.process
```

Common CloudEvent fields:

```json
{
  "specversion": "1.0",
  "type": "<configured subject>",
  "source": "group-expiry-scheduler",
  "subject": "<task_id>",
  "id": "<uuid>",
  "time": "<publish time>",
  "datacontenttype": "application/json",
  "data": {}
}
```

Group expiry data:

```json
{
  "task_id": "<task_id>",
  "workspace_id": "<workspace_id>",
  "group_id": "<group_id>",
  "expiration_bucket": "<yyyy-MM-dd>"
}
```

Individual-member expiry data:

```json
{
  "task_id": "<task_id>",
  "group_id": "<group_id>",
  "nt_account": "<nt_account>",
  "expiration_bucket": "<yyyy-MM-dd>"
}
```

Event field notes:

- `source` is always `group-expiry-scheduler` for both event types.
- `subject` is the task ID.
- `id` is generated per publish attempt.
- `time` is generated from the scheduler service clock.
- The CloudEvent data fields remain backward-compatible with existing `group-service` parsers.

## Publishing and Failure Handling

The scheduler service should depend on a publisher interface such as:

```go
type CommandPublisher interface {
    PublishGroupExpiryCommand(ctx context.Context, task expiry.GroupTask) error
    PublishIndividualMemberExpiryCommand(ctx context.Context, task expiry.IndividualMemberTask) error
}
```

The publisher implementation can live in `cmd/group-expiry-scheduler` or an internal scheduler infrastructure file and use `internal/shared/eventbus.Producer` with `eventbus.WithPublishTimeout`.

Failure behavior:

- Single task publish failure: log `Warn` with `err`, `task_type`, `task_id`, and relevant identity fields, then continue.
- CloudEvent build failure: log `Warn` and continue. This should only happen with inconsistent persisted task data or a code defect.
- MongoDB due query failure: return a job-level error and stop the current job run.
- NATS connection or producer creation failure: fail startup.
- gocron job registration failure: fail startup.

The scheduler must not delete task documents after publish. Task deletion remains part of group-service command handling.

## Scheduling

The implementation should use `github.com/go-co-op/gocron/v2`.

Relevant gocron capabilities:

- `CronJob(crontab, withSeconds)` schedules jobs with cron syntax.
- `WithLocation(location)` sets the scheduler timezone.
- `NewTask` can pass a context-aware task function so shutdown can cancel the job context.

Configuration controls:

- `GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION`
- `GROUP_EXPIRY_SCHEDULER_CRON_WITH_SECONDS`, default `false`
- `GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE`, default `UTC`

Startup validation should use `gocron.NewDefaultCron(cronWithSeconds).IsValid(...)` with the configured scheduler location and current time.

The job runner should include a non-blocking in-process overlap guard. When a cron trigger fires while the previous job is still active, the wrapper logs and returns without invoking the scheduler service:

```txt
group expiry scheduler job skipped because previous run is still active
```

Rationale:

- The explicit guard guarantees the required skip log because the task wrapper still runs on the overlapping trigger.
- The guard is intentionally in-process only. Distributed leader election or distributed locks are out of scope for this phase.
- `gocron.WithSingletonMode(gocron.LimitModeReschedule)` may still be used if the implementation can also preserve the explicit skip log through gocron listeners or monitors, but it is not required for correctness when the wrapper guard is present.

## Logging

Use `log/slog` exclusively.

Job start:

```txt
group expiry scheduler job started
```

Fields:

- `run_id`
- `group_due_bucket`
- `individual_member_due_bucket`

Task publish failure:

```txt
failed to publish group expiry command
failed to publish individual member expiry command
```

Fields:

- `err`
- `run_id`
- `task_type`
- `task_id`
- `workspace_id` for group tasks
- `group_id`
- `nt_account` for individual-member tasks
- `expiration_bucket`

Job finish:

```txt
group expiry scheduler job finished
```

Fields:

- `run_id`
- `group_scanned`
- `group_published`
- `group_failed`
- `individual_member_scanned`
- `individual_member_published`
- `individual_member_failed`
- `duration`

Job-level failure:

```txt
group expiry scheduler job failed
```

Fields:

- `err`
- `run_id`
- scan statistics collected before failure
- `duration`

## Configuration

Configuration comes from environment variables through viper. Missing `.env` files must not fail startup.

Required settings:

```txt
GROUP_EXPIRY_SCHEDULER_HTTP_ADDR
GROUP_EXPIRY_SCHEDULER_MONGODB_URI
GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE
GROUP_EXPIRY_SCHEDULER_NATS_URL
GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT
GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT
GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION
```

Optional settings:

```txt
GROUP_EXPIRY_SCHEDULER_ENV=development
GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT=10s
GROUP_EXPIRY_SCHEDULER_BATCH_SIZE=20
GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT=15s
GROUP_EXPIRY_SCHEDULER_CRON_WITH_SECONDS=false
GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE=UTC
GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE=UTC
GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE=UTC
```

Validation rules:

- Environment must be valid according to `internal/shared/environment`.
- Required strings must be non-empty after trimming.
- Shutdown timeout and publish timeout must be positive.
- Batch size must be greater than zero.
- Scheduler timezone must load with `time.LoadLocation`.
- Cron expression must be valid for `GROUP_EXPIRY_SCHEDULER_CRON_WITH_SECONDS` and the scheduler location.
- Bucket timezones must parse with `group.ParseExpirationBucketLocation`, matching existing group-service support for `UTC` and fixed offsets such as `UTC+8` or `UTC-05:00`.

## Health and Shutdown

Expose:

```http
GET /health/liveness
```

Use `internal/shared/health` with the same lightweight process indicator pattern used by existing services.

Liveness should not check MongoDB or NATS in this phase. It answers whether the process can serve at all, not whether temporary external dependencies are reachable. A readiness endpoint can be added later if deployment needs traffic gating.

Shutdown requirements:

- The process context should be canceled on `SIGINT` or `SIGTERM`.
- Echo should stop with the configured graceful timeout.
- gocron should use context-aware shutdown where practical.
- NATS connections should close during shutdown.
- MongoDB should disconnect with the configured shutdown timeout.
- A running job should observe context cancellation through service/repository/publisher calls.

## Local Runtime

`docker-compose.yml` should add a `group-expiry-scheduler` service.

The local container should depend on:

- `mongo-init`
- `nats-init`

The local env should point to the same MongoDB database and NATS server as `group-service`.

Suggested local defaults:

```txt
GROUP_EXPIRY_SCHEDULER_HTTP_ADDR=:8084
GROUP_EXPIRY_SCHEDULER_MONGODB_URI=mongodb://mongodb:27017/?replicaSet=rs0
GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE=workspace_permission_management
GROUP_EXPIRY_SCHEDULER_NATS_URL=nats://nats:4222
GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT=app.todo.group.expiry.process
GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT=app.todo.group.individual-member.expiry.process
GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION=* * * * *
GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE=UTC
GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE=UTC
```

`nats-init` should continue to ensure the two existing group-service command streams and consumers:

- `GROUP_EXPIRY` with subject `app.todo.group.expiry.process`
- `INDIVIDUAL_MEMBER_EXPIRY` with subject `app.todo.group.individual-member.expiry.process`

The scheduler only requires those stream subjects to exist for publishing. The group-service durable consumers remain owned by group-service configuration.

## Testing Strategy

Config tests:

- Loads required and optional settings.
- Applies defaults.
- Rejects missing required values.
- Rejects invalid environment values.
- Rejects invalid cron expressions.
- Rejects invalid scheduler timezone values.
- Rejects invalid bucket timezone values.
- Rejects non-positive batch size and timeouts.

Shared expiry repository tests:

- Creates expected indexes for both task collections.
- Inserts and deletes group expiry tasks.
- Inserts and deletes individual-member expiry tasks.
- Finds tasks by full command identity for existing group-service command handling.
- Lists due group tasks with `expiration_bucket <= due_bucket`.
- Lists due individual-member tasks with `expiration_bucket <= due_bucket`.
- Uses cursor pagination across multiple buckets and IDs.
- Does not return tasks after the cursor.

Scheduler service tests:

- Scans group tasks until empty.
- Scans individual-member tasks until empty.
- Uses separate bucket timezones for the two task types.
- Publishes all tasks across multiple batches.
- Continues after single group task publish failure.
- Continues after single individual-member task publish failure.
- Stops the current job on group task query failure.
- Stops the current job on individual-member task query failure.
- Returns accurate scanned, published, and failed counts.

Transport tests:

- Builds group expiry command CloudEvents accepted by `group-service` parser.
- Builds individual-member expiry command CloudEvents accepted by `group-service` parser.
- Uses `source: group-expiry-scheduler` for both event types.
- Uses configured subject as CloudEvent `type`.
- Uses task ID as CloudEvent `subject`.

Composition tests:

- Wires config into eventbus producer publish timeout.
- Registers liveness route.
- Builds gocron job from cron expression and verifies overlap guard behavior.
- Ensures repository indexes during startup.

Repository-wide verification:

```bash
go test ./...
```

## Rollout and Compatibility

Implementation order should minimize risk:

1. Add `internal/shared/repositories/expiry` with behavior equivalent to current task collection logic.
2. Refactor `group-service` repository to use the shared package without changing command contracts.
3. Add scheduler config, transport, service, and composition root.
4. Add local Docker Compose service wiring.
5. Run `go test ./...`.

Compatibility notes:

- NATS subjects do not change.
- CloudEvent data field names do not change.
- The scheduler's CloudEvent `source` becomes `group-expiry-scheduler` for both task types. Existing group-service parsing does not validate source, so this is backward-compatible.
- Task collection schemas and indexes remain unchanged.
- Duplicate command publication remains safe because group-service command handling is idempotent.

## Alternatives Considered

### New service with shared expiry repository

This is the selected approach.

Pros:

- Matches the requested independent `group-expiry-scheduler` service.
- Removes duplicated task schema and cursor query logic.
- Keeps service-private imports out of shared code.
- Preserves group-service transaction ownership.

Cons:

- Requires a small refactor of `MongoGroupRepository`.

### Scheduler-owned read-only repository

This would leave group-service repository code untouched and add read-only task collection queries inside `internal/group-expiry-scheduler/repositories`.

Pros:

- Smaller initial refactor.

Cons:

- Duplicates task documents, collection names, indexes, and cursor behavior.
- Does not satisfy the goal of extracting task collection logic to a shared package.

### Scheduler inside group-service

This would add gocron directly to `cmd/group-service`.

Pros:

- Fewer deployed processes.

Cons:

- Does not match the requested independent service.
- Expands group-service responsibilities beyond API and command consumption.
- Makes operational scaling and failure isolation worse.
