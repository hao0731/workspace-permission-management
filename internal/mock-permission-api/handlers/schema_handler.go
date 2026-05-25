package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"

	permission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
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
	e.POST("/api/v1/relationships/write", handler.WriteRelationships)
}

func (h *SchemaHandler) WriteSchema(c *echo.Context) error {
	var request permission.RegisterResourceAttributesRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return c.JSON(http.StatusBadRequest, permission.ErrorResponse{
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

func (h *SchemaHandler) WriteRelationships(c *echo.Context) error {
	var request permission.WriteRelationshipsRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return c.JSON(http.StatusBadRequest, permission.ErrorResponse{
			Code:    http.StatusBadRequest,
			Error:   "validation_failed",
			Message: "Invalid relationships write payload",
		})
	}
	h.logger.InfoContext(c.Request().Context(), "mock permission relationships write received",
		"payload", request,
		"update_count", len(request.Updates),
	)
	response := permission.WriteRelationshipsResponse{
		Deletes: make([]permission.UpdatedRelationshipTask, 0),
		Writes:  make([]permission.UpdatedRelationshipTask, 0),
	}
	for _, update := range request.Updates {
		task := permission.UpdatedRelationshipTask{
			Relationship: update.Relationship,
			Success:      true,
		}
		switch update.Operation {
		case permission.RelationshipOperationDelete:
			response.Deletes = append(response.Deletes, task)
		default:
			response.Writes = append(response.Writes, task)
		}
	}
	return c.JSON(http.StatusOK, response)
}
