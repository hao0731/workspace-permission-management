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

type UpsertInput struct {
	ID          string
	WorkspaceID string
	FunctionKey string
	DisplayName string
	Type        string
	Tags        []string
	EventTime   time.Time
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

type Page struct {
	Resources   []Resource
	HasNextPage bool
	NextCursor  *Cursor
}

type UpsertStatus string

const (
	UpsertStatusInserted UpsertStatus = "inserted"
	UpsertStatusUpdated  UpsertStatus = "updated"
	UpsertStatusIgnored  UpsertStatus = "ignored"
)
