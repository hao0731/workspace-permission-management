package logger

import (
	"log/slog"
	"os"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

type Option func(*options)

type options struct {
	level slog.Leveler
}

func WithLevel(level slog.Leveler) Option {
	return func(opts *options) {
		opts.level = level
	}
}

func New(env environment.Environment, opts ...Option) *slog.Logger {
	config := options{}
	for _, opt := range opts {
		opt(&config)
	}

	handlerOptions := &slog.HandlerOptions{Level: config.level}
	if env == environment.Production {
		return slog.New(slog.NewJSONHandler(os.Stdout, handlerOptions))
	}

	return slog.New(slog.NewTextHandler(os.Stdout, handlerOptions))
}
