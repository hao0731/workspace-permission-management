package workspace

import (
	"fmt"
	"strings"
)

func (w Workspace) Normalize() Workspace {
	w.ID = strings.TrimSpace(w.ID)
	w.Name = strings.TrimSpace(w.Name)
	w.Description = strings.TrimSpace(w.Description)
	w.OwnerNTAccount = strings.TrimSpace(w.OwnerNTAccount)
	return w
}

func (w Workspace) Validate() error {
	normalized := w.Normalize()
	if normalized.ID == "" {
		return invalidInput("workspace id is required")
	}
	if normalized.Name == "" {
		return invalidInput("workspace name is required")
	}
	if normalized.Description == "" {
		return invalidInput("workspace description is required")
	}
	if normalized.OwnerNTAccount == "" {
		return invalidInput("owner nt account is required")
	}
	if normalized.CreatedAt.IsZero() {
		return invalidInput("created at is required")
	}
	if normalized.UpdatedAt.IsZero() {
		return invalidInput("updated at is required")
	}
	return nil
}

func (r ResourceRequest) Normalize() ResourceRequest {
	r.ResourceName = strings.TrimSpace(r.ResourceName)
	return r
}

func (r ResourceRequest) Validate() error {
	normalized := r.Normalize()
	if normalized.ResourceName == "" {
		return invalidInput("resource name is required")
	}
	return nil
}

func (input CreateInput) Normalize() CreateInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.OwnerNTAccount = strings.TrimSpace(input.OwnerNTAccount)
	if input.Documents != nil {
		normalized := input.Documents.Normalize()
		input.Documents = &normalized
	}
	if input.Tasks != nil {
		normalized := input.Tasks.Normalize()
		input.Tasks = &normalized
	}
	if input.Drive != nil {
		normalized := input.Drive.Normalize()
		input.Drive = &normalized
	}
	return input
}

func (input CreateInput) Validate() error {
	normalized := input.Normalize()
	if normalized.Name == "" {
		return invalidInput("workspace name is required")
	}
	if normalized.Description == "" {
		return invalidInput("workspace description is required")
	}
	if normalized.OwnerNTAccount == "" {
		return invalidInput("owner nt account is required")
	}
	if normalized.Documents != nil {
		if err := normalized.Documents.Validate(); err != nil {
			return err
		}
	}
	if normalized.Tasks != nil {
		if err := normalized.Tasks.Validate(); err != nil {
			return err
		}
	}
	if normalized.Drive != nil {
		if err := normalized.Drive.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (query GetQuery) Normalize() GetQuery {
	query.ID = strings.TrimSpace(query.ID)
	return query
}

func (query GetQuery) Validate() error {
	normalized := query.Normalize()
	if normalized.ID == "" {
		return invalidInput("workspace id is required")
	}
	return nil
}

func (input FavoriteInput) Normalize() FavoriteInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.NTAccount = strings.TrimSpace(input.NTAccount)
	return input
}

func (input FavoriteInput) Validate() error {
	normalized := input.Normalize()
	if normalized.WorkspaceID == "" {
		return invalidInput("workspace id is required")
	}
	if normalized.NTAccount == "" {
		return invalidInput("nt account is required")
	}
	return nil
}

func (w UserFavoriteWorkspace) Normalize() UserFavoriteWorkspace {
	w.ID = strings.TrimSpace(w.ID)
	w.NTAccount = strings.TrimSpace(w.NTAccount)
	w.WorkspaceID = strings.TrimSpace(w.WorkspaceID)
	return w
}

func (w UserFavoriteWorkspace) Validate() error {
	normalized := w.Normalize()
	if normalized.ID == "" {
		return invalidInput("favorite id is required")
	}
	if normalized.NTAccount == "" {
		return invalidInput("nt account is required")
	}
	if normalized.WorkspaceID == "" {
		return invalidInput("workspace id is required")
	}
	if normalized.CreatedAt.IsZero() {
		return invalidInput("created at is required")
	}
	if normalized.UpdatedAt.IsZero() {
		return invalidInput("updated at is required")
	}
	return nil
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
