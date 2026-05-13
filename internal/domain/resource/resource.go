package resource

import "time"

type Resource struct {
	ID          string
	WorkspaceID string
	FunctionKey string
	DisplayName string
	Type        string
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ResourceCreateCommand struct {
	WorkspaceID  string
	AppName      string
	ResourceName string
	ResourceType string
	EventID      string
	EventTime    time.Time
}

func (c ResourceCreateCommand) Subject() string {
	return "cmd.app." + c.AppName + ".resource.create"
}

type ResourceUpsertEvent struct {
	ResourceID   string
	DisplayName  string
	ResourceType string
	ResourceTags []string
	FunctionKey  string
	WorkspaceID  string
	EventID      string
	EventTime    time.Time
}

func (e ResourceUpsertEvent) Subject() string {
	return "app." + e.FunctionKey + ".resource.upserted"
}

type Cursor struct {
	CreatedAt time.Time
	ID        string
}

type ListQuery struct {
	WorkspaceID string
	FunctionKey string
	Limit       int
	Cursor      *Cursor
}

type DeleteInput struct {
	WorkspaceID string
	FunctionKey string
	ResourceID  string
}

type DeletedEvent struct {
	WorkspaceID string
	FunctionKey string
	ResourceID  string
	EventID     string
	EventTime   time.Time
}

type Page struct {
	Resources   []Resource
	HasNextPage bool
	NextCursor  *Cursor
}

type UpsertStatus string

type DeleteStatus string

const (
	UpsertStatusInserted UpsertStatus = "inserted"
	UpsertStatusUpdated  UpsertStatus = "updated"
	UpsertStatusIgnored  UpsertStatus = "ignored"
)

const (
	DeleteStatusDeleted  DeleteStatus = "deleted"
	DeleteStatusNotFound DeleteStatus = "not_found"
)
