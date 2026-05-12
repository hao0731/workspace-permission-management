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
