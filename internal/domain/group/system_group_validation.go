package group

import (
	"fmt"
	"strings"
)

func (input SystemGroupCreateInput) Validate() error {
	if err := validateSystemID(input.SystemID); err != nil {
		return err
	}
	if strings.TrimSpace(input.Name) == "" {
		return invalidInput("name is required")
	}
	if input.GroupingRules == nil {
		return invalidInput("grouping_rules is required")
	}
	jobTypeCount := 0
	for _, rule := range input.GroupingRules {
		if rule.AttributeKey == GroupAttributeJobType {
			jobTypeCount++
			if jobTypeCount > 1 {
				return invalidInput("only one job_type rule is allowed")
			}
		}
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (query SystemGroupListQuery) Validate() error {
	if err := validateSystemID(query.SystemID); err != nil {
		return err
	}
	if query.Limit <= 0 {
		return invalidInput("limit must be greater than zero")
	}
	if query.Cursor != nil {
		if query.Cursor.CreatedAt.IsZero() {
			return invalidInput("cursor created_at is required")
		}
		if strings.TrimSpace(query.Cursor.ID) == "" {
			return invalidInput("cursor id is required")
		}
	}
	return nil
}

func (rule SystemGroupRule) Validate() error {
	if rule.Operator != OperatorEq {
		return invalidInput("system group rule operator must be eq")
	}
	switch rule.AttributeKey {
	case GroupAttributeOrganization:
		return validateSystemGroupMultiRule(rule, "organization")
	case GroupAttributeJobTag:
		return validateSystemGroupMultiRule(rule, "job_tag")
	case GroupAttributeJobLevel:
		return validateSystemGroupSingleRule(rule, "job_level")
	case GroupAttributeJobType:
		if err := validateSystemGroupSingleRule(rule, "job_type"); err != nil {
			return err
		}
		value, ok := rule.Value.(string)
		if !ok {
			return invalidInput("job_type rule value must be a string")
		}
		if !IsValidSystemGroupJobType(value) {
			return invalidInput("job_type value must be DL, IDL, or ALL")
		}
		return nil
	default:
		return invalidInput(fmt.Sprintf("system group rule attribute_key is invalid: %s", rule.AttributeKey))
	}
}

func IsValidSystemGroupJobType(value string) bool {
	switch value {
	case SystemGroupJobTypeDL, SystemGroupJobTypeIDL, SystemGroupJobTypeALL:
		return true
	default:
		return false
	}
}

func validateSystemGroupMultiRule(rule SystemGroupRule, name string) error {
	if !rule.Multi {
		return invalidInput(fmt.Sprintf("%s rule must be multi", name))
	}
	values, ok := stringSliceValue(rule.Value)
	if !ok {
		return invalidInput(fmt.Sprintf("%s rule value must be a string array", name))
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return invalidInput("system group rule value must not be empty")
		}
	}
	return nil
}

func validateSystemGroupSingleRule(rule SystemGroupRule, name string) error {
	if rule.Multi {
		return invalidInput(fmt.Sprintf("%s rule must not be multi", name))
	}
	value, ok := rule.Value.(string)
	if !ok {
		return invalidInput(fmt.Sprintf("%s rule value must be a string", name))
	}
	if strings.TrimSpace(value) == "" {
		return invalidInput("system group rule value must not be empty")
	}
	return nil
}

func validateSystemID(systemID string) error {
	trimmed := strings.TrimSpace(systemID)
	if trimmed == "" {
		return invalidInput("system id is required")
	}
	if strings.ContainsAny(trimmed, " \t\n\r.") {
		return invalidInput("system id must be a single subject token")
	}
	return nil
}
