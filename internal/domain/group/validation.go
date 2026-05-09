package group

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

func (input CreateInput) Validate(now time.Time, opts ...ValidateOption) error {
	options := defaultValidateOptions()
	for _, opt := range opts {
		if opt != nil {
			opt.applyValidateOption(&options)
		}
	}
	options = options.withDefaults()
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(input.Name) == "" {
		return invalidInput("name is required")
	}
	if input.GroupingRule.ExpirationDate.IsZero() {
		return invalidInput("grouping rule expiration date is required")
	}
	if !input.GroupingRule.ExpirationDate.After(now) {
		return invalidInput("grouping rule expiration date must be in the future")
	}
	if len(input.GroupingRule.Rules) == 0 && len(input.IndividualMembers) == 0 {
		return invalidInput("at least one membership source is required")
	}
	if len(input.GroupingRule.Rules) > options.maxGroupingRules {
		return invalidInput(fmt.Sprintf("grouping rules must not exceed %d items", options.maxGroupingRules))
	}
	if len(input.IndividualMembers) > options.maxIndividualMembers {
		return invalidInput(fmt.Sprintf("individual members must not exceed %d items", options.maxIndividualMembers))
	}
	for _, rule := range input.GroupingRule.Rules {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return validateIndividualMembers(input.IndividualMembers, now)
}

func (query GetQuery) Validate() error {
	return validateGroupIdentity(query.WorkspaceID, query.GroupID)
}

func (input DeleteInput) Validate() error {
	return validateGroupIdentity(input.WorkspaceID, input.GroupID)
}

func (input UpdateGroupingRuleInput) Validate(now time.Time, opts ...ValidateOption) error {
	if err := validateGroupIdentity(input.WorkspaceID, input.GroupID); err != nil {
		return err
	}
	options := defaultValidateOptions()
	for _, opt := range opts {
		if opt != nil {
			opt.applyValidateOption(&options)
		}
	}
	options = options.withDefaults()
	if input.ExpirationDate.IsZero() {
		return invalidInput("grouping rule expiration date is required")
	}
	if !input.ExpirationDate.After(now) {
		return invalidInput("grouping rule expiration date must be in the future")
	}
	if len(input.Rules) > options.maxGroupingRules {
		return invalidInput(fmt.Sprintf("grouping rules must not exceed %d items", options.maxGroupingRules))
	}
	for _, rule := range input.Rules {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (query ListIndividualMembersQuery) Validate() error {
	if err := validateGroupIdentity(query.WorkspaceID, query.GroupID); err != nil {
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

func (rule Rule) Validate() error {
	if strings.TrimSpace(rule.AttributeKey) == "" {
		return invalidInput("rule attribute key is required")
	}
	if !IsValidOperator(rule.Operator) {
		return invalidInput(fmt.Sprintf("rule operator is invalid: %s", rule.Operator))
	}
	if rule.Multi {
		length, valueAt, ok := arrayValue(rule.Value)
		if !ok || length == 0 {
			return invalidInput("multi-value rule value must be a non-empty array")
		}
		for i := 0; i < length; i++ {
			if isNilValue(valueAt(i)) {
				return invalidInput("multi-value rule value items must not be null")
			}
		}
		return nil
	}
	if isNilValue(rule.Value) {
		return invalidInput("single-value rule value is required")
	}
	if _, _, ok := arrayValue(rule.Value); ok {
		return invalidInput("single-value rule value must not be an array")
	}
	return nil
}

func IsValidOperator(operator Operator) bool {
	switch operator {
	case OperatorEq, OperatorNotEq, OperatorGt, OperatorGte, OperatorLt, OperatorLte:
		return true
	default:
		return false
	}
}

func validateIndividualMembers(members []IndividualMember, now time.Time) error {
	seen := map[string]struct{}{}
	for _, member := range members {
		account := strings.TrimSpace(member.NTAccount)
		if account == "" {
			return invalidInput("individual member nt account is required")
		}
		if _, ok := seen[account]; ok {
			return invalidInput(fmt.Sprintf("duplicate individual member nt account %q", account))
		}
		seen[account] = struct{}{}
		if member.ExpirationDate.IsZero() {
			return invalidInput("individual member expiration date is required")
		}
		if !member.ExpirationDate.After(now) {
			return invalidInput("individual member expiration date must be in the future")
		}
	}
	return nil
}

func validateGroupIdentity(workspaceID string, groupID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(groupID) == "" {
		return invalidInput("group id is required")
	}
	return nil
}

func arrayValue(value any) (int, func(int) any, bool) {
	if value == nil {
		return 0, nil, false
	}
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return 0, nil, false
	}
	return v.Len(), func(index int) any {
		return v.Index(index).Interface()
	}, true
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
