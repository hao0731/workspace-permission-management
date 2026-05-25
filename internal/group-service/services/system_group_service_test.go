package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	permission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)

type fakeSystemGroupRepository struct {
	createGroup      group.SystemGroup
	createProjection group.SystemGroupRelationshipProjection
	listQuery        group.SystemGroupListQuery
	page             group.SystemGroupPage
	err              error
	createCalls      int
	listCalls        int
}

type fakeSystemGroupPermissionClient struct {
	repo                   *fakeSystemGroupRepository
	parameter              permission.WriteRelationshipsParameter
	result                 permission.WriteRelationshipsResult
	err                    error
	resultFunc             func(permission.WriteRelationshipsParameter) permission.WriteRelationshipsResult
	calls                  int
	calledBeforeRepository bool
}

func (f *fakeSystemGroupPermissionClient) WriteRelationships(ctx context.Context, parameter permission.WriteRelationshipsParameter) (permission.WriteRelationshipsResult, error) {
	f.calls++
	f.parameter = parameter
	if f.repo != nil && f.repo.createCalls == 0 {
		f.calledBeforeRepository = true
	}
	if f.err != nil {
		return permission.WriteRelationshipsResult{}, f.err
	}
	if f.resultFunc != nil {
		return f.resultFunc(parameter), nil
	}
	return f.result, nil
}

func (f *fakeSystemGroupRepository) CreateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error) {
	f.createCalls++
	f.createGroup = model
	f.createProjection = projection
	if f.err != nil {
		return group.SystemGroup{}, f.err
	}
	return model, nil
}

func (f *fakeSystemGroupRepository) ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error) {
	f.listCalls++
	f.listQuery = query
	if f.err != nil {
		return group.SystemGroupPage{}, f.err
	}
	return f.page, nil
}

func fixedSystemGroupNow() time.Time {
	return time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
}

func validServiceSystemGroupInput() group.SystemGroupCreateInput {
	return group.SystemGroupCreateInput{
		SystemID: "system-a",
		Name:     "System Admins",
		GroupingRules: []group.SystemGroupRule{
			{AttributeKey: group.GroupAttributeOrganization, Operator: group.OperatorEq, Multi: true, Value: []string{"ORG-200", "ORG-100", "ORG-100"}},
			{AttributeKey: group.GroupAttributeJobLevel, Operator: group.OperatorEq, Multi: false, Value: "M2"},
			{AttributeKey: group.GroupAttributeJobTag, Operator: group.OperatorEq, Multi: true, Value: []string{"a4_reviewer", group.SystemGroupSecretarySentinel}},
		},
	}
}

func TestSystemGroupServiceCreateSystemGroup(t *testing.T) {
	repository := &fakeSystemGroupRepository{}
	permissionClient := &fakeSystemGroupPermissionClient{repo: repository}
	service := NewSystemGroupService(repository,
		WithSystemGroupPermissionClient(permissionClient),
		WithSystemGroupClock(fixedSystemGroupNow),
		WithSystemGroupIDGenerator(func() string { return "group-1" }),
	)

	model, permissionErrors, err := service.CreateSystemGroup(context.Background(), validServiceSystemGroupInput())
	if err != nil {
		t.Fatalf("CreateSystemGroup error = %v, want nil", err)
	}
	if len(permissionErrors) != 0 {
		t.Fatalf("permission errors = %#v, want empty", permissionErrors)
	}
	if permissionClient.calls != 1 {
		t.Fatalf("permission client calls = %d, want 1", permissionClient.calls)
	}
	if !permissionClient.calledBeforeRepository {
		t.Fatal("permission client was not called before repository create")
	}
	if len(permissionClient.parameter.Tasks) != 4 {
		t.Fatalf("permission tasks len = %d, want 4", len(permissionClient.parameter.Tasks))
	}
	for _, task := range permissionClient.parameter.Tasks {
		if task.Operator != permission.RelationshipOperationCreate {
			t.Fatalf("permission task operator = %q, want create", task.Operator)
		}
	}
	if repository.createCalls != 1 {
		t.Fatalf("CreateSystemGroup repository calls = %d, want 1", repository.createCalls)
	}
	if model.ID != "group-1" || model.SystemID != "system-a" || model.Name != "System Admins" {
		t.Fatalf("model = %+v, want normalized group", model)
	}
	if !model.CreatedAt.Equal(fixedSystemGroupNow()) || !model.UpdatedAt.Equal(fixedSystemGroupNow()) {
		t.Fatalf("timestamps = %s/%s, want fixed now", model.CreatedAt, model.UpdatedAt)
	}
	if repository.createProjection.SystemID != "system-a" || repository.createProjection.GroupID != "group-1" {
		t.Fatalf("projection identity = %+v, want system-a/group-1", repository.createProjection)
	}
	if len(repository.createProjection.Relationships) != 4 {
		t.Fatalf("relationships len = %d, want 4", len(repository.createProjection.Relationships))
	}
}

