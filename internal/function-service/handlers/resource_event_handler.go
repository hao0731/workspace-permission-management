package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

var ErrRetryableEvent = errors.New("retryable event handling error")

type EventResourceService interface {
	UpsertResource(ctx context.Context, input resource.UpsertInput) (resource.UpsertStatus, error)
}

type ResourceEventHandler struct {
	service      EventResourceService
	expectedType string
	logger       *slog.Logger
}

func NewResourceEventHandler(service EventResourceService, expectedType string, logger *slog.Logger) *ResourceEventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ResourceEventHandler{service: service, expectedType: expectedType, logger: logger}
}

func (h *ResourceEventHandler) Handle(ctx context.Context, msg eventbus.Message) eventbus.HandleResult {
	input, err := transport.ParseResourceUpsertEvent(msg.Data, h.expectedType)
	if err != nil {
		h.logger.Warn("terminating invalid resource event", "err", err, "subject", msg.Subject)
		return eventbus.HandleResultTerminate
	}

	status, err := h.service.UpsertResource(ctx, input)
	if err != nil {
		if errors.Is(err, ErrRetryableEvent) {
			h.logger.Warn("retrying resource event", "err", err, "resource_id", input.ID)
			return eventbus.HandleResultRetry
		}
		if errors.Is(err, resource.ErrInvalidInput) {
			h.logger.Warn("terminating invalid resource event input", "err", err, "resource_id", input.ID)
			return eventbus.HandleResultTerminate
		}
		h.logger.Warn("retrying resource event after service error", "err", err, "resource_id", input.ID)
		return eventbus.HandleResultRetry
	}

	h.logger.Info("handled resource event", "resource_id", input.ID, "status", status)
	return eventbus.HandleResultAck
}
