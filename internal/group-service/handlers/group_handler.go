package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/group-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/http/exception"
	"github.com/hao0731/workspace-permission-management/internal/shared/pagination"
	"github.com/labstack/echo/v5"
)

type HTTPGroupService interface {
	CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error)
	GetGroup(ctx context.Context, query group.GetQuery) (*group.Group, error)
	DeleteGroup(ctx context.Context, input group.DeleteInput) error
	UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput) error
	ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error)
	AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error)
	UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput) error
	DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput) error
	CreateSystemGroup(ctx context.Context, input group.SystemGroupCreateInput) (group.SystemGroup, error)
	ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error)
}

type GroupHandler struct {
	service          HTTPGroupService
	logger           *slog.Logger
	paginationHelper *pagination.PaginationHelper
}

type groupPathParams struct {
	workspaceID string
	groupID     string
}

func NewGroupHandler(service HTTPGroupService, logger *slog.Logger, paginationHelper *pagination.PaginationHelper) *GroupHandler {
	return &GroupHandler{service: service, logger: logger, paginationHelper: paginationHelper}
}

func RegisterRoutes(e *echo.Echo, handler *GroupHandler) {
	e.POST("/api/v1/workspaces/:workspace_id/groups", handler.CreateGroup)
	e.GET("/api/v1/workspaces/:workspace_id/groups/:group_id", handler.GetGroup)
	e.DELETE("/api/v1/workspaces/:workspace_id/groups/:group_id", handler.DeleteGroup)
	e.PUT("/api/v1/workspaces/:workspace_id/groups/:group_id/grouping-rules", handler.UpdateGroupingRule)
	e.GET("/api/v1/workspaces/:workspace_id/groups/:group_id/individual-members", handler.ListIndividualMembers)
	e.POST("/api/v1/workspaces/:workspace_id/groups/:group_id/individual-members", handler.AddIndividualMembers)
	e.PATCH("/api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account", handler.UpdateIndividualMemberExpiration)
	e.DELETE("/api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account", handler.DeleteIndividualMember)
	e.POST("/api/v1/systems/:system_id/groups", handler.CreateSystemGroup)
	e.GET("/api/v1/systems/:system_id/groups", handler.ListSystemGroups)
}

func newGroupPathParams(c *echo.Context) groupPathParams {
	return groupPathParams{workspaceID: c.Param("workspace_id"), groupID: c.Param("group_id")}
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

func (h *GroupHandler) CreateSystemGroup(c *echo.Context) error {
	systemID := c.Param("system_id")
	request, err := transport.DecodeSystemGroupCreateRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(systemID)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}
	model, err := h.service.CreateSystemGroup(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to create system group", "err", err, "system_id", systemID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusCreated, transport.NewSystemGroupCreateResponse(model))
}

func (h *GroupHandler) GetGroup(c *echo.Context) error {
	params := newGroupPathParams(c)
	model, err := h.service.GetGroup(c.Request().Context(), group.GetQuery{
		WorkspaceID: params.workspaceID,
		GroupID:     params.groupID,
	})
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to get group", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, transport.NewGroupGetResponse(model))
}

func (h *GroupHandler) DeleteGroup(c *echo.Context) error {
	params := newGroupPathParams(c)
	err := h.service.DeleteGroup(c.Request().Context(), group.DeleteInput{
		WorkspaceID: params.workspaceID,
		GroupID:     params.groupID,
	})
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to delete group", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *GroupHandler) UpdateGroupingRule(c *echo.Context) error {
	params := newGroupPathParams(c)
	request, err := transport.DecodeGroupGroupingRulesRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(params.workspaceID, params.groupID)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}
	if err := h.service.UpdateGroupingRule(c.Request().Context(), input); err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, group.ErrNotFound) {
			return c.JSON(http.StatusNotFound, exception.WrapResponse(exception.New("not_found", "Group not found", exception.WithDetails(map[string]any{}))))
		}
		h.logger.Warn("failed to update grouping rule", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *GroupHandler) ListIndividualMembers(c *echo.Context) error {
	params := newGroupPathParams(c)
	limit, err := h.paginationHelper.ParseLimit(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	token, err := h.paginationHelper.ParseToken(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	cursor, err := transport.DecodeIndividualMemberNextToken(token)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	page, err := h.service.ListIndividualMembers(c.Request().Context(), group.ListIndividualMembersQuery{
		WorkspaceID: params.workspaceID,
		GroupID:     params.groupID,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to list group individual members", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	response, err := transport.NewIndividualMemberListResponse(page)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, response)
}

func (h *GroupHandler) ListSystemGroups(c *echo.Context) error {
	systemID := c.Param("system_id")
	limit, err := h.paginationHelper.ParseLimit(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	token, err := h.paginationHelper.ParseToken(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	cursor, err := transport.DecodeSystemGroupNextToken(token)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError(err.Error()))
	}
	page, err := h.service.ListSystemGroups(c.Request().Context(), group.SystemGroupListQuery{
		SystemID: systemID,
		Limit:    limit,
		Cursor:   cursor,
	})
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to list system groups", "err", err, "system_id", systemID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	response, err := transport.NewSystemGroupListResponse(page)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusOK, response)
}

func (h *GroupHandler) AddIndividualMembers(c *echo.Context) error {
	params := newGroupPathParams(c)
	request, err := transport.DecodeIndividualMembersAddRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(params.workspaceID, params.groupID)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}
	members, err := h.service.AddIndividualMembers(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, group.ErrDuplicateMember) {
			return c.JSON(http.StatusConflict, exception.WrapResponse(exception.New("conflict", "Individual member already exists", exception.WithDetails(map[string]any{}))))
		}
		if errors.Is(err, group.ErrNotFound) {
			return c.JSON(http.StatusNotFound, exception.WrapResponse(exception.New("not_found", "Group not found", exception.WithDetails(map[string]any{}))))
		}
		h.logger.Warn("failed to add group individual members", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.JSON(http.StatusCreated, transport.NewIndividualMembersAddResponse(members))
}

func (h *GroupHandler) UpdateIndividualMemberExpiration(c *echo.Context) error {
	params := newGroupPathParams(c)
	request, err := transport.DecodeIndividualMemberExpirationUpdateRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(params.workspaceID, params.groupID, c.Param("nt_account"))
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}
	if err := h.service.UpdateIndividualMemberExpiration(c.Request().Context(), input); err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, group.ErrNotFound) {
			return c.JSON(http.StatusNotFound, exception.WrapResponse(exception.New("not_found", "Individual member not found", exception.WithDetails(map[string]any{}))))
		}
		h.logger.Warn("failed to update group individual member expiration", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID, "nt_account", c.Param("nt_account"))
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *GroupHandler) DeleteIndividualMember(c *echo.Context) error {
	params := newGroupPathParams(c)
	err := h.service.DeleteIndividualMember(c.Request().Context(), group.DeleteIndividualMemberInput{
		WorkspaceID: params.workspaceID,
		GroupID:     params.groupID,
		NTAccount:   c.Param("nt_account"),
	})
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		h.logger.Warn("failed to delete group individual member", "err", err, "workspace_id", params.workspaceID, "group_id", params.groupID, "nt_account", c.Param("nt_account"))
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	return c.NoContent(http.StatusNoContent)
}

func validationError(message string) exception.ErrorResponse {
	return exception.WrapResponse(exception.New("validation_failed", message, exception.WithDetails(map[string]any{})))
}
