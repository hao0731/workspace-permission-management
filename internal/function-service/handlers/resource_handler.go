package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/labstack/echo/v5"
)

type HTTPResourceService interface {
	ListResources(ctx context.Context, query resource.ListQuery) (resource.Page, error)
	DeleteResource(ctx context.Context, input resource.DeleteInput) (resource.DeleteStatus, error)
}

type ResourceHandler struct {
	service HTTPResourceService
	logger  *slog.Logger
}

type resourcePathParams struct {
	workspaceID string
	functionKey string
	resourceID  string
}

func NewResourceHandler(service HTTPResourceService, logger *slog.Logger) *ResourceHandler {
	return &ResourceHandler{service: service, logger: logger}
}

func newResourcePathParams(c *echo.Context) resourcePathParams {
	return resourcePathParams{
		workspaceID: c.Param("workspace_id"),
		functionKey: c.Param("function_key"),
		resourceID:  c.Param("resource_id"),
	}
}

func RegisterRoutes(e *echo.Echo, handler *ResourceHandler) {
	e.GET("/api/v1/workspaces/:workspace_id/functions/:function_key/resources", handler.ListResources)
	e.DELETE("/api/v1/workspaces/:workspace_id/functions/:function_key/resources/:resource_id", handler.DeleteResource)
}

func (h *ResourceHandler) ListResources(c *echo.Context) error {
	params := newResourcePathParams(c)
	limit, err := transport.ParseLimit(c.QueryParam("limit"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	cursor, err := transport.DecodeNextToken(c.QueryParam("next_token"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}

	page, err := h.service.ListResources(c.Request().Context(), resource.ListQuery{
		WorkspaceID: params.workspaceID,
		FunctionKey: params.functionKey,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusInternalServerError, transport.ErrorResponse{
			Error: transport.ErrorBody{
				Code:    "internal_error",
				Message: "Internal server error",
			},
		})
	}

	response, err := transport.NewResourceListResponse(page)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, transport.ErrorResponse{
			Error: transport.ErrorBody{
				Code:    "internal_error",
				Message: "Internal server error",
			},
		})
	}
	return c.JSON(http.StatusOK, response)
}

func (h *ResourceHandler) DeleteResource(c *echo.Context) error {
	params := newResourcePathParams(c)
	status, err := h.service.DeleteResource(c.Request().Context(), resource.DeleteInput{
		WorkspaceID: params.workspaceID,
		FunctionKey: params.functionKey,
		ResourceID:  params.resourceID,
	})
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to delete resource",
			"err", err,
			"workspace_id", params.workspaceID,
			"function_key", params.functionKey,
			"resource_id", params.resourceID,
		)
		return c.JSON(http.StatusInternalServerError, transport.ErrorResponse{
			Error: transport.ErrorBody{
				Code:    "internal_error",
				Message: "Internal server error",
			},
		})
	}
	if status == resource.DeleteStatusNotFound {
		h.logger.Info("resource delete target already absent",
			"workspace_id", params.workspaceID,
			"function_key", params.functionKey,
			"resource_id", params.resourceID,
		)
		return c.NoContent(http.StatusNoContent)
	}
	return c.NoContent(http.StatusNoContent)
}

func validationError(message string) transport.ErrorResponse {
	return transport.ErrorResponse{
		Error: transport.ErrorBody{
			Code:    "validation_failed",
			Message: message,
			Details: map[string]any{},
		},
	}
}
