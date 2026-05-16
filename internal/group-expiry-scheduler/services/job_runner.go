package services

import (
	"context"
	"log/slog"
	"sync/atomic"
)

type JobRunner struct {
	run    func(context.Context)
	logger *slog.Logger
	active atomic.Bool
}

func NewJobRunner(run func(context.Context), logger *slog.Logger) *JobRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &JobRunner{run: run, logger: logger}
}

func (r *JobRunner) Run(ctx context.Context) {
	if !r.active.CompareAndSwap(false, true) {
		r.logger.Warn("group expiry scheduler job skipped because previous run is still active")
		return
	}
	defer r.active.Store(false)
	r.run(ctx)
}
