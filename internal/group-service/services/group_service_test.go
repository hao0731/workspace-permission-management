package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type fakeGroupRepository struct {
	input                group.Group
	getQuery             group.GetQuery
	deleteInput          group.DeleteInput
	updateInput          group.UpdateGroupingRuleInput
	listQuery            group.ListIndividualMembersQuery
	addInput             group.AddIndividualMembersInput
	memberUpdate         group.UpdateIndividualMemberExpirationInput
	memberDelete         group.DeleteIndividualMemberInput
	expireInput          group.ExpireGroupingRuleCommand
	expireMemberInput    group.ExpireIndividualMemberCommand
	model                *group.Group
	page                 group.IndividualMemberPage
	addedMembers         []group.IndividualMember
	expireStatus         group.ExpireGroupingRuleStatus
	expireMemberStatus   group.ExpireIndividualMemberStatus
	err                  error
	expireErr            error
	expireMemberErr      error
	calls                int
	getCalls             int
	deleteCalls          int
	updateCalls          int
	listCalls            int
	memberAddCalls       int
	memberUpdCalls       int
	memberDelCalls       int
	deleteTimestamp      time.Time
	updateTimestamp      time.Time
	memberUpdTime        time.Time
	memberDelTime        time.Time
	expiredAt            time.Time
	expireLocation       *time.Location
	memberExpiredAt      time.Time
	memberExpireLocation *time.Location
}

func (f *fakeGroupRepository) Create(ctx context.Context, input group.Group) (group.Group, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return group.Group{}, f.err
	}
	return input, nil
}

func (f *fakeGroupRepository) Get(ctx context.Context, query group.GetQuery) (*group.Group, error) {
	f.getCalls++
	f.getQuery = query
	if f.err != nil {
		return nil, f.err
	}
	return f.model, nil
}

func (f *fakeGroupRepository) Delete(ctx context.Context, input group.DeleteInput, deletedAt time.Time) error {
	f.deleteCalls++
	f.deleteInput = input
	f.deleteTimestamp = deletedAt
	return f.err
}

func (f *fakeGroupRepository) UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput, updatedAt time.Time) error {
	f.updateCalls++
	f.updateInput = input
	f.updateTimestamp = updatedAt
	return f.err
}

func (f *fakeGroupRepository) ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error) {
	f.listCalls++
	f.listQuery = query
	if f.err != nil {
		return group.IndividualMemberPage{}, f.err
	}
	return f.page, nil
}

func (f *fakeGroupRepository) AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error) {
	f.memberAddCalls++
	f.addInput = input
	if f.err != nil {
		return nil, f.err
	}
	if f.addedMembers != nil {
		return f.addedMembers, nil
	}
	return input.IndividualMembers, nil
}

func (f *fakeGroupRepository) UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput, updatedAt time.Time) error {
	f.memberUpdCalls++
	f.memberUpdate = input
	f.memberUpdTime = updatedAt
	return f.err
}

func (f *fakeGroupRepository) DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput, deletedAt time.Time) error {
	f.memberDelCalls++
	f.memberDelete = input
	f.memberDelTime = deletedAt
	return f.err
}

func (f *fakeGroupRepository) ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireGroupingRuleStatus, error) {
	f.expireInput = input
	f.expiredAt = expiredAt
	f.expireLocation = bucketLocation
	if f.expireStatus == "" {
		f.expireStatus = group.ExpireGroupingRuleStatusExpired
	}
	return f.expireStatus, f.expireErr
}

func (f *fakeGroupRepository) ExpireIndividualMember(ctx context.Context, input group.ExpireIndividualMemberCommand, expiredAt time.Time, bucketLocation *time.Location) (group.ExpireIndividualMemberStatus, error) {
	f.expireMemberInput = input
	f.memberExpiredAt = expiredAt
	f.memberExpireLocation = bucketLocation
	if f.expireMemberStatus == "" {
		f.expireMemberStatus = group.ExpireIndividualMemberStatusExpired
	}
	return f.expireMemberStatus, f.expireMemberErr
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
}

func serviceFutureTime() time.Time {
	return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
}

func validServiceCreateInput() group.CreateInput {
	return group.CreateInput{
		WorkspaceID: "workspace-1",
		Name:        " Design Reviewers ",
		Description: "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: " department ",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: serviceFutureTime(),
		},
		IndividualMembers: []group.IndividualMember{{
			NTAccount:      " user1 ",
			ExpirationDate: serviceFutureTime(),
		}},
	}
}

