package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

type fakePermissionRepository struct {
	input    permission.Permission
	query    permission.GetQuery
	model    permission.Permission
	found    bool
	calls    int
	getCalls int
	err      error
	getErr   error
}

func (f *fakePermissionRepository) Save(ctx context.Context, input permission.Permission) (permission.Permission, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return permission.Permission{}, f.err
	}
	return input, nil
}

func (f *fakePermissionRepository) Get(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error) {
	f.getCalls++
	f.query = query
	if f.getErr != nil {
		return permission.Permission{}, false, f.getErr
	}
	return f.model, f.found, nil
}

func validPermissionSaveInput() permission.SaveInput {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return permission.SaveInput{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		OfficePermission: &permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
			ExtraRules: []permission.ExtraRule{
				{
					RuleID:         "rule-provided",
					GroupIDs:       []string{"group-1"},
					ActionID:       "edit",
					ResourceTags:   []string{"section_1"},
					ExpirationDate: expiration,
				},
				{
					GroupIDs:       []string{"group-2"},
					ActionID:       "delete",
					ResourceTags:   []string{"section_2"},
					ExpirationDate: expiration,
				},
			},
		},
		RemotePermission: &permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	}
}

func TestPermissionServiceSavePermissionGeneratesIDs(t *testing.T) {
	repo := &fakePermissionRepository{}
	ids := []string{"permission-1", "rule-generated-1"}
	service := NewPermissionService(repo, WithPermissionIDGenerator(func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}))

	got, err := service.SavePermission(context.Background(), validPermissionSaveInput())
	if err != nil {
		t.Fatalf("SavePermission error = %v, want nil", err)
	}
	if got.ID != "permission-1" {
		t.Fatalf("permission id = %q, want permission-1", got.ID)
	}
	if repo.input.OfficePermission.ExtraRules[0].RuleID != "rule-provided" {
		t.Fatalf("first rule id = %q, want rule-provided", repo.input.OfficePermission.ExtraRules[0].RuleID)
	}
	if repo.input.OfficePermission.ExtraRules[1].RuleID != "rule-generated-1" {
		t.Fatalf("second rule id = %q, want rule-generated-1", repo.input.OfficePermission.ExtraRules[1].RuleID)
	}
	if repo.input.RemotePermission.BaselineRule.Enabled {
		t.Fatal("remote baseline enabled = true, want false")
	}
}

func TestPermissionServiceSavePermissionDeduplicatesSemanticExtraRules(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	input := validPermissionSaveInput()
	input.OfficePermission.ExtraRules = []permission.ExtraRule{
		{
			GroupIDs:       []string{"group-1", "group-2"},
			ActionID:       "edit",
			ResourceTags:   []string{"section_1"},
			ExpirationDate: expiration,
		},
		{
			RuleID:         "dropped-rule",
			GroupIDs:       []string{"group-2", "group-1"},
			ActionID:       "edit",
			ResourceTags:   []string{"section_1"},
			ExpirationDate: expiration,
		},
	}
	repo := &fakePermissionRepository{}
	ids := []string{"permission-1", "rule-generated-1"}
	service := NewPermissionService(repo, WithPermissionIDGenerator(func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}))

	got, err := service.SavePermission(context.Background(), input)
	if err != nil {
		t.Fatalf("SavePermission error = %v, want nil", err)
	}
	if len(got.OfficePermission.ExtraRules) != 1 {
		t.Fatalf("office extra rules len = %d, want 1", len(got.OfficePermission.ExtraRules))
	}
	if got.OfficePermission.ExtraRules[0].RuleID != "rule-generated-1" {
		t.Fatalf("kept rule id = %q, want generated id for first rule", got.OfficePermission.ExtraRules[0].RuleID)
	}
}

