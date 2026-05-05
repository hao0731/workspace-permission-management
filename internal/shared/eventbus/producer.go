package eventbus

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const defaultPublishTimeout = 15 * time.Second

type publishSource interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

type publishOptions struct {
	timeout time.Duration
}

type PublishOption func(*publishOptions)

func WithPublishTimeout(timeout time.Duration) PublishOption {
	return func(opts *publishOptions) {
		opts.timeout = timeout
	}
}

type Producer struct {
	source publishSource
	logger *slog.Logger
}

func newProducer(source publishSource, logger *slog.Logger) *Producer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Producer{source: source, logger: logger}
}

func (p *Producer) Publish(ctx context.Context, subject string, data []byte, opts ...PublishOption) error {
	options := publishOptions{timeout: defaultPublishTimeout}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if options.timeout <= 0 {
		return fmt.Errorf("publish timeout must be positive")
	}

	publishCtx, cancel := context.WithTimeout(ctx, options.timeout)
	defer cancel()

	if err := p.source.Publish(publishCtx, subject, data); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}
	return nil
}
