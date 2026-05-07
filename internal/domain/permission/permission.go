package permission

import "time"

type Permission struct {
	ID               string
	WorkspaceID      string
	FunctionKey      string
	OfficePermission PermissionSection
	RemotePermission PermissionSection
}

type SaveInput struct {
	WorkspaceID      string
	FunctionKey      string
	OfficePermission *PermissionSection
	RemotePermission *PermissionSection
}

type PermissionSection struct {
	BaselineRule BaselineRule
	ExtraRules   []ExtraRule
}

type BaselineRule struct {
	ActionID     string
	ResourceTags []string
	Enabled      bool
}

type ExtraRule struct {
	RuleID         string
	GroupIDs       []string
	ActionID       string
	ResourceTags   []string
	ExpirationDate time.Time
}
