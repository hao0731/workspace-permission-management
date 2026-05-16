package services

import (
	"context"
	"log/slog"
	"sync"
	"testing"
)

func TestJobRunnerSkipsOverlap(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	runner := NewJobRunner(func(ctx context.Context) {
		calls++
		if calls == 1 {
			close(started)
			<-release
		}
	}, slog.Default())

	done := make(chan struct{})
	go func() {
		runner.Run(context.Background())
		close(done)
	}()
	<-started
	runner.Run(context.Background())
	close(release)
	<-done

	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestJobRunnerAllowsSequentialRuns(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	runner := NewJobRunner(func(ctx context.Context) {
		mu.Lock()
		defer mu.Unlock()
		calls++
	}, slog.Default())

	runner.Run(context.Background())
	runner.Run(context.Background())

	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}
