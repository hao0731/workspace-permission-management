package workspace

import "time"

type Workspace struct {
	ID             string
	Name           string
	Description    string
	OwnerNTAccount string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ResourceSection string

const (
	ResourceSectionDocuments ResourceSection = "documents"
	ResourceSectionTasks     ResourceSection = "tasks"
	ResourceSectionDrive     ResourceSection = "drive"
)

type ResourceRequest struct {
	ResourceName string
}

type CreateInput struct {
	Name           string
	Description    string
	OwnerNTAccount string
	Documents      *ResourceRequest
	Tasks          *ResourceRequest
	Drive          *ResourceRequest
}

type GetQuery struct {
	ID string
}

type FavoriteInput struct {
	WorkspaceID string
	NTAccount   string
	Favorite    bool
}

type UserFavoriteWorkspace struct {
	ID          string
	NTAccount   string
	WorkspaceID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
