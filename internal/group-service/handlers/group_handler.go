package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/group-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/http/exception"
	"github.com/labstack/echo/v5"
)

type HTTPGroupService interface {
	CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error)
}

type GroupHandler struct {
	service HTTPGroupService
	logger  *slog.Logger
}

type groupPathParams struct {
	workspaceID string
}

func NewGroupHandler(service HTTPGroupService, logger *slog.Logger) *GroupHandler {
	return &GroupHandler{service: service, logger: logger}
}

func RegisterRoutes(e *echo.Echo, handler *GroupHandler) {
	e.POST("/api/v1/workspaces/:workspace_id/groups", handler.CreateGroup)
}

func newGroupPathParams(c *echo.Context) groupPathParams {
	return groupPathParams{workspaceID: c.Param("workspace_id")}
}

func (h *GroupHandler) CreateGroup(c *echo.Context) error {
	params := newGroupPathParams(c)
	request, err := transport.DecodeGroupCreateRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(params.workspaceID)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}

	model, err := h.service.CreateGroup(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, group.ErrDuplicateName) {
			return c.JSON(http.StatusConflict, exception.WrapResponse(exception.New("conflict", "Group name already exists", exception.WithDetails(map[string]any{}))))
		}
		h.logger.Warn("failed to create group", "err", err, "workspace_id", params.workspaceID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusCreated, transport.NewGroupCreateResponse(model))
}

func validationError(message string) exception.ErrorResponse {
	return exception.WrapResponse(exception.New("validation_failed", message, exception.WithDetails(map[string]any{})))
}
