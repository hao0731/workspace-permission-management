package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
)

type fakeGroupRepository struct {
	input group.Group
	err   error
	calls int
}

func (f *fakeGroupRepository) Create(ctx context.Context, input group.Group) (group.Group, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return group.Group{}, f.err
	}
	return input, nil
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
	ids := []string{"group-1", "member-1"}
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
		name   string
		limits group.ValidationLimits
		mutate func(*group.CreateInput)
	}{
		{
			name: "too many grouping rules",
			limits: group.ValidationLimits{
				MaxGroupingRules:     1,
				MaxIndividualMembers: 1000,
			},
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
			name: "too many individual members",
			limits: group.ValidationLimits{
				MaxGroupingRules:     10,
				MaxIndividualMembers: 1,
			},
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
				WithGroupValidationLimits(tt.limits),
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