func TestBuildSystemGroupRelationshipProjectionFallbacks(t *testing.T) {
	projection, err := buildSystemGroupRelationshipProjection("system-a", "group-1", []group.SystemGroupRule{}, fixedSystemGroupNow())
	if err != nil {
		t.Fatalf("projection error = %v, want nil", err)
	}
	if len(projection.Relationships) != 2 {
		t.Fatalf("relationships len = %d, want HR and A4 all employee fallbacks", len(projection.Relationships))
	}
}

func TestBuildSystemGroupRelationshipProjectionSecretaryOnlyBuildsStaticAndA4Fallback(t *testing.T) {
	projection, err := buildSystemGroupRelationshipProjection("system-a", "group-1", []group.SystemGroupRule{{
		AttributeKey: group.GroupAttributeJobTag,
		Operator:     group.OperatorEq,
		Multi:        true,
		Value:        []string{group.SystemGroupSecretarySentinel},
	}}, fixedSystemGroupNow())
	if err != nil {
		t.Fatalf("projection error = %v, want nil", err)
	}
	if len(projection.Relationships) != 3 {
		t.Fatalf("relationships len = %d, want HR fallback, static secretary, A4 fallback", len(projection.Relationships))
	}
}

func TestBuildSystemGroupRelationshipProjectionNonSecretaryTagsDoNotBuildStatic(t *testing.T) {
	projection, err := buildSystemGroupRelationshipProjection("system-a", "group-1", []group.SystemGroupRule{{
		AttributeKey: group.GroupAttributeJobTag,
		Operator:     group.OperatorEq,
		Multi:        true,
		Value:        []string{"a4_writer", "a4_reader"},
	}}, fixedSystemGroupNow())
	if err != nil {
		t.Fatalf("projection error = %v, want nil", err)
	}
	if len(projection.Relationships) != 3 {
		t.Fatalf("relationships len = %d, want HR fallback plus two A4 roles", len(projection.Relationships))
	}
}

func TestRelationshipChecksumUsesRelationshipJSON(t *testing.T) {
	projection, err := buildSystemGroupRelationshipProjection("system-a", "group-1", []group.SystemGroupRule{}, fixedSystemGroupNow())
	if err != nil {
		t.Fatalf("projection error = %v, want nil", err)
	}
	raw, err := json.Marshal(projection.Relationships[0].Relationship)
	if err != nil {
		t.Fatalf("marshal relationship: %v", err)
	}
	sum := sha256.Sum256(raw)
	want := hex.EncodeToString(sum[:])
	if projection.Relationships[0].Checksum != want {
		t.Fatalf("checksum = %q, want %q", projection.Relationships[0].Checksum, want)
	}
}

func TestSystemGroupServiceValidationFailureDoesNotCallRepository(t *testing.T) {
	repository := &fakeSystemGroupRepository{}
	service := NewSystemGroupService(repository)

	_, _, err := service.CreateSystemGroup(context.Background(), group.SystemGroupCreateInput{SystemID: "system-a", Name: " "})
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("CreateSystemGroup error = %v, want ErrInvalidInput", err)
	}
	if repository.createCalls != 0 {
		t.Fatalf("repository calls = %d, want 0", repository.createCalls)
	}
}

func TestSystemGroupServiceCreateSystemGroupPermissionFailureDoesNotCallRepository(t *testing.T) {
	repository := &fakeSystemGroupRepository{}
	permissionClient := &fakeSystemGroupPermissionClient{err: errors.New("permission unavailable")}
	service := NewSystemGroupService(repository,
		WithSystemGroupPermissionClient(permissionClient),
		WithSystemGroupClock(fixedSystemGroupNow),
		WithSystemGroupIDGenerator(func() string { return "group-1" }),
	)

	_, _, err := service.CreateSystemGroup(context.Background(), validServiceSystemGroupInput())
	if !errors.Is(err, ErrSystemGroupPermissionWriteFailed) {
		t.Fatalf("CreateSystemGroup error = %v, want ErrSystemGroupPermissionWriteFailed", err)
	}
	if repository.createCalls != 0 {
		t.Fatalf("repository calls = %d, want 0", repository.createCalls)
	}
	if permissionClient.calls != 1 {
		t.Fatalf("permission client calls = %d, want 1", permissionClient.calls)
	}
}

