---
doc_id: design.mock-function
doc_type: design
title: Mock function design
status: implemented

tags:
  - mock
  - function
  - nats

code_paths:
  - cmd/mock-function/**
  - internal/mock-function/**
  - internal/domain/resource/**
  - examples/nats/**

related:
  designs:
    - design.workspace-service
    - design.workspace-service-command
    - design.function-service
    - design.resource-command-event-contracts
  adrs: []

last_updated_at: 2026-05-30

summary: >
  Read this when changing the mock function service, resource-create command
  consumption, or function resource upsert event publication.
---

# Mock Function Design

## Background

`mock-function` is a local integration service used to simulate application functions for workspace creation. It consumes resource-create commands from `workspace-service`, logs each accepted command, and publishes function resource upsert events that match the existing `function-service` ingestion contract.

Related designs:

- [Workspace Service Design](workspace-service.md)
- [Workspace Service Command Design](workspace-service-command-design.md)
- [Function Service Design](function-service.md)
- [Resource Command and Event Domain Contracts](resource-command-event-contracts.md)

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- Consumed command CloudEvents and published resource upsert CloudEvents are explicit contracts.
- NATS and JetStream types stay in infrastructure and composition-root code.
- Command handlers parse CloudEvents, classify ack/retry/terminate outcomes, and delegate publish behavior through service boundaries.
- CloudEvent DTOs and mappers belong in `internal/mock-function/transport`.
- Shared resource-create command and resource-upsert event models belong in `internal/domain/resource`.
- `internal/domain/mockfunction` must not be introduced or retained; mock-function-specific behavior belongs in `internal/mock-function/*`.
- This design is stored under `docs/designs/`.

## Goals

- Build `mock-function` as an independent backend service.
- Consume three configured resource-create command subjects.
- Configure the three application names used by those subjects.
- Log each valid received resource-create command.
- Publish one function resource upsert event for each valid command.
- Generate a random UUID for `data.resource_id`.
- Set `data.function_key` to the application name from the matched command subject.
- Set `data.display_name` to the command `data.resource_name`.
- Set `data.resource_type` to the command `data.resource_type`.
- Set `data.resource_tags` to an empty array.
- Set `data.workspace_id` to the command `data.workspace_id`.
- Use `resource.ResourceCreateCommand` and `resource.ResourceUpsertEvent` from `internal/domain/resource` as the domain contracts on both sides of CloudEvent transport.
- Use `internal/shared/health` for liveness.

## Non-Goals

- Do not persist mock resources.
- Do not expose resource management HTTP APIs.
- Do not implement idempotency for duplicate command delivery.
- Do not validate application names against a function registry.
- Do not add permissions or group behavior.
- Do not add frontend changes.

## Recommended Architecture

`mock-function` should follow the backend service layout:

```plaintext
cmd/mock-function
internal/mock-function/config
internal/mock-function/handlers
internal/mock-function/services
internal/mock-function/transport
internal/domain/resource
```

Responsibilities:

- `cmd/mock-function`: composition root. It loads configuration, creates the logger, connects to NATS, registers health routes, creates the JetStream consumer and producer, starts the consumer and HTTP server, and handles graceful shutdown.
- `internal/mock-function/config`: environment-based configuration and validation for HTTP, NATS, application names, stream, durable, fetch count, max wait, publish timeout, and shutdown timeout.
- `internal/mock-function/handlers`: JetStream command handler that parses the command event, logs message handling, invokes the service, and returns ack/retry/terminate outcomes.
- `internal/mock-function/services`: command handling workflow, resource ID generation, clock usage, and upsert event publishing.
- `internal/mock-function/transport`: CloudEvent command parser and resource upsert event builder.
- `internal/domain/resource`: shared resource command and event contract types, validation, and subject helpers.

`mock-function` must not define a service-specific domain package for these contracts. `ResourceCreateCommand` and `ResourceUpsertEvent` are resource-domain contracts because they are created by one service, serialized through CloudEvents, and restored by another service.

## Configuration

Required settings:

- `MOCK_FUNCTION_HTTP_ADDR`
- `MOCK_FUNCTION_NATS_URL`
- `MOCK_FUNCTION_RESOURCE_CREATE_STREAM`
- `MOCK_FUNCTION_RESOURCE_CREATE_DURABLE`
- `MOCK_FUNCTION_DOCUMENTS_APP_NAME`
- `MOCK_FUNCTION_TASKS_APP_NAME`
- `MOCK_FUNCTION_DRIVE_APP_NAME`

Optional settings with defaults:

- `MOCK_FUNCTION_ENV`: default `development`
- `MOCK_FUNCTION_SHUTDOWN_TIMEOUT`: default `10s`
- `MOCK_FUNCTION_RESOURCE_CREATE_FETCH_COUNT`: default `20`
- `MOCK_FUNCTION_RESOURCE_CREATE_MAX_WAIT`: default `5s`
- `MOCK_FUNCTION_RESOURCE_UPSERT_PUBLISH_TIMEOUT`: default `15s`

Derived command subjects:

```txt
cmd.app.<MOCK_FUNCTION_DOCUMENTS_APP_NAME>.resource.create
cmd.app.<MOCK_FUNCTION_TASKS_APP_NAME>.resource.create
cmd.app.<MOCK_FUNCTION_DRIVE_APP_NAME>.resource.create
```

Derived upsert subjects:

```txt
app.<MOCK_FUNCTION_DOCUMENTS_APP_NAME>.resource.upserted
app.<MOCK_FUNCTION_TASKS_APP_NAME>.resource.upserted
app.<MOCK_FUNCTION_DRIVE_APP_NAME>.resource.upserted
```

Validation rules:

- Required string values must be non-empty after trimming.
- App names must be valid NATS subject tokens and must not contain `.` or whitespace in the first version.
- Fetch count must be greater than zero.
- Max wait, publish timeout, and shutdown timeout must be positive.
- Duplicate app names are allowed but should be logged at startup at `warn` level because multiple configured command subjects collapse to the same subject.

## Resource Create Command Contract

Consumed subject and CloudEvent type:

```txt
cmd.app.<APP_NAME>.resource.create
```

Domain contract:

- `internal/domain/resource.ResourceCreateCommand`
- `Subject()` returns `cmd.app.<APP_NAME>.resource.create`.
- `Validate()` wraps `resource.ErrInvalidInput` for missing workspace ID, app name, resource name, resource type, event ID, or event time.
- `workspace-service` builds this type before CloudEvent serialization.
- `mock-function` parses the CloudEvent back into this same type.

CloudEvent envelope:

```json
{
  "specversion": "1.0",
  "type": "cmd.app.<APP_NAME>.resource.create",
  "source": "workspace-service",
  "subject": "<WORKSPACE_ID>",
  "id": "<UUID>",
  "time": "2026-05-05T07:31:00Z",
  "datacontenttype": "application/json",
  "data": {
    "workspace_id": "<WORKSPACE_ID>",
    "resource_name": "<RESOURCE_NAME>",
    "resource_type": "<RESOURCE_TYPE>"
  }
}
```

Validation rules:

- `specversion` must be `1.0`.
- `type` must match the message subject.
- Message subject must be one of the configured derived command subjects.
- `datacontenttype` must be `application/json`.
- `time` must be a valid timestamp.
- `subject` must match `data.workspace_id`.
- `data.workspace_id`, `data.resource_name`, and `data.resource_type` must be non-empty strings after trimming.

Invalid envelope or data shape is a poison message and should be terminated rather than retried.

## Resource Upsert Event Contract

Published subject and CloudEvent type:

```txt
app.<APP_NAME>.resource.upserted
```

This matches the existing `function-service` resource upsert ingestion pattern documented in [Function Service Design](function-service.md#resource-upsert-event-contract).

Domain contract:

- `internal/domain/resource.ResourceUpsertEvent`
- `Subject()` returns `app.<FUNCTION_KEY>.resource.upserted`.
- `Validate()` wraps `resource.ErrInvalidInput` for missing resource ID, display name, resource type, function key, workspace ID, event ID, event time, or blank tags.
- `mock-function` builds this type before CloudEvent serialization.
- `function-service` parses the CloudEvent back into this same type and uses it directly as the upsert workflow input.

CloudEvent envelope:

```json
{
  "specversion": "1.0",
  "type": "app.<APP_NAME>.resource.upserted",
  "source": "mock-function",
  "subject": "<RESOURCE_ID>",
  "id": "<UUID>",
  "time": "2026-05-05T07:31:00Z",
  "datacontenttype": "application/json",
  "data": {
    "resource_id": "<RESOURCE_ID>",
    "display_name": "<RESOURCE_NAME>",
    "resource_type": "<RESOURCE_TYPE>",
    "resource_tags": [],
    "function_key": "<APP_NAME>",
    "workspace_id": "<WORKSPACE_ID>"
  }
}
```

Generation rules:

- `specversion` is `1.0`.
- `type` equals the derived upsert subject.
- `source` is `mock-function`.
- `subject` equals `data.resource_id`.
- `id` is a generated CloudEvent ID.
- `time` is generated by `mock-function`.
- `data.resource_id` is a generated random UUID.
- `data.display_name` is the create command `data.resource_name`.
- `data.resource_type` is the create command `data.resource_type`.
- `data.resource_tags` is an empty array.
- `data.function_key` is the matched `<APP_NAME>`.
- `data.workspace_id` is the create command `data.workspace_id`.

## Handling Semantics

For each received message:

1. Parse the CloudEvent envelope.
2. Validate the message subject, CloudEvent type, subject, and data payload.
3. Derive `<APP_NAME>` from the matched configured command subject.
4. Restore the message to `resource.ResourceCreateCommand`.
5. Log command receipt with structured fields.
6. Generate a random resource UUID and CloudEvent ID.
7. Build `resource.ResourceUpsertEvent`.
8. Publish the resource upsert event to `app.<APP_NAME>.resource.upserted`.
9. Ack the command when publish succeeds.

Error classification:

- Malformed JSON, invalid CloudEvent envelope, unknown subject, or invalid data returns `HandleResultTerminate`.
- Upsert publish failure returns `HandleResultRetry`.
- Unexpected service errors return `HandleResultRetry`.

Idempotency note:

- Duplicate delivery can create multiple mock resources because the service intentionally generates a random `resource_id` for each successful handling attempt.
- This is acceptable for the mock service because it is a local integration utility, not the authoritative resource owner.

## Logging

Command receipt log fields:

- `workspace_id`
- `app_name`
- `resource_name`
- `resource_type`
- `command_subject`
- `command_event_id`

Publish success log fields:

- `workspace_id`
- `app_name`
- `resource_id`
- `upsert_subject`
- `upsert_event_id`

Publish failure log fields:

- `err`
- `workspace_id`
- `app_name`
- `resource_name`
- `resource_type`
- `upsert_subject`

## Health and Shutdown

`cmd/mock-function/main.go` should use `internal/shared/health`.

Liveness endpoint:

```http
GET /health/liveness
```

The first implementation should use a `process` indicator. The service must close the NATS connection during shutdown and stop the HTTP server and JetStream consumer through a shared process context.

## Testing Strategy

Config tests:

- Load three app names and derive the three command subjects.
- Derive matching upsert subjects.
- Reject missing app names.
- Reject invalid app names with dots or whitespace.
- Reject non-positive fetch count, max wait, publish timeout, and shutdown timeout.

Transport tests:

- Parse valid resource-create command into `resource.ResourceCreateCommand`.
- Reject command whose CloudEvent type does not match message subject.
- Reject command whose subject does not match `data.workspace_id`.
- Reject unknown command subject.
- Reject empty `workspace_id`, `resource_name`, or `resource_type`.
- Build resource upsert event from `resource.ResourceUpsertEvent` matching the existing function-service contract.
- Preserve round-trip compatibility with `function-service` parsing of the upsert CloudEvent.

Service tests:

- Valid command publishes one resource upsert event.
- Published upsert uses command `resource_name` as `display_name`.
- Published upsert uses command `resource_type`.
- Published upsert uses empty `resource_tags`.
- Published upsert uses app name as `function_key`.
- Publish failure is returned for retry.

Handler tests:

- Valid message returns `HandleResultAck`.
- Malformed message returns `HandleResultTerminate`.
- Publisher failure returns `HandleResultRetry`.
