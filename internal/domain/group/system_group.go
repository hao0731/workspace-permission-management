package group

import (
	"strings"
	"time"
)

type GroupAttributeKey string

const (
	GroupAttributeOrganization   GroupAttributeKey = "organization"
	GroupAttributeJobLevel       GroupAttributeKey = "job_level"
	GroupAttributeJobType        GroupAttributeKey = "job_type"
	GroupAttributeJobTag         GroupAttributeKey = "job_tag"
	SystemGroupSecretarySentinel string            = "_internal_secretary_"
	SystemGroupJobTypeDL         string            = "DL"
	SystemGroupJobTypeIDL        string            = "IDL"
	SystemGroupJobTypeALL        string            = "ALL"
)

type SystemGroup struct {
	ID            string
	SystemID      string
	Name          string
	GroupingRules []SystemGroupRule
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SystemGroupRule struct {
	AttributeKey GroupAttributeKey
	Operator     Operator
	Multi        bool
	Value        any
}

type SystemGroupCreateInput struct {
	SystemID      string
	Name          string
	GroupingRules []SystemGroupRule
}

type SystemGroupUpdateInput struct {
	SystemID      string
	GroupID       string
	Name          string
	GroupingRules []SystemGroupRule
}

type SystemGroupCursor struct {
	CreatedAt time.Time
	ID        string
}

type SystemGroupListQuery struct {
	SystemID string
	Limit    int
	Cursor   *SystemGroupCursor
}

type SystemGroupPage struct {
	Groups      []SystemGroup
	HasNextPage bool
	NextCursor  *SystemGroupCursor
}

type RelationshipInfo struct {
	Relationship any
	Checksum     string
}

type SystemGroupRelationshipProjection struct {
	SystemID      string
	GroupID       string
	Relationships []RelationshipInfo
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (input SystemGroupCreateInput) Normalize() SystemGroupCreateInput {
	input.SystemID = strings.TrimSpace(input.SystemID)
	input.Name = strings.TrimSpace(input.Name)
	for i := range input.GroupingRules {
		input.GroupingRules[i] = input.GroupingRules[i].Normalize()
	}
	return input
}

func (input SystemGroupUpdateInput) Normalize() SystemGroupUpdateInput {
	input.SystemID = strings.TrimSpace(input.SystemID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	input.Name = strings.TrimSpace(input.Name)
	for i := range input.GroupingRules {
		input.GroupingRules[i] = input.GroupingRules[i].Normalize()
	}
	return input
}

func (rule SystemGroupRule) Normalize() SystemGroupRule {
	rule.AttributeKey = GroupAttributeKey(strings.TrimSpace(string(rule.AttributeKey)))
	rule.Operator = Operator(strings.TrimSpace(string(rule.Operator)))
	if rule.Multi {
		if values, ok := stringSliceValue(rule.Value); ok {
			out := make([]string, 0, len(values))
			for _, value := range values {
				out = append(out, strings.TrimSpace(value))
			}
			rule.Value = out
		}
		return rule
	}
	if value, ok := rule.Value.(string); ok {
		rule.Value = strings.TrimSpace(value)
	}
	return rule
}

func (query SystemGroupListQuery) Normalize() SystemGroupListQuery {
	query.SystemID = strings.TrimSpace(query.SystemID)
	if query.Cursor != nil {
		query.Cursor.ID = strings.TrimSpace(query.Cursor.ID)
	}
	return query
}

func stringSliceValue(value any) ([]string, bool) {
	values, ok := value.([]string)
	return values, ok
}
