# Resource Command and Event Domain Contracts

## Background

The workspace creation flow crosses three backend services:

- `workspace-service` publishes resource-create commands after a workspace is created.
- `mock-function` consumes resource-create commands and publishes resource-upsert events.
- `function-service` consumes resource-upsert events and stores the resource projection.

The first implementation placed command and event models in more than one domain package. `internal/domain/mockfunction` duplicated resource-domain concepts, while `internal/domain/workspace.ResourceCreateCommand` represented a command whose payload is consumed by the resource creation flow rather than by the workspace aggregate itself. These shared contracts should be owned by `internal/domain/resource`.

Related designs:

- [Workspace Service Command Design](workspace-service-command-design.md)
- [Mock Function Design](mock-function.md)
- [Function Service Design](function-service.md)
- [Resource Input Validation Refactor Design](resource-input-validation-refactor.md)

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- Cross-service command and event payloads are explicit backend contracts.
- Domain contracts stay independent of CloudEvents, NATS, JetStream, Echo, MongoDB, and service-specific packages.
- CloudEvent envelope parsing and construction remain in service `transport` packages.
- Services depend on domain contracts and consumer-side interfaces, not broker SDK types.
- This design is stored under `docs/designs/` and cross-referenced by affected service designs.

## Goals

- Move the cross-service resource command and event models into `internal/domain/resource`.
- Remove `internal/domain/mockfunction` completely.
- Replace `internal/domain/workspace.ResourceCreateCommand` with `internal/domain/resource.ResourceCreateCommand`.
- Let `workspace-service` build a `resource.ResourceCreateCommand`, convert it to a CloudEvent, and let `mock-function` parse that CloudEvent back into the same domain command type.
- Let `mock-function` build a `resource.ResourceUpsertEvent`, convert it to a CloudEvent, and let `function-service` parse that CloudEvent back into the same domain event type.
- Replace `resource.UpsertInput` with `resource.ResourceUpsertEvent` as the function-service upsert workflow input so the parsed event is persisted directly without a second domain input type.
- Preserve existing CloudEvent subjects, JSON field names, validation behavior, and service boundaries.

## Non-Goals

- Do not change NATS subjects or CloudEvent JSON payload fields.
- Do not move CloudEvent SDK usage into `internal/domain/resource`.
- Do not add a guaranteed outbox or retry mechanism.
- Do not change workspace create API behavior.
- Do not add a function registry or app-name validation beyond existing config validation.

## Domain Ownership

`internal/domain/resource` should own these cross-service contract types:

```go
type ResourceCreateCommand struct {
	WorkspaceID  string
	AppName      string
	ResourceName string
	ResourceType string
	EventID      string
	EventTime    time.Time
}

type ResourceUpsertEvent struct {
	ResourceID   string
	DisplayName  string
	ResourceType string
	ResourceTags []string
	FunctionKey  string
	WorkspaceID  string
	EventID      string
	EventTime    time.Time
}
```

`ResourceCreateCommand.Subject()` returns:

```txt
cmd.app.<APP_NAME>.resource.create
```

`ResourceUpsertEvent.Subject()` returns:

```txt
app.<FUNCTION_KEY>.resource.upserted
```

`function-service` subscribes to the fixed wildcard filter `app.*.resource.upserted`. Its transport parser accepts concrete CloudEvent `type` values that match that pattern and the event's `function_key`; the wildcard pattern itself is not a valid published event type.

Both types should provide `Normalize()` and `Validate()` methods. Validation errors should wrap `resource.ErrInvalidInput`, allowing event handlers to classify invalid command or event data without importing service-specific domain packages.

`ResourceUpsertEvent` replaces `UpsertInput` for the resource upsert use case. Function-service handlers, services, and repositories should pass this type directly through the upsert workflow after transport parsing. Repository implementations can map the event fields to persistence fields at the repository boundary:

| `ResourceUpsertEvent` field | Persistence meaning |
| --- | --- |
| `ResourceID` | resource document `_id` |
| `WorkspaceID` | workspace scope |
| `FunctionKey` | owning function key |
| `DisplayName` | resource display name |
| `ResourceType` | resource type |
| `ResourceTags` | resource tags, cloned before persistence when needed |
| `EventTime` | source event time for insert/update ordering |

`EventID` remains event metadata for logging, diagnostics, and tests. It is not part of the persisted resource projection unless a future design explicitly adds event audit fields.

## Workspace Boundary

`internal/domain/workspace` should not own `ResourceCreateCommand`.

Workspace-specific request sections, such as `documents`, `tasks`, and `drive`, remain workspace-service concerns because they describe how a workspace create request chooses which commands to emit. They are not part of the resource-create CloudEvent contract.

If `workspace-service` needs the request section for logging or publish ordering, the service layer can carry a local unexported pair of:

```go
type sectionResourceCommand struct {
	section workspace.ResourceSection
	command resource.ResourceCreateCommand
}
```

