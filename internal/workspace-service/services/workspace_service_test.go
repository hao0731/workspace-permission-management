package services

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
)

type fakeWorkspaceRepository struct {
	input workspace.Workspace
	calls int
	err   error
}

func (f *fakeWorkspaceRepository) Create(_ context.Context, input workspace.Workspace) (workspace.Workspace, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return workspace.Workspace{}, f.err
	}
	return input, nil
}

type fakeHRClient struct {
	user  domainhr.User
	err   error
	calls int
	input string
}

func (f *fakeHRClient) Get(_ context.Context, ntAccount string) (domainhr.User, error) {
	f.calls++
	f.input = ntAccount
	if f.err != nil {
		return domainhr.User{}, f.err
	}
	return f.user, nil
}

func (f *fakeHRClient) BatchGet(context.Context, []string) ([]domainhr.User, error) {
	return nil, nil
}

type fakeCommandPublisher struct {
	commands []resource.ResourceCreateCommand
	errs     []error
}

func (f *fakeCommandPublisher) PublishResourceCreateCommand(_ context.Context, command resource.ResourceCreateCommand) error {
	f.commands = append(f.commands, command)
	if len(f.errs) == 0 {
		return nil
	}
	err := f.errs[0]
	f.errs = f.errs[1:]
	return err
}

func TestWorkspaceServiceCreateWorkspace(t *testing.T) {
	repo := &fakeWorkspaceRepository{}
	hrClient := &fakeHRClient{user: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"}}
	publisher := &fakeCommandPublisher{}
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	service := NewWorkspaceService(repo, hrClient, publisher,
		WithClock(func() time.Time { return now }),
		WithIDGenerator(sequenceIDs("workspace-1", "event-1", "event-2", "event-3")),
		WithResourceMappings(ResourceMappings{
			Documents: ResourceMapping{AppName: "documents", ResourceType: "document"},
			Tasks:     ResourceMapping{AppName: "tasks", ResourceType: "task"},
			Drive:     ResourceMapping{AppName: "drive", ResourceType: "file"},
		}),
	)

	result, err := service.CreateWorkspace(context.Background(), workspace.CreateInput{
		Name:           "Planning",
		Description:    "Planning workspace",
		OwnerNTAccount: "user1",
		Documents:      &workspace.ResourceRequest{ResourceName: "Docs"},
		Tasks:          &workspace.ResourceRequest{ResourceName: "Tasks"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	if result.Workspace.ID != "workspace-1" || result.Owner.DisplayName != "Test User 測試員" {
		t.Fatalf("result = %+v", result)
	}
	if repo.input.OwnerNTAccount != "user1" {
		t.Fatalf("repo input = %+v", repo.input)
	}
	if len(publisher.commands) != 2 {
		t.Fatalf("commands = %+v, want 2", publisher.commands)
	}
	if publisher.commands[0].AppName != "documents" || publisher.commands[0].ResourceType != "document" {
		t.Fatalf("documents command = %+v", publisher.commands[0])
	}
	if publisher.commands[1].AppName != "tasks" || publisher.commands[1].ResourceType != "task" {
		t.Fatalf("commands order = %+v", publisher.commands)
	}
}

func TestWorkspaceServiceHRFailureDoesNotPersistOrPublish(t *testing.T) {
	repo := &fakeWorkspaceRepository{}
	hrClient := &fakeHRClient{err: errors.New("hr unavailable")}
	publisher := &fakeCommandPublisher{}
	service := NewWorkspaceService(repo, hrClient, publisher)

	_, err := service.CreateWorkspace(context.Background(), workspace.CreateInput{
		Name:           "Planning",
		Description:    "Planning workspace",
		OwnerNTAccount: "user1",
		Documents:      &workspace.ResourceRequest{ResourceName: "Docs"},
	})
	if !errors.Is(err, ErrHRLookupFailed) {
		t.Fatalf("CreateWorkspace() error = %v, want ErrHRLookupFailed", err)
	}
	if repo.calls != 0 || len(publisher.commands) != 0 {
		t.Fatalf("repo calls=%d commands=%d, want 0/0", repo.calls, len(publisher.commands))
	}
}

func TestWorkspaceServiceRepositoryFailureDoesNotPublish(t *testing.T) {
	repo := &fakeWorkspaceRepository{err: errors.New("insert failed")}
	hrClient := &fakeHRClient{user: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"}}
	publisher := &fakeCommandPublisher{}
	service := NewWorkspaceService(repo, hrClient, publisher, WithIDGenerator(sequenceIDs("workspace-1")))

	_, err := service.CreateWorkspace(context.Background(), workspace.CreateInput{
		Name:           "Planning",
		Description:    "Planning workspace",
		OwnerNTAccount: "user1",
		Documents:      &workspace.ResourceRequest{ResourceName: "Docs"},
	})
	if err == nil {
		t.Fatal("CreateWorkspace() error = nil, want error")
	}
	if len(publisher.commands) != 0 {
		t.Fatalf("commands = %+v, want none", publisher.commands)
	}
}

func TestWorkspaceServicePublishFailureReturnsSuccessAndContinues(t *testing.T) {
	var logBuffer bytes.Buffer
	repo := &fakeWorkspaceRepository{}
	hrClient := &fakeHRClient{user: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"}}
	publisher := &fakeCommandPublisher{errs: []error{errors.New("publish documents failed"), nil}}
	service := NewWorkspaceService(repo, hrClient, publisher,
		WithLogger(slog.New(slog.NewTextHandler(&logBuffer, nil))),
		WithIDGenerator(sequenceIDs("workspace-1", "event-1", "event-2")),
		WithResourceMappings(ResourceMappings{
			Documents: ResourceMapping{AppName: "documents", ResourceType: "document"},
			Tasks:     ResourceMapping{AppName: "tasks", ResourceType: "task"},
		}),
	)

	result, err := service.CreateWorkspace(context.Background(), workspace.CreateInput{
		Name:           "Planning",
		Description:    "Planning workspace",
		OwnerNTAccount: "user1",
		Documents:      &workspace.ResourceRequest{ResourceName: "Docs"},
		Tasks:          &workspace.ResourceRequest{ResourceName: "Tasks"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v, want nil", err)
	}
	if result.Workspace.ID != "workspace-1" {
		t.Fatalf("workspace id = %q", result.Workspace.ID)
	}
	if len(publisher.commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(publisher.commands))
	}
	if !strings.Contains(logBuffer.String(), "failed to publish resource create command") {
		t.Fatalf("log = %s", logBuffer.String())
	}
}

func sequenceIDs(ids ...string) func() string {
	index := 0
	return func() string {
		if index >= len(ids) {
			return "extra-id"
		}
		id := ids[index]
		index++
		return id
	}
}
