package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/group-service/services"
	"github.com/hao0731/workspace-permission-management/internal/shared/pagination"
	"github.com/labstack/echo/v5"
)

type fakeHTTPGroupService struct {
	input             group.CreateInput
	getQuery          group.GetQuery
	deleteInput       group.DeleteInput
	updateInput       group.UpdateGroupingRuleInput
	listQuery         group.ListIndividualMembersQuery
	addMembersInput   group.AddIndividualMembersInput
	memberUpdate      group.UpdateIndividualMemberExpirationInput
	memberDelete      group.DeleteIndividualMemberInput
	systemGroupInput  group.SystemGroupCreateInput
	systemGroupQuery  group.SystemGroupListQuery
	model             group.Group
	groupPtr          *group.Group
	page              group.IndividualMemberPage
	systemGroupModel  group.SystemGroup
	systemGroupPage   group.SystemGroupPage
	systemGroupErrors []string
	addedMembers      []group.IndividualMember
	err               error
	calls             int
	getCalls          int
	deleteCalls       int
	updateCalls       int
	listCalls         int
	memberAddCalls    int
	memberUpdCalls    int
	memberDelCalls    int
	systemCreateCalls int
	systemListCalls   int
}

func (f *fakeHTTPGroupService) CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return group.Group{}, f.err
	}
	return f.model, nil
}

func (f *fakeHTTPGroupService) GetGroup(ctx context.Context, query group.GetQuery) (*group.Group, error) {
	f.getCalls++
	f.getQuery = query
	if f.err != nil {
		return nil, f.err
	}
	return f.groupPtr, nil
}

func (f *fakeHTTPGroupService) DeleteGroup(ctx context.Context, input group.DeleteInput) error {
	f.deleteCalls++
	f.deleteInput = input
	return f.err
}

func (f *fakeHTTPGroupService) UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput) error {
	f.updateCalls++
	f.updateInput = input
	return f.err
}

func (f *fakeHTTPGroupService) ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error) {
	f.listCalls++
	f.listQuery = query
	if f.err != nil {
		return group.IndividualMemberPage{}, f.err
	}
	return f.page, nil
}

func (f *fakeHTTPGroupService) AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error) {
	f.memberAddCalls++
	f.addMembersInput = input
	if f.err != nil {
		return nil, f.err
	}
	return f.addedMembers, nil
}

func (f *fakeHTTPGroupService) UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput) error {
	f.memberUpdCalls++
	f.memberUpdate = input
	return f.err
}

func (f *fakeHTTPGroupService) DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput) error {
	f.memberDelCalls++
	f.memberDelete = input
	return f.err
}

func (f *fakeHTTPGroupService) CreateSystemGroup(ctx context.Context, input group.SystemGroupCreateInput) (group.SystemGroup, []string, error) {
	f.systemCreateCalls++
	f.systemGroupInput = input
	if f.err != nil {
		return group.SystemGroup{}, nil, f.err
	}
	return f.systemGroupModel, f.systemGroupErrors, nil
}

func (f *fakeHTTPGroupService) ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error) {
	f.systemListCalls++
	f.systemGroupQuery = query
	if f.err != nil {
		return group.SystemGroupPage{}, f.err
	}
	return f.systemGroupPage, nil
}

func validGroupRequestBody() string {
	return `{
		"name": "Design Reviewers",
		"description": "Employees who can review design documents.",
		"grouping_rule": {
			"rules": [
				{
					"attribute_key": "department",
					"operator": "eq",
					"multi": false,
					"value": "ABCD-123"
				}
			],
			"expiration_date": "2026-06-01T00:00:00Z"
		},
		"individual_members": [
			{
				"nt_account": "user1",
				"expiration_date": "2026-06-01T00:00:00Z"
			}
		]
	}`
}

func groupModel() group.Group {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return group.Group{
		ID:             "group-1",
		WorkspaceID:    "workspace-1",
		Name:           "Design Reviewers",
		NormalizedName: "Design Reviewers",
		Description:    "Employees who can review design documents.",
		GroupingRule: group.GroupingRule{
			Rules: []group.Rule{{
				AttributeKey: "department",
				Operator:     group.OperatorEq,
				Multi:        false,
				Value:        "ABCD-123",
			}},
			ExpirationDate: expiration,
		},
		IndividualMembers: []group.IndividualMember{{
			ID:             "member-1",
			GroupID:        "group-1",
			NTAccount:      "user1",
			ExpirationDate: expiration,
		}},
	}
}

