package handlers

import (
	"context"
	"log/slog"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/group-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type GroupExpiryService interface {
	ExpireGroupingRule(ctx context.Context, input group.ExpireGroupingRuleCommand) (group.ExpireGroupingRuleStatus, error)
}

type GroupExpiryEventHandler struct {
	handler expiryCommandEventHandler[group.ExpireGroupingRuleCommand, group.ExpireGroupingRuleStatus]
}

func NewGroupExpiryEventHandler(service GroupExpiryService, expectedType string, logger *slog.Logger) *GroupExpiryEventHandler {
	return &GroupExpiryEventHandler{
		handler: newExpiryCommandEventHandler(
			expectedType,
			"group expiry command",
			logger,
			transport.ParseGroupExpiryCommandEvent,
			service.ExpireGroupingRule,
			groupExpiryCommandFields,
		),
	}
}

func (h *GroupExpiryEventHandler) Handle(ctx context.Context, msg eventbus.Message) eventbus.HandleResult {
	return h.handler.handle(ctx, msg)
}

func groupExpiryCommandFields(input group.ExpireGroupingRuleCommand) []any {
	return []any{
		"task_id", input.TaskID,
		"workspace_id", input.WorkspaceID,
		"group_id", input.GroupID,
		"expiration_bucket", input.ExpirationBucket,
	}
}