func TestGroupServiceCreateGroup(t *testing.T) {
	repository := &fakeGroupRepository{}
	ids := []string{"group-1", "member-1", "member-task-1", "task-1"}
	service := NewGroupService(repository,
		WithGroupClock(fixedNow),
		WithGroupIDGenerator(func() string {
			id := ids[0]
			ids = ids[1:]
			return id
		}),
	)

	model, err := service.CreateGroup(context.Background(), validServiceCreateInput())
	if err != nil {
		t.Fatalf("CreateGroup error = %v, want nil", err)
	}
	if repository.calls != 1 {
		t.Fatalf("repository calls = %d, want 1", repository.calls)
	}
	if model.ID != "group-1" {
		t.Fatalf("ID = %q, want group-1", model.ID)
	}
	if model.Name != "Design Reviewers" {
		t.Fatalf("Name = %q, want Design Reviewers", model.Name)
	}
	if model.NormalizedName != "Design Reviewers" {
		t.Fatalf("NormalizedName = %q, want Design Reviewers", model.NormalizedName)
	}
	if model.GroupingRule.Rules[0].AttributeKey != "department" {
		t.Fatalf("AttributeKey = %q, want department", model.GroupingRule.Rules[0].AttributeKey)
	}
	if model.IndividualMembers[0].ID != "member-1" {
		t.Fatalf("member ID = %q, want member-1", model.IndividualMembers[0].ID)
	}
	if model.IndividualMembers[0].GroupID != "group-1" {
		t.Fatalf("member GroupID = %q, want group-1", model.IndividualMembers[0].GroupID)
	}
	if !model.CreatedAt.Equal(fixedNow()) || !model.UpdatedAt.Equal(fixedNow()) {
		t.Fatalf("timestamps = %s/%s, want fixed now", model.CreatedAt, model.UpdatedAt)
	}
}

func TestGroupServiceCreateGroupCreatesExpiryTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 16, 30, 0, 0, time.UTC)
	location, err := group.ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}
	service := NewGroupService(repository,
		WithGroupIDGenerator(sequenceGenerator("group-1", "task-1")),
		WithGroupClock(func() time.Time { return now }),
		WithGroupExpiryBucketLocation(location),
	)

	_, err = service.CreateGroup(context.Background(), group.CreateInput{
		WorkspaceID: "workspace-1",
		Name:        "Reviewers",
		GroupingRule: group.GroupingRule{
			Rules:          []group.Rule{{AttributeKey: "department", Operator: group.OperatorEq, Value: "ABCD-123"}},
			ExpirationDate: now.Add(24 * time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	if repository.input.ExpiryTask == nil {
		t.Fatal("created group expiry task is nil")
	}
	if repository.input.ExpiryTask.ID != "task-1" {
		t.Fatalf("task id = %q, want task-1", repository.input.ExpiryTask.ID)
	}
	if repository.input.ExpiryTask.ExpirationBucket != "2026-05-12" {
		t.Fatalf("expiration bucket = %q, want 2026-05-12", repository.input.ExpiryTask.ExpirationBucket)
	}
	if repository.input.GroupingRule.ExpiredAt != nil {
		t.Fatalf("expired_at = %v, want nil", repository.input.GroupingRule.ExpiredAt)
	}
}

func TestGroupServiceCreateGroupCreatesIndividualMemberExpiryTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 16, 30, 0, 0, time.UTC)
	location, err := group.ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}
	service := NewGroupService(repository,
		WithGroupIDGenerator(sequenceGenerator("group-1", "member-1", "member-task-1", "group-task-1")),
		WithGroupClock(func() time.Time { return now }),
		WithIndividualMemberExpiryBucketLocation(location),
	)

	_, err = service.CreateGroup(context.Background(), validServiceCreateInput())
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	task := repository.input.IndividualMembers[0].ExpiryTask
	if task == nil {
		t.Fatal("individual member expiry task is nil")
	}
	if task.ID != "member-task-1" || task.GroupID != "group-1" || task.NTAccount != "user1" {
		t.Fatalf("task = %+v, want member-task-1/group-1/user1", task)
	}
	if task.ExpirationBucket != "2026-06-01" {
		t.Fatalf("expiration bucket = %q, want 2026-06-01", task.ExpirationBucket)
	}
}