func TestPermissionServiceSavePermissionRejectsInvalidInput(t *testing.T) {
	repo := &fakePermissionRepository{}
	service := NewPermissionService(repo)
	input := validPermissionSaveInput()
	input.WorkspaceID = ""

	_, err := service.SavePermission(context.Background(), input)
	if !errors.Is(err, permission.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if repo.calls != 0 {
		t.Fatalf("repository calls = %d, want 0", repo.calls)
	}
}

func TestPermissionServiceSavePermissionWrapsRepositoryError(t *testing.T) {
	repo := &fakePermissionRepository{err: errors.New("database unavailable")}
	service := NewPermissionService(repo)

	_, err := service.SavePermission(context.Background(), validPermissionSaveInput())
	if err == nil {
		t.Fatal("SavePermission error = nil, want error")
	}
}

func TestPermissionServiceSavePermissionAssignsTimestamps(t *testing.T) {
	repo := &fakePermissionRepository{}
	now := time.Date(2026, 5, 9, 1, 2, 3, 0, time.UTC)
	ids := []string{"permission-1", "rule-generated-1"}
	service := NewPermissionService(repo,
		WithPermissionIDGenerator(func() string {
			id := ids[0]
			ids = ids[1:]
			return id
		}),
		WithPermissionClock(func() time.Time {
			return now
		}),
	)

	got, err := service.SavePermission(context.Background(), validPermissionSaveInput())
	if err != nil {
		t.Fatalf("SavePermission error = %v, want nil", err)
	}
	if !got.CreatedAt.Equal(now) || !got.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = %s/%s, want %s", got.CreatedAt, got.UpdatedAt, now)
	}
	if !repo.input.CreatedAt.Equal(now) || !repo.input.UpdatedAt.Equal(now) {
		t.Fatalf("repository timestamps = %s/%s, want %s", repo.input.CreatedAt, repo.input.UpdatedAt, now)
	}
}

func TestPermissionServiceGetPermissionReturnsFoundModel(t *testing.T) {
	model := permission.Permission{
		ID:          "permission-1",
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
		OfficePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"section_1"},
				Enabled:      true,
			},
		},
		RemotePermission: permission.PermissionSection{
			BaselineRule: permission.BaselineRule{
				ActionID:     "view",
				ResourceTags: []string{"remote"},
				Enabled:      false,
			},
		},
	}
	repo := &fakePermissionRepository{model: model, found: true}
	service := NewPermissionService(repo)

	got, found, err := service.GetPermission(context.Background(), permission.GetQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
	})
	if err != nil {
		t.Fatalf("GetPermission error = %v, want nil", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if got.ID != "permission-1" {
		t.Fatalf("permission id = %q, want permission-1", got.ID)
	}
	if repo.query.WorkspaceID != "workspace-1" || repo.query.FunctionKey != "todo" {
		t.Fatalf("query = %+v, want workspace-1/todo", repo.query)
	}
}

func TestPermissionServiceGetPermissionReturnsNotFound(t *testing.T) {
	repo := &fakePermissionRepository{found: false}
	service := NewPermissionService(repo)

	_, found, err := service.GetPermission(context.Background(), permission.GetQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
	})
	if err != nil {
		t.Fatalf("GetPermission error = %v, want nil", err)
	}
	if found {
		t.Fatal("found = true, want false")
	}
}

func TestPermissionServiceGetPermissionRejectsInvalidQuery(t *testing.T) {
	repo := &fakePermissionRepository{}
	service := NewPermissionService(repo)

	_, _, err := service.GetPermission(context.Background(), permission.GetQuery{
		WorkspaceID: "",
		FunctionKey: "todo",
	})
	if !errors.Is(err, permission.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if repo.getCalls != 0 {
		t.Fatalf("repository get calls = %d, want 0", repo.getCalls)
	}
}

func TestPermissionServiceGetPermissionWrapsRepositoryError(t *testing.T) {
	repo := &fakePermissionRepository{getErr: errors.New("database unavailable")}
	service := NewPermissionService(repo)

	_, _, err := service.GetPermission(context.Background(), permission.GetQuery{
		WorkspaceID: "workspace-1",
		FunctionKey: "todo",
	})
	if err == nil {
		t.Fatal("GetPermission error = nil, want error")
	}
}