This keeps the shared command contract in `internal/domain/resource` while preserving section-aware logs inside `workspace-service`.

## Service Transport Flow

`workspace-service`:

1. Builds `resource.ResourceCreateCommand` values after workspace persistence.
2. Calls `internal/workspace-service/transport.NewResourceCreateEvent(command)`.
3. The transport builder validates the domain command, creates the CloudEvent envelope, and serializes the documented JSON payload.

`mock-function`:

1. Consumes `cmd.app.<APP_NAME>.resource.create`.
2. Calls `internal/mock-function/transport.ParseResourceCreateCommandEvent(...)`.
3. The transport parser validates the CloudEvent envelope and returns `resource.ResourceCreateCommand`.
4. The service validates the command, generates a resource ID and event metadata, and builds `resource.ResourceUpsertEvent`.
5. The upsert event publisher calls `internal/mock-function/transport.NewResourceUpsertEvent(event)`.

`function-service`:

1. Consumes the fixed `app.*.resource.upserted` subject pattern.
2. Calls `internal/function-service/transport.ParseResourceUpsertEvent(...)`.
3. The transport parser validates the CloudEvent envelope and returns `resource.ResourceUpsertEvent`.
4. The handler invokes the resource upsert workflow with `resource.ResourceUpsertEvent` directly.
5. The service validates the event and passes it to the repository without creating `UpsertInput`.

The domain package never imports CloudEvents. Transport packages remain responsible for envelope validation, data DTO tags, and JSON serialization.

## Migration Impact

Implementation should:

- Move `ResourceCreateCommand`, `ResourceUpsertEvent`, their `Subject()`, `Normalize()`, and `Validate()` methods into `internal/domain/resource`.
- Delete `internal/domain/mockfunction`.
- Update `workspace-service` publisher interfaces and transport builders to accept `resource.ResourceCreateCommand`.
- Update `mock-function` handlers, services, transport, and tests to import `internal/domain/resource`.
- Update `function-service` resource-upsert transport, handlers, services, repositories, and tests to use `resource.ResourceUpsertEvent` directly.
- Remove `resource.UpsertInput` after all upsert call sites are migrated.
- Keep CloudEvent payloads and subjects unchanged.

## Testing Strategy

Domain tests:

- `ResourceCreateCommand.Validate()` rejects missing workspace ID, app name, resource name, resource type, event ID, and event time.
- `ResourceCreateCommand.Subject()` returns `cmd.app.<APP_NAME>.resource.create`.
- `ResourceUpsertEvent.Validate()` rejects missing resource ID, display name, resource type, function key, workspace ID, event ID, event time, and blank tags.
- `ResourceUpsertEvent.Subject()` returns `app.<FUNCTION_KEY>.resource.upserted`.
- `ResourceUpsertEvent.Normalize()` trims string fields without mutating tags.

Transport tests:

- `workspace-service` builds a CloudEvent from `resource.ResourceCreateCommand`.
- `mock-function` parses that CloudEvent back into `resource.ResourceCreateCommand`.
- `mock-function` builds a CloudEvent from `resource.ResourceUpsertEvent`.
- `function-service` parses that CloudEvent back into `resource.ResourceUpsertEvent`.
- Existing poison-message classification remains unchanged for invalid envelopes and invalid data.

Service and repository tests:

- `function-service` service accepts `resource.ResourceUpsertEvent` directly.
- Repository upsert accepts `resource.ResourceUpsertEvent` and preserves existing insert, update, same-time update, and older-event ignore behavior.
- Repository mapping clones resource tags before storing or returning projection data.

Verification command:

```bash
go test ./...
```

## Architecture Decisions

1. Put resource command and event models in `internal/domain/resource`.
   - Rationale: Both contracts describe resource lifecycle messages, not mock-function-specific behavior or workspace aggregate state.
   - Trade-off: `workspace-service` imports the resource domain for command publishing, but this matches the explicit cross-service resource contract.

2. Keep workspace request sections outside `ResourceCreateCommand`.
   - Rationale: Sections are workspace create request selectors and logging context, not serialized command data.
   - Trade-off: Workspace-service may need a small local wrapper when it wants to log section names next to resource commands.

3. Parse CloudEvents into domain event or command types before service workflows.
   - Rationale: A CloudEvent built by one service can be restored by the consuming service into the same contract type, reducing duplicate mapping rules.
   - Trade-off: Transport packages still need DTO structs for JSON field tags because domain types remain transport-agnostic.

4. Replace `UpsertInput` with `ResourceUpsertEvent` for upsert workflows.
   - Rationale: The CloudEvent payload already is the resource upsert command for the projection. Reusing the event type directly avoids duplicate structs and removes a mapping step that can drift.
   - Trade-off: The repository receives event metadata such as `EventID`; repository code should ignore metadata it does not persist while still using `EventTime` for ordering.
