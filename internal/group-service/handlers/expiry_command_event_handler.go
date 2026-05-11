package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type expiryCommandEventHandler[C any, S any] struct {
	expectedType string
	commandName  string
	logger       *slog.Logger
	parse        func([]byte, string) (C, error)
	expire       func(context.Context, C) (S, error)
	fields       func(C) []any
}

func newExpiryCommandEventHandler[C any, S any](
	expectedType string,
	commandName string,
	logger *slog.Logger,
	parse func([]byte, string) (C, error),
	expire func(context.Context, C) (S, error),
	fields func(C) []any,
) expiryCommandEventHandler[C, S] {
	if logger == nil {
		logger = slog.Default()
	}
	return expiryCommandEventHandler[C, S]{
		expectedType: expectedType,
		commandName:  commandName,
		logger:       logger,
		parse:        parse,
		expire:       expire,
		fields:       fields,
	}
}

func (h expiryCommandEventHandler[C, S]) handle(ctx context.Context, msg eventbus.Message) eventbus.HandleResult {
	input, err := h.parse(msg.Data, h.expectedType)
	if err != nil {
		h.logger.Warn("terminating invalid "+h.commandName, "err", err, "subject", msg.Subject)
		return eventbus.HandleResultTerminate
	}

	status, err := h.expire(ctx, input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			h.logger.Warn("terminating invalid "+h.commandName+" input", commandLogFields(err, h.fields(input))...)
			return eventbus.HandleResultTerminate
		}
		h.logger.Warn("retrying "+h.commandName, commandLogFields(err, h.fields(input))...)
		return eventbus.HandleResultRetry
	}

	h.logger.Info("handled "+h.commandName, append(h.fields(input), "status", status)...)
	return eventbus.HandleResultAck
}

func commandLogFields(err error, fields []any) []any {
	values := make([]any, 0, len(fields)+2)
	values = append(values, "err", err)
	return append(values, fields...)
}
