package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"

	permissionapi "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api"
)

type SchemaHandler struct {
	logger *slog.Logger
}

func NewSchemaHandler(logger *slog.Logger) *SchemaHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SchemaHandler{logger: logger}
}

func RegisterRoutes(e *echo.Echo, handler *SchemaHandler) {
	e.POST("/api/v1/schema/write", handler.WriteSchema)
}

func (h *SchemaHandler) WriteSchema(c *echo.Context) error {
	var request permissionapi.RegisterResourceAttributesRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return c.JSON(http.StatusBadRequest, permissionapi.ErrorResponse{
			Code:    http.StatusBadRequest,
			Error:   "validation_failed",
			Message: "Invalid schema write payload",
		})
	}
	h.logger.InfoContext(c.Request().Context(), "mock permission schema write received",
		"payload", request,
		"relation_count", len(request.Relations),
	)
	return c.NoContent(http.StatusOK)
}
