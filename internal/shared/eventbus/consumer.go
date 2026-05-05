package eventbus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type HandleResult string

const (
	HandleResultAck       HandleResult = "ack"
	HandleResultRetry     HandleResult = "retry"
	HandleResultTerminate HandleResult = "terminate"

	initialFetchRetryBackoff = 100 * time.Millisecond
	maxFetchRetryBackoff     = 2 * time.Second
)

type Handler interface {
	Handle(ctx context.Context, msg Message) HandleResult
}

type Config struct {
	Stream    string
	Subjects  []string
	Durable   string
	BatchSize int
	MaxWait   time.Duration
}

type Consumer struct {
	source  messageSource
	handler Handler
	config  Config
	logger  *slog.Logger
}

type messageSource interface {
	Fetch(ctx context.Context, batchSize int, maxWait time.Duration) ([]receivedMessage, error)
}

type receivedMessage interface {
	Message() Message
	Ack() error
	Nak() error
	Term() error
}

func newConsumer(source messageSource, handler Handler, config Config, logger *slog.Logger) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Consumer{
		source:  source,
		handler: handler,
		config:  config,
		logger:  logger,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	fetchRetryBackoff := initialFetchRetryBackoff
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		messages, err := c.source.Fetch(ctx, c.config.BatchSize, c.config.MaxWait)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}

			if !shouldRetryFetchError(err) {
				return fmt.Errorf("fetch messages: %w", err)
			}

			c.logger.Warn("failed to fetch jetstream messages; retrying",
				"err", err,
				"backoff", fetchRetryBackoff,
			)

			if !sleepWithContext(ctx, fetchRetryBackoff) {
				return nil
			}

			fetchRetryBackoff *= 2
			if fetchRetryBackoff > maxFetchRetryBackoff {
				fetchRetryBackoff = maxFetchRetryBackoff
			}

			continue
		}

		fetchRetryBackoff = initialFetchRetryBackoff

		for _, message := range messages {
			if err := c.handleMessage(ctx, message); err != nil {
				return err
			}
		}
	}
}

func shouldRetryFetchError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func sleepWithContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (c *Consumer) handleMessage(ctx context.Context, message receivedMessage) error {
	result := c.handler.Handle(ctx, message.Message())
	switch result {
	case HandleResultAck:
		return message.Ack()
	case HandleResultRetry:
		return message.Nak()
	case HandleResultTerminate:
		return message.Term()
	default:
		if err := message.Term(); err != nil {
			return err
		}
		return fmt.Errorf("unknown handle result: %s", result)
	}
}
