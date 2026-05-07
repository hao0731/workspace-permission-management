package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

type PermissionSaveRequest struct {
	OfficePermission *PermissionSectionRequest `json:"office_permission"`
	RemotePermission *PermissionSectionRequest `json:"remote_permission"`
}

type PermissionSectionRequest struct {
	BaselineRule *BaselineRuleRequest `json:"baseline_rule"`
	ExtraRules   []ExtraRuleRequest   `json:"extra_rules"`
}

type BaselineRuleRequest struct {
	ActionID     string   `json:"action_id"`
	ResourceTags []string `json:"resource_tags"`
	Enabled      *bool    `json:"enabled"`
}

type ExtraRuleRequest struct {
	RuleID         string   `json:"rule_id,omitempty"`
	GroupIDs       []string `json:"group_ids"`
	ActionID       string   `json:"action_id"`
	ResourceTags   []string `json:"resource_tags"`
	ExpirationDate JSONTime `json:"expiration_date"`
}

func DecodePermissionSaveRequest(body io.Reader) (PermissionSaveRequest, error) {
	var request PermissionSaveRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return PermissionSaveRequest{}, fmt.Errorf("decode permission request: %w", err)
	}
	return request, nil
}

func (request PermissionSaveRequest) ToDomain(workspaceID, functionKey string) (permission.SaveInput, error) {
	officePermission, err := requestSectionToDomain("office", request.OfficePermission)
	if err != nil {
		return permission.SaveInput{}, err
	}
	remotePermission, err := requestSectionToDomain("remote", request.RemotePermission)
	if err != nil {
		return permission.SaveInput{}, err
	}
	return permission.SaveInput{
		WorkspaceID:      workspaceID,
		FunctionKey:      functionKey,
		OfficePermission: officePermission,
		RemotePermission: remotePermission,
	}, nil
}

func requestSectionToDomain(label string, request *PermissionSectionRequest) (*permission.PermissionSection, error) {
	if request == nil {
		return nil, invalidPermissionRequest(fmt.Sprintf("%s permission is required", label))
	}
	if request.BaselineRule == nil {
		return nil, invalidPermissionRequest(fmt.Sprintf("%s baseline rule is required", label))
	}
	if request.BaselineRule.Enabled == nil {
		return nil, invalidPermissionRequest(fmt.Sprintf("%s baseline enabled is required", label))
	}
	extraRules := make([]permission.ExtraRule, 0, len(request.ExtraRules))
	for _, rule := range request.ExtraRules {
		extraRules = append(extraRules, permission.ExtraRule{
			RuleID:         rule.RuleID,
			GroupIDs:       append([]string(nil), rule.GroupIDs...),
			ActionID:       rule.ActionID,
			ResourceTags:   append([]string(nil), rule.ResourceTags...),
			ExpirationDate: rule.ExpirationDate.Time,
		})
	}
	return &permission.PermissionSection{
		BaselineRule: permission.BaselineRule{
			ActionID:     request.BaselineRule.ActionID,
			ResourceTags: append([]string(nil), request.BaselineRule.ResourceTags...),
			Enabled:      *request.BaselineRule.Enabled,
		},
		ExtraRules: extraRules,
	}, nil
}

func invalidPermissionRequest(message string) error {
	return fmt.Errorf("%w: %s", permission.ErrInvalidInput, message)
}

type JSONTime struct {
	Time time.Time
}

func (t *JSONTime) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("expiration_date must be RFC3339 timestamp")
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return fmt.Errorf("expiration_date must be RFC3339 timestamp")
	}
	t.Time = parsed
	return nil
}
