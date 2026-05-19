package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/services"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/http/exception"
	"github.com/labstack/echo/v5"
)

type HTTPSystemResourceService interface {
	SaveSystemResources(ctx context.Context, input resource.ResourceDefinitionSaveInput) ([]resource.ResourceDefinition, error)
	ListSystemResources(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error)
	GetSystemResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) ([]resource.ResourceAttribute, error)
}

type SystemResourceHandler struct {
	service HTTPSystemResourceService
	logger  *slog.Logger
}

func NewSystemResourceHandler(service HTTPSystemResourceService, logger *slog.Logger) *SystemResourceHandler {
	return &SystemResourceHandler{service: service, logger: logger}
}

func RegisterSystemResourceRoutes(e *echo.Echo, handler *SystemResourceHandler) {
	e.POST("/api/v1/systems/:system_id/resources", handler.SaveSystemResources)
	e.GET("/api/v1/systems/:system_id/resources", handler.ListSystemResources)
	e.GET("/api/v1/systems/:system_id/resource-attributes", handler.GetSystemResourceAttributes)
}

func (h *SystemResourceHandler) SaveSystemResources(c *echo.Context) error {
	request, err := transport.DecodeSystemResourceSaveRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(c.Param("system_id"))
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}
	definitions, err := h.service.SaveSystemResources(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, services.ErrPermissionRegistrationFailed) {
			return c.JSON(http.StatusBadGateway, exception.WrapResponse(exception.New("permission_registration_failed", "Failed to register resource attributes")))
		}
		h.logger.Warn("failed to save system resources", "err", err, "system_id", c.Param("system_id"))
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, transport.NewSystemResourcesResponse(definitions))
}

func (h *SystemResourceHandler) ListSystemResources(c *echo.Context) error {
	definitions, err := h.service.ListSystemResources(c.Request().Context(), resource.ResourceDefinitionsQuery{SystemID: c.Param("system_id")})
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to list system resources", "err", err, "system_id", c.Param("system_id"))
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, transport.NewSystemResourcesResponse(definitions))
}

func (h *SystemResourceHandler) GetSystemResourceAttributes(c *echo.Context) error {
	attributes, err := h.service.GetSystemResourceAttributes(c.Request().Context(), resource.ResourceAttributesQuery{SystemID: c.Param("system_id")})
	if err != nil {
		if errors.Is(err, resource.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to get system resource attributes", "err", err, "system_id", c.Param("system_id"))
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, transport.NewSystemResourceAttributesResponse(attributes))
}
