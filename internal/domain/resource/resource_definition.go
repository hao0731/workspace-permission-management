package resource

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type ResourceDefinitionType string

const (
	ResourceDefinitionKindType   ResourceDefinitionType = "type"
	ResourceDefinitionKindTag    ResourceDefinitionType = "tag"
	ResourceDefinitionKindAction ResourceDefinitionType = "action"
)

var resourceDefinitionKeyPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

type ResourceDefinition struct {
	ID          string
	SystemID    string
	Type        ResourceDefinitionType
	Label       string
	Key         string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ResourceDefinitionInput struct {
	Type        ResourceDefinitionType
	Label       string
	Key         string
	Description string
}

type ResourceDefinitionSaveInput struct {
	SystemID  string
	Resources []ResourceDefinitionInput
}

type ResourceDefinitionsQuery struct {
	SystemID string
}

type ResourceAttributesQuery struct {
	SystemID string
}

type ResourceDefinitionLimits struct {
	Types   int
	Actions int
	Tags    int
}

type ResourceAttributes struct {
	ID        string
	SystemID  string
	Values    []ResourceAttribute
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (input ResourceDefinitionInput) Normalize() ResourceDefinitionInput {
	input.Type = ResourceDefinitionType(strings.TrimSpace(string(input.Type)))
	input.Label = strings.TrimSpace(input.Label)
	input.Key = strings.TrimSpace(input.Key)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func (input ResourceDefinitionInput) Validate() error {
	normalized := input.Normalize()
	if !normalized.Type.IsValid() {
		return invalidInput("resource type must be type, tag, or action")
	}
	if normalized.Key == "" {
		return invalidInput("resource key is required")
	}
	if utf8.RuneCountInString(normalized.Key) > 15 {
		return invalidInput("resource key must be at most 15 characters")
	}
	if !resourceDefinitionKeyPattern.MatchString(normalized.Key) {
		return invalidInput("resource key must contain only lower-case letters, numbers, and underscores")
	}
	if normalized.Label == "" {
		return invalidInput("resource label is required")
	}
	if utf8.RuneCountInString(normalized.Label) > 20 {
		return invalidInput("resource label must be at most 20 characters")
	}
	if utf8.RuneCountInString(normalized.Description) > 2000 {
		return invalidInput("resource description must be at most 2000 characters")
	}
	return nil
}

func (input ResourceDefinitionSaveInput) Normalize() ResourceDefinitionSaveInput {
	normalized := ResourceDefinitionSaveInput{
		SystemID:  strings.TrimSpace(input.SystemID),
		Resources: make([]ResourceDefinitionInput, len(input.Resources)),
	}
	for i, resourceInput := range input.Resources {
		normalized.Resources[i] = resourceInput.Normalize()
	}
	return normalized
}

func (input ResourceDefinitionSaveInput) Validate() error {
	normalized := input.Normalize()
	if err := validateSystemID(normalized.SystemID); err != nil {
		return err
	}
	if len(normalized.Resources) == 0 {
		return invalidInput("resources are required")
	}

	seen := make(map[string]struct{}, len(normalized.Resources))
	for _, resourceInput := range normalized.Resources {
		if err := resourceInput.Validate(); err != nil {
			return err
		}
		identity := fmt.Sprintf("%s/%s", resourceInput.Type, resourceInput.Key)
		if _, ok := seen[identity]; ok {
			return invalidInput(fmt.Sprintf("duplicate resource definition %s", identity))
		}
		seen[identity] = struct{}{}
	}
	return nil
}

func (query ResourceDefinitionsQuery) Validate() error {
	return validateSystemID(strings.TrimSpace(query.SystemID))
}

func (query ResourceAttributesQuery) Validate() error {
	return validateSystemID(strings.TrimSpace(query.SystemID))
}

func (kind ResourceDefinitionType) IsValid() bool {
	switch kind {
	case ResourceDefinitionKindType, ResourceDefinitionKindTag, ResourceDefinitionKindAction:
		return true
	default:
		return false
	}
}

func (limits ResourceDefinitionLimits) Validate() error {
	if limits.Types <= 0 {
		return invalidInput("resource type limit must be greater than zero")
	}
	if limits.Actions <= 0 {
		return invalidInput("resource action limit must be greater than zero")
	}
	if limits.Tags <= 0 {
		return invalidInput("resource tag limit must be greater than zero")
	}
	return nil
}

func ValidateResourceDefinitionCounts(definitions []ResourceDefinition, limits ResourceDefinitionLimits) error {
	if err := limits.Validate(); err != nil {
		return err
	}

	var typeCount, actionCount, tagCount int
	for _, definition := range definitions {
		switch definition.Type {
		case ResourceDefinitionKindType:
			typeCount++
		case ResourceDefinitionKindAction:
			actionCount++
		case ResourceDefinitionKindTag:
			tagCount++
		default:
			return invalidInput("resource type must be type, tag, or action")
		}
	}

	if typeCount > limits.Types {
		return invalidInput("resource type limit exceeded")
	}
	if actionCount > limits.Actions {
		return invalidInput("resource action limit exceeded")
	}
	if tagCount > limits.Tags {
		return invalidInput("resource tag limit exceeded")
	}
	return nil
}

func validateSystemID(systemID string) error {
	if systemID == "" {
		return invalidInput("system id is required")
	}
	if strings.Contains(systemID, ".") {
		return invalidInput("system id must be a single subject token")
	}
	if strings.IndexFunc(systemID, unicode.IsSpace) >= 0 {
		return invalidInput("system id must be a single subject token")
	}
	return nil
}
