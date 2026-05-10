package transport

import (
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type GroupCreateResponse struct {
	Group GroupResponse `json:"group"`
}

type GroupGetResponse struct {
	Group *GroupSummaryResponse `json:"group"`
}

type GroupResponse struct {
	ID                string                     `json:"id"`
	Name              string                     `json:"name"`
	Description       string                     `json:"description"`
	GroupingRule      GroupingRuleResponse       `json:"grouping_rule"`
	IndividualMembers []IndividualMemberResponse `json:"individual_members"`
}

type GroupSummaryResponse struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	GroupingRule GroupingRuleResponse `json:"grouping_rule"`
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

type IndividualMemberListResponse struct {
	Members  []IndividualMemberResponse `json:"members"`
	PageInfo PageInfoResponse           `json:"page_info"`
}

type IndividualMembersAddResponse struct {
	Members []IndividualMemberResponse `json:"members"`
}

type PageInfoResponse struct {
	HasNextPage bool   `json:"has_next_page"`
	NextToken   string `json:"next_token"`
}

func NewGroupCreateResponse(model group.Group) GroupCreateResponse {
	return GroupCreateResponse{Group: newGroupResponse(model)}
}

func NewGroupGetResponse(model *group.Group) GroupGetResponse {
	if model == nil {
		return GroupGetResponse{Group: nil}
	}
	return GroupGetResponse{Group: newGroupSummaryResponse(*model)}
}

func newGroupResponse(model group.Group) GroupResponse {
	members := make([]IndividualMemberResponse, 0, len(model.IndividualMembers))
	for _, member := range model.IndividualMembers {
		members = append(members, IndividualMemberResponse{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
		})
	}
	return GroupResponse{
		ID:                model.ID,
		Name:              model.Name,
		Description:       model.Description,
		GroupingRule:      newGroupingRuleResponse(model.GroupingRule),
		IndividualMembers: members,
	}
}

func newGroupSummaryResponse(model group.Group) *GroupSummaryResponse {
	return &GroupSummaryResponse{
		ID:           model.ID,
		Name:         model.Name,
		Description:  model.Description,
		GroupingRule: newGroupingRuleResponse(model.GroupingRule),
	}
}

func newGroupingRuleResponse(rule group.GroupingRule) GroupingRuleResponse {
	rules := make([]RuleResponse, 0, len(rule.Rules))
	for _, item := range rule.Rules {
		rules = append(rules, RuleResponse{
			AttributeKey: item.AttributeKey,
			Operator:     item.Operator,
			Multi:        item.Multi,
			Value:        item.Value,
		})
	}
	return GroupingRuleResponse{Rules: rules, ExpirationDate: rule.ExpirationDate}
}

func NewIndividualMemberListResponse(page group.IndividualMemberPage) (IndividualMemberListResponse, error) {
	members := make([]IndividualMemberResponse, 0, len(page.Members))
	for _, member := range page.Members {
		members = append(members, IndividualMemberResponse{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
		})
	}
	nextToken, err := EncodeIndividualMemberNextToken(page.NextCursor)
	if err != nil {
		return IndividualMemberListResponse{}, err
	}
	return IndividualMemberListResponse{
		Members: members,
		PageInfo: PageInfoResponse{
			HasNextPage: page.HasNextPage,
			NextToken:   nextToken,
		},
	}, nil
}

func NewIndividualMembersAddResponse(members []group.IndividualMember) IndividualMembersAddResponse {
	responseMembers := make([]IndividualMemberResponse, 0, len(members))
	for _, member := range members {
		responseMembers = append(responseMembers, IndividualMemberResponse{
			NTAccount:      member.NTAccount,
			ExpirationDate: member.ExpirationDate,
		})
	}
	return IndividualMembersAddResponse{Members: responseMembers}
}
