package permission

import (
	"fmt"
	"strings"
)

func (input SaveInput) Validate() error {
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return invalidInput("workspace id is required")
	}
	if strings.TrimSpace(input.FunctionKey) == "" {
		return invalidInput("function key is required")
	}
	if input.OfficePermission == nil {
		return invalidInput("office permission is required")
	}
	if input.RemotePermission == nil {
		return invalidInput("remote permission is required")
	}
	if err := validateSection("office", *input.OfficePermission); err != nil {
		return err
	}
	if err := validateSection("remote", *input.RemotePermission); err != nil {
		return err
	}
	if err := validateUniqueRuleIDs(input); err != nil {
		return err
	}
	return nil
}

func validateSection(label string, section PermissionSection) error {
	if strings.TrimSpace(section.BaselineRule.ActionID) == "" {
		return invalidInput(fmt.Sprintf("%s baseline action id is required", label))
	}
	if len(section.BaselineRule.ResourceTags) == 0 {
		return invalidInput(fmt.Sprintf("%s baseline resource tags are required", label))
	}
	for _, tag := range section.BaselineRule.ResourceTags {
		if strings.TrimSpace(tag) == "" {
			return invalidInput(fmt.Sprintf("%s baseline resource tags must be non-empty strings", label))
		}
	}
	for _, rule := range section.ExtraRules {
		if strings.TrimSpace(rule.RuleID) == "" && rule.RuleID != "" {
			return invalidInput(fmt.Sprintf("%s extra rule rule id must be non-empty when provided", label))
		}
		if len(rule.GroupIDs) == 0 {
			return invalidInput(fmt.Sprintf("%s extra rule group ids are required", label))
		}
		for _, groupID := range rule.GroupIDs {
			if strings.TrimSpace(groupID) == "" {
				return invalidInput(fmt.Sprintf("%s extra rule group ids must be non-empty strings", label))
			}
		}
		if strings.TrimSpace(rule.ActionID) == "" {
			return invalidInput(fmt.Sprintf("%s extra rule action id is required", label))
		}
		if len(rule.ResourceTags) == 0 {
			return invalidInput(fmt.Sprintf("%s extra rule resource tags are required", label))
		}
		for _, tag := range rule.ResourceTags {
			if strings.TrimSpace(tag) == "" {
				return invalidInput(fmt.Sprintf("%s extra rule resource tags must be non-empty strings", label))
			}
		}
		if rule.ExpirationDate.IsZero() {
			return invalidInput(fmt.Sprintf("%s extra rule expiration date is required", label))
		}
	}
	return nil
}

func validateUniqueRuleIDs(input SaveInput) error {
	seen := map[string]struct{}{}
	for _, rule := range input.OfficePermission.ExtraRules {
		if err := checkRuleID(rule.RuleID, seen); err != nil {
			return err
		}
	}
	for _, rule := range input.RemotePermission.ExtraRules {
		if err := checkRuleID(rule.RuleID, seen); err != nil {
			return err
		}
	}
	return nil
}

func checkRuleID(ruleID string, seen map[string]struct{}) error {
	if ruleID == "" {
		return nil
	}
	if _, ok := seen[ruleID]; ok {
		return invalidInput(fmt.Sprintf("duplicate rule id %q", ruleID))
	}
	seen[ruleID] = struct{}{}
	return nil
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
