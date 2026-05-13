package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
	clienthr "github.com/hao0731/workspace-permission-management/internal/shared/interactions/hr"
)

var ErrHRLookupFailed = errors.New("hr lookup failed")

type WorkspaceRepository interface {
	Create(ctx context.Context, input workspace.Workspace) (workspace.Workspace, error)
}

type ResourceCreateCommandPublisher interface {
	PublishResourceCreateCommand(ctx context.Context, command resource.ResourceCreateCommand) error
}

type sectionResourceCreateCommand struct {
	section workspace.ResourceSection
	command resource.ResourceCreateCommand
}

type CreateWorkspaceResult struct {
	Workspace workspace.Workspace
	Owner     domainhr.User
}

type ResourceMapping struct {
	AppName      string
	ResourceType string
}

type ResourceMappings struct {
	Documents ResourceMapping
	Tasks     ResourceMapping
	Drive     ResourceMapping
}

type Option func(*WorkspaceService)

func WithClock(clock func() time.Time) Option {
	return func(s *WorkspaceService) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func WithIDGenerator(generator func() string) Option {
	return func(s *WorkspaceService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}

func WithResourceMappings(mappings ResourceMappings) Option {
	return func(s *WorkspaceService) {
		s.mappings = mappings
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *WorkspaceService) {
		if logger != nil {
			s.logger = logger
		}
	}
}

type WorkspaceService struct {
	repository  WorkspaceRepository
	hrClient    clienthr.Client
	publisher   ResourceCreateCommandPublisher
	mappings    ResourceMappings
	clock       func() time.Time
	idGenerator func() string
	logger      *slog.Logger
}

func NewWorkspaceService(repository WorkspaceRepository, hrClient clienthr.Client, publisher ResourceCreateCommandPublisher, opts ...Option) *WorkspaceService {
	service := &WorkspaceService{
		repository:  repository,
		hrClient:    hrClient,
		publisher:   publisher,
		clock:       func() time.Time { return time.Now().UTC() },
		idGenerator: uuid.NewString,
		logger:      slog.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *WorkspaceService) CreateWorkspace(ctx context.Context, input workspace.CreateInput) (CreateWorkspaceResult, error) {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return CreateWorkspaceResult{}, err
	}

	owner, err := s.hrClient.Get(ctx, input.OwnerNTAccount)
	if err != nil {
		return CreateWorkspaceResult{}, fmt.Errorf("%w: %w", ErrHRLookupFailed, err)
	}
	owner = owner.Normalize()

	now := s.clock()
	model := workspace.Workspace{
		ID:             s.idGenerator(),
		Name:           input.Name,
		Description:    input.Description,
		OwnerNTAccount: input.OwnerNTAccount,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	created, err := s.repository.Create(ctx, model)
	if err != nil {
		return CreateWorkspaceResult{}, fmt.Errorf("create workspace: %w", err)
	}

	s.publishResourceCreateCommands(ctx, created.ID, input)

	return CreateWorkspaceResult{Workspace: created, Owner: owner}, nil
}

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
