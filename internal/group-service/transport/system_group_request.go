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

func (request SystemGroupCreateRequest) ToDomain(systemID string) (group.SystemGroupCreateInput, error) {
	if request.GroupingRules == nil {
		return group.SystemGroupCreateInput{}, invalidGroupRequest("grouping_rules is required")
	}
	rules := make([]group.SystemGroupRule, 0, len(request.GroupingRules))
	for _, rule := range request.GroupingRules {
		if rule.Multi == nil {
			return group.SystemGroupCreateInput{}, invalidGroupRequest("rule multi is required")
		}
		value, err := systemGroupRuleValue(rule.Value, *rule.Multi)
		if err != nil {
			return group.SystemGroupCreateInput{}, err
		}
		rules = append(rules, group.SystemGroupRule{
			AttributeKey: group.GroupAttributeKey(rule.AttributeKey),
			Operator:     group.Operator(rule.Operator),
			Multi:        *rule.Multi,
			Value:        value,
		})
	}
	return group.SystemGroupCreateInput{
		SystemID:      systemID,
		Name:          request.Name,
		GroupingRules: rules,
	}, nil
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
