package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/caveat"
	permissionrelation "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/relation"
	permissionrelationship "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/relationship"
)

func buildSystemGroupRelationshipProjection(systemID string, groupID string, rules []group.SystemGroupRule, now time.Time) (group.SystemGroupRelationshipProjection, error) {
	organizationIDs := dedupeSystemGroupRuleValues(rules, group.GroupAttributeOrganization, "")
	jobLevels := dedupeSystemGroupRuleValues(rules, group.GroupAttributeJobLevel, "")
	jobTags := dedupeSystemGroupRuleValues(rules, group.GroupAttributeJobTag, "")
	jobType := firstSystemGroupRuleValue(rules, group.GroupAttributeJobType)
	containsSecretary := containsString(jobTags, group.SystemGroupSecretarySentinel)
	a4Roles := removeString(jobTags, group.SystemGroupSecretarySentinel)

	relationships := make([]any, 0)
	if len(organizationIDs) == 0 {
		relationships = append(relationships, permissionrelationship.NewAllEmployeeToGroupForHRRelationship(groupID))
	} else {
		for _, organizationID := range organizationIDs {
			relationships = append(relationships, permissionrelationship.NewOrganizationToGroupRelationship(groupID, organizationID))
		}
	}

	if jobType != "" || len(jobLevels) > 0 || containsSecretary {
		options := make([]caveat.StaticAttributesCheckOption, 0, 3)
		if jobType != "" {
			options = append(options, caveat.WithAllowedTypes([]string{jobType}))
		}
		if len(jobLevels) > 0 {
			options = append(options, caveat.WithAllowedLevels(jobLevels))
		}
		if containsSecretary {
			options = append(options, caveat.WithIsContainSecretary(true))
		}
		relationships = append(relationships, permissionrelationship.NewGroupWithStaticAttributesRelationship(groupID, options...))
	}

	if len(a4Roles) == 0 {
		relationships = append(relationships, permissionrelationship.NewAllEmployeeToGroupForA4Relationship(groupID))
	} else {
		for _, role := range a4Roles {
			relationships = append(relationships, permissionrelationship.NewA4RoleToGroupRelationship(groupID, role))
		}
	}

	infos := make([]group.RelationshipInfo, 0, len(relationships))
	for _, relationship := range relationships {
		checksum, err := relationshipChecksum(relationship)
		if err != nil {
			return group.SystemGroupRelationshipProjection{}, err
		}
		infos = append(infos, group.RelationshipInfo{Relationship: relationship, Checksum: checksum})
	}
	return group.SystemGroupRelationshipProjection{
		SystemID:      systemID,
		GroupID:       groupID,
		Relationships: infos,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func relationshipChecksum(relationship any) (string, error) {
	data, err := json.Marshal(relationship)
	if err != nil {
		return "", fmt.Errorf("marshal relationship for checksum: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func dedupeSystemGroupRuleValues(rules []group.SystemGroupRule, key group.GroupAttributeKey, exclude string) []string {
	seen := map[string]struct{}{}
	for _, rule := range rules {
		if rule.AttributeKey != key {
			continue
		}
		if rule.Multi {
			values, ok := rule.Value.([]string)
			if !ok {
				continue
			}
			for _, value := range values {
				if value == exclude {
					continue
				}
				seen[value] = struct{}{}
			}
			continue
		}
		value, ok := rule.Value.(string)
		if ok && value != exclude {
			seen[value] = struct{}{}
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func firstSystemGroupRuleValue(rules []group.SystemGroupRule, key group.GroupAttributeKey) string {
	for _, rule := range rules {
		if rule.AttributeKey != key {
			continue
		}
		value, ok := rule.Value.(string)
		if ok {
			return value
		}
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func permissionRelationshipValue(value any) (permissionrelationship.Relationship, error) {
	switch relationship := value.(type) {
	case permissionrelationship.Relationship:
		return relationship, nil
	case *permissionrelationship.Relationship:
		if relationship == nil {
			return permissionrelationship.Relationship{}, fmt.Errorf("system group relationship is nil")
		}
		return *relationship, nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return permissionrelationship.Relationship{}, fmt.Errorf("marshal system group relationship value: %w", err)
		}
		var decoded permissionrelationship.Relationship
		if err := json.Unmarshal(data, &decoded); err != nil {
			return permissionrelationship.Relationship{}, fmt.Errorf("unmarshal system group relationship value: %w", err)
		}
		return decoded, nil
	}
}

func rebuildSystemGroupFromRelationshipProjection(model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error) {
	organizationIDs := map[string]struct{}{}
	jobLevels := map[string]struct{}{}
	a4Roles := map[string]struct{}{}
	jobType := ""
	containsSecretary := false

	for _, info := range projection.Relationships {
		relationship, err := permissionRelationshipValue(info.Relationship)
		if err != nil {
			return group.SystemGroup{}, err
		}
		switch relationship.Relation {
		case permissionrelation.HRMemberRelation:
			if relationship.Subject.Object.ObjectType == "organization" {
				organizationIDs[relationship.Subject.Object.ObjectID] = struct{}{}
			}
		case permissionrelation.CheckedMemberRelation:
			params, ok, err := staticAttributesCheckParam(relationship.Caveat)
			if err != nil {
				return group.SystemGroup{}, err
			}
			if !ok {
				continue
			}
			if len(params.AllowedTypes) > 0 {
				jobType = params.AllowedTypes[0]
			}
			for _, level := range params.AllowedLevels {
				jobLevels[level] = struct{}{}
			}
			if params.IsContainSecretary {
				containsSecretary = true
			}
		case permissionrelation.A4RoleMemberRelation:
			if relationship.Subject.Object.ObjectType == "business_role" {
				a4Roles[relationship.Subject.Object.ObjectID] = struct{}{}
			}
		}
	}

	rules := make([]group.SystemGroupRule, 0)
	if values := sortedMapKeys(organizationIDs); len(values) > 0 {
		rules = append(rules, group.SystemGroupRule{
			AttributeKey: group.GroupAttributeOrganization,
			Operator:     group.OperatorEq,
			Multi:        true,
			Value:        values,
		})
	}
	if jobType != "" {
		rules = append(rules, group.SystemGroupRule{
			AttributeKey: group.GroupAttributeJobType,
			Operator:     group.OperatorEq,
			Multi:        false,
			Value:        jobType,
		})
	}
	for _, level := range sortedMapKeys(jobLevels) {
		rules = append(rules, group.SystemGroupRule{
			AttributeKey: group.GroupAttributeJobLevel,
			Operator:     group.OperatorEq,
			Multi:        false,
			Value:        level,
		})
	}
	jobTags := sortedMapKeys(a4Roles)
	if containsSecretary {
		jobTags = append(jobTags, group.SystemGroupSecretarySentinel)
	}
	if len(jobTags) > 0 {
		rules = append(rules, group.SystemGroupRule{
			AttributeKey: group.GroupAttributeJobTag,
			Operator:     group.OperatorEq,
			Multi:        true,
			Value:        jobTags,
		})
	}

	model.GroupingRules = rules
	return model, nil
}

func staticAttributesCheckParam(cav *caveat.Caveat) (caveat.StaticAttributesCheckParam, bool, error) {
	if cav == nil || cav.Name != "static_attributes_check" {
		return caveat.StaticAttributesCheckParam{}, false, nil
	}
	switch context := cav.Context.(type) {
	case caveat.StaticAttributesCheckParam:
		return context, true, nil
	case map[string]any:
		return caveat.StaticAttributesCheckParam{
			AllowedTypes:       stringValuesFromAny(context["allowed_types"]),
			AllowedLevels:      stringValuesFromAny(context["allowed_levels"]),
			IsContainSecretary: boolValueFromAny(context["is_contain_secretary"]),
		}, true, nil
	default:
		data, err := json.Marshal(context)
		if err != nil {
			return caveat.StaticAttributesCheckParam{}, false, fmt.Errorf("marshal static attributes caveat context: %w", err)
		}
		var params caveat.StaticAttributesCheckParam
		if err := json.Unmarshal(data, &params); err != nil {
			return caveat.StaticAttributesCheckParam{}, false, fmt.Errorf("unmarshal static attributes caveat context: %w", err)
		}
		return params, true, nil
	}
}

func stringValuesFromAny(value any) []string {
	switch values := value.(type) {
	case []string:
		return append([]string(nil), values...)
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			stringValue, ok := value.(string)
			if ok {
				out = append(out, stringValue)
			}
		}
		return out
	default:
		return nil
	}
}

func boolValueFromAny(value any) bool {
	boolValue, ok := value.(bool)
	return ok && boolValue
}

func sortedMapKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
