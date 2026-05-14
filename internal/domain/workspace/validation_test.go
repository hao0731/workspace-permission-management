package workspace

import (
	"errors"
	"testing"
	"time"
)

func TestCreateInputValidateRejectsRequiredFields(t *testing.T) {
	tests := []CreateInput{
		{Description: "desc", OwnerNTAccount: "user1"},
		{Name: "name", OwnerNTAccount: "user1"},
		{Name: "name", Description: "desc"},
		{Name: "name", Description: "desc", OwnerNTAccount: "user1", Documents: &ResourceRequest{}},
	}
	for _, input := range tests {
		if err := input.Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate() error = %v, want ErrInvalidInput", err)
		}
	}
}

func TestCreateInputNormalize(t *testing.T) {
	input := CreateInput{
		Name:           " Project ",
		Description:    " Description ",
		OwnerNTAccount: " user1 ",
		Documents:      &ResourceRequest{ResourceName: " Docs "},
	}
	normalized := input.Normalize()
	if normalized.Name != "Project" || normalized.Description != "Description" || normalized.OwnerNTAccount != "user1" || normalized.Documents.ResourceName != "Docs" {
		t.Fatalf("Normalize() = %+v", normalized)
	}
}

func TestCreateInputValidateAcceptsOmittedResources(t *testing.T) {
	input := CreateInput{Name: "Project", Description: "Description", OwnerNTAccount: "user1"}
	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestGetQueryNormalize(t *testing.T) {
	query := GetQuery{ID: " workspace-1 "}

	normalized := query.Normalize()

	if normalized.ID != "workspace-1" {
		t.Fatalf("Normalize().ID = %q, want workspace-1", normalized.ID)
	}
}

func TestGetQueryValidateRejectsEmptyID(t *testing.T) {
	query := GetQuery{ID: "   "}

	if err := query.Normalize().Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Validate() error = %v, want ErrInvalidInput", err)
	}
}

func TestGetQueryValidateAcceptsWorkspaceID(t *testing.T) {
	query := GetQuery{ID: "workspace-1"}

	if err := query.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestFavoriteInputNormalize(t *testing.T) {
	input := FavoriteInput{
		WorkspaceID: " workspace-1 ",
		NTAccount:   " user1 ",
		Favorite:    false,
	}

	normalized := input.Normalize()

	if normalized.WorkspaceID != "workspace-1" || normalized.NTAccount != "user1" {
		t.Fatalf("Normalize() = %+v, want trimmed workspace/user", normalized)
	}
	if normalized.Favorite {
		t.Fatal("Normalize().Favorite = true, want false preserved")
	}
}

func TestFavoriteInputValidateRejectsEmptyIdentity(t *testing.T) {
	tests := []FavoriteInput{
		{NTAccount: "user1", Favorite: true},
		{WorkspaceID: "workspace-1", Favorite: true},
		{WorkspaceID: "   ", NTAccount: "user1", Favorite: true},
		{WorkspaceID: "workspace-1", NTAccount: "   ", Favorite: true},
	}
	for _, input := range tests {
		if err := input.Normalize().Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate() error = %v, want ErrInvalidInput for input %+v", err, input)
		}
	}
}

func TestFavoriteInputValidateAcceptsFavoriteFalse(t *testing.T) {
	input := FavoriteInput{WorkspaceID: "workspace-1", NTAccount: "user1", Favorite: false}

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestUserFavoriteWorkspaceNormalize(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	model := UserFavoriteWorkspace{
		ID:          " favorite-1 ",
		NTAccount:   " user1 ",
		WorkspaceID: " workspace-1 ",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	normalized := model.Normalize()

	if normalized.ID != "favorite-1" || normalized.NTAccount != "user1" || normalized.WorkspaceID != "workspace-1" {
		t.Fatalf("Normalize() = %+v, want trimmed identity fields", normalized)
	}
}

func TestUserFavoriteWorkspaceValidateRejectsMissingFields(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	tests := []UserFavoriteWorkspace{
		{NTAccount: "user1", WorkspaceID: "workspace-1", CreatedAt: now, UpdatedAt: now},
		{ID: "favorite-1", WorkspaceID: "workspace-1", CreatedAt: now, UpdatedAt: now},
		{ID: "favorite-1", NTAccount: "user1", CreatedAt: now, UpdatedAt: now},
		{ID: "favorite-1", NTAccount: "user1", WorkspaceID: "workspace-1", UpdatedAt: now},
		{ID: "favorite-1", NTAccount: "user1", WorkspaceID: "workspace-1", CreatedAt: now},
	}
	for _, model := range tests {
		if err := model.Normalize().Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate() error = %v, want ErrInvalidInput for model %+v", err, model)
		}
	}
}
