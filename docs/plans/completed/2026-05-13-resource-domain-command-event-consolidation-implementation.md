# Resource Domain Command/Event Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate resource-create commands and resource-upsert events into `internal/domain/resource`, remove `internal/domain/mockfunction`, and make `ResourceUpsertEvent` directly replace `UpsertInput` in the function-service upsert workflow.

**Architecture:** `internal/domain/resource` owns the cross-service resource command/event contracts and validation. Service `transport` packages keep CloudEvent parsing/building, services keep workflow decisions, and repositories map resource-domain events to persistence fields. Workspace request sections remain workspace-service-local context for ordering and logging, not serialized command data.

**Tech Stack:** Go 1.25, CloudEvents Go SDK, NATS JetStream via `internal/shared/eventbus`, MongoDB Go driver, `go test`.

---

## Source Designs and Policies

- Source design: [../../designs/resource-command-event-contracts.md](../../designs/resource-command-event-contracts.md)
- Related design: [../../designs/mock-function.md](../../designs/mock-function.md)
- Related design: [../../designs/function-service.md](../../designs/function-service.md)
- Related design: [../../designs/workspace-service-command-design.md](../../designs/workspace-service-command-design.md)
- Policy: [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- Policy: [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- This is backend implementation plus design-plan documentation work.
- Domain packages must not depend on CloudEvents, NATS, MongoDB, Echo, or service packages.
- Transport packages may depend on domain packages for DTO-to-domain mapping.
- Services depend on domain contracts and consumer-side interfaces.
- This implementation plan lives under `docs/plans/active/`.

## File Map

- Modify `internal/domain/resource/resource.go`: add `ResourceCreateCommand` and `ResourceUpsertEvent`; remove `UpsertInput` after call sites migrate.
- Modify `internal/domain/resource/validation.go`: move command/event validation into resource domain; keep delete/list validation.
- Modify `internal/domain/resource/validation_test.go`: replace upsert-input tests with resource-upsert-event tests and add resource-create-command tests.
- Modify `internal/domain/workspace/workspace.go`: remove `ResourceCreateCommand`; keep `ResourceSection`.
- Modify `internal/domain/workspace/validation.go`: remove `ResourceCreateCommand` validation.
- Modify `internal/domain/workspace/validation_test.go`: remove command tests that move to resource domain.
- Modify `internal/workspace-service/services/workspace_service.go`: publish `resource.ResourceCreateCommand` while retaining section context locally.
- Modify `internal/workspace-service/services/workspace_service_test.go`: update fake publisher and assertions to resource-domain commands.
- Modify `internal/workspace-service/transport/resource_create_event.go`: accept `resource.ResourceCreateCommand`.
- Modify `internal/workspace-service/transport/resource_create_event_test.go`: use resource-domain command.
- Modify `cmd/workspace-service/resource_create_publisher.go`: accept resource-domain command.
- Modify `cmd/workspace-service/resource_create_publisher_test.go`: use resource-domain command.
- Modify `internal/mock-function/handlers/resource_create_event_handler.go`: use `resource.ResourceCreateCommand` and `resource.ErrInvalidInput`.
- Modify `internal/mock-function/handlers/resource_create_event_handler_test.go`: use resource-domain command and error.
- Modify `internal/mock-function/services/resource_service.go`: use `resource.ResourceCreateCommand` and `resource.ResourceUpsertEvent`.
- Modify `internal/mock-function/services/resource_service_test.go`: use resource-domain command/event.
- Modify `internal/mock-function/transport/resource_create_event.go`: parse into `resource.ResourceCreateCommand`.
- Modify `internal/mock-function/transport/resource_create_event_test.go`: assert event ID and time are restored.
- Modify `internal/mock-function/transport/resource_upsert_event.go`: build from `resource.ResourceUpsertEvent`.
- Modify `internal/mock-function/transport/resource_upsert_event_test.go`: use resource-domain event.
- Modify `cmd/mock-function/resource_upsert_publisher.go`: accept resource-domain event.
- Modify `cmd/mock-function/resource_upsert_publisher_test.go`: use resource-domain event.
- Delete `internal/domain/mockfunction/errors.go`.
- Delete `internal/domain/mockfunction/mockfunction.go`.
- Delete `internal/domain/mockfunction/validation.go`.
- Delete `internal/domain/mockfunction/validation_test.go`.
- Modify `internal/function-service/transport/resource_event.go`: parse CloudEvents into `resource.ResourceUpsertEvent`.
- Modify `internal/function-service/transport/resource_event_test.go`: assert `ResourceID`, `ResourceType`, `ResourceTags`, and `EventID`.
- Modify `internal/function-service/handlers/resource_event_handler.go`: pass `resource.ResourceUpsertEvent` directly to the service.
- Modify `internal/function-service/handlers/resource_event_handler_test.go`: update fake service to accept `ResourceUpsertEvent`.
- Modify `internal/function-service/services/resource_service.go`: make repository and service upsert accept `ResourceUpsertEvent`.
- Modify `internal/function-service/services/resource_service_test.go`: update fake repository and validation tests.
- Modify `internal/function-service/repositories/mongo_resource_repository.go`: make `Upsert` and duplicate retry accept `ResourceUpsertEvent`.
- Modify `internal/function-service/repositories/mongo_resource_repository_test.go`: use `ResourceUpsertEvent`.

---

### Task 1: Add Resource-Domain Command and Event Contracts

**Files:**
- Modify: `internal/domain/resource/resource.go`
- Modify: `internal/domain/resource/validation.go`
- Modify: `internal/domain/resource/validation_test.go`

- [ ] **Step 1: Replace upsert-input tests with resource event and command tests**

In `internal/domain/resource/validation_test.go`, replace `validUpsertInput`, `TestUpsertInputValidateAcceptsValidInput`, and `TestUpsertInputValidateRejectsInvalidFields` with:

```go
func validResourceUpsertEvent() ResourceUpsertEvent {
	return ResourceUpsertEvent{
		ResourceID:   "resource-1",
		WorkspaceID:  "workspace-1",
		FunctionKey:  "todo",
		DisplayName:  "Spec",
		ResourceType: "document",
		ResourceTags: []string{"section_1"},
		EventID:      "event-1",
		EventTime:    time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC),
	}
}

func validResourceCreateCommand() ResourceCreateCommand {
	return ResourceCreateCommand{
		WorkspaceID:  "workspace-1",
		AppName:      "documents",
		ResourceName: "Docs",
		ResourceType: "document",
		EventID:      "event-1",
		EventTime:    time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
	}
}

func TestResourceCreateCommandValidateAcceptsValidCommand(t *testing.T) {
	if err := validResourceCreateCommand().Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestResourceCreateCommandValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*ResourceCreateCommand)
		wantMessage string
	}{
		{name: "blank workspace id", mutate: func(c *ResourceCreateCommand) { c.WorkspaceID = "   " }, wantMessage: "workspace id is required"},
		{name: "blank app name", mutate: func(c *ResourceCreateCommand) { c.AppName = "   " }, wantMessage: "app name is required"},
		{name: "blank resource name", mutate: func(c *ResourceCreateCommand) { c.ResourceName = "   " }, wantMessage: "resource name is required"},
		{name: "blank resource type", mutate: func(c *ResourceCreateCommand) { c.ResourceType = "   " }, wantMessage: "resource type is required"},
		{name: "blank event id", mutate: func(c *ResourceCreateCommand) { c.EventID = "   " }, wantMessage: "event id is required"},
		{name: "zero event time", mutate: func(c *ResourceCreateCommand) { c.EventTime = time.Time{} }, wantMessage: "event time is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := validResourceCreateCommand()
			tt.mutate(&command)
			requireInvalidInput(t, command.Validate(), tt.wantMessage)
		})
	}
}

func TestResourceCreateCommandNormalizeAndSubject(t *testing.T) {
	command := ResourceCreateCommand{
		WorkspaceID:  " workspace-1 ",
		AppName:      " documents ",
		ResourceName: " Docs ",
		ResourceType: " document ",
		EventID:      " event-1 ",
	}
	normalized := command.Normalize()
	if normalized.WorkspaceID != "workspace-1" || normalized.AppName != "documents" || normalized.ResourceName != "Docs" || normalized.ResourceType != "document" || normalized.EventID != "event-1" {
		t.Fatalf("Normalize() = %+v", normalized)
	}
	if normalized.Subject() != "cmd.app.documents.resource.create" {
		t.Fatalf("Subject() = %q", normalized.Subject())
	}
}

func TestResourceUpsertEventValidateAcceptsValidEvent(t *testing.T) {
	if err := validResourceUpsertEvent().Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestResourceUpsertEventValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*ResourceUpsertEvent)
		wantMessage string
	}{
		{name: "blank resource id", mutate: func(e *ResourceUpsertEvent) { e.ResourceID = "   " }, wantMessage: "resource id is required"},
		{name: "blank workspace id", mutate: func(e *ResourceUpsertEvent) { e.WorkspaceID = "   " }, wantMessage: "workspace id is required"},
		{name: "blank function key", mutate: func(e *ResourceUpsertEvent) { e.FunctionKey = "   " }, wantMessage: "function key is required"},
		{name: "blank display name", mutate: func(e *ResourceUpsertEvent) { e.DisplayName = "   " }, wantMessage: "display name is required"},
		{name: "blank resource type", mutate: func(e *ResourceUpsertEvent) { e.ResourceType = "   " }, wantMessage: "resource type is required"},
		{name: "blank event id", mutate: func(e *ResourceUpsertEvent) { e.EventID = "   " }, wantMessage: "event id is required"},
		{name: "zero event time", mutate: func(e *ResourceUpsertEvent) { e.EventTime = time.Time{} }, wantMessage: "event time is required"},
		{name: "blank resource tag", mutate: func(e *ResourceUpsertEvent) { e.ResourceTags = []string{"section_1", "   "} }, wantMessage: "resource tags must be non-empty strings"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := validResourceUpsertEvent()
			tt.mutate(&event)
			requireInvalidInput(t, event.Validate(), tt.wantMessage)
		})
	}
}

func TestResourceUpsertEventNormalizeAndSubject(t *testing.T) {
	event := ResourceUpsertEvent{
		ResourceID:   " resource-1 ",
		DisplayName:  " Spec ",
		ResourceType: " document ",
		FunctionKey:  " todo ",
		WorkspaceID:  " workspace-1 ",
		EventID:      " event-1 ",
	}
	normalized := event.Normalize()
	if normalized.ResourceID != "resource-1" || normalized.DisplayName != "Spec" || normalized.ResourceType != "document" || normalized.FunctionKey != "todo" || normalized.WorkspaceID != "workspace-1" || normalized.EventID != "event-1" {
		t.Fatalf("Normalize() = %+v", normalized)
	}
	if normalized.Subject() != "app.todo.resource.upserted" {
		t.Fatalf("Subject() = %q", normalized.Subject())
	}
}
```

- [ ] **Step 2: Run the resource domain tests and confirm the new tests fail**

Run:

```bash
go test ./internal/domain/resource
```

Expected: FAIL with undefined `ResourceCreateCommand` and `ResourceUpsertEvent`.

- [ ] **Step 3: Add the resource-domain types**

In `internal/domain/resource/resource.go`, remove the `UpsertInput` type and add:

```go
type ResourceCreateCommand struct {
	WorkspaceID  string
	AppName      string
	ResourceName string
	ResourceType string
	EventID      string
	EventTime    time.Time
}

func (c ResourceCreateCommand) Subject() string {
	return "cmd.app." + c.AppName + ".resource.create"
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

func (e ResourceUpsertEvent) Subject() string {
	return "app." + e.FunctionKey + ".resource.upserted"
}
```

- [ ] **Step 4: Add resource command/event validation**

In `internal/domain/resource/validation.go`, remove `func (input UpsertInput) Validate() error` and add:

```go
func (c ResourceCreateCommand) Normalize() ResourceCreateCommand {
	c.WorkspaceID = strings.TrimSpace(c.WorkspaceID)
	c.AppName = strings.TrimSpace(c.AppName)
	c.ResourceName = strings.TrimSpace(c.ResourceName)
	c.ResourceType = strings.TrimSpace(c.ResourceType)
	c.EventID = strings.TrimSpace(c.EventID)
	return c
}

func (c ResourceCreateCommand) Validate() error {
	normalized := c.Normalize()
	if normalized.WorkspaceID == "" {
		return invalidInput("workspace id is required")
	}
	if normalized.AppName == "" {
		return invalidInput("app name is required")
	}
	if normalized.ResourceName == "" {
		return invalidInput("resource name is required")
	}
	if normalized.ResourceType == "" {
		return invalidInput("resource type is required")
	}
	if normalized.EventID == "" {
		return invalidInput("event id is required")
	}
	if normalized.EventTime.IsZero() {
		return invalidInput("event time is required")
	}
	return nil
}

func (e ResourceUpsertEvent) Normalize() ResourceUpsertEvent {
	e.ResourceID = strings.TrimSpace(e.ResourceID)
	e.DisplayName = strings.TrimSpace(e.DisplayName)
	e.ResourceType = strings.TrimSpace(e.ResourceType)
	e.FunctionKey = strings.TrimSpace(e.FunctionKey)
	e.WorkspaceID = strings.TrimSpace(e.WorkspaceID)
	e.EventID = strings.TrimSpace(e.EventID)
	return e
}

func (e ResourceUpsertEvent) Validate() error {
	normalized := e.Normalize()
	if normalized.ResourceID == "" {
		return invalidInput("resource id is required")
	}
	if normalized.DisplayName == "" {
		return invalidInput("display name is required")
	}
	if normalized.ResourceType == "" {
		return invalidInput("resource type is required")
	}
	if normalized.FunctionKey == "" {
		return invalidInput("function key is required")
	}
	if normalized.WorkspaceID == "" {
		return invalidInput("workspace id is required")
	}
	if normalized.EventID == "" {
		return invalidInput("event id is required")
	}
	if normalized.EventTime.IsZero() {
		return invalidInput("event time is required")
	}
	for _, tag := range normalized.ResourceTags {
		if strings.TrimSpace(tag) == "" {
			return invalidInput("resource tags must be non-empty strings")
		}
	}
	return nil
}
```

- [ ] **Step 5: Run the resource domain tests**

Run:

```bash
go test ./internal/domain/resource
```

Expected: PASS.

- [ ] **Step 6: Commit the resource-domain contracts**

Run:

```bash
git add internal/domain/resource/resource.go internal/domain/resource/validation.go internal/domain/resource/validation_test.go
git commit -m "refactor: add resource command event contracts"
```

---

### Task 2: Move Workspace Resource Commands to Resource Domain

**Files:**
- Modify: `internal/domain/workspace/workspace.go`
- Modify: `internal/domain/workspace/validation.go`
- Modify: `internal/domain/workspace/validation_test.go`
- Modify: `internal/workspace-service/services/workspace_service.go`
- Modify: `internal/workspace-service/services/workspace_service_test.go`
- Modify: `internal/workspace-service/transport/resource_create_event.go`
- Modify: `internal/workspace-service/transport/resource_create_event_test.go`
- Modify: `cmd/workspace-service/resource_create_publisher.go`
- Modify: `cmd/workspace-service/resource_create_publisher_test.go`

- [ ] **Step 1: Update workspace-service tests to expect `resource.ResourceCreateCommand`**

In `internal/workspace-service/services/workspace_service_test.go`, change the fake publisher and command assertions to use the resource domain:

```go
import (
	// existing imports
	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type publishedResourceCreateCommand struct {
	section workspace.ResourceSection
	command resource.ResourceCreateCommand
}

type fakeCommandPublisher struct {
	commands []resource.ResourceCreateCommand
	err      error
}

func (f *fakeCommandPublisher) PublishResourceCreateCommand(_ context.Context, command resource.ResourceCreateCommand) error {
	f.commands = append(f.commands, command)
	return f.err
}
```

Update assertions that previously read `command.Section` so section expectations are verified through publish order and command fields. For the existing documents/tasks/drive order test, assert:

```go
if publisher.commands[0].AppName != "documents" || publisher.commands[0].ResourceType != "document" {
	t.Fatalf("documents command = %+v", publisher.commands[0])
}
if publisher.commands[1].AppName != "tasks" || publisher.commands[1].ResourceType != "task" {
	t.Fatalf("tasks command = %+v", publisher.commands[1])
}
if publisher.commands[2].AppName != "drive" || publisher.commands[2].ResourceType != "file" {
	t.Fatalf("drive command = %+v", publisher.commands[2])
}
```

In `internal/workspace-service/transport/resource_create_event_test.go` and `cmd/workspace-service/resource_create_publisher_test.go`, replace `workspace.ResourceCreateCommand{...}` with `resource.ResourceCreateCommand{...}` and import `internal/domain/resource`.

- [ ] **Step 2: Run workspace command tests and confirm they fail**

Run:

```bash
go test ./internal/domain/workspace ./internal/workspace-service/services ./internal/workspace-service/transport ./cmd/workspace-service
```

Expected: FAIL because production code still uses `workspace.ResourceCreateCommand`.

- [ ] **Step 3: Remove command ownership from workspace domain**

In `internal/domain/workspace/workspace.go`, remove this type and method:

```go
type ResourceCreateCommand struct {
	WorkspaceID  string
	Section      ResourceSection
	AppName      string
	ResourceName string
	ResourceType string
	EventID      string
	EventTime    time.Time
}

func (c ResourceCreateCommand) Subject() string {
	return "cmd.app." + c.AppName + ".resource.create"
}
```

In `internal/domain/workspace/validation.go`, remove `ResourceCreateCommand.Normalize()` and `ResourceCreateCommand.Validate()`.

In `internal/domain/workspace/validation_test.go`, remove `TestResourceCreateCommandValidate` and `TestResourceCreateCommandSubject`.

- [ ] **Step 4: Add service-local section context in workspace service**

In `internal/workspace-service/services/workspace_service.go`, import resource:

```go
import (
	// existing imports
	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)
```

Change the publisher interface:

```go
type ResourceCreateCommandPublisher interface {
	PublishResourceCreateCommand(ctx context.Context, command resource.ResourceCreateCommand) error
}
```

Add a service-local wrapper:

```go
type sectionResourceCreateCommand struct {
	section workspace.ResourceSection
	command resource.ResourceCreateCommand
}
```

Update publish and command construction:

```go
func (s *WorkspaceService) publishResourceCreateCommands(ctx context.Context, workspaceID string, input workspace.CreateInput) {
	commands := s.resourceCreateCommands(workspaceID, input)
	for _, item := range commands {
		command := item.command
		if s.publisher == nil {
			s.logger.Error("failed to publish resource create command",
				"err", "publisher is not configured",
				"workspace_id", command.WorkspaceID,
				"resource_section", item.section,
				"app_name", command.AppName,
				"resource_type", command.ResourceType,
				"resource_name", command.ResourceName,
				"subject", command.Subject(),
				"event_id", command.EventID,
			)
			continue
		}
		if err := s.publisher.PublishResourceCreateCommand(ctx, command); err != nil {
			s.logger.Error("failed to publish resource create command",
				"err", err,
				"workspace_id", command.WorkspaceID,
				"resource_section", item.section,
				"app_name", command.AppName,
				"resource_type", command.ResourceType,
				"resource_name", command.ResourceName,
				"subject", command.Subject(),
				"event_id", command.EventID,
			)
		}
	}
}

func (s *WorkspaceService) resourceCreateCommands(workspaceID string, input workspace.CreateInput) []sectionResourceCreateCommand {
	commands := make([]sectionResourceCreateCommand, 0, 3)
	if input.Documents != nil {
		commands = append(commands, s.newCommand(workspaceID, workspace.ResourceSectionDocuments, *input.Documents, s.mappings.Documents))
	}
	if input.Tasks != nil {
		commands = append(commands, s.newCommand(workspaceID, workspace.ResourceSectionTasks, *input.Tasks, s.mappings.Tasks))
	}
	if input.Drive != nil {
		commands = append(commands, s.newCommand(workspaceID, workspace.ResourceSectionDrive, *input.Drive, s.mappings.Drive))
	}
	return commands
}

func (s *WorkspaceService) newCommand(workspaceID string, section workspace.ResourceSection, request workspace.ResourceRequest, mapping ResourceMapping) sectionResourceCreateCommand {
	return sectionResourceCreateCommand{
		section: section,
		command: resource.ResourceCreateCommand{
			WorkspaceID:  workspaceID,
			AppName:      mapping.AppName,
			ResourceName: request.Normalize().ResourceName,
			ResourceType: mapping.ResourceType,
			EventID:      s.idGenerator(),
			EventTime:    s.clock(),
		}.Normalize(),
	}
}
```

- [ ] **Step 5: Update workspace transport and publisher signatures**

In `internal/workspace-service/transport/resource_create_event.go`, replace the workspace import with resource and change the function signature:

```go
import "github.com/hao0731/workspace-permission-management/internal/domain/resource"

func NewResourceCreateEvent(command resource.ResourceCreateCommand) ([]byte, error) {
	command = command.Normalize()
	if err := command.Validate(); err != nil {
		return nil, err
	}
	// existing CloudEvent construction remains unchanged
}
```

In `cmd/workspace-service/resource_create_publisher.go`, replace the workspace import with resource and change:

```go
func (p resourceCreatePublisher) PublishResourceCreateCommand(ctx context.Context, command resource.ResourceCreateCommand) error {
	data, err := transport.NewResourceCreateEvent(command)
	if err != nil {
		return fmt.Errorf("build resource create event: %w", err)
	}
	if err := p.publisher.Publish(ctx, command.Subject(), data, p.opts...); err != nil {
		return fmt.Errorf("publish resource create event: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: Format and run workspace tests**

Run:

```bash
gofmt -w internal/domain/workspace/workspace.go internal/domain/workspace/validation.go internal/domain/workspace/validation_test.go internal/workspace-service/services/workspace_service.go internal/workspace-service/services/workspace_service_test.go internal/workspace-service/transport/resource_create_event.go internal/workspace-service/transport/resource_create_event_test.go cmd/workspace-service/resource_create_publisher.go cmd/workspace-service/resource_create_publisher_test.go
go test ./internal/domain/workspace ./internal/workspace-service/services ./internal/workspace-service/transport ./cmd/workspace-service
```

Expected: PASS.

- [ ] **Step 7: Commit workspace command migration**

Run:

```bash
git add internal/domain/workspace internal/workspace-service cmd/workspace-service
git commit -m "refactor: use resource create command in workspace service"
```

---

### Task 3: Move Mock Function to Resource Domain Contracts

**Files:**
- Modify: `internal/mock-function/handlers/resource_create_event_handler.go`
- Modify: `internal/mock-function/handlers/resource_create_event_handler_test.go`
- Modify: `internal/mock-function/services/resource_service.go`
- Modify: `internal/mock-function/services/resource_service_test.go`
- Modify: `internal/mock-function/transport/resource_create_event.go`
- Modify: `internal/mock-function/transport/resource_create_event_test.go`
- Modify: `internal/mock-function/transport/resource_upsert_event.go`
- Modify: `internal/mock-function/transport/resource_upsert_event_test.go`
- Modify: `cmd/mock-function/resource_upsert_publisher.go`
- Modify: `cmd/mock-function/resource_upsert_publisher_test.go`
- Delete: `internal/domain/mockfunction/errors.go`
- Delete: `internal/domain/mockfunction/mockfunction.go`
- Delete: `internal/domain/mockfunction/validation.go`
- Delete: `internal/domain/mockfunction/validation_test.go`

- [ ] **Step 1: Update mock-function tests to import resource**

Replace imports of `internal/domain/mockfunction` with `internal/domain/resource` in mock-function handler, service, transport, and cmd tests.

In tests, replace:

```go
mockfunction.ResourceCreateCommand
mockfunction.ResourceUpsertEvent
mockfunction.ErrInvalidInput
```

with:

```go
resource.ResourceCreateCommand
resource.ResourceUpsertEvent
resource.ErrInvalidInput
```

In `internal/mock-function/transport/resource_create_event_test.go`, assert parsed event metadata:

```go
if command.EventID != "event-1" {
	t.Fatalf("EventID = %q, want event-1", command.EventID)
}
wantTime := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
if !command.EventTime.Equal(wantTime) {
	t.Fatalf("EventTime = %s, want %s", command.EventTime, wantTime)
}
```

- [ ] **Step 2: Run mock-function tests and confirm they fail**

Run:

```bash
go test ./internal/mock-function/... ./cmd/mock-function
```

Expected: FAIL because production code still imports `internal/domain/mockfunction`.

- [ ] **Step 3: Update mock-function resource-create parser**

In `internal/mock-function/transport/resource_create_event.go`, replace the mockfunction import with resource and change the signature:

```go
import "github.com/hao0731/workspace-permission-management/internal/domain/resource"

func ParseResourceCreateCommandEvent(data []byte, messageSubject string, subjectAppNames map[string]string) (resource.ResourceCreateCommand, error) {
	appName, ok := subjectAppNames[messageSubject]
	if !ok {
		return resource.ResourceCreateCommand{}, fmt.Errorf("unknown resource create subject %q", messageSubject)
	}
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return resource.ResourceCreateCommand{}, fmt.Errorf("parse cloudevent: %w", err)
	}
	if err := event.Validate(); err != nil {
		return resource.ResourceCreateCommand{}, fmt.Errorf("validate cloudevent: %w", err)
	}
	if event.SpecVersion() != cloudEventSpecVersion {
		return resource.ResourceCreateCommand{}, fmt.Errorf("unsupported cloudevent specversion %q", event.SpecVersion())
	}
	if event.Type() != messageSubject {
		return resource.ResourceCreateCommand{}, fmt.Errorf("cloudevent type %q does not match subject %q", event.Type(), messageSubject)
	}
	if event.DataContentType() != "application/json" {
		return resource.ResourceCreateCommand{}, fmt.Errorf("cloudevent datacontenttype must be application/json")
	}
	if event.Time().IsZero() {
		return resource.ResourceCreateCommand{}, fmt.Errorf("cloudevent time is required")
	}
	var payload resourceCreateData
	if err := event.DataAs(&payload); err != nil {
		return resource.ResourceCreateCommand{}, fmt.Errorf("parse cloudevent data: %w", err)
	}
	if event.Subject() != payload.WorkspaceID {
		return resource.ResourceCreateCommand{}, fmt.Errorf("cloudevent subject must match data.workspace_id")
	}
	command := resource.ResourceCreateCommand{
		WorkspaceID:  payload.WorkspaceID,
		AppName:      appName,
		ResourceName: payload.ResourceName,
		ResourceType: payload.ResourceType,
		EventID:      event.ID(),
		EventTime:    event.Time(),
	}.Normalize()
	if strings.TrimSpace(command.WorkspaceID) == "" || strings.TrimSpace(command.ResourceName) == "" || strings.TrimSpace(command.ResourceType) == "" {
		return resource.ResourceCreateCommand{}, fmt.Errorf("resource create command data contains empty required field")
	}
	return command, nil
}
```

- [ ] **Step 4: Update mock-function handler, service, transport builder, and publisher**

Use these signatures:

```go
type ResourceCreateService interface {
	HandleResourceCreate(ctx context.Context, command resource.ResourceCreateCommand) error
}

type ResourceUpsertPublisher interface {
	PublishResourceUpsert(ctx context.Context, event resource.ResourceUpsertEvent) error
}

func (s *ResourceService) HandleResourceCreate(ctx context.Context, command resource.ResourceCreateCommand) error

func NewResourceUpsertEvent(input resource.ResourceUpsertEvent) ([]byte, string, error)

func (p resourceUpsertPublisher) PublishResourceUpsert(ctx context.Context, event resource.ResourceUpsertEvent) error
```

In `internal/mock-function/handlers/resource_create_event_handler.go`, classify invalid command input with:

```go
if errors.Is(err, resource.ErrInvalidInput) {
	h.logger.Warn("terminating invalid resource create command input",
		"err", err,
		"workspace_id", command.WorkspaceID,
		"app_name", command.AppName,
		"resource_name", command.ResourceName,
		"resource_type", command.ResourceType,
	)
	return eventbus.HandleResultTerminate
}
```

In `internal/mock-function/services/resource_service.go`, build the upsert event with:

```go
event := resource.ResourceUpsertEvent{
	ResourceID:   s.idGenerator(),
	DisplayName:  command.ResourceName,
	ResourceType: command.ResourceType,
	ResourceTags: []string{},
	FunctionKey:  command.AppName,
	WorkspaceID:  command.WorkspaceID,
	EventID:      s.idGenerator(),
	EventTime:    s.clock(),
}
```

- [ ] **Step 5: Delete the mockfunction domain package**

Remove:

```bash
internal/domain/mockfunction/errors.go
internal/domain/mockfunction/mockfunction.go
internal/domain/mockfunction/validation.go
internal/domain/mockfunction/validation_test.go
```

- [ ] **Step 6: Verify no mockfunction domain imports remain**

Run:

```bash
rg -n 'domain/mockfunction|mockfunction\.' internal cmd
```

Expected: no output.

- [ ] **Step 7: Format and run mock-function tests**

Run:

```bash
gofmt -w internal/mock-function/handlers/resource_create_event_handler.go internal/mock-function/handlers/resource_create_event_handler_test.go internal/mock-function/services/resource_service.go internal/mock-function/services/resource_service_test.go internal/mock-function/transport/resource_create_event.go internal/mock-function/transport/resource_create_event_test.go internal/mock-function/transport/resource_upsert_event.go internal/mock-function/transport/resource_upsert_event_test.go cmd/mock-function/resource_upsert_publisher.go cmd/mock-function/resource_upsert_publisher_test.go
go test ./internal/mock-function/... ./cmd/mock-function
```

Expected: PASS.

- [ ] **Step 8: Commit mock-function migration**

Run:

```bash
git add internal/domain/mockfunction internal/mock-function cmd/mock-function
git commit -m "refactor: use resource contracts in mock function"
```

---

### Task 4: Replace Function-Service UpsertInput with ResourceUpsertEvent

**Files:**
- Modify: `internal/function-service/transport/resource_event.go`
- Modify: `internal/function-service/transport/resource_event_test.go`
- Modify: `internal/function-service/handlers/resource_event_handler.go`
- Modify: `internal/function-service/handlers/resource_event_handler_test.go`
- Modify: `internal/function-service/services/resource_service.go`
- Modify: `internal/function-service/services/resource_service_test.go`
- Modify: `internal/function-service/repositories/mongo_resource_repository.go`
- Modify: `internal/function-service/repositories/mongo_resource_repository_test.go`
- Modify: `internal/domain/resource/resource.go`

- [ ] **Step 1: Update function-service tests to use ResourceUpsertEvent**

In `internal/function-service/transport/resource_event_test.go`, change field assertions:

```go
if got.ResourceID != "resource-1" || got.DisplayName != "Spec" || got.ResourceType != "document" {
	t.Fatalf("parsed event = %+v, want resource-1/Spec/document", got)
}
if got.WorkspaceID != "workspace-1" || got.FunctionKey != "todo" {
	t.Fatalf("parsed scope = %s/%s, want workspace-1/todo", got.WorkspaceID, got.FunctionKey)
}
if got.EventID != "event-1" {
	t.Fatalf("EventID = %q, want event-1", got.EventID)
}
if len(got.ResourceTags) != 1 || got.ResourceTags[0] != "section_1" {
	t.Fatalf("tags = %#v, want [section_1]", got.ResourceTags)
}
```

In `internal/function-service/services/resource_service_test.go`, update the fake repository:

```go
type fakeResourceRepository struct {
	upsertStatus resource.UpsertStatus
	upsertEvent  resource.ResourceUpsertEvent
	upsertCalls  int
	upsertErr    error
	// existing list/delete fields stay unchanged
}

func (f *fakeResourceRepository) Upsert(ctx context.Context, event resource.ResourceUpsertEvent) (resource.UpsertStatus, error) {
	f.upsertCalls++
	f.upsertEvent = event
	if f.upsertErr != nil {
		return "", f.upsertErr
	}
	return f.upsertStatus, nil
}
```

Update service tests to call:

```go
got, err := service.UpsertResource(context.Background(), resource.ResourceUpsertEvent{
	ResourceID:   "resource-1",
	WorkspaceID:  "workspace-1",
	FunctionKey:  "todo",
	DisplayName:  "Spec",
	ResourceType: "document",
	ResourceTags: []string{"section_1"},
	EventID:      "event-1",
	EventTime:    eventTime,
})
```

In `internal/function-service/handlers/resource_event_handler_test.go`, update the fake event service:

```go
type fakeEventResourceService struct {
	event resource.ResourceUpsertEvent
	err   error
}

func (f *fakeEventResourceService) UpsertResource(ctx context.Context, event resource.ResourceUpsertEvent) (resource.UpsertStatus, error) {
	f.event = event
	if f.err != nil {
		return "", f.err
	}
	return resource.UpsertStatusInserted, nil
}
```

- [ ] **Step 2: Run function-service tests and confirm they fail**

Run:

```bash
go test ./internal/function-service/transport ./internal/function-service/handlers ./internal/function-service/services ./internal/function-service/repositories
```

Expected: FAIL because production signatures still use `resource.UpsertInput`.

- [ ] **Step 3: Parse resource upsert CloudEvents into ResourceUpsertEvent**

In `internal/function-service/transport/resource_event.go`, change the signature and return type:

```go
func ParseResourceUpsertEvent(data []byte, expectedType string) (resource.ResourceUpsertEvent, error) {
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("parse cloudevent: %w", err)
	}
	if err := event.Validate(); err != nil {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("validate cloudevent: %w", err)
	}
	if event.SpecVersion() != cloudEventSpecVersion {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("unsupported cloudevent specversion %q", event.SpecVersion())
	}
	if event.Type() != expectedType {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("cloudevent type %q does not match expected %q", event.Type(), expectedType)
	}
	if event.DataContentType() != "application/json" {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("cloudevent datacontenttype must be application/json")
	}
	var payload resourceUpsertData
	if err := event.DataAs(&payload); err != nil {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("parse cloudevent data: %w", err)
	}
	if event.Subject() != payload.ResourceID {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("cloudevent subject must match data.resource_id")
	}
	if strings.TrimSpace(payload.ResourceID) == "" ||
		strings.TrimSpace(payload.WorkspaceID) == "" ||
		strings.TrimSpace(payload.FunctionKey) == "" ||
		strings.TrimSpace(payload.DisplayName) == "" ||
		strings.TrimSpace(payload.ResourceType) == "" {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("resource event data contains empty required field")
	}
	for _, tag := range payload.ResourceTags {
		if strings.TrimSpace(tag) == "" {
			return resource.ResourceUpsertEvent{}, fmt.Errorf("resource_tags must contain non-empty strings")
		}
	}
	if event.Time().IsZero() {
		return resource.ResourceUpsertEvent{}, fmt.Errorf("cloudevent time is required")
	}
	return resource.ResourceUpsertEvent{
		ResourceID:   payload.ResourceID,
		WorkspaceID:  payload.WorkspaceID,
		FunctionKey:  payload.FunctionKey,
		DisplayName:  payload.DisplayName,
		ResourceType: payload.ResourceType,
		ResourceTags: append([]string(nil), payload.ResourceTags...),
		EventID:      event.ID(),
		EventTime:    event.Time(),
	}, nil
}
```

- [ ] **Step 4: Update function-service handler and service interfaces**

In `internal/function-service/handlers/resource_event_handler.go`, change:

```go
type EventResourceService interface {
	UpsertResource(ctx context.Context, event resource.ResourceUpsertEvent) (resource.UpsertStatus, error)
}
```

Inside `Handle`, rename `input` to `event` and log `event.ResourceID`:

```go
event, err := transport.ParseResourceUpsertEvent(msg.Data, h.expectedType)
if err != nil {
	h.logger.Warn("terminating invalid resource event", "err", err, "subject", msg.Subject)
	return eventbus.HandleResultTerminate
}

