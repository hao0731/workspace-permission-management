package group

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

func (input CreateInput) Validate(now time.Time) error {
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
	for _, rule := range input.GroupingRule.Rules {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return validateIndividualMembers(input.IndividualMembers, now)
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
