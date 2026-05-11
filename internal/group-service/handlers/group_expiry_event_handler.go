package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/group-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type GroupExpiryService interface {
	ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand) (group.ExpireGroupingRuleStatus, error)
}

type GroupExpiryEventHandler struct {
	service      GroupExpiryService
	expectedType string
	logger       *slog.Logger
}

func NewGroupExpiryEventHandler(service GroupExpiryService, expectedType string, logger *slog.Logger) *GroupExpiryEventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &GroupExpiryEventHandler{service: service, expectedType: expectedType, logger: logger}
}

func (h *GroupExpiryEventHandler) Handle(ctx context.Context, msg eventbus.Message) eventbus.HandleResult {
	input, err := transport.ParseGroupExpiryCommandEvent(msg.Data, h.expectedType)
	if err != nil {
		h.logger.Warn("terminating invalid group expiry command", "err", err, "subject", msg.Subject)
		return eventbus.HandleResultTerminate
	}

	status, err := h.service.ExpireGroupingRule(ctx, input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			h.logger.Warn("terminating invalid group expiry command input",
				"err", err,
				"task_id", input.TaskID,
				"workspace_id", input.WorkspaceID,
				"group_id", input.GroupID,
				"expiration_bucket", input.ExpirationBucket,
			)
			return eventbus.HandleResultTerminate
		}
		h.logger.Warn("retrying group expiry command",
			"err", err,
			"task_id", input.TaskID,
			"workspace_id", input.WorkspaceID,
			"group_id", input.GroupID,
			"expiration_bucket", input.ExpirationBucket,
		)
		return eventbus.HandleResultRetry
	}

	h.logger.Info("handled group expiry command",
		"task_id", input.TaskID,
		"workspace_id", input.WorkspaceID,
		"group_id", input.GroupID,
		"expiration_bucket", input.ExpirationBucket,
		"status", status,
	)
	return eventbus.HandleResultAck
}