func TestGroupServiceCreateGroupValidationFailureDoesNotCallRepository(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))
	input := validServiceCreateInput()
	input.Name = " "

	_, err := service.CreateGroup(context.Background(), input)
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("CreateGroup error = %v, want ErrInvalidInput", err)
	}
	if repository.calls != 0 {
		t.Fatalf("repository calls = %d, want 0", repository.calls)
	}
}

func TestGroupServiceCreateGroupConfiguredLimitFailureDoesNotCallRepository(t *testing.T) {
	tests := []struct {
		name                 string
		maxGroupingRules     int
		maxIndividualMembers int
		mutate               func(*group.CreateInput)
	}{
		{
			name:                 "too many grouping rules",
			maxGroupingRules:     1,
			maxIndividualMembers: 1000,
			mutate: func(input *group.CreateInput) {
				input.GroupingRule.Rules = []group.Rule{
					{
						AttributeKey: "department",
						Operator:     group.OperatorEq,
						Multi:        false,
						Value:        "ABCD-123",
					},
					{
						AttributeKey: "job_code",
						Operator:     group.OperatorEq,
						Multi:        false,
						Value:        "EFGH-456",
					},
				}
			},
		},
		{
			name:                 "too many individual members",
			maxGroupingRules:     10,
			maxIndividualMembers: 1,
			mutate: func(input *group.CreateInput) {
				input.IndividualMembers = []group.IndividualMember{
					{NTAccount: "user1", ExpirationDate: serviceFutureTime()},
					{NTAccount: "user2", ExpirationDate: serviceFutureTime()},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &fakeGroupRepository{}
			service := NewGroupService(repository,
				WithGroupClock(fixedNow),
				WithGroupValidationLimits(tt.maxIndividualMembers, tt.maxGroupingRules),
			)
			input := validServiceCreateInput()
			tt.mutate(&input)

			_, err := service.CreateGroup(context.Background(), input)
			if !errors.Is(err, group.ErrInvalidInput) {
				t.Fatalf("CreateGroup error = %v, want ErrInvalidInput", err)
			}
			if repository.calls != 0 {
				t.Fatalf("repository calls = %d, want 0", repository.calls)
			}
		})
	}
}

func TestGroupServiceCreateGroupDuplicateName(t *testing.T) {
	repository := &fakeGroupRepository{err: group.ErrDuplicateName}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	_, err := service.CreateGroup(context.Background(), validServiceCreateInput())
	if !errors.Is(err, group.ErrDuplicateName) {
		t.Fatalf("CreateGroup error = %v, want ErrDuplicateName", err)
	}
}

func TestGroupServiceCreateGroupRepositoryFailure(t *testing.T) {
	repository := &fakeGroupRepository{err: errors.New("database unavailable")}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	_, err := service.CreateGroup(context.Background(), validServiceCreateInput())
	if err == nil {
		t.Fatal("CreateGroup error = nil, want error")
	}
	if errors.Is(err, group.ErrDuplicateName) {
		t.Fatalf("CreateGroup error = %v, should not be ErrDuplicateName", err)
	}
}

func TestGroupServiceGetGroup(t *testing.T) {
	model := group.Group{ID: "group-1", WorkspaceID: "workspace-1", Name: "Design Reviewers"}
	repository := &fakeGroupRepository{model: &model}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	got, err := service.GetGroup(context.Background(), group.GetQuery{WorkspaceID: " workspace-1 ", GroupID: " group-1 "})
	if err != nil {
		t.Fatalf("GetGroup error = %v, want nil", err)
	}
	if got == nil || got.ID != "group-1" {
		t.Fatalf("group = %+v, want group-1", got)
	}
	if repository.getQuery.WorkspaceID != "workspace-1" || repository.getQuery.GroupID != "group-1" {
		t.Fatalf("query = %+v, want trimmed values", repository.getQuery)
	}
}

func TestGroupServiceDeleteGroup(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	if err := service.DeleteGroup(context.Background(), group.DeleteInput{WorkspaceID: " workspace-1 ", GroupID: " group-1 "}); err != nil {
		t.Fatalf("DeleteGroup error = %v, want nil", err)
	}
	if repository.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", repository.deleteCalls)
	}
	if !repository.deleteTimestamp.Equal(fixedNow()) {
		t.Fatalf("deletedAt = %s, want fixed now", repository.deleteTimestamp)
	}
}

