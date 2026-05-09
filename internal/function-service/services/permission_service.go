package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
)

type PermissionRepository interface {
	Save(ctx context.Context, input permission.Permission) (permission.Permission, error)
	Get(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error)
}

type PermissionOption func(*PermissionService)

func WithPermissionIDGenerator(generator func() string) PermissionOption {
	return func(s *PermissionService) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}

func WithPermissionClock(clock func() time.Time) PermissionOption {
	return func(s *PermissionService) {
		if clock != nil {
			s.now = clock
		}
	}
}

type PermissionService struct {
	repository  PermissionRepository
	idGenerator func() string
	now         func() time.Time
}

func NewPermissionService(repository PermissionRepository, opts ...PermissionOption) *PermissionService {
	service := &PermissionService{
		repository:  repository,
		idGenerator: uuid.NewString,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *PermissionService) SavePermission(ctx context.Context, input permission.SaveInput) (permission.Permission, error) {
	if err := input.Validate(); err != nil {
		return permission.Permission{}, err
	}

	now := s.now()
	model := permission.Permission{
		ID:               s.idGenerator(),
		WorkspaceID:      input.WorkspaceID,
		FunctionKey:      input.FunctionKey,
		CreatedAt:        now,
		UpdatedAt:        now,
		OfficePermission: s.normalizeSection(*input.OfficePermission),
		RemotePermission: s.normalizeSection(*input.RemotePermission),
	}

	s.assignMissingRuleIDs(&model)

	saved, err := s.repository.Save(ctx, model)
	if err != nil {
		return permission.Permission{}, fmt.Errorf("save permissions: %w", err)
	}
	return saved, nil
}

func (s *PermissionService) GetPermission(ctx context.Context, query permission.GetQuery) (permission.Permission, bool, error) {
	if err := query.Validate(); err != nil {
		return permission.Permission{}, false, err
	}

	model, found, err := s.repository.Get(ctx, query)
	if err != nil {
		return permission.Permission{}, false, fmt.Errorf("get permissions: %w", err)
	}
	return model, found, nil
}

func (s *PermissionService) normalizeSection(section permission.PermissionSection) permission.PermissionSection {
	seen := map[string]struct{}{}
	extraRules := make([]permission.ExtraRule, 0, len(section.ExtraRules))
	for _, rule := range section.ExtraRules {
		key := semanticRuleKey(rule)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		extraRules = append(extraRules, cloneExtraRule(rule))
	}
	return permission.PermissionSection{
		BaselineRule: permission.BaselineRule{
			ActionID:     section.BaselineRule.ActionID,
			ResourceTags: append([]string(nil), section.BaselineRule.ResourceTags...),
			Enabled:      section.BaselineRule.Enabled,
		},
		ExtraRules: extraRules,
	}
}

func (s *PermissionService) assignMissingRuleIDs(model *permission.Permission) {
	used := map[string]struct{}{}
	collectRuleIDs(model.OfficePermission.ExtraRules, used)
	collectRuleIDs(model.RemotePermission.ExtraRules, used)
	assignRuleIDs(model.OfficePermission.ExtraRules, used, s.idGenerator)
	assignRuleIDs(model.RemotePermission.ExtraRules, used, s.idGenerator)
}

func collectRuleIDs(rules []permission.ExtraRule, used map[string]struct{}) {
	for _, rule := range rules {
		if rule.RuleID != "" {
			used[rule.RuleID] = struct{}{}
		}
	}
}

func assignRuleIDs(rules []permission.ExtraRule, used map[string]struct{}, generator func() string) {
	for i := range rules {
		if rules[i].RuleID != "" {
			continue
		}
		rules[i].RuleID = nextUniqueID(used, generator)
	}
}

func nextUniqueID(used map[string]struct{}, generator func() string) string {
	for {
		id := generator()
		if strings.TrimSpace(id) == "" {
			continue
		}
		if _, ok := used[id]; ok {
			continue
		}
		used[id] = struct{}{}
		return id
	}
}

func semanticRuleKey(rule permission.ExtraRule) string {
	return strings.Join([]string{
		strings.Join(canonicalStrings(rule.GroupIDs), "\x00"),
		rule.ActionID,
		strings.Join(canonicalStrings(rule.ResourceTags), "\x00"),
		rule.ExpirationDate.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}, "\x1f")
}

func canonicalStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func cloneExtraRule(rule permission.ExtraRule) permission.ExtraRule {
	return permission.ExtraRule{
		RuleID:         rule.RuleID,
		GroupIDs:       append([]string(nil), rule.GroupIDs...),
		ActionID:       rule.ActionID,
		ResourceTags:   append([]string(nil), rule.ResourceTags...),
		ExpirationDate: rule.ExpirationDate,
	}
}
