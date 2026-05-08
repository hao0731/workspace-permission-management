package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/permission"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	sharedexception "github.com/hao0731/workspace-permission-management/internal/shared/http/exception"
	"github.com/labstack/echo/v5"
)

type HTTPPermissionService interface {
	SavePermission(ctx context.Context, input permission.SaveInput) (permission.Permission, error)
}

type PermissionHandler struct {
	service HTTPPermissionService
	logger  *slog.Logger
}

type permissionPathParams struct {
	workspaceID string
	functionKey string
}

func NewPermissionHandler(service HTTPPermissionService, logger *slog.Logger) *PermissionHandler {
	return &PermissionHandler{service: service, logger: logger}
}

func RegisterPermissionRoutes(e *echo.Echo, handler *PermissionHandler) {
	e.PUT("/api/v1/workspaces/:workspace_id/functions/:function_key/permissions", handler.SavePermissions)
}

func newPermissionPathParams(c *echo.Context) permissionPathParams {
	return permissionPathParams{
		workspaceID: c.Param("workspace_id"),
		functionKey: c.Param("function_key"),
	}
}

func (h *PermissionHandler) SavePermissions(c *echo.Context) error {
	params := newPermissionPathParams(c)
	request, err := transport.DecodePermissionSaveRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}

	input, err := request.ToDomain(params.workspaceID, params.functionKey)
	if err != nil {
		if errors.Is(err, permission.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}

	model, err := h.service.SavePermission(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, permission.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to save permissions",
			"err", err,
			"workspace_id", params.workspaceID,
			"function_key", params.functionKey,
		)
		return c.JSON(http.StatusInternalServerError, sharedexception.WrapResponse(sharedexception.New("internal_error", "Internal server error")))
	}

	return c.JSON(http.StatusOK, transport.NewPermissionSaveResponse(model))
}
