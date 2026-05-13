package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/mock-function/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type ResourceCreateService interface {
	HandleResourceCreate(ctx context.Context, command resource.ResourceCreateCommand) error
}

type ResourceCreateEventHandler struct {
	service         ResourceCreateService
	subjectAppNames map[string]string
	logger          *slog.Logger
}

func NewResourceCreateEventHandler(service ResourceCreateService, subjectAppNames map[string]string, logger *slog.Logger) *ResourceCreateEventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	copied := make(map[string]string, len(subjectAppNames))
	for subject, appName := range subjectAppNames {
		copied[subject] = appName
	}
	return &ResourceCreateEventHandler{service: service, subjectAppNames: copied, logger: logger}
}

func (h *ResourceCreateEventHandler) Handle(ctx context.Context, msg eventbus.Message) eventbus.HandleResult {
	command, err := transport.ParseResourceCreateCommandEvent(msg.Data, msg.Subject, h.subjectAppNames)
	if err != nil {
		h.logger.Warn("terminating invalid resource create command", "err", err, "subject", msg.Subject)
		return eventbus.HandleResultTerminate
	}
	if err := h.service.HandleResourceCreate(ctx, command); err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			h.logger.Warn("terminating invalid resource create command input",
				"err", err,
				"workspace_id", command.WorkspaceID,
				"app_name", command.AppName,
				"resource_name", command.ResourceName,
				"resource_type", command.ResourceType,
			)
			return eventbus.HandleResultTerminate
		}
		h.logger.Warn("retrying resource create command",
			"err", err,
			"workspace_id", command.WorkspaceID,
			"app_name", command.AppName,
			"resource_name", command.ResourceName,
			"resource_type", command.ResourceType,
		)
		return eventbus.HandleResultRetry
	}
	h.logger.Info("handled resource create command",
		"workspace_id", command.WorkspaceID,
		"app_name", command.AppName,
		"resource_name", command.ResourceName,
		"resource_type", command.ResourceType,
	)
	return eventbus.HandleResultAck
}
