# Group Individual Member Expiry Command Design

## Background

`group-service` stores explicit individual members in `group_individual_members`. Each member has an `expiration_date`, but expiration should be applied asynchronously so HTTP writes only record the intended expiration and a compact task for later processing.

This design adds an outbox-like MongoDB collection named `individual_member_expiry_task` and a JetStream command consumer that marks an individual member as expired when the member expiration bucket is processed.

Entry point and shared service concerns are documented in [Group Service Design](group-service.md). Individual member HTTP APIs and the member collection are documented in [Group Individual Members API Design](group-service-individual-members.md). Grouping-rule expiration is documented separately in [Group Expiry Command Design](group-service-group-expiry-command.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- MongoDB documents, indexes, JetStream settings, and CloudEvent payloads are explicit contracts.
- JetStream and NATS types stay in handler and composition-root infrastructure, not domain or service logic.
- The command handler parses the CloudEvent envelope, classifies ack/retry/terminate outcomes, and delegates business behavior to services.
- The service owns individual-member expiration workflow and depends on consumer-side repository interfaces.
- The repository owns MongoDB reads, writes, indexes, and transaction mechanics.
- This design is stored under `docs/designs/` and linked from the group-service entry design.

## Goals

- Create `individual_member_expiry_task` documents when individual members are created through group create or member add workflows.
- Replace the active individual-member expiry task when an individual member's expiration date is updated.
- Remove active individual-member expiry tasks when a member or group is soft-deleted.
- Add `group_individual_members.expired_at` to record when the individual member was marked expired.
- Consume a JetStream command event that instructs `group-service` to process one individual-member expiry task.
- Use CloudEvents as the command envelope.
- Keep stream, durable name, subject, fetch count, max wait, and expiration-bucket timezone configurable.
- Make command handling idempotent for duplicate delivery, stale commands, deleted members, already-expired members, and expiration-date changes after command publication.
- Mark `group_individual_members.expired_at` and delete the matching `individual_member_expiry_task` in one MongoDB transaction.

## Non-Goals

- Do not design the scheduler or dispatcher in this command-focused document. The scheduler is documented separately in [Group Expiry Scheduler Design](group-expiry-scheduler.md).
- Do not expose `expired_at` in public HTTP member or group responses in this phase.
- Do not publish membership-changed events.
- Do not evaluate permissions or dynamic grouping rules.
- Do not add frontend changes.

## Expiry Task Lifecycle

The expiry task represents the currently active expiration task for one individual member record.

Create group:

- If `individual_members` is non-empty, create one `individual_member_expiry_task` document per inserted individual member in the same MongoDB transaction as the `groups` and `group_individual_members` inserts.
- Each task `_id` is a service-generated UUID.
- Each task `expiration_bucket` is derived from that member's `expiration_date` using the configured individual-member bucket timezone.
- Insert no individual-member expiry tasks when the create request has no individual members.

Add individual members:

- Insert the requested `group_individual_members` documents and matching `individual_member_expiry_task` documents in the same MongoDB transaction.
- Each newly added member gets exactly one active expiry task.

Update individual member expiration:

- Update the active member's `expiration_date`, set `updated_at`, reset `expired_at` to `null`, delete any existing expiry task for `group_id + nt_account`, and insert a replacement task with a new UUID and recalculated `expiration_bucket` in one MongoDB transaction.
- A new task ID makes old published commands stale by identity, even when the old and new expiration dates fall in the same bucket.
- Resetting `expired_at` allows an expired member to be extended through the existing PATCH endpoint instead of requiring delete and re-add.

Delete individual member:

- Soft-delete the active member and delete any active `individual_member_expiry_task` for `group_id + nt_account` in the same MongoDB transaction.
- If the member or group is already missing, the delete API remains idempotent and no task cleanup is required beyond best-effort matching deletes inside the transaction.

Delete group:

- Soft-delete the active group and active individual members as documented in [Group API Design](group-service-group.md#delete-group-api).
- Delete any `individual_member_expiry_task` documents for that `group_id` in the same transaction so commands for deleted groups or members become stale.

Rationale:

- Keeping task writes in the same MongoDB transaction as member writes prevents a member from pointing at an expiration date that has no matching task, and prevents a task from pointing at an uncommitted member state.
- The task collection has no `workspace_id` because `group_individual_members` also scopes by `group_id`; `group_id` is a service-generated UUID and is sufficient for member lookup.

## Expiration Bucket Timezone

`expiration_bucket` uses the string format `yyyy-MM-dd`.

The bucket date is computed from `group_individual_members.expiration_date` after converting the instant to the configured fixed-offset timezone.

Configuration:

- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE`, default `UTC`.

Accepted first-version values:

- `UTC`
- Fixed offsets such as `UTC+8`, `UTC+08`, `UTC+08:00`, `UTC-5`, or `UTC-05:00`.

Invalid timezone values fail startup.

The same parsed timezone must be injected into:

- Group create workflow individual-member task generation.
- Individual-member add workflow task generation.
- Individual-member expiration update task replacement.
- Command handling bucket validation.

Rationale:

- This mirrors the grouping-rule expiry bucket behavior while keeping the deployment contract separate.
- A separate setting lets deployments align or intentionally separate grouping-rule and individual-member bucket definitions.

## individual_member_expiry_task Collection

Collection: `individual_member_expiry_task`

Document schema:

```ts
{
  "_id": string,
  "group_id": string,
  "nt_account": string,
  "expiration_bucket": string
}
```

Field notes:

- `_id` is a service-generated UUID and is the command `data.task_id`.
- `group_id` references `groups._id` and scopes the task to one group.
- `nt_account` is the trimmed individual member account.
- `expiration_bucket` is the individual-member expiration bucket in `yyyy-MM-dd` format using the configured bucket timezone.

Indexes:

```txt
unique { group_id: 1, nt_account: 1 }
{ expiration_bucket: 1, _id: 1 }
```

Rationale:

- The unique `group_id + nt_account` index enforces at most one active expiry task per individual member.
- The `expiration_bucket + _id` index supports a future scheduler or dispatcher that scans due task buckets deterministically.
- `_id` already has MongoDB's default unique index for direct command processing.

## group_individual_members Collection Change

`group_individual_members` gains an internal `expired_at` field:

```ts
{
  "_id": string,
  "group_id": string,
  "nt_account": string,
  "expiration_date": Date,
  "expired_at": Date | null,
  "created_at": Date,
  "updated_at": Date,
  "deleted_at": Date | null
}
```

Field notes:

- `expired_at` is `null` when the individual member has not been processed as expired.
- `expired_at` is set to the service-generated command handling timestamp when the member is expired by the command consumer.
- `expired_at` is reset to `null` whenever the member expiration date is updated.
- `expired_at` is persistence metadata and is not returned by existing HTTP group or member responses in this phase.

Compatibility notes:

- Existing documents without `expired_at` should be treated as if the field were `null`.
- No immediate backfill is required for correctness if repository filters use `expired_at == null OR expired_at missing` semantics where an unexpired membership is required.
- The existing active-record definition for list, update, delete, and duplicate checks remains `deleted_at == null`.
- Permission evaluation and membership-source checks that require an unexpired individual member should use `deleted_at == null` and `expired_at == null`.

## Command CloudEvent Contract

Default NATS subject and CloudEvent type:

```txt
app.todo.group.individual-member.expiry.process
```

The subject remains configurable. The first implementation should use the configured subject as the expected CloudEvent `type`, matching the group-expiry command handling pattern.

CloudEvent envelope:

```json
{
  "specversion": "1.0",
  "type": "app.todo.group.individual-member.expiry.process",
  "source": "<COMMAND_SOURCE>",
  "subject": "<TASK_ID>",
  "id": "<UUID>",
  "time": "2026-05-10T10:00:00Z",
  "datacontenttype": "application/json",
  "data": {
    "task_id": "<UUID>",
    "group_id": "<GROUP_ID>",
    "nt_account": "<NT_ACCOUNT>",
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
- `group_id`
- `nt_account`
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
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM`
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE`
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT`

Optional settings:

- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT`, default `20`.
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT`, default `5s`.
- `GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE`, default `UTC`.

Validation rules:

- Required string values must be non-empty after trimming.
- Fetch count must be greater than zero.
- Max wait must be positive.
- Bucket timezone must be `UTC` or a supported fixed offset.

Startup behavior:

- `cmd/group-service/main.go` connects to NATS after MongoDB setup.
- The composition root creates a second `internal/shared/eventbus` JetStream consumer with the configured stream, durable, subject, fetch count, and max wait.
- The service fails startup when the stream or durable consumer does not exist, or when the durable consumer filter does not include the configured subject.
- HTTP server, grouping-rule expiry consumer, and individual-member expiry consumer run as sibling goroutines and share shutdown through the process context.

## Command Handling Semantics

The command handler should return explicit eventbus outcomes:

- `Ack`: command was processed, was already processed, or is stale and safe to ignore.
- `Retry`: repository, transaction, MongoDB, or context errors that may succeed on redelivery.
- `Terminate`: invalid CloudEvent envelope, invalid data shape, unsupported type, or other poison-message cases.

Structured logs should include:

- `task_id`
- `group_id`
- `nt_account`
- `expiration_bucket`
- `err` when present
- a status such as `expired`, `stale_task`, `stale_member`, `already_expired`, `stale_bucket`, or `retry`

## Expire Individual Member Workflow

Domain input:

```go
type ExpireIndividualMemberCommand struct {
    TaskID           string
    GroupID          string
    NTAccount        string
    ExpirationBucket string
}
```

Service workflow:

1. Normalize and validate command input.
2. Generate one UTC `now` timestamp for `expired_at`.
3. Call the repository with the command input, `now`, and the configured bucket timezone.
4. Return a domain-oriented status so the handler can log whether the command expired the member or was stale.

Repository transaction workflow:

1. Start a MongoDB transaction.
2. Read `individual_member_expiry_task` by `_id = task_id`, `group_id`, `nt_account`, and `expiration_bucket`.
3. If no matching task exists, return stale success.
4. Read the active individual member by `group_id`, `nt_account`, and `deleted_at: null`.
5. If no active member exists, delete the task and return stale-member success.
6. If `expired_at` is already non-null, delete the task and return already-expired success.
7. Compute the bucket for the current `expiration_date` using the configured timezone.
8. If the computed bucket does not equal command `expiration_bucket`, log stale bucket and return stale success without changing the member.
9. Set `group_individual_members.expired_at = now` and `group_individual_members.updated_at = now`.
10. Delete the matching `individual_member_expiry_task` document by `_id`.
11. Commit the transaction.

Idempotency:

- Duplicate delivery after successful processing finds no task and acknowledges.
- Commands for updated individual-member expirations find no task because update creates a new task ID.
- Commands whose bucket no longer matches the current member expiration date acknowledge without marking the member expired.
- Commands for soft-deleted members acknowledge and remove the orphaned task if it still exists.

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
- Active individual member is missing.
- Individual member is already expired.
- Current individual-member expiration date falls outside the command bucket.

## Testing Strategy

Domain tests:

- Command validation rejects empty `task_id`, `group_id`, `nt_account`, or `expiration_bucket`.
- Command validation rejects bucket strings that are not `yyyy-MM-dd`.
- Bucket timezone parser accepts `UTC` and fixed offsets such as `UTC+8`.
- Bucket timezone parser rejects invalid values.
- Bucket calculation uses the configured timezone.

Transport tests:

- Valid CloudEvent maps to `ExpireIndividualMemberCommand`.
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
- Add, create, and update workflows generate deterministic individual-member expiry task IDs and buckets.

Repository tests:

- `EnsureIndexes` creates the `individual_member_expiry_task` unique member index and bucket scan index.
- Create group writes `groups`, `group_individual_members`, `group_expiry_task` when needed, and `individual_member_expiry_task` documents in one transaction.
- Add individual members writes member documents and matching expiry tasks in one transaction.
- Update individual member expiration updates `expiration_date`, resets `expired_at`, replaces the matching task, and commits atomically.
- Delete individual member soft-deletes the member and deletes the matching task in one transaction.
- Delete group soft-deletes the group and members, deletes group expiry tasks, and deletes individual-member expiry tasks in one transaction.
- Expire command sets `expired_at`, updates `updated_at`, and deletes the matching task in one transaction.
- Expire command returns stale success when task is missing, member is missing, already expired, or bucket mismatched.
- Transaction failure rolls back both the member update and task delete.

Config and composition-root tests:

- Required NATS and individual-member command consumer settings are validated.
- Optional fetch count, max wait, and bucket timezone defaults are applied.
- Invalid bucket timezone fails config validation.
- `cmd/group-service` wires HTTP, grouping-rule expiry, and individual-member expiry consumer startup without leaking NATS types into services.

Repository-wide verification for implementation:

```bash
go test ./...
```

MongoDB transaction tests may require the local Docker Compose replica set, matching the existing group-service transaction caveat.

## Architecture Decisions and Trade-Offs

- Dedicated design document: keeps asynchronous individual-member command behavior separate from HTTP API behavior while keeping it linked from the group-service entry point.
- Task collection instead of deriving tasks only from `group_individual_members`: gives external schedulers or dispatchers a compact due-task scan surface, at the cost of maintaining task consistency in member write transactions.
- New task ID on every expiration update: makes stale commands easy to detect, at the cost of deleting and recreating task documents instead of updating in place.
- No `workspace_id` in task or command data: follows the current member schema and user-provided contract, but operational logs must rely on `group_id` and `nt_account` unless they perform a group lookup.
- Separate JetStream consumer and config: keeps grouping-rule and individual-member expiry contracts independently deployable, at the cost of additional configuration surface.
- Fixed-offset bucket timezone config: satisfies `UTC+8` style bucket boundaries with simple deterministic parsing, but does not model daylight-saving changes.
- `expired_at` on individual members: records asynchronous expiration state without changing public HTTP responses, but downstream permission evaluation must explicitly check it when determining effective membership.
