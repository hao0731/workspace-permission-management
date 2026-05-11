package handlers

import (
	"context"
	"log/slog"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/group-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type IndividualMemberExpiryService interface {
	ExpireIndividualMember(ctx context.Context, input group.ExpireIndividualMemberCommand) (group.ExpireIndividualMemberStatus, error)
}

type IndividualMemberExpiryEventHandler struct {
	handler expiryCommandEventHandler[group.ExpireIndividualMemberCommand, group.ExpireIndividualMemberStatus]
}

func NewIndividualMemberExpiryEventHandler(service IndividualMemberExpiryService, expectedType string, logger *slog.Logger) *IndividualMemberExpiryEventHandler {
	return &IndividualMemberExpiryEventHandler{
		handler: newExpiryCommandEventHandler(
			expectedType,
			"individual member expiry command",
			logger,
			transport.ParseIndividualMemberExpiryCommandEvent,
			service.ExpireIndividualMember,
			individualMemberExpiryCommandFields,
		),
	}
}

func (h *IndividualMemberExpiryEventHandler) Handle(ctx context.Context, msg eventbus.Message) eventbus.HandleResult {
	return h.handler.handle(ctx, msg)
}

func individualMemberExpiryCommandFields(input group.ExpireIndividualMemberCommand) []any {
	return []any{
		"task_id", input.TaskID,
		"group_id", input.GroupID,
		"nt_account", input.NTAccount,
		"expiration_bucket", input.ExpirationBucket,
	}
}
