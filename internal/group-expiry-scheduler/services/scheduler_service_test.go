package services

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

type fakeTaskRepository struct {
	groupBatches  [][]expiry.GroupTask
	memberBatches [][]expiry.IndividualMemberTask
	groupErr      error
	memberErr     error
	groupCalls    int
	memberCalls   int
}

func (f *fakeTaskRepository) ListDueGroupTasks(_ context.Context, _ string, _ *expiry.Cursor, _ int) ([]expiry.GroupTask, error) {
	if f.groupErr != nil {
		return nil, f.groupErr
	}
	if f.groupCalls >= len(f.groupBatches) {
		return nil, nil
	}
	batch := f.groupBatches[f.groupCalls]
	f.groupCalls++
	return batch, nil
}

func (f *fakeTaskRepository) ListDueIndividualMemberTasks(_ context.Context, _ string, _ *expiry.Cursor, _ int) ([]expiry.IndividualMemberTask, error) {
	if f.memberErr != nil {
		return nil, f.memberErr
	}
	if f.memberCalls >= len(f.memberBatches) {
		return nil, nil
	}
	batch := f.memberBatches[f.memberCalls]
	f.memberCalls++
	return batch, nil
}

type fakeCommandPublisher struct {
	groupPublished  []string
	memberPublished []string
	groupErrByID    map[string]error
	memberErrByID   map[string]error
}

func (f *fakeCommandPublisher) PublishGroupExpiryCommand(_ context.Context, task expiry.GroupTask) error {
	f.groupPublished = append(f.groupPublished, task.ID)
	return f.groupErrByID[task.ID]
}

func (f *fakeCommandPublisher) PublishIndividualMemberExpiryCommand(_ context.Context, task expiry.IndividualMemberTask) error {
	f.memberPublished = append(f.memberPublished, task.ID)
	return f.memberErrByID[task.ID]
}

func TestSchedulerServiceRunScansUntilEmpty(t *testing.T) {
	repository := &fakeTaskRepository{
		groupBatches: [][]expiry.GroupTask{
			{{ID: "group-task-1", ExpirationBucket: "2026-05-15"}, {ID: "group-task-2", ExpirationBucket: "2026-05-16"}},
			{{ID: "group-task-3", ExpirationBucket: "2026-05-16"}},
		},
		memberBatches: [][]expiry.IndividualMemberTask{
			{{ID: "member-task-1", ExpirationBucket: "2026-05-16"}},
		},
	}
	publisher := &fakeCommandPublisher{}
	service := NewSchedulerService(repository, publisher,
		WithLogger(slog.Default()),
		WithBatchSize(2),
		WithClock(func() time.Time { return time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC) }),
		WithRunIDGenerator(func() string { return "run-1" }),
	)

	stats, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	if stats.GroupScanned != 3 || stats.GroupPublished != 3 || stats.IndividualMemberScanned != 1 || stats.IndividualMemberPublished != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	if repository.groupCalls != 2 || repository.memberCalls != 1 {
		t.Fatalf("calls = group:%d member:%d", repository.groupCalls, repository.memberCalls)
	}
}

func TestSchedulerServiceRunContinuesAfterPublishFailure(t *testing.T) {
	repository := &fakeTaskRepository{
		groupBatches:  [][]expiry.GroupTask{{{ID: "group-task-1"}, {ID: "group-task-2"}}},
		memberBatches: [][]expiry.IndividualMemberTask{{{ID: "member-task-1"}, {ID: "member-task-2"}}},
	}
	publisher := &fakeCommandPublisher{
		groupErrByID:  map[string]error{"group-task-1": errors.New("publish failed")},
		memberErrByID: map[string]error{"member-task-1": errors.New("publish failed")},
	}
	service := NewSchedulerService(repository, publisher, WithLogger(slog.Default()), WithRunIDGenerator(func() string { return "run-1" }))

	stats, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	if stats.GroupFailed != 1 || stats.GroupPublished != 1 || stats.IndividualMemberFailed != 1 || stats.IndividualMemberPublished != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestSchedulerServiceRunStopsOnQueryFailure(t *testing.T) {
	repository := &fakeTaskRepository{groupErr: errors.New("mongo unavailable")}
	publisher := &fakeCommandPublisher{}
	service := NewSchedulerService(repository, publisher, WithLogger(slog.Default()), WithRunIDGenerator(func() string { return "run-1" }))

	_, err := service.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want error")
	}
}