func TestGroupServiceUpdateGroupingRule(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    " workspace-1 ",
		GroupID:        " group-1 ",
		ExpirationDate: serviceFutureTime(),
		Rules: []group.Rule{{
			AttributeKey: " department ",
			Operator:     group.OperatorEq,
			Multi:        false,
			Value:        "ABCD-123",
		}},
	})
	if err != nil {
		t.Fatalf("UpdateGroupingRule error = %v, want nil", err)
	}
	if repository.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", repository.updateCalls)
	}
	if repository.updateInput.Rules[0].AttributeKey != "department" {
		t.Fatalf("rules = %+v, want trimmed department", repository.updateInput.Rules)
	}
	if !repository.updateTimestamp.Equal(fixedNow()) {
		t.Fatalf("updatedAt = %s, want fixed now", repository.updateTimestamp)
	}
}

func TestGroupServiceUpdateGroupingRuleCreatesExpiryTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	service := NewGroupService(repository,
		WithGroupIDGenerator(sequenceGenerator("task-1")),
		WithGroupClock(func() time.Time { return now }),
	)

	err := service.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		Rules:          []group.Rule{{AttributeKey: "department", Operator: group.OperatorEq, Value: "ABCD-123"}},
		ExpirationDate: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("UpdateGroupingRule() error = %v", err)
	}
	if repository.updateInput.ExpiryTask == nil {
		t.Fatal("updated grouping rule expiry task is nil")
	}
	if repository.updateInput.ExpiryTask.ID != "task-1" {
		t.Fatalf("task id = %q, want task-1", repository.updateInput.ExpiryTask.ID)
	}
}

func TestGroupServiceUpdateGroupingRuleWithoutRulesClearsExpiryTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	service := NewGroupService(repository, WithGroupClock(func() time.Time { return now }))

	err := service.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		Rules:          []group.Rule{},
		ExpirationDate: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("UpdateGroupingRule() error = %v", err)
	}
	if repository.updateInput.ExpiryTask != nil {
		t.Fatalf("expiry task = %+v, want nil", repository.updateInput.ExpiryTask)
	}
}

func TestGroupServiceExpireGroupingRule(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{expireStatus: group.ExpireGroupingRuleStatusStaleTask}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	location, err := group.ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}
	service := NewGroupService(repository,
		WithGroupClock(func() time.Time { return now }),
		WithGroupExpiryBucketLocation(location),
	)

	status, err := service.ExpireGroupingRule(context.Background(), group.ExpireGroupingRuleCommand{
		TaskID:           "task-1",
		WorkspaceID:      "workspace-1",
		GroupID:          "group-1",
		ExpirationBucket: "2026-05-10",
	})
	if err != nil {
		t.Fatalf("ExpireGroupingRule() error = %v", err)
	}
	if status != group.ExpireGroupingRuleStatusStaleTask {
		t.Fatalf("status = %s, want stale_task", status)
	}
	if !repository.expiredAt.Equal(now) {
		t.Fatalf("expiredAt = %s, want %s", repository.expiredAt, now)
	}
	if repository.expireLocation.String() != "UTC+08:00" {
		t.Fatalf("location = %s, want UTC+08:00", repository.expireLocation)
	}
}

func TestGroupServiceExpireIndividualMember(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{expireMemberStatus: group.ExpireIndividualMemberStatusStaleTask}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	service := newIndividualMemberExpiryTestService(t, repository, now)

	status, err := service.ExpireIndividualMember(context.Background(), group.ExpireIndividualMemberCommand{
		TaskID:           "task-1",
		GroupID:          "group-1",
		NTAccount:        "user1",
		ExpirationBucket: "2026-05-10",
	})
	if err != nil {
		t.Fatalf("ExpireIndividualMember() error = %v", err)
	}
	if status != group.ExpireIndividualMemberStatusStaleTask {
		t.Fatalf("status = %s, want stale_task", status)
	}
	assertIndividualMemberExpiryCall(t, repository, now)
}

func newIndividualMemberExpiryTestService(t *testing.T, repository *fakeGroupRepository, now time.Time) *GroupService {
	t.Helper()

	location, err := group.ParseExpirationBucketLocation("UTC+8")
	if err != nil {
		t.Fatal(err)
	}
	return NewGroupService(repository,
		WithGroupClock(func() time.Time { return now }),
		WithIndividualMemberExpiryBucketLocation(location),
	)
}

