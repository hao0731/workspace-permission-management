package transport

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
)

type WorkspaceCreateRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Owner       string           `json:"owner"`
	Documents   *ResourceRequest `json:"documents"`
	Tasks       *ResourceRequest `json:"tasks"`
	Drive       *ResourceRequest `json:"drive"`
}

type WorkspaceFavoriteRequest struct {
	Favorite *bool `json:"favorite"`
}

type ResourceRequest struct {
	ResourceName string `json:"resource_name"`
}

func DecodeWorkspaceCreateRequest(body io.Reader) (WorkspaceCreateRequest, error) {
	var request WorkspaceCreateRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return WorkspaceCreateRequest{}, fmt.Errorf("decode workspace create request: %w", err)
	}
	return request, nil
}

func DecodeWorkspaceFavoriteRequest(body io.Reader) (WorkspaceFavoriteRequest, error) {
	var request WorkspaceFavoriteRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return WorkspaceFavoriteRequest{}, fmt.Errorf("decode workspace favorite request: %w", err)
	}
	return request, nil
}

func (request WorkspaceCreateRequest) ToDomain() (workspace.CreateInput, error) {
	input := workspace.CreateInput{
		Name:           request.Name,
		Description:    request.Description,
		OwnerNTAccount: request.Owner,
		Documents:      newDomainResourceRequest(request.Documents),
		Tasks:          newDomainResourceRequest(request.Tasks),
		Drive:          newDomainResourceRequest(request.Drive),
	}.Normalize()
	if err := input.Validate(); err != nil {
		return workspace.CreateInput{}, err
	}
	return input, nil
}

func (request WorkspaceFavoriteRequest) ToDomain(workspaceID string, ntAccount string) (workspace.FavoriteInput, error) {
	if request.Favorite == nil {
		return workspace.FavoriteInput{}, fmt.Errorf("%w: favorite is required", workspace.ErrInvalidInput)
	}
	input := workspace.FavoriteInput{
		WorkspaceID: workspaceID,
		NTAccount:   ntAccount,
		Favorite:    *request.Favorite,
	}.Normalize()
	if err := input.Validate(); err != nil {
		return workspace.FavoriteInput{}, err
	}
	return input, nil
}

func newDomainResourceRequest(request *ResourceRequest) *workspace.ResourceRequest {
	if request == nil {
		return nil
	}
	return &workspace.ResourceRequest{ResourceName: request.ResourceName}
}
