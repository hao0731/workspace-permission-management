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

type GroupGroupingRulesRequest struct {
	Rules          []RuleRequest `json:"rules"`
	ExpirationDate JSONTime      `json:"expiration_date"`
}

type IndividualMembersAddRequest struct {
	IndividualMembers []IndividualMemberRequest `json:"individual_members"`
}

type IndividualMemberExpirationUpdateRequest struct {
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
	rules, err := newDomainRules(request.GroupingRule.Rules)
	if err != nil {
		return group.CreateInput{}, err
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

func DecodeGroupGroupingRulesRequest(body io.Reader) (GroupGroupingRulesRequest, error) {
	var request GroupGroupingRulesRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return GroupGroupingRulesRequest{}, fmt.Errorf("decode group grouping rules request: %w", err)
	}
	return request, nil
}

func DecodeIndividualMembersAddRequest(body io.Reader) (IndividualMembersAddRequest, error) {
	var request IndividualMembersAddRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return IndividualMembersAddRequest{}, fmt.Errorf("decode individual members add request: %w", err)
	}
	return request, nil
}

func (request IndividualMembersAddRequest) ToDomain(workspaceID string, groupID string) (group.AddIndividualMembersInput, error) {
	members := make([]group.IndividualMember, 0, len(request.IndividualMembers))
	for _, member := range request.IndividualMembers {
		if member.ExpirationDate.Time.IsZero() {
			return group.AddIndividualMembersInput{}, invalidGroupRequest("individual member expiration_date is required")
		}
		members = append(members, group.IndividualMember{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate.Time,
		})
	}
	return group.AddIndividualMembersInput{
		WorkspaceID:       workspaceID,
		GroupID:           groupID,
		IndividualMembers: members,
	}, nil
}

func DecodeIndividualMemberExpirationUpdateRequest(body io.Reader) (IndividualMemberExpirationUpdateRequest, error) {
	var request IndividualMemberExpirationUpdateRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return IndividualMemberExpirationUpdateRequest{}, fmt.Errorf("decode individual member expiration update request: %w", err)
	}
	return request, nil
}

func (request IndividualMemberExpirationUpdateRequest) ToDomain(workspaceID string, groupID string, ntAccount string) (group.UpdateIndividualMemberExpirationInput, error) {
	if request.ExpirationDate.Time.IsZero() {
		return group.UpdateIndividualMemberExpirationInput{}, invalidGroupRequest("expiration_date is required")
	}
	return group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    workspaceID,
		GroupID:        groupID,
		NTAccount:      ntAccount,
		ExpirationDate: request.ExpirationDate.Time,
	}, nil
}

func (request GroupGroupingRulesRequest) ToDomain(workspaceID string, groupID string) (group.UpdateGroupingRuleInput, error) {
	rules, err := newDomainRules(request.Rules)
	if err != nil {
		return group.UpdateGroupingRuleInput{}, err
	}
	return group.UpdateGroupingRuleInput{
		WorkspaceID:    workspaceID,
		GroupID:        groupID,
		Rules:          rules,
		ExpirationDate: request.ExpirationDate.Time,
	}, nil
}

func newDomainRules(input []RuleRequest) ([]group.Rule, error) {
	rules := make([]group.Rule, 0, len(input))
	for _, rule := range input {
		if rule.Multi == nil {
			return nil, invalidGroupRequest("rule multi is required")
		}
		rules = append(rules, group.Rule{
			AttributeKey: rule.AttributeKey,
			Operator:     group.Operator(rule.Operator),
			Multi:        *rule.Multi,
			Value:        rule.Value,
		})
	}
	return rules, nil
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