func TestSystemGroupServiceCreateSystemGroupFiltersFailedPermissionRelationships(t *testing.T) {
	repository := &fakeSystemGroupRepository{}
	permissionClient := &fakeSystemGroupPermissionClient{
		resultFunc: func(parameter permission.WriteRelationshipsParameter) permission.WriteRelationshipsResult {
			failed := parameter.Tasks[1]
			return permission.WriteRelationshipsResult{
				FailedTasks: []permission.FailedRelationshipTask{{
					RelationshipTask: failed,
					Error:            "organization rejected",
				}},
			}
		},
	}
	var logBuffer bytes.Buffer
	service := NewSystemGroupService(repository,
		WithSystemGroupPermissionClient(permissionClient),
		WithSystemGroupClock(fixedSystemGroupNow),
		WithSystemGroupIDGenerator(func() string { return "group-1" }),
		WithSystemGroupLogger(slog.New(slog.NewTextHandler(&logBuffer, nil))),
	)

	model, permissionErrors, err := service.CreateSystemGroup(context.Background(), validServiceSystemGroupInput())
	if err != nil {
		t.Fatalf("CreateSystemGroup error = %v, want nil", err)
	}
	if len(permissionErrors) != 1 || permissionErrors[0] != "organization rejected" {
		t.Fatalf("permission errors = %#v, want organization rejected", permissionErrors)
	}
	if len(repository.createProjection.Relationships) != 3 {
		t.Fatalf("saved relationships len = %d, want failed relationship removed", len(repository.createProjection.Relationships))
	}
	orgValues := systemGroupRuleValues(model.GroupingRules, group.GroupAttributeOrganization)
	if len(orgValues) != 1 || orgValues[0] != "ORG-100" {
		t.Fatalf("organization rule values = %#v, want [ORG-100]", orgValues)
	}
	savedOrgValues := systemGroupRuleValues(repository.createGroup.GroupingRules, group.GroupAttributeOrganization)
	if len(savedOrgValues) != 1 || savedOrgValues[0] != "ORG-100" {
		t.Fatalf("saved organization rule values = %#v, want [ORG-100]", savedOrgValues)
	}
	output := logBuffer.String()
	if !strings.Contains(output, "permission API relationship write partially failed") {
		t.Fatalf("log output = %q, want partial failure warning", output)
	}
	if !strings.Contains(output, "system_id=system-a") {
		t.Fatalf("log output = %q, want system_id", output)
	}
	if !strings.Contains(output, "group_id=group-1") {
		t.Fatalf("log output = %q, want group_id", output)
	}
	if !strings.Contains(output, "failed_task_count=1") {
		t.Fatalf("log output = %q, want failed_task_count", output)
	}
	if !strings.Contains(output, "organization rejected") {
		t.Fatalf("log output = %q, want permission error", output)
	}
}

func TestSystemGroupServiceCreateSystemGroupRebuildsRulesFromAcceptedRelationships(t *testing.T) {
	repository := &fakeSystemGroupRepository{}
	permissionClient := &fakeSystemGroupPermissionClient{
		resultFunc: func(parameter permission.WriteRelationshipsParameter) permission.WriteRelationshipsResult {
			failed := parameter.Tasks[2]
			return permission.WriteRelationshipsResult{
				FailedTasks: []permission.FailedRelationshipTask{{
					RelationshipTask: failed,
					Error:            "static attributes rejected",
				}},
			}
		},
	}
	service := NewSystemGroupService(repository,
		WithSystemGroupPermissionClient(permissionClient),
		WithSystemGroupClock(fixedSystemGroupNow),
		WithSystemGroupIDGenerator(func() string { return "group-1" }),
	)

	model, permissionErrors, err := service.CreateSystemGroup(context.Background(), validServiceSystemGroupInput())
	if err != nil {
		t.Fatalf("CreateSystemGroup error = %v, want nil", err)
	}
	if len(permissionErrors) != 1 || permissionErrors[0] != "static attributes rejected" {
		t.Fatalf("permission errors = %#v, want static attributes rejected", permissionErrors)
	}
	if values := systemGroupRuleValues(model.GroupingRules, group.GroupAttributeJobLevel); len(values) != 0 {
		t.Fatalf("job_level values = %#v, want static relationship removed", values)
	}
	tagValues := systemGroupRuleValues(model.GroupingRules, group.GroupAttributeJobTag)
	if len(tagValues) != 1 || tagValues[0] != "a4_reviewer" {
		t.Fatalf("job_tag values = %#v, want [a4_reviewer]", tagValues)
	}
}

func TestSystemGroupServiceListSystemGroups(t *testing.T) {
	repository := &fakeSystemGroupRepository{page: group.SystemGroupPage{Groups: []group.SystemGroup{{ID: "group-1"}}}}
	service := NewSystemGroupService(repository)

	page, err := service.ListSystemGroups(context.Background(), group.SystemGroupListQuery{SystemID: " system-a ", Limit: 20})
	if err != nil {
		t.Fatalf("ListSystemGroups error = %v, want nil", err)
	}
	if len(page.Groups) != 1 || repository.listQuery.SystemID != "system-a" {
		t.Fatalf("page/query = %+v/%+v, want normalized list", page, repository.listQuery)
	}
}

func systemGroupRuleValues(rules []group.SystemGroupRule, key group.GroupAttributeKey) []string {
	values := make([]string, 0)
	for _, rule := range rules {
		if rule.AttributeKey != key {
			continue
		}
		if rule.Multi {
			ruleValues, ok := rule.Value.([]string)
			if ok {
				values = append(values, ruleValues...)
			}
			continue
		}
		value, ok := rule.Value.(string)
		if ok {
			values = append(values, value)
		}
	}
	return values
}