func assertIndividualMemberExpiryCall(t *testing.T, repository *fakeGroupRepository, now time.Time) {
	t.Helper()

	if !repository.memberExpiredAt.Equal(now) {
		t.Fatalf("expiredAt = %s, want %s", repository.memberExpiredAt, now)
	}
	if repository.memberExpireLocation.String() != "UTC+08:00" {
		t.Fatalf("location = %s, want UTC+08:00", repository.memberExpireLocation)
	}
}

func TestGroupServiceUpdateGroupingRuleNotFound(t *testing.T) {
	repository := &fakeGroupRepository{err: group.ErrNotFound}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.UpdateGroupingRule(context.Background(), group.UpdateGroupingRuleInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		ExpirationDate: serviceFutureTime(),
	})
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("UpdateGroupingRule error = %v, want ErrNotFound", err)
	}
}

func TestGroupServiceListIndividualMembers(t *testing.T) {
	page := group.IndividualMemberPage{
		Members: []group.IndividualMember{{ID: "member-1", GroupID: "group-1", NTAccount: "user1"}},
	}
	repository := &fakeGroupRepository{page: page}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	got, err := service.ListIndividualMembers(context.Background(), group.ListIndividualMembersQuery{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		Limit:       20,
	})
	if err != nil {
		t.Fatalf("ListIndividualMembers error = %v, want nil", err)
	}
	if len(got.Members) != 1 || got.Members[0].NTAccount != "user1" {
		t.Fatalf("members = %+v, want user1", got.Members)
	}
	if repository.listQuery.WorkspaceID != "workspace-1" || repository.listQuery.GroupID != "group-1" {
		t.Fatalf("query = %+v, want trimmed identity", repository.listQuery)
	}
}

func TestGroupServiceAddIndividualMembers(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository,
		WithGroupClock(fixedNow),
		WithGroupIDGenerator(sequenceGenerator("member-2", "member-task-2")),
	)

	members, err := service.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		IndividualMembers: []group.IndividualMember{{
			NTAccount:      " user2 ",
			ExpirationDate: serviceFutureTime(),
		}},
	})
	if err != nil {
		t.Fatalf("AddIndividualMembers error = %v, want nil", err)
	}
	if repository.memberAddCalls != 1 {
		t.Fatalf("member add calls = %d, want 1", repository.memberAddCalls)
	}
	if len(members) != 1 || members[0].ID != "member-2" || members[0].GroupID != "group-1" {
		t.Fatalf("members = %+v, want generated member for group-1", members)
	}
	if !members[0].CreatedAt.Equal(fixedNow()) || !members[0].UpdatedAt.Equal(fixedNow()) {
		t.Fatalf("timestamps = %s/%s, want fixed now", members[0].CreatedAt, members[0].UpdatedAt)
	}
}

func TestGroupServiceAddIndividualMembersCreatesExpiryTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	service := NewGroupService(repository,
		WithGroupIDGenerator(sequenceGenerator("member-2", "member-task-2")),
		WithGroupClock(func() time.Time { return now }),
	)

	_, err := service.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		IndividualMembers: []group.IndividualMember{{
			NTAccount:      "user2",
			ExpirationDate: now.Add(24 * time.Hour),
		}},
	})
	if err != nil {
		t.Fatalf("AddIndividualMembers() error = %v", err)
	}
	task := repository.addInput.IndividualMembers[0].ExpiryTask
	if task == nil {
		t.Fatal("individual member expiry task is nil")
	}
	if task.ID != "member-task-2" || task.GroupID != "group-1" || task.NTAccount != "user2" {
		t.Fatalf("task = %+v, want member-task-2/group-1/user2", task)
	}
	if task.ExpirationBucket != "2026-05-11" {
		t.Fatalf("expiration bucket = %q, want 2026-05-11", task.ExpirationBucket)
	}
}

func TestGroupServiceAddIndividualMembersValidationFailureDoesNotCallRepository(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	_, err := service.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
	})
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("AddIndividualMembers error = %v, want ErrInvalidInput", err)
	}
	if repository.memberAddCalls != 0 {
		t.Fatalf("member add calls = %d, want 0", repository.memberAddCalls)
	}
}

