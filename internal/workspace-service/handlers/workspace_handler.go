package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
	"github.com/hao0731/workspace-permission-management/internal/shared/http/exception"
	"github.com/hao0731/workspace-permission-management/internal/workspace-service/services"
	"github.com/hao0731/workspace-permission-management/internal/workspace-service/transport"
	"github.com/labstack/echo/v5"
)

type HTTPWorkspaceService interface {
	CreateWorkspace(ctx context.Context, input workspace.CreateInput) (services.CreateWorkspaceResult, error)
	GetWorkspace(ctx context.Context, input workspace.GetQuery) (services.GetWorkspaceResult, error)
	SetWorkspaceFavorite(ctx context.Context, input workspace.FavoriteInput) error
}

type WorkspaceHandler struct {
	service HTTPWorkspaceService
	logger  *slog.Logger
}

func NewWorkspaceHandler(service HTTPWorkspaceService, logger *slog.Logger) *WorkspaceHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &WorkspaceHandler{service: service, logger: logger}
}

func RegisterRoutes(e *echo.Echo, handler *WorkspaceHandler) {
	e.POST("/api/v1/workspaces", handler.CreateWorkspace)
	e.GET("/api/v1/workspaces/:workspace_id", handler.GetWorkspace)
	e.POST("/api/v1/workspaces/:workspace_id/favorite", handler.SetWorkspaceFavorite)
}

func (h *WorkspaceHandler) CreateWorkspace(c *echo.Context) error {
	request, err := transport.DecodeWorkspaceCreateRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	input, err := request.ToDomain()
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	result, err := h.service.CreateWorkspace(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, workspace.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, services.ErrHRLookupFailed) {
			return c.JSON(http.StatusBadGateway, exception.WrapResponse(exception.New("hr_lookup_failed", "Failed to resolve workspace owner")))
		}
		h.logger.Warn("failed to create workspace", "err", err)
		return c.JSON(http.StatusInternalServerError, internalError())
	}
	return c.JSON(http.StatusCreated, transport.NewWorkspaceCreateResponse(result.Workspace, result.Owner))
}

func (h *WorkspaceHandler) GetWorkspace(c *echo.Context) error {
	workspaceID := c.Param("workspace_id")
	result, err := h.service.GetWorkspace(c.Request().Context(), workspace.GetQuery{ID: workspaceID})
	if err != nil {
		if errors.Is(err, workspace.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, services.ErrHRLookupFailed) {
			return c.JSON(http.StatusBadGateway, exception.WrapResponse(exception.New("hr_lookup_failed", "Failed to resolve workspace owner")))
		}
		h.logger.Warn("failed to get workspace", "err", err, "workspace_id", workspaceID)
		return c.JSON(http.StatusInternalServerError, internalError())
	}
	if !result.Found {
		return c.JSON(http.StatusOK, transport.NewWorkspaceGetNotFoundResponse())
	}
	return c.JSON(http.StatusOK, transport.NewWorkspaceGetResponse(result.Workspace, result.Owner))
}

func (h *WorkspaceHandler) SetWorkspaceFavorite(c *echo.Context) error {
	request, err := transport.DecodeWorkspaceFavoriteRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	input, err := request.ToDomain(c.Param("workspace_id"), c.Request().Header.Get("X-User-Id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	if err := h.service.SetWorkspaceFavorite(c.Request().Context(), input); err != nil {
		if errors.Is(err, workspace.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, workspace.ErrNotFound) {
			return c.JSON(http.StatusNotFound, workspaceNotFoundError())
		}
		h.logger.Warn("failed to set workspace favorite", "err", err, "workspace_id", input.WorkspaceID, "nt_account", input.NTAccount)
		return c.JSON(http.StatusInternalServerError, internalError())
	}
	return c.NoContent(http.StatusNoContent)
}

func validationError(message string) exception.ErrorResponse {
	return exception.WrapResponse(exception.New("validation_failed", message, exception.WithDetails(map[string]any{})))
}

func workspaceNotFoundError() exception.ErrorResponse {
	return exception.WrapResponse(exception.New("workspace_not_found", "Workspace not found", exception.WithDetails(map[string]any{})))
}

func internalError() exception.ErrorResponse {
	return exception.WrapResponse(exception.New("internal_error", "Internal server error"))
}
