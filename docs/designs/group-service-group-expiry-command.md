---
doc_id: design.group-service-group-expiry-command
doc_type: design
title: Group expiry command design
status: implemented

tags:
  - group
  - expiry
  - command

code_paths:
  - internal/group-service/**
  - internal/domain/group/**
  - internal/shared/repositories/expiry/**
  - cmd/group-service/**

related:
  designs:
    - design.group-service
    - design.group-service-group
    - design.group-expiry-scheduler
    - design.group-service-individual-member-expiry-command
  adrs: []

last_updated_at: 2026-05-30

summary: >
  Read this when changing grouping-rule expiry task persistence, expiry command
  CloudEvents, or idempotent group expiry handling.
---

# Group Expiry Command Design

## Background

`group-service` stores dynamic grouping rules on the `groups` collection. A grouping rule has an expiration date, but expiration should be applied asynchronously so API writes do not need to wait until the rule becomes expired.

This design adds an outbox-like MongoDB collection named `group_expiry_task` and a JetStream command consumer that marks grouping rules as expired when their expiration bucket is processed.

Entry point and shared service concerns are documented in [Group Service Design](group-service.md). Group create, delete, and grouping-rule replacement behavior is documented in [Group API Design](group-service-group.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- MongoDB documents, indexes, JetStream settings, and CloudEvent payloads are explicit contracts.
- JetStream and NATS types stay in handler and composition-root infrastructure, not domain or service logic.
- The command handler parses the CloudEvent envelope, classifies ack/retry/terminate outcomes, and delegates business behavior to services.
- The service owns grouping-rule expiration workflow and depends on consumer-side repository interfaces.
- The repository owns MongoDB reads, writes, indexes, and transaction mechanics.
- This design is stored under `docs/designs/` and linked from the group-service entry design.

## Goals

- Create `group_expiry_task` documents when a group is created with dynamic grouping rules.
- Replace the active expiry task when a group's dynamic grouping rules are replaced.
- Remove the active expiry task when a group no longer has dynamic grouping rules or when the group is soft-deleted.
- Add `groups.grouping_rule.expired_at` to record when the grouping rule was marked expired.
- Consume a JetStream command event that instructs `group-service` to process one expiry task.
- Use CloudEvents as the command envelope.
- Keep stream, durable name, subject, fetch count, max wait, and expiration-bucket timezone configurable.
- Make command handling idempotent for duplicate delivery, stale commands, and grouping-rule changes after command publication.
- Mark `groups.grouping_rule.expired_at` and delete the matching `group_expiry_task` in one MongoDB transaction.

## Non-Goals

- Do not design the scheduler or dispatcher in this command-focused document. The scheduler is documented separately in [Group Expiry Scheduler Design](group-expiry-scheduler.md).
- Do not expose `expired_at` in the public HTTP group response in this phase.
- Do not evaluate employee attributes or materialize group membership.
- Do not expire explicit individual members; individual-member expiration is handled by [Group Individual Member Expiry Command Design](group-service-individual-member-expiry-command.md).
- Do not add frontend changes.

## Expiry Task Lifecycle

The expiry task represents the currently active dynamic grouping-rule expiration for one group.

Create group:

- If `grouping_rule.rules` is non-empty, create one `group_expiry_task` document in the same MongoDB transaction as the `groups` insert.
- The task `_id` is a service-generated UUID.
- The task `expiration_bucket` is derived from `groups.grouping_rule.expiration_date` using the configured bucket timezone.
- If `grouping_rule.rules` is empty, create no task.

Replace grouping rules:

- Replace the active group's `grouping_rule` and expiry task in the same MongoDB transaction.
- If the replacement `rules` array is non-empty, delete any existing `group_expiry_task` for the group and insert a new one with a new UUID and recalculated `expiration_bucket`.
- If the replacement `rules` array is empty, delete any existing `group_expiry_task` for the group and leave `groups.grouping_rule.expired_at` as `null`.
- Replacing rules also resets `groups.grouping_rule.expired_at` to `null` because the replacement creates a fresh grouping-rule lifecycle.

Delete group:

- Soft-delete the active group and active individual members as documented in [Group API Design](group-service-group.md#delete-group-api).
- Delete any `group_expiry_task` documents for that `workspace_id + group_id` in the same transaction so commands for deleted groups become stale.

Rationale:

- A new task ID per grouping-rule replacement makes old published commands stale by identity, even when the old and new expiration dates fall in the same bucket.
- Keeping task writes in the same MongoDB transaction as group writes prevents a group from pointing at an expiration date that has no matching task, and prevents a task from pointing at an uncommitted grouping-rule state.

## Expiration Bucket Timezone

`expiration_bucket` uses the string format `yyyy-MM-dd`.

The bucket date is computed from `groups.grouping_rule.expiration_date` after converting the instant to the configured fixed-offset timezone.

Configuration:

- `GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE`, default `UTC`.

Accepted first-version values:

- `UTC`
- Fixed offsets such as `UTC+8`, `UTC+08`, `UTC+08:00`, `UTC-5`, or `UTC-05:00`.

Invalid timezone values fail startup.

The same parsed timezone must be injected into:

- API create workflow task generation.
- API grouping-rule replacement task generation.
- Command handling bucket validation.

Rationale:

- Defaulting to `UTC` matches the service's existing UTC clock behavior.
- Allowing fixed offsets supports business bucket definitions such as `UTC+8` without introducing IANA timezone database behavior or daylight-saving ambiguity in this phase.

## group_expiry_task Collection

Collection: `group_expiry_task`

Document schema:

```ts
{
  "_id": string,
  "workspace_id": string,
  "group_id": string,
  "expiration_bucket": string
}
```

Field notes:

- `_id` is a service-generated UUID and is the command `data.task_id`.
- `workspace_id` scopes the task to one workspace.
- `group_id` references `groups._id`.
- `expiration_bucket` is the grouping-rule expiration bucket in `yyyy-MM-dd` format using the configured bucket timezone.

Indexes:

```txt
unique { workspace_id: 1, group_id: 1 }
{ expiration_bucket: 1, _id: 1 }
```

Rationale:

- The unique `workspace_id + group_id` index enforces at most one active expiry task per group.
- The `expiration_bucket + _id` index supports a future scheduler or dispatcher that scans due task buckets deterministically.
- `_id` already has MongoDB's default unique index for direct command processing.

## Groups Collection Change

`groups.grouping_rule` gains an internal `expired_at` field:

```ts
{
  "grouping_rule": {
    "rules": [
      {
        "attribute_key": string,
        "operator": "eq" | "not_eq" | "gt" | "gte" | "lt" | "lte",
        "multi": boolean,
        "value": unknown | unknown[]
      }
    ],
    "expiration_date": Date,
    "expired_at": Date | null
  }
}
```

Field notes:

- `expired_at` is `null` when the grouping rule is active or waiting for expiration processing.
- `expired_at` is set to the service-generated command handling timestamp when the rule is expired by the command consumer.
- `expired_at` is reset to `null` whenever grouping rules are replaced.
- `expired_at` is persistence metadata and is not returned by existing HTTP group responses in this phase.

Compatibility notes:

- Existing documents without `grouping_rule.expired_at` should be treated as if the field were `null`.
- No immediate backfill is required for correctness if repository filters use `expired_at == null OR expired_at missing` semantics where needed.

## Command CloudEvent Contract

Default NATS subject and CloudEvent type:

```txt
app.todo.group.expiry.process
```

The subject remains configurable. The first implementation should use the configured subject as the expected CloudEvent `type`, matching the group-service command handling pattern.

CloudEvent envelope:

```json
{
  "specversion": "1.0",
  "type": "app.todo.group.expiry.process",
  "source": "<COMMAND_SOURCE>",
  "subject": "<TASK_ID>",
  "id": "<UUID>",
  "time": "2026-05-10T10:00:00Z",
  "datacontenttype": "application/json",
  "data": {
    "task_id": "<UUID>",
    "workspace_id": "<WORKSPACE_ID>",
    "group_id": "<GROUP_ID>",
    "expiration_bucket": "<yyyy-MM-dd>"
  }
}
```

Required envelope fields:

- `specversion`
- `type`
- `source`
- `subject`
- `id`
- `time`
- `datacontenttype`
- `data`

Required data fields:

- `task_id`
- `workspace_id`
- `group_id`
- `expiration_bucket`

Validation rules:

- `specversion` must be `1.0`.
- `type` must match the configured command subject.
- `datacontenttype` must be `application/json`.
- `time` must be a valid timestamp.
- `subject` must match `data.task_id`.
- Data fields must be non-empty strings after trimming.
- `data.expiration_bucket` must match `yyyy-MM-dd`.

Invalid envelope or data shape is a poison message and should be terminated rather than retried.

## Configuration

Required settings:

- `GROUP_SERVICE_NATS_URL`
- `GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM`
- `GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE`
- `GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT`

Optional settings:

- `GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT`, default `20`.
- `GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT`, default `5s`.
- `GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE`, default `UTC`.

Validation rules:

- Required string values must be non-empty after trimming.
- Fetch count must be greater than zero.
- Max wait must be positive.
- Bucket timezone must be `UTC` or a supported fixed offset.

Startup behavior:

- `cmd/group-service/main.go` connects to NATS after MongoDB setup.
- The composition root creates an `internal/shared/eventbus` JetStream consumer with the configured stream, durable, subject, fetch count, and max wait.
- The service fails startup when the stream or durable consumer does not exist, or when the durable consumer filter does not include the configured subject.
- HTTP server and JetStream consumer run as sibling goroutines and share shutdown through the process context, matching `function-service`.

## Command Handling Semantics

The command handler should return explicit eventbus outcomes:

- `Ack`: command was processed, was already processed, or is stale and safe to ignore.
- `Retry`: repository, transaction, MongoDB, or context errors that may succeed on redelivery.
- `Terminate`: invalid CloudEvent envelope, invalid data shape, unsupported type, or other poison-message cases.

Structured logs should include:

- `task_id`
- `workspace_id`
- `group_id`
- `expiration_bucket`
- `err` when present
- a status such as `expired`, `stale_task`, `stale_bucket`, `already_expired`, or `retry`

## Expire Grouping Rule Workflow

Domain input:

```go
type ExpireGroupingRuleCommand struct {
    TaskID           string
    WorkspaceID      string
    GroupID          string
    ExpirationBucket string
}
```

Service workflow:

1. Normalize and validate command input.
2. Generate one UTC `now` timestamp for `expired_at`.
3. Call the repository with the command input, `now`, and the configured bucket timezone.
4. Return a domain-oriented status so the handler can log whether the command expired the rule or was stale.

Repository transaction workflow:

1. Start a MongoDB transaction.
2. Read `group_expiry_task` by `_id = task_id`, `workspace_id`, `group_id`, and `expiration_bucket`.
3. If no matching task exists, return stale success.
4. Read the active group by `_id = group_id`, `workspace_id`, and `deleted_at: null`.
5. If no active group exists, delete the task and return stale success.
6. If `grouping_rule.expired_at` is already non-null, delete the task and return already-expired success.
7. Compute the bucket for the current `grouping_rule.expiration_date` using the configured timezone.
8. If the computed bucket does not equal command `expiration_bucket`, log stale bucket and return stale success without changing the group.
9. Set `groups.grouping_rule.expired_at = now` and `groups.updated_at = now`.
10. Delete the matching `group_expiry_task` document by `_id`.
11. Commit the transaction.

Idempotency:

- Duplicate delivery after successful processing finds no task and acknowledges.
- Commands for replaced grouping rules find no task because replacement creates a new task ID.
- Commands whose bucket no longer matches the current grouping rule acknowledge without marking the rule expired.
- Commands for soft-deleted groups acknowledge and remove the orphaned task if it still exists.

## Error Handling

Poison-message cases:

- Malformed JSON.
- Invalid CloudEvent envelope.
- Unsupported CloudEvent spec version.
- Unexpected CloudEvent type.
- Non-JSON data content type.
- Missing or empty required data fields.
- Invalid `expiration_bucket` format.
- CloudEvent `subject` mismatch with `data.task_id`.

Retryable cases:

- MongoDB read, update, delete, transaction, or session failures.
- Context errors while repository work is in progress, unless the process is shutting down.
- Unexpected service errors not classified as invalid input.

Ack-with-log cases:

- Matching task is missing.
- Active group is missing.
- Grouping rule is already expired.
- Current grouping-rule expiration date falls outside the command bucket.

## Testing Strategy

Domain tests:

- Command validation rejects empty `task_id`, `workspace_id`, `group_id`, or `expiration_bucket`.
- Command validation rejects bucket strings that are not `yyyy-MM-dd`.
- Bucket timezone parser accepts `UTC` and fixed offsets such as `UTC+8`.
- Bucket timezone parser rejects invalid values.
- Bucket calculation uses the configured timezone.

Transport tests:

- Valid CloudEvent maps to `ExpireGroupingRuleCommand`.
- Unsupported `specversion`, mismatched `type`, non-JSON content type, invalid `subject`, missing data fields, and invalid bucket format are rejected.
- CloudEvent `time` is required and valid.

Handler tests:

- Invalid CloudEvents return `HandleResultTerminate`.
- Invalid command data returns `HandleResultTerminate`.
- Stale command statuses return `HandleResultAck` and log the status.
- Successful expiration returns `HandleResultAck`.
- Retryable service errors return `HandleResultRetry`.

Service tests:

- Validation failures do not call the repository.
- Successful command passes deterministic `now` and bucket timezone to the repository.
- Repository stale statuses are preserved for handler logging.
- Repository failures are wrapped with context.

Repository tests:

- `EnsureIndexes` creates the `group_expiry_task` unique group index and bucket scan index.
- Create group writes `groups`, `group_individual_members` when present, and `group_expiry_task` in one transaction.
- Create group without dynamic grouping rules writes no `group_expiry_task`.
- Replace grouping rules deletes the old task and creates a new task when rules are non-empty.
- Replace grouping rules deletes the old task when rules are empty.
- Delete group soft-deletes the group and members and deletes group expiry tasks in one transaction.
- Expire command sets `grouping_rule.expired_at`, updates `updated_at`, and deletes the matching task in one transaction.
- Expire command returns stale success when task is missing, group is missing, already expired, or bucket mismatched.
- Transaction failure rolls back both the group update and task delete.

Config and composition-root tests:

- Required NATS and command consumer settings are validated.
- Optional fetch count, max wait, and bucket timezone defaults are applied.
- Invalid bucket timezone fails config validation.
- `cmd/group-service` wires HTTP and JetStream consumer startup without leaking NATS types into services.

Repository-wide verification for implementation:

```bash
go test ./...
```

MongoDB transaction tests may require the local Docker Compose replica set, matching the existing group-service transaction caveat.

## Architecture Decisions and Trade-Offs

- Dedicated design document: keeps asynchronous command behavior separate from HTTP API behavior while keeping it linked from the group-service entry point.
- Task collection instead of deriving tasks only from `groups`: gives external schedulers or dispatchers a compact due-task scan surface, at the cost of maintaining task consistency in group write transactions.
- New task ID on every grouping-rule replacement: makes stale commands easy to detect, at the cost of deleting and recreating task documents instead of updating in place.
- Fixed-offset bucket timezone config: satisfies `UTC+8` style bucket boundaries with simple deterministic parsing, but does not model daylight-saving changes.
- `expired_at` under `groups.grouping_rule`: scopes expiration state to the dynamic rule lifecycle rather than the entire group, so explicit individual members can remain independent.
- Ack stale commands: supports JetStream at-least-once delivery without infinite retries for old commands, but requires structured logs for operational visibility.
