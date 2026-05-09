package transport

import (
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type GroupCreateResponse struct {
	Group GroupResponse `json:"group"`
}

type GroupResponse struct {
	ID                string                     `json:"id"`
	Name              string                     `json:"name"`
	Description       string                     `json:"description"`
	GroupingRule      GroupingRuleResponse       `json:"grouping_rule"`
	IndividualMembers []IndividualMemberResponse `json:"individual_members"`
}

type GroupingRuleResponse struct {
	Rules          []RuleResponse `json:"rules"`
	ExpirationDate time.Time      `json:"expiration_date"`
}

type RuleResponse struct {
	AttributeKey string         `json:"attribute_key"`
	Operator     group.Operator `json:"operator"`
	Multi        bool           `json:"multi"`
	Value        any            `json:"value"`
}

type IndividualMemberResponse struct {
	NTAccount      string    `json:"nt_account"`
	ExpirationDate time.Time `json:"expiration_date"`
}

func NewGroupCreateResponse(model group.Group) GroupCreateResponse {
	return GroupCreateResponse{Group: newGroupResponse(model)}
}

func newGroupResponse(model group.Group) GroupResponse {
	rules := make([]RuleResponse, 0, len(model.GroupingRule.Rules))
	for _, rule := range model.GroupingRule.Rules {
		rules = append(rules, RuleResponse{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	members := make([]IndividualMemberResponse, 0, len(model.IndividualMembers))
	for _, member := range model.IndividualMembers {
		members = append(members, IndividualMemberResponse{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
		})
	}
	return GroupResponse{
		ID:          model.ID,
		Name:        model.Name,
		Description: model.Description,
		GroupingRule: GroupingRuleResponse{
			Rules:          rules,
			ExpirationDate: model.GroupingRule.ExpirationDate,
		},
		IndividualMembers: members,
	}
}
