package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type GroupCreateRequest struct {
	Name              string                    `json:"name"`
	Description       string                    `json:"description"`
	GroupingRule      *GroupingRuleRequest      `json:"grouping_rule"`
	IndividualMembers []IndividualMemberRequest `json:"individual_members"`
}

type GroupingRuleRequest struct {
	Rules          []RuleRequest `json:"rules"`
	ExpirationDate JSONTime      `json:"expiration_date"`
}

type RuleRequest struct {
	AttributeKey string `json:"attribute_key"`
	Operator     string `json:"operator"`
	Multi        *bool  `json:"multi"`
	Value        any    `json:"value"`
}

type IndividualMemberRequest struct {
	NTAccount      string   `json:"nt_account"`
	ExpirationDate JSONTime `json:"expiration_date"`
}

func DecodeGroupCreateRequest(body io.Reader) (GroupCreateRequest, error) {
	var request GroupCreateRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return GroupCreateRequest{}, fmt.Errorf("decode group create request: %w", err)
	}
	return request, nil
}

func (request GroupCreateRequest) ToDomain(workspaceID string) (group.CreateInput, error) {
	if request.GroupingRule == nil {
		return group.CreateInput{}, invalidGroupRequest("grouping rule is required")
	}
	rules := make([]group.Rule, 0, len(request.GroupingRule.Rules))
	for _, rule := range request.GroupingRule.Rules {
		if rule.Multi == nil {
			return group.CreateInput{}, invalidGroupRequest("rule multi is required")
		}
		rules = append(rules, group.Rule{
			AttributeKey: rule.AttributeKey,
			Operator:     group.Operator(rule.Operator),
			Multi:        *rule.Multi,
			Value:        rule.Value,
		})
	}
	members := make([]group.IndividualMember, 0, len(request.IndividualMembers))
	for _, member := range request.IndividualMembers {
		members = append(members, group.IndividualMember{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate.Time,
		})
	}
	return group.CreateInput{
		WorkspaceID:       workspaceID,
		Name:              request.Name,
		Description:       request.Description,
		GroupingRule:      group.GroupingRule{Rules: rules, ExpirationDate: request.GroupingRule.ExpirationDate.Time},
		IndividualMembers: members,
	}, nil
}

func invalidGroupRequest(message string) error {
	return fmt.Errorf("%w: %s", group.ErrInvalidInput, message)
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
