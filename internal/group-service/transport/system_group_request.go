package transport

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type SystemGroupCreateRequest struct {
	Name          string                   `json:"name"`
	GroupingRules []SystemGroupRuleRequest `json:"grouping_rules"`
}

type SystemGroupUpdateRequest struct {
	Name          string                   `json:"name"`
	GroupingRules []SystemGroupRuleRequest `json:"grouping_rules"`
}

type SystemGroupRuleRequest struct {
	AttributeKey string `json:"attribute_key"`
	Operator     string `json:"operator"`
	Multi        *bool  `json:"multi"`
	Value        any    `json:"value"`
}

func DecodeSystemGroupCreateRequest(body io.Reader) (SystemGroupCreateRequest, error) {
	var request SystemGroupCreateRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return SystemGroupCreateRequest{}, fmt.Errorf("decode system group create request: %w", err)
	}
	return request, nil
}

func DecodeSystemGroupUpdateRequest(body io.Reader) (SystemGroupUpdateRequest, error) {
	var request SystemGroupUpdateRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return SystemGroupUpdateRequest{}, fmt.Errorf("decode system group update request: %w", err)
	}
	return request, nil
}

func (request SystemGroupCreateRequest) ToDomain(systemID string) (group.SystemGroupCreateInput, error) {
	rules, err := systemGroupRulesToDomain(request.GroupingRules)
	if err != nil {
		return group.SystemGroupCreateInput{}, err
	}
	return group.SystemGroupCreateInput{
		SystemID:      systemID,
		Name:          request.Name,
		GroupingRules: rules,
	}, nil
}

func (request SystemGroupUpdateRequest) ToDomain(systemID string, groupID string) (group.SystemGroupUpdateInput, error) {
	rules, err := systemGroupRulesToDomain(request.GroupingRules)
	if err != nil {
		return group.SystemGroupUpdateInput{}, err
	}
	return group.SystemGroupUpdateInput{
		SystemID:      systemID,
		GroupID:       groupID,
		Name:          request.Name,
		GroupingRules: rules,
	}, nil
}

func systemGroupRulesToDomain(requestRules []SystemGroupRuleRequest) ([]group.SystemGroupRule, error) {
	if requestRules == nil {
		return nil, invalidGroupRequest("grouping_rules is required")
	}
	rules := make([]group.SystemGroupRule, 0, len(requestRules))
	for _, rule := range requestRules {
		if rule.Multi == nil {
			return nil, invalidGroupRequest("rule multi is required")
		}
		value, err := systemGroupRuleValue(rule.Value, *rule.Multi)
		if err != nil {
			return nil, err
		}
		rules = append(rules, group.SystemGroupRule{
			AttributeKey: group.GroupAttributeKey(rule.AttributeKey),
			Operator:     group.Operator(rule.Operator),
			Multi:        *rule.Multi,
			Value:        value,
		})
	}
	return rules, nil
}

func systemGroupRuleValue(value any, multi bool) (any, error) {
	if multi {
		rawValues, ok := value.([]any)
		if !ok {
			return nil, invalidGroupRequest("multi rule value must be an array")
		}
		values := make([]string, 0, len(rawValues))
		for _, raw := range rawValues {
			valueString, ok := raw.(string)
			if !ok {
				return nil, invalidGroupRequest("multi rule value items must be strings")
			}
			values = append(values, valueString)
		}
		return values, nil
	}
	valueString, ok := value.(string)
	if !ok {
		return nil, invalidGroupRequest("single rule value must be a string")
	}
	return valueString, nil
}
