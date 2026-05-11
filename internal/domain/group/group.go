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
	ExpiryTask        *ExpiryTask
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

type GetQuery struct {
	WorkspaceID string
	GroupID     string
}

type DeleteInput struct {
	WorkspaceID string
	GroupID     string
}

type UpdateGroupingRuleInput struct {
	WorkspaceID    string
	GroupID        string
	Rules          []Rule
	ExpirationDate time.Time
	ExpiryTask     *ExpiryTask
}

type GroupingRule struct {
	Rules          []Rule
	ExpirationDate time.Time
	ExpiredAt      *time.Time
}

type ExpiryTask struct {
	ID               string
	WorkspaceID      string
	GroupID          string
	ExpirationBucket string
}

type ExpireGroupingRuleCommand struct {
	TaskID           string
	WorkspaceID      string
	GroupID          string
	ExpirationBucket string
}

type ExpireGroupingRuleStatus string

const (
	ExpireGroupingRuleStatusExpired        ExpireGroupingRuleStatus = "expired"
	ExpireGroupingRuleStatusStaleTask      ExpireGroupingRuleStatus = "stale_task"
	ExpireGroupingRuleStatusStaleGroup     ExpireGroupingRuleStatus = "stale_group"
	ExpireGroupingRuleStatusAlreadyExpired ExpireGroupingRuleStatus = "already_expired"
	ExpireGroupingRuleStatusStaleBucket    ExpireGroupingRuleStatus = "stale_bucket"
)

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
	ExpiredAt      *time.Time
	ExpiryTask     *IndividualMemberExpiryTask
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type IndividualMemberExpiryTask struct {
	ID               string
	GroupID          string
	NTAccount        string
	ExpirationBucket string
}

type ExpireIndividualMemberCommand struct {
	TaskID           string
	GroupID          string
	NTAccount        string
	ExpirationBucket string
}

type ExpireIndividualMemberStatus string

const (
	ExpireIndividualMemberStatusExpired        ExpireIndividualMemberStatus = "expired"
	ExpireIndividualMemberStatusStaleTask      ExpireIndividualMemberStatus = "stale_task"
	ExpireIndividualMemberStatusStaleMember    ExpireIndividualMemberStatus = "stale_member"
	ExpireIndividualMemberStatusAlreadyExpired ExpireIndividualMemberStatus = "already_expired"
	ExpireIndividualMemberStatusStaleBucket    ExpireIndividualMemberStatus = "stale_bucket"
)

type IndividualMemberCursor struct {
	CreatedAt time.Time
	ID        string
}

type ListIndividualMembersQuery struct {
	WorkspaceID string
	GroupID     string
	Limit       int
	Cursor      *IndividualMemberCursor
}

type AddIndividualMembersInput struct {
	WorkspaceID       string
	GroupID           string
	IndividualMembers []IndividualMember
}

type UpdateIndividualMemberExpirationInput struct {
	WorkspaceID    string
	GroupID        string
	NTAccount      string
	ExpirationDate time.Time
	ExpiryTask     *IndividualMemberExpiryTask
}

type DeleteIndividualMemberInput struct {
	WorkspaceID string
	GroupID     string
	NTAccount   string
}

type IndividualMemberPage struct {
	Members     []IndividualMember
	HasNextPage bool
	NextCursor  *IndividualMemberCursor
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

func (query GetQuery) Normalize() GetQuery {
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.GroupID = strings.TrimSpace(query.GroupID)
	return query
}

func (input DeleteInput) Normalize() DeleteInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	return input
}

func (input UpdateGroupingRuleInput) Normalize() UpdateGroupingRuleInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	for i := range input.Rules {
		input.Rules[i] = input.Rules[i].Normalize()
	}
	return input
}

func (rule GroupingRule) Normalize() GroupingRule {
	for i := range rule.Rules {
		rule.Rules[i] = rule.Rules[i].Normalize()
	}
	return rule
}

func (task ExpiryTask) Normalize() ExpiryTask {
	task.ID = strings.TrimSpace(task.ID)
	task.WorkspaceID = strings.TrimSpace(task.WorkspaceID)
	task.GroupID = strings.TrimSpace(task.GroupID)
	task.ExpirationBucket = strings.TrimSpace(task.ExpirationBucket)
	return task
}

func (command ExpireGroupingRuleCommand) Normalize() ExpireGroupingRuleCommand {
	command.TaskID = strings.TrimSpace(command.TaskID)
	command.WorkspaceID = strings.TrimSpace(command.WorkspaceID)
	command.GroupID = strings.TrimSpace(command.GroupID)
	command.ExpirationBucket = strings.TrimSpace(command.ExpirationBucket)
	return command
}

func (task IndividualMemberExpiryTask) Normalize() IndividualMemberExpiryTask {
	task.ID = strings.TrimSpace(task.ID)
	task.GroupID = strings.TrimSpace(task.GroupID)
	task.NTAccount = strings.TrimSpace(task.NTAccount)
	task.ExpirationBucket = strings.TrimSpace(task.ExpirationBucket)
	return task
}

func (command ExpireIndividualMemberCommand) Normalize() ExpireIndividualMemberCommand {
	command.TaskID = strings.TrimSpace(command.TaskID)
	command.GroupID = strings.TrimSpace(command.GroupID)
	command.NTAccount = strings.TrimSpace(command.NTAccount)
	command.ExpirationBucket = strings.TrimSpace(command.ExpirationBucket)
	return command
}

func (rule Rule) Normalize() Rule {
	rule.AttributeKey = strings.TrimSpace(rule.AttributeKey)
	return rule
}

func (member IndividualMember) Normalize() IndividualMember {
	member.NTAccount = strings.TrimSpace(member.NTAccount)
	return member
}

func (query ListIndividualMembersQuery) Normalize() ListIndividualMembersQuery {
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.GroupID = strings.TrimSpace(query.GroupID)
	if query.Cursor != nil {
		query.Cursor.ID = strings.TrimSpace(query.Cursor.ID)
	}
	return query
}

func (input AddIndividualMembersInput) Normalize() AddIndividualMembersInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	for i := range input.IndividualMembers {
		input.IndividualMembers[i] = input.IndividualMembers[i].Normalize()
	}
	return input
}

func (input UpdateIndividualMemberExpirationInput) Normalize() UpdateIndividualMemberExpirationInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	input.NTAccount = strings.TrimSpace(input.NTAccount)
	return input
}

func (input DeleteIndividualMemberInput) Normalize() DeleteIndividualMemberInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	input.NTAccount = strings.TrimSpace(input.NTAccount)
	return input
}
