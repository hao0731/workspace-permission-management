package group

import (
	"strings"
	"time"
)

type Operator string

const (
	OperatorEq    Operator = "eq"
	OperatorNotEq Operator = "not_eq"
	OperatorGt    Operator = "gt"
	OperatorGte   Operator = "gte"
	OperatorLt    Operator = "lt"
	OperatorLte   Operator = "lte"
)

const (
	DefaultMaxIndividualMembers = 1000
	DefaultMaxGroupingRules     = 10
)

type Group struct {
	ID                string
	WorkspaceID       string
	Name              string
	NormalizedName    string
	Description       string
	GroupingRule      GroupingRule
	IndividualMembers []IndividualMember
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

type CreateInput struct {
	WorkspaceID       string
	Name              string
	Description       string
	GroupingRule      GroupingRule
	IndividualMembers []IndividualMember
}

type GroupingRule struct {
	Rules          []Rule
	ExpirationDate time.Time
}

type Rule struct {
	AttributeKey string
	Operator     Operator
	Multi        bool
	Value        any
}

type IndividualMember struct {
	ID             string
	GroupID        string
	NTAccount      string
	ExpirationDate time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type ValidateOption interface {
	applyValidateOption(*validateOptions)
}

type validateOptionFunc func(*validateOptions)

type validateOptions struct {
	maxIndividualMembers int
	maxGroupingRules     int
}

func (fn validateOptionFunc) applyValidateOption(options *validateOptions) {
	fn(options)
}

func WithMaxIndividualMembers(max int) ValidateOption {
	return validateOptionFunc(func(options *validateOptions) {
		options.maxIndividualMembers = max
	})
}

func WithMaxGroupingRules(max int) ValidateOption {
	return validateOptionFunc(func(options *validateOptions) {
		options.maxGroupingRules = max
	})
}

func defaultValidateOptions() validateOptions {
	return validateOptions{
		maxIndividualMembers: DefaultMaxIndividualMembers,
		maxGroupingRules:     DefaultMaxGroupingRules,
	}
}

func (options validateOptions) withDefaults() validateOptions {
	if options.maxIndividualMembers <= 0 {
		options.maxIndividualMembers = DefaultMaxIndividualMembers
	}
	if options.maxGroupingRules <= 0 {
		options.maxGroupingRules = DefaultMaxGroupingRules
	}
	return options
}

func (input CreateInput) Normalize() CreateInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.Name = strings.TrimSpace(input.Name)
	input.GroupingRule = input.GroupingRule.Normalize()
	for i := range input.IndividualMembers {
		input.IndividualMembers[i] = input.IndividualMembers[i].Normalize()
	}
	return input
}

func (rule GroupingRule) Normalize() GroupingRule {
	for i := range rule.Rules {
		rule.Rules[i] = rule.Rules[i].Normalize()
	}
	return rule
}

func (rule Rule) Normalize() Rule {
	rule.AttributeKey = strings.TrimSpace(rule.AttributeKey)
	return rule
}

func (member IndividualMember) Normalize() IndividualMember {
	member.NTAccount = strings.TrimSpace(member.NTAccount)
	return member
}
