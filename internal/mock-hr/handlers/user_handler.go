package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
	"github.com/hao0731/workspace-permission-management/internal/mock-hr/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/http/exception"
	"github.com/labstack/echo/v5"
)

type UserService interface {
	Get(ctx context.Context, ntAccount string) (domainhr.User, error)
	BatchGet(ctx context.Context, ntAccounts []string) ([]domainhr.User, error)
}

type UserHandler struct {
	service UserService
	logger  *slog.Logger
}

func NewUserHandler(service UserService, logger *slog.Logger) *UserHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &UserHandler{service: service, logger: logger}
}

func RegisterRoutes(e *echo.Echo, handler *UserHandler) {
	e.GET("/api/v1/users/:nt_account", handler.GetUser)
	e.POST("/api/v1/user-list", handler.BatchGetUsers)
}

func (h *UserHandler) GetUser(c *echo.Context) error {
	ntAccount, err := url.PathUnescape(c.Param("nt_account"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("nt_account is invalid"))
	}
	user, err := h.service.Get(c.Request().Context(), ntAccount)
	if err != nil {
		if errors.Is(err, domainhr.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to get mock hr user", "err", err, "nt_account", ntAccount)
		return c.JSON(http.StatusInternalServerError, internalError())
	}
	return c.JSON(http.StatusOK, transport.NewUserResponse(user))
}

func (h *UserHandler) BatchGetUsers(c *echo.Context) error {
	request, err := transport.DecodeUserListRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	accounts, err := request.ToDomain()
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	users, err := h.service.BatchGet(c.Request().Context(), accounts)
	if err != nil {
		if errors.Is(err, domainhr.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to batch get mock hr users", "err", err)
		return c.JSON(http.StatusInternalServerError, internalError())
	}
	return c.JSON(http.StatusOK, transport.NewUserListResponse(users))
}

func validationError(message string) exception.ErrorResponse {
	return exception.WrapResponse(exception.New("validation_failed", message, exception.WithDetails(map[string]any{})))
}

func internalError() exception.ErrorResponse {
	return exception.WrapResponse(exception.New("internal_error", "Internal server error"))
}