func TestGroupServiceAddIndividualMembersPreservesKnownErrors(t *testing.T) {
	for _, knownErr := range []error{group.ErrDuplicateMember, group.ErrNotFound} {
		t.Run(knownErr.Error(), func(t *testing.T) {
			repository := &fakeGroupRepository{err: knownErr}
			service := NewGroupService(repository, WithGroupClock(fixedNow))

			_, err := service.AddIndividualMembers(context.Background(), group.AddIndividualMembersInput{
				WorkspaceID: "workspace-1",
				GroupID:     "group-1",
				IndividualMembers: []group.IndividualMember{{
					NTAccount:      "user2",
					ExpirationDate: serviceFutureTime(),
				}},
			})
			if !errors.Is(err, knownErr) {
				t.Fatalf("AddIndividualMembers error = %v, want %v", err, knownErr)
			}
		})
	}
}

func TestGroupServiceUpdateIndividualMemberExpiration(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    " workspace-1 ",
		GroupID:        " group-1 ",
		NTAccount:      " user2 ",
		ExpirationDate: serviceFutureTime(),
	})
	if err != nil {
		t.Fatalf("UpdateIndividualMemberExpiration error = %v, want nil", err)
	}
	if repository.memberUpdCalls != 1 {
		t.Fatalf("member update calls = %d, want 1", repository.memberUpdCalls)
	}
	if repository.memberUpdate.NTAccount != "user2" {
		t.Fatalf("member update = %+v, want trimmed user2", repository.memberUpdate)
	}
	if !repository.memberUpdTime.Equal(fixedNow()) {
		t.Fatalf("updatedAt = %s, want fixed now", repository.memberUpdTime)
	}
}

func TestGroupServiceUpdateIndividualMemberExpirationCreatesReplacementTask(t *testing.T) {
	t.Parallel()

	repository := &fakeGroupRepository{}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	service := NewGroupService(repository,
		WithGroupIDGenerator(sequenceGenerator("member-task-new")),
		WithGroupClock(func() time.Time { return now }),
	)

	err := service.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		NTAccount:      "user2",
		ExpirationDate: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("UpdateIndividualMemberExpiration() error = %v", err)
	}
	task := repository.memberUpdate.ExpiryTask
	if task == nil {
		t.Fatal("replacement task is nil")
	}
	if task.ID != "member-task-new" || task.GroupID != "group-1" || task.NTAccount != "user2" {
		t.Fatalf("task = %+v, want member-task-new/group-1/user2", task)
	}
}

func TestGroupServiceUpdateIndividualMemberExpirationNotFound(t *testing.T) {
	repository := &fakeGroupRepository{err: group.ErrNotFound}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.UpdateIndividualMemberExpiration(context.Background(), group.UpdateIndividualMemberExpirationInput{
		WorkspaceID:    "workspace-1",
		GroupID:        "group-1",
		NTAccount:      "user2",
		ExpirationDate: serviceFutureTime(),
	})
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("UpdateIndividualMemberExpiration error = %v, want ErrNotFound", err)
	}
}

func TestGroupServiceDeleteIndividualMember(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.DeleteIndividualMember(context.Background(), group.DeleteIndividualMemberInput{
		WorkspaceID: " workspace-1 ",
		GroupID:     " group-1 ",
		NTAccount:   " user2 ",
	})
	if err != nil {
		t.Fatalf("DeleteIndividualMember error = %v, want nil", err)
	}
	if repository.memberDelCalls != 1 {
		t.Fatalf("member delete calls = %d, want 1", repository.memberDelCalls)
	}
	if repository.memberDelete.NTAccount != "user2" {
		t.Fatalf("member delete = %+v, want trimmed user2", repository.memberDelete)
	}
	if !repository.memberDelTime.Equal(fixedNow()) {
		t.Fatalf("deletedAt = %s, want fixed now", repository.memberDelTime)
	}
}

func TestGroupServiceDeleteIndividualMemberValidationFailureDoesNotCallRepository(t *testing.T) {
	repository := &fakeGroupRepository{}
	service := NewGroupService(repository, WithGroupClock(fixedNow))

	err := service.DeleteIndividualMember(context.Background(), group.DeleteIndividualMemberInput{
		WorkspaceID: "workspace-1",
		GroupID:     "group-1",
		NTAccount:   " ",
	})
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("DeleteIndividualMember error = %v, want ErrInvalidInput", err)
	}
	if repository.memberDelCalls != 0 {
		t.Fatalf("member delete calls = %d, want 0", repository.memberDelCalls)
	}
}

func sequenceGenerator(values ...string) func() string {
	index := 0
	return func() string {
		if index >= len(values) {
			return values[len(values)-1]
		}
		value := values[index]
		index++
		return value
	}
}
