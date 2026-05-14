package transport

import (
	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
)

type WorkspaceCreateResponse struct {
	Workspace WorkspaceResponse `json:"workspace"`
}

type WorkspaceGetResponse struct {
	Workspace *WorkspaceResponse `json:"workspace"`
}

type WorkspaceResponse struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Owner       OwnerResponse `json:"owner"`
}

type OwnerResponse struct {
	NTAccount   string `json:"nt_account"`
	DisplayName string `json:"display_name"`
}

func NewWorkspaceCreateResponse(workspace workspace.Workspace, owner domainhr.User) WorkspaceCreateResponse {
	return WorkspaceCreateResponse{Workspace: newWorkspaceResponse(workspace, owner)}
}

func NewWorkspaceGetResponse(workspace workspace.Workspace, owner domainhr.User) WorkspaceGetResponse {
	response := newWorkspaceResponse(workspace, owner)
	return WorkspaceGetResponse{Workspace: &response}
}

func NewWorkspaceGetNotFoundResponse() WorkspaceGetResponse {
	return WorkspaceGetResponse{}
}

func newWorkspaceResponse(workspace workspace.Workspace, owner domainhr.User) WorkspaceResponse {
	workspace = workspace.Normalize()
	owner = owner.Normalize()
	return WorkspaceResponse{
		ID:          workspace.ID,
		Name:        workspace.Name,
		Description: workspace.Description,
		Owner: OwnerResponse{
			NTAccount:   owner.NTAccount,
			DisplayName: owner.DisplayName,
		},
	}
}
