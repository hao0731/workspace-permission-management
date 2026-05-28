package transport

import (
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type SystemGroupCreateResponse struct {
	Group SystemGroupResponse `json:"group"`
}

type SystemGroupCreatePartialResponse struct {
	Group  SystemGroupResponse `json:"group"`
	Errors []string            `json:"errors"`
}

type SystemGroupUpdateResponse struct {
	Group SystemGroupResponse `json:"group"`
}

type SystemGroupUpdatePartialResponse struct {
	Group  SystemGroupResponse `json:"group"`
	Errors []string            `json:"errors"`
}

type SystemGroupListResponse struct {
	Groups   []SystemGroupResponse `json:"groups"`
	PageInfo PageInfoResponse      `json:"page_info"`
}

type SystemGroupResponse struct {
	ID            string                    `json:"id"`
	Name          string                    `json:"name"`
	GroupingRules []SystemGroupRuleResponse `json:"grouping_rules"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
}

type SystemGroupRuleResponse struct {
	AttributeKey group.GroupAttributeKey `json:"attribute_key"`
	Operator     group.Operator          `json:"operator"`
	Multi        bool                    `json:"multi"`
	Value        any                     `json:"value"`
}

func NewSystemGroupCreateResponse(model group.SystemGroup) SystemGroupCreateResponse {
	return SystemGroupCreateResponse{Group: newSystemGroupResponse(model)}
}

func NewSystemGroupCreatePartialResponse(model group.SystemGroup, errors []string) SystemGroupCreatePartialResponse {
	return SystemGroupCreatePartialResponse{
		Group:  newSystemGroupResponse(model),
		Errors: append([]string(nil), errors...),
	}
}

func NewSystemGroupUpdateResponse(model group.SystemGroup) SystemGroupUpdateResponse {
	return SystemGroupUpdateResponse{Group: newSystemGroupResponse(model)}
}

func NewSystemGroupUpdatePartialResponse(model group.SystemGroup, errors []string) SystemGroupUpdatePartialResponse {
	return SystemGroupUpdatePartialResponse{
		Group:  newSystemGroupResponse(model),
		Errors: append([]string(nil), errors...),
	}
}

func NewSystemGroupListResponse(page group.SystemGroupPage) (SystemGroupListResponse, error) {
	groups := make([]SystemGroupResponse, 0, len(page.Groups))
	for _, model := range page.Groups {
		groups = append(groups, newSystemGroupResponse(model))
	}
	nextToken, err := EncodeSystemGroupNextToken(page.NextCursor)
	if err != nil {
		return SystemGroupListResponse{}, err
	}
	return SystemGroupListResponse{
		Groups: groups,
		PageInfo: PageInfoResponse{
			HasNextPage: page.HasNextPage,
			NextToken:   nextToken,
		},
	}, nil
}

func newSystemGroupResponse(model group.SystemGroup) SystemGroupResponse {
	rules := make([]SystemGroupRuleResponse, 0, len(model.GroupingRules))
	for _, rule := range model.GroupingRules {
		rules = append(rules, SystemGroupRuleResponse{
			AttributeKey: rule.AttributeKey,
			Operator:     rule.Operator,
			Multi:        rule.Multi,
			Value:        rule.Value,
		})
	}
	return SystemGroupResponse{
		ID:            model.ID,
		Name:          model.Name,
		GroupingRules: rules,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}
