package transport

import (
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

type PermissionSaveResponse struct {
	Permissions PermissionResponse `json:"permissions"`
}

type PermissionResponse struct {
	OfficePermission PermissionSectionResponse `json:"office_permission"`
	RemotePermission PermissionSectionResponse `json:"remote_permission"`
}

type PermissionSectionResponse struct {
	BaselineRule BaselineRuleResponse `json:"baseline_rule"`
	ExtraRules   []ExtraRuleResponse  `json:"extra_rules"`
}

type BaselineRuleResponse struct {
	ActionID     string   `json:"action_id"`
	ResourceTags []string `json:"resource_tags"`
	Enabled      bool     `json:"enabled"`
}

type ExtraRuleResponse struct {
	RuleID         string    `json:"rule_id"`
	GroupIDs       []string  `json:"group_ids"`
	ActionID       string    `json:"action_id"`
	ResourceTags   []string  `json:"resource_tags"`
	ExpirationDate time.Time `json:"expiration_date"`
}

func NewPermissionSaveResponse(model permission.Permission) PermissionSaveResponse {
	return PermissionSaveResponse{
		Permissions: PermissionResponse{
			OfficePermission: permissionSectionResponse(model.OfficePermission),
			RemotePermission: permissionSectionResponse(model.RemotePermission),
		},
	}
}

func permissionSectionResponse(section permission.PermissionSection) PermissionSectionResponse {
	extraRules := make([]ExtraRuleResponse, 0, len(section.ExtraRules))
	for _, rule := range section.ExtraRules {
		extraRules = append(extraRules, ExtraRuleResponse{
			RuleID:         rule.RuleID,
			GroupIDs:       append([]string(nil), rule.GroupIDs...),
			ActionID:       rule.ActionID,
			ResourceTags:   append([]string(nil), rule.ResourceTags...),
			ExpirationDate: rule.ExpirationDate,
		})
	}
	return PermissionSectionResponse{
		BaselineRule: BaselineRuleResponse{
			ActionID:     section.BaselineRule.ActionID,
			ResourceTags: append([]string(nil), section.BaselineRule.ResourceTags...),
			Enabled:      section.BaselineRule.Enabled,
		},
		ExtraRules: extraRules,
	}
}
