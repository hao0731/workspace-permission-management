package mockfunction

import "time"

type ResourceCreateCommand struct {
	WorkspaceID  string
	AppName      string
	ResourceName string
	ResourceType string
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