func TestGroupHandlerCreateGroup(t *testing.T) {
	service := &fakeHTTPGroupService{model: groupModel()}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if service.calls != 1 {
		t.Fatalf("service calls = %d, want 1", service.calls)
	}
	if service.input.WorkspaceID != "workspace-1" {
		t.Fatalf("WorkspaceID = %q, want workspace-1", service.input.WorkspaceID)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["group"]; !ok {
		t.Fatal("response missing group")
	}
}

func TestGroupHandlerRejectsMalformedJSON(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(`{"name":`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if service.calls != 0 {
		t.Fatalf("service calls = %d, want 0", service.calls)
	}
}

func TestGroupHandlerValidationError(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrInvalidInput}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGroupHandlerDuplicateName(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrDuplicateName}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestGroupHandlerServiceFailure(t *testing.T) {
	service := &fakeHTTPGroupService{err: errors.New("database unavailable")}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups", strings.NewReader(validGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestGroupHandlerGetGroup(t *testing.T) {
	model := groupModel()
	service := &fakeHTTPGroupService{groupPtr: &model}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/groups/group-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.getQuery.WorkspaceID != "workspace-1" || service.getQuery.GroupID != "group-1" {
		t.Fatalf("query = %+v, want path params", service.getQuery)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["group"] == nil {
		t.Fatal("group = nil, want object")
	}
}

func TestGroupHandlerGetGroupMissing(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/groups/missing", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"group":null`) {
		t.Fatalf("body = %s, want group null", rec.Body.String())
	}
}

func TestGroupHandlerDeleteGroup(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/groups/group-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if service.deleteInput.GroupID != "group-1" {
		t.Fatalf("delete input = %+v, want group-1", service.deleteInput)
	}
}

func TestGroupHandlerUpdateGroupingRule(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/groups/group-1/grouping-rules", strings.NewReader(`{
		"rules": [{"attribute_key": "department", "operator": "eq", "multi": false, "value": "ABCD-123"}],
		"expiration_date": "2026-06-01T00:00:00Z"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if service.updateInput.GroupID != "group-1" || len(service.updateInput.Rules) != 1 {
		t.Fatalf("update input = %+v, want group-1 and one rule", service.updateInput)
	}
}

func TestGroupHandlerUpdateGroupingRuleMissingGroup(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrNotFound}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/workspaces/workspace-1/groups/missing/grouping-rules", strings.NewReader(`{
		"rules": [],
		"expiration_date": "2026-06-01T00:00:00Z"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGroupHandlerListIndividualMembers(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	service := &fakeHTTPGroupService{page: group.IndividualMemberPage{
		Members: []group.IndividualMember{{ID: "member-1", GroupID: "group-1", NTAccount: "user1", ExpirationDate: expiration}},
	}}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members?limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.listQuery.Limit != 20 || service.listQuery.GroupID != "group-1" {
		t.Fatalf("list query = %+v, want limit 20 and group-1", service.listQuery)
	}
	if !strings.Contains(rec.Body.String(), `"members"`) {
		t.Fatalf("body = %s, want members", rec.Body.String())
	}
}

func TestGroupHandlerAddIndividualMembers(t *testing.T) {
	expiration := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	service := &fakeHTTPGroupService{addedMembers: []group.IndividualMember{{ID: "member-2", GroupID: "group-1", NTAccount: "user2", ExpirationDate: expiration}}}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members", strings.NewReader(`{
		"individual_members": [{"nt_account": "user2", "expiration_date": "2026-06-01T00:00:00Z"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if service.memberAddCalls != 1 || service.addMembersInput.GroupID != "group-1" {
		t.Fatalf("add input = %+v calls = %d, want group-1 and one call", service.addMembersInput, service.memberAddCalls)
	}
	if !strings.Contains(rec.Body.String(), `"members"`) {
		t.Fatalf("body = %s, want members", rec.Body.String())
	}
}

func TestGroupHandlerAddIndividualMembersDuplicateReturnsConflict(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrDuplicateMember}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members", strings.NewReader(`{
		"individual_members": [{"nt_account": "user2", "expiration_date": "2026-06-01T00:00:00Z"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestGroupHandlerAddIndividualMembersMissingGroupReturnsNotFound(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrNotFound}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/workspaces/workspace-1/groups/missing/individual-members", strings.NewReader(`{
		"individual_members": [{"nt_account": "user2", "expiration_date": "2026-06-01T00:00:00Z"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGroupHandlerUpdateIndividualMemberExpiration(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members/user2", strings.NewReader(`{
		"expiration_date": "2026-07-01T00:00:00Z"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if service.memberUpdate.NTAccount != "user2" || service.memberUpdate.GroupID != "group-1" {
		t.Fatalf("member update = %+v, want user2/group-1", service.memberUpdate)
	}
}

func TestGroupHandlerUpdateIndividualMemberExpirationMissingReturnsNotFound(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrNotFound}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members/missing", strings.NewReader(`{
		"expiration_date": "2026-07-01T00:00:00Z"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGroupHandlerDeleteIndividualMember(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/groups/group-1/individual-members/user2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if service.memberDelete.NTAccount != "user2" || service.memberDelete.GroupID != "group-1" {
		t.Fatalf("member delete = %+v, want user2/group-1", service.memberDelete)
	}
}

func TestGroupHandlerDeleteIndividualMemberMissingReturnsNoContent(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/workspaces/workspace-1/groups/missing/individual-members/user2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func systemGroupHandlerModel() group.SystemGroup {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	return group.SystemGroup{
		ID:       "group-1",
		SystemID: "system-a",
		Name:     "System Admins",
		GroupingRules: []group.SystemGroupRule{{
			AttributeKey: group.GroupAttributeOrganization,
			Operator:     group.OperatorEq,
			Multi:        true,
			Value:        []string{"ORG-100"},
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func validSystemGroupRequestBody() string {
	return `{
		"name": "System Admins",
		"grouping_rules": [
			{"attribute_key": "organization", "operator": "eq", "multi": true, "value": ["ORG-100"]}
		]
	}`
}

func TestGroupHandlerCreateSystemGroup(t *testing.T) {
	service := &fakeHTTPGroupService{systemGroupModel: systemGroupHandlerModel()}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/systems/system-a/groups", strings.NewReader(validSystemGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if service.systemCreateCalls != 1 || service.systemGroupInput.SystemID != "system-a" {
		t.Fatalf("service calls/input = %d/%+v, want system create", service.systemCreateCalls, service.systemGroupInput)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["group"]; !ok {
		t.Fatal("response missing group")
	}
}

func TestGroupHandlerCreateSystemGroupPartialPermissionFailure(t *testing.T) {
	service := &fakeHTTPGroupService{
		systemGroupModel:  systemGroupHandlerModel(),
		systemGroupErrors: []string{"organization rejected"},
	}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/systems/system-a/groups", strings.NewReader(validSystemGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	var body struct {
		Group  map[string]any `json:"group"`
		Errors []string       `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Group["id"] != "group-1" {
		t.Fatalf("group id = %v, want group-1", body.Group["id"])
	}
	if len(body.Errors) != 1 || body.Errors[0] != "organization rejected" {
		t.Fatalf("errors = %#v, want organization rejected", body.Errors)
	}
}

func TestGroupHandlerCreateSystemGroupPermissionWriteFailure(t *testing.T) {
	service := &fakeHTTPGroupService{err: services.ErrSystemGroupPermissionWriteFailed}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/systems/system-a/groups", strings.NewReader(validSystemGroupRequestBody()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	var body map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"]["code"] != "permission_write_failed" {
		t.Fatalf("error code = %v, want permission_write_failed", body["error"]["code"])
	}
}

func TestGroupHandlerCreateSystemGroupValidationError(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrInvalidInput}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/systems/system-a/groups", strings.NewReader(validSystemGroupRequestBody()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGroupHandlerListSystemGroups(t *testing.T) {
	service := &fakeHTTPGroupService{systemGroupPage: group.SystemGroupPage{Groups: []group.SystemGroup{systemGroupHandlerModel()}}}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/systems/system-a/groups?limit=10", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.systemListCalls != 1 || service.systemGroupQuery.SystemID != "system-a" || service.systemGroupQuery.Limit != 10 {
		t.Fatalf("query = %+v, want system-a limit 10", service.systemGroupQuery)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["groups"]; !ok {
		t.Fatal("response missing groups")
	}
}

func TestGroupHandlerListSystemGroupsInvalidLimit(t *testing.T) {
	service := &fakeHTTPGroupService{}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/systems/system-a/groups?limit=51", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if service.systemListCalls != 0 {
		t.Fatalf("service calls = %d, want 0", service.systemListCalls)
	}
}
