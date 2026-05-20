package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
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
	service := NewSystemGroupService(repository,
		WithSystemGroupClock(fixedSystemGroupNow),
		WithSystemGroupIDGenerator(func() string { return "group-1" }),
	)

	model, err := service.CreateSystemGroup(context.Background(), validServiceSystemGroupInput())
	if err != nil {
		t.Fatalf("CreateSystemGroup error = %v, want nil", err)
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

	_, err := service.CreateSystemGroup(context.Background(), group.SystemGroupCreateInput{SystemID: "system-a", Name: " "})
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("CreateSystemGroup error = %v, want ErrInvalidInput", err)
	}
	if repository.createCalls != 0 {
		t.Fatalf("repository calls = %d, want 0", repository.createCalls)
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