status, err := h.service.UpsertResource(ctx, event)
if err != nil {
	if errors.Is(err, ErrRetryableEvent) {
		h.logger.Warn("retrying resource event", "err", err, "resource_id", event.ResourceID)
		return eventbus.HandleResultRetry
	}
	if errors.Is(err, resource.ErrInvalidInput) {
		h.logger.Warn("terminating invalid resource event input", "err", err, "resource_id", event.ResourceID)
		return eventbus.HandleResultTerminate
	}
	h.logger.Warn("retrying resource event after service error", "err", err, "resource_id", event.ResourceID)
	return eventbus.HandleResultRetry
}

h.logger.Info("handled resource event", "resource_id", event.ResourceID, "status", status)
return eventbus.HandleResultAck
```

In `internal/function-service/services/resource_service.go`, change:

```go
type ResourceRepository interface {
	Upsert(ctx context.Context, event resource.ResourceUpsertEvent) (resource.UpsertStatus, error)
	List(ctx context.Context, query resource.ListQuery) (resource.Page, error)
	Delete(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error)
}

func (s *ResourceService) UpsertResource(ctx context.Context, event resource.ResourceUpsertEvent) (resource.UpsertStatus, error) {
	if err := event.Validate(); err != nil {
		return "", err
	}
	status, err := s.repository.Upsert(ctx, event)
	if err != nil {
		return "", fmt.Errorf("upsert resource: %w", err)
	}
	return status, nil
}
```

- [ ] **Step 5: Update Mongo repository to persist ResourceUpsertEvent**

In `internal/function-service/repositories/mongo_resource_repository.go`, change `Upsert` and `retryUpdateAfterDuplicate` signatures and field mapping:

```go
func (r *MongoResourceRepository) Upsert(ctx context.Context, event resource.ResourceUpsertEvent) (resource.UpsertStatus, error) {
	update := bson.M{
		"$set": bson.M{
			"workspace_id":  event.WorkspaceID,
			"function_key":  event.FunctionKey,
			"display_name":  event.DisplayName,
			"resource_type": event.ResourceType,
			"resource_tags": append([]string(nil), event.ResourceTags...),
			"updated_at":    event.EventTime,
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{
		"_id":        event.ResourceID,
		"updated_at": bson.M{"$lte": event.EventTime},
	}, update)
	if err != nil {
		return "", fmt.Errorf("update current resource: %w", err)
	}
	if result.MatchedCount > 0 {
		return resource.UpsertStatusUpdated, nil
	}

	doc := resourceDocument{
		ID:           event.ResourceID,
		WorkspaceID:  event.WorkspaceID,
		FunctionKey:  event.FunctionKey,
		DisplayName:  event.DisplayName,
		ResourceType: event.ResourceType,
		ResourceTags: append([]string(nil), event.ResourceTags...),
		CreatedAt:    event.EventTime,
		UpdatedAt:    event.EventTime,
	}
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			status, retryErr := r.retryUpdateAfterDuplicate(ctx, event)
			if retryErr != nil {
				return "", retryErr
			}
			return status, nil
		}
		return "", fmt.Errorf("insert resource: %w", err)
	}
	return resource.UpsertStatusInserted, nil
}
```

Use the same event field names in `retryUpdateAfterDuplicate`.

- [ ] **Step 6: Remove UpsertInput from resource domain**

Confirm no production references remain:

```bash
rg -n 'UpsertInput' internal cmd --glob '*.go'
```

Expected: only references in files not yet updated. After updating all references, remove `type UpsertInput` from `internal/domain/resource/resource.go`.

- [ ] **Step 7: Format and run function-service tests**

Run:

```bash
gofmt -w internal/function-service/transport/resource_event.go internal/function-service/transport/resource_event_test.go internal/function-service/handlers/resource_event_handler.go internal/function-service/handlers/resource_event_handler_test.go internal/function-service/services/resource_service.go internal/function-service/services/resource_service_test.go internal/function-service/repositories/mongo_resource_repository.go internal/function-service/repositories/mongo_resource_repository_test.go internal/domain/resource/resource.go
go test ./internal/function-service/transport ./internal/function-service/handlers ./internal/function-service/services ./internal/function-service/repositories ./internal/domain/resource
```

Expected: PASS.

- [ ] **Step 8: Commit function-service upsert event migration**

Run:

```bash
git add internal/function-service internal/domain/resource
git commit -m "refactor: use resource upsert event directly"
```

---

### Task 5: Repository-Wide Cleanup and Verification

**Files:**
- Verify: all modified Go files
- Verify: `docs/designs/resource-command-event-contracts.md`
- Verify: `docs/designs/function-service.md`
- Verify: `docs/designs/mock-function.md`
- Verify: this implementation plan

- [ ] **Step 1: Confirm removed concepts no longer appear in production code**

Run:

```bash
rg -n 'domain/mockfunction|mockfunction\.|UpsertInput|workspace\.ResourceCreateCommand' internal cmd --glob '*.go'
```

Expected: no output.

- [ ] **Step 2: Confirm resource-domain command/event imports are used**

Run:

```bash
rg -n 'ResourceCreateCommand|ResourceUpsertEvent' internal/domain/resource internal/workspace-service internal/mock-function internal/function-service cmd --glob '*.go'
```

Expected: output shows `ResourceCreateCommand` and `ResourceUpsertEvent` owned by `internal/domain/resource` and used by the three services.

- [ ] **Step 3: Run full Go test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Run final diff checks**

Run:

```bash
git diff --check
git status --short
```

Expected: `git diff --check` has no output. `git status --short` shows only intentional changes from this plan and any pre-existing user-owned changes such as `lefthook.yml`.

- [ ] **Step 5: Commit final cleanup if needed**

If Task 5 produced only formatting or cleanup changes, run:

```bash
git add internal cmd docs/plans/active/2026-05-13-resource-domain-command-event-consolidation-implementation.md
git commit -m "chore: finalize resource contract consolidation"
```

If there are no cleanup changes, do not create an empty commit.

---

## Self-Review

Spec coverage:

- `internal/domain/mockfunction` removal is covered by Task 3.
- `ResourceCreateCommand` ownership moves from workspace/mockfunction to resource in Tasks 1, 2, and 3.
- `ResourceUpsertEvent` ownership moves from mockfunction to resource in Tasks 1 and 3.
- `ResourceUpsertEvent` directly replaces `UpsertInput` in function-service service/repository flow in Task 4.
- CloudEvent command/event restoration is covered by the existing transport test files updated in Tasks 2, 3, and 4 without adding a separate contract-test package.
- Repository-wide cleanup and verification are covered by Task 5.

Placeholder scan:

- No placeholder markers or open-ended implementation steps are used.
- Each code-changing task includes concrete file paths, signatures, commands, and expected results.

Type consistency:

- Shared command type is consistently `resource.ResourceCreateCommand`.
- Shared upsert event type is consistently `resource.ResourceUpsertEvent`.
- Workspace section context remains `workspace.ResourceSection` and is carried only by `sectionResourceCreateCommand`.
- `resource.UpsertInput` is removed after function-service call sites migrate.
