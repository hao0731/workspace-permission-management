package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/caveat"
	permissionrelationship "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/relationship"
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
