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
	input         workspace.Workspace
	calls         int
	err           error
	getQuery      workspace.GetQuery
	getCalls      int
	getWorkspace  workspace.Workspace
	getFound      bool
	getErr        error
	favoriteInput workspace.UserFavoriteWorkspace
	favoriteCalls int
	favoriteErr   error
	deleteInput   workspace.FavoriteInput
	deleteCalls   int
	deleteErr     error
}

func (f *fakeWorkspaceRepository) Create(_ context.Context, input workspace.Workspace) (workspace.Workspace, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return workspace.Workspace{}, f.err
	}
	return input, nil
}

func (f *fakeWorkspaceRepository) Get(_ context.Context, query workspace.GetQuery) (workspace.Workspace, bool, error) {
	f.getCalls++
	f.getQuery = query
	if f.getErr != nil {
		return workspace.Workspace{}, false, f.getErr
	}
	return f.getWorkspace, f.getFound, nil
}

func (f *fakeWorkspaceRepository) UpsertFavorite(_ context.Context, input workspace.UserFavoriteWorkspace) error {
	f.favoriteCalls++
	f.favoriteInput = input
	if f.favoriteErr != nil {
		return f.favoriteErr
	}
	return nil
}

func (f *fakeWorkspaceRepository) DeleteFavorite(_ context.Context, input workspace.FavoriteInput) error {
	f.deleteCalls++
	f.deleteInput = input
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
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

func TestWorkspaceServiceGetWorkspaceFound(t *testing.T) {
	repo := &fakeWorkspaceRepository{
		getWorkspace: workspace.Workspace{
			ID:             "workspace-1",
			Name:           "Planning",
			Description:    "Planning workspace",
			OwnerNTAccount: "user1",
		},
		getFound: true,
	}
	hrClient := &fakeHRClient{user: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"}}
	service := NewWorkspaceService(repo, hrClient, nil)

	result, err := service.GetWorkspace(context.Background(), workspace.GetQuery{ID: " workspace-1 "})
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if !result.Found {
		t.Fatal("GetWorkspace() Found = false, want true")
	}
	if result.Workspace.ID != "workspace-1" || result.Owner.DisplayName != "Test User 測試員" {
		t.Fatalf("result = %+v", result)
	}
	if repo.getQuery.ID != "workspace-1" {
		t.Fatalf("repo query = %+v, want trimmed workspace id", repo.getQuery)
	}
	if hrClient.calls != 1 || hrClient.input != "user1" {
		t.Fatalf("hr calls=%d input=%q, want 1/user1", hrClient.calls, hrClient.input)
	}
}

func TestWorkspaceServiceGetWorkspaceMissingDoesNotCallHR(t *testing.T) {
	repo := &fakeWorkspaceRepository{getFound: false}
	hrClient := &fakeHRClient{user: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"}}
	service := NewWorkspaceService(repo, hrClient, nil)

	result, err := service.GetWorkspace(context.Background(), workspace.GetQuery{ID: "missing-workspace"})
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if result.Found {
		t.Fatalf("GetWorkspace() Found = true with result %+v, want false", result)
	}
	if hrClient.calls != 0 {
		t.Fatalf("hr calls = %d, want 0", hrClient.calls)
	}
}

func TestWorkspaceServiceGetWorkspaceHRFailure(t *testing.T) {
	repo := &fakeWorkspaceRepository{
		getWorkspace: workspace.Workspace{
			ID:             "workspace-1",
			Name:           "Planning",
			Description:    "Planning workspace",
			OwnerNTAccount: "user1",
		},
		getFound: true,
	}
	hrClient := &fakeHRClient{err: errors.New("hr unavailable")}
	service := NewWorkspaceService(repo, hrClient, nil)

	_, err := service.GetWorkspace(context.Background(), workspace.GetQuery{ID: "workspace-1"})
	if !errors.Is(err, ErrHRLookupFailed) {
		t.Fatalf("GetWorkspace() error = %v, want ErrHRLookupFailed", err)
	}
}

func TestWorkspaceServiceGetWorkspaceRepositoryFailure(t *testing.T) {
	repo := &fakeWorkspaceRepository{getErr: errors.New("find failed")}
	hrClient := &fakeHRClient{}
	service := NewWorkspaceService(repo, hrClient, nil)

	_, err := service.GetWorkspace(context.Background(), workspace.GetQuery{ID: "workspace-1"})
	if err == nil {
		t.Fatal("GetWorkspace() error = nil, want error")
	}
	if hrClient.calls != 0 {
		t.Fatalf("hr calls = %d, want 0", hrClient.calls)
	}
}

func TestWorkspaceServiceSetWorkspaceFavoriteUpsertsWhenWorkspaceExists(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	repo := &fakeWorkspaceRepository{getFound: true}
	hrClient := &fakeHRClient{}
	service := NewWorkspaceService(repo, hrClient, nil,
		WithClock(func() time.Time { return now }),
		WithIDGenerator(sequenceIDs("favorite-1")),
	)

	err := service.SetWorkspaceFavorite(context.Background(), workspace.FavoriteInput{
		WorkspaceID: " workspace-1 ",
		NTAccount:   " user1 ",
		Favorite:    true,
	})
	if err != nil {
		t.Fatalf("SetWorkspaceFavorite() error = %v, want nil", err)
	}
	if repo.getCalls != 1 || repo.getQuery.ID != "workspace-1" {
		t.Fatalf("get calls=%d query=%+v, want workspace existence check", repo.getCalls, repo.getQuery)
	}
	if repo.favoriteCalls != 1 {
		t.Fatalf("favorite calls = %d, want 1", repo.favoriteCalls)
	}
	if repo.favoriteInput.ID != "favorite-1" || repo.favoriteInput.NTAccount != "user1" || repo.favoriteInput.WorkspaceID != "workspace-1" {
		t.Fatalf("favorite input = %+v", repo.favoriteInput)
	}
	if !repo.favoriteInput.CreatedAt.Equal(now) || !repo.favoriteInput.UpdatedAt.Equal(now) {
		t.Fatalf("favorite timestamps = %+v, want %v", repo.favoriteInput, now)
	}
	if hrClient.calls != 0 {
		t.Fatalf("hr calls = %d, want 0", hrClient.calls)
	}
}

func TestWorkspaceServiceSetWorkspaceFavoriteMissingWorkspace(t *testing.T) {
	repo := &fakeWorkspaceRepository{getFound: false}
	service := NewWorkspaceService(repo, &fakeHRClient{}, nil)

	err := service.SetWorkspaceFavorite(context.Background(), workspace.FavoriteInput{
		WorkspaceID: "missing-workspace",
		NTAccount:   "user1",
		Favorite:    true,
	})
	if !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("SetWorkspaceFavorite() error = %v, want ErrNotFound", err)
	}
	if repo.favoriteCalls != 0 || repo.deleteCalls != 0 {
		t.Fatalf("favorite calls=%d delete calls=%d, want no favorite write", repo.favoriteCalls, repo.deleteCalls)
	}
}

func TestWorkspaceServiceClearWorkspaceFavoriteDeletesWhenWorkspaceExists(t *testing.T) {
	repo := &fakeWorkspaceRepository{getFound: true}
	service := NewWorkspaceService(repo, &fakeHRClient{}, nil)

	err := service.SetWorkspaceFavorite(context.Background(), workspace.FavoriteInput{
		WorkspaceID: " workspace-1 ",
		NTAccount:   " user1 ",
		Favorite:    false,
	})
	if err != nil {
		t.Fatalf("SetWorkspaceFavorite() error = %v, want nil", err)
	}
	if repo.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", repo.deleteCalls)
	}
	if repo.deleteInput.WorkspaceID != "workspace-1" || repo.deleteInput.NTAccount != "user1" {
		t.Fatalf("delete input = %+v, want trimmed identity", repo.deleteInput)
	}
	if repo.favoriteCalls != 0 {
		t.Fatalf("favorite calls = %d, want 0", repo.favoriteCalls)
	}
}

func TestWorkspaceServiceFavoriteRepositoryFailure(t *testing.T) {
	repo := &fakeWorkspaceRepository{getFound: true, favoriteErr: errors.New("favorite write failed")}
	service := NewWorkspaceService(repo, &fakeHRClient{}, nil, WithIDGenerator(sequenceIDs("favorite-1")))

	err := service.SetWorkspaceFavorite(context.Background(), workspace.FavoriteInput{
		WorkspaceID: "workspace-1",
		NTAccount:   "user1",
		Favorite:    true,
	})
	if err == nil {
		t.Fatal("SetWorkspaceFavorite() error = nil, want error")
	}
}

func TestWorkspaceServiceClearFavoriteRepositoryFailure(t *testing.T) {
	repo := &fakeWorkspaceRepository{getFound: true, deleteErr: errors.New("favorite delete failed")}
	service := NewWorkspaceService(repo, &fakeHRClient{}, nil)

	err := service.SetWorkspaceFavorite(context.Background(), workspace.FavoriteInput{
		WorkspaceID: "workspace-1",
		NTAccount:   "user1",
		Favorite:    false,
	})
	if err == nil {
		t.Fatal("SetWorkspaceFavorite() error = nil, want error")
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
