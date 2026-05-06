package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/labstack/echo/v5"
)

type HTTPResourceService interface {
	ListResources(ctx context.Context, query resource.ListQuery) (resource.Page, error)
}

type ResourceHandler struct {
	service HTTPResourceService
}

func NewResourceHandler(service HTTPResourceService) *ResourceHandler {
	return &ResourceHandler{service: service}
}

func RegisterRoutes(e *echo.Echo, handler *ResourceHandler) {
	e.GET("/api/v1/workspaces/:workspace_id/functions/:function_key/resources", handler.ListResources)
}

func (h *ResourceHandler) ListResources(c *echo.Context) error {
	limit, err := transport.ParseLimit(c.QueryParam("limit"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	cursor, err := transport.DecodeNextToken(c.QueryParam("next_token"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}

	page, err := h.service.ListResources(c.Request().Context(), resource.ListQuery{
		WorkspaceID: c.Param("workspace_id"),
		FunctionKey: c.Param("function_key"),
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

func validationError(message string) transport.ErrorResponse {
	return transport.ErrorResponse{
		Error: transport.ErrorBody{
			Code:    "validation_failed",
			Message: message,
			Details: map[string]any{},
		},
	}
}
