package permission

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	permissionobject "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/object"
	permissionrelation "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/relation"
	permissionrelationship "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/relationship"
	permissionsubject "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/subject"
)

type resourceAttributeRegistrar interface {
	RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error
}

var _ resourceAttributeRegistrar = (*Client)(nil)

type relationshipWriter interface {
	WriteRelationships(ctx context.Context, parameter WriteRelationshipsParameter) (WriteRelationshipsResult, error)
}

var _ relationshipWriter = (*Client)(nil)

func TestClientRegisterResourceAttributesSendsSchemaWriteRequest(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotContentType string
	var gotAPIKey string
	var gotRequest RegisterResourceAttributesRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotAPIKey = r.Header.Get("X-API-Key")
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	attrs := []resource.ResourceAttribute{
		resource.ResourceAttribute("can_edit_private_repo"),
		resource.ResourceAttribute("can_view_public_repo"),
	}
	client := New(" "+server.URL+"/ ", "secret-key", "X-API-Key")

	if err := client.RegisterResourceAttributes(context.Background(), "todo", attrs); err != nil {
		t.Fatalf("RegisterResourceAttributes error = %v, want nil", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v1/schema/write" {
		t.Fatalf("path = %q, want /api/v1/schema/write", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotAPIKey != "secret-key" {
		t.Fatalf("X-API-Key = %q, want secret-key", gotAPIKey)
	}
	if gotRequest.Definition != "todo" {
		t.Fatalf("definition = %q, want todo", gotRequest.Definition)
	}
	if len(gotRequest.Relations) != 2 {
		t.Fatalf("relations len = %d, want 2", len(gotRequest.Relations))
	}
	if gotRequest.Relations[0].ResourceAttribute != resource.ResourceAttribute("can_edit_private_repo") {
		t.Fatalf("first resAttr = %q", gotRequest.Relations[0].ResourceAttribute)
	}
	if gotRequest.Relations[0].Condition != "enable_dynamic_context" {
		t.Fatalf("condition = %q, want enable_dynamic_context", gotRequest.Relations[0].Condition)
	}
	if gotRequest.Relations[0].IsPublic {
		t.Fatal("isPublic = true, want false")
	}
	if attrs[0] != resource.ResourceAttribute("can_edit_private_repo") {
		t.Fatalf("attrs mutated to %#v", attrs)
	}
}

func TestClientRegisterResourceAttributesReturnsAPIErrorForNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Code:    400,
			Error:   "validation_failed",
			Message: "Invalid schema write payload",
		})
	}))
	defer server.Close()

	client := New(server.URL, "secret-key", "X-API-Key")
	err := client.RegisterResourceAttributes(context.Background(), "todo", []resource.ResourceAttribute{
		resource.ResourceAttribute("can_edit_private_repo"),
	})
	if err == nil {
		t.Fatal("RegisterResourceAttributes error = nil, want error")
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", apiErr.StatusCode)
	}
	if apiErr.Response.Error != "validation_failed" || apiErr.Response.Message != "Invalid schema write payload" {
		t.Fatalf("response = %+v", apiErr.Response)
	}
}

func TestClientRegisterResourceAttributesReturnsDecodeErrorForMalformedErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client := New(server.URL, "secret-key", "X-API-Key")
	err := client.RegisterResourceAttributes(context.Background(), "todo", []resource.ResourceAttribute{
		resource.ResourceAttribute("can_edit_private_repo"),
	})
	if err == nil {
		t.Fatal("RegisterResourceAttributes error = nil, want error")
	}
	if !strings.Contains(err.Error(), "decode permission API error response") {
		t.Fatalf("error = %q, want decode context", err.Error())
	}
}

func TestClientRegisterResourceAttributesReturnsRequestFailure(t *testing.T) {
	client := New("http://permission.local", "secret-key", "X-API-Key", WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		}),
	}))

	err := client.RegisterResourceAttributes(context.Background(), "todo", []resource.ResourceAttribute{
		resource.ResourceAttribute("can_edit_private_repo"),
	})
	if err == nil {
		t.Fatal("RegisterResourceAttributes error = nil, want error")
	}
	if !strings.Contains(err.Error(), "send permission API request") {
		t.Fatalf("error = %q, want send context", err.Error())
	}
}

func TestClientWriteRelationshipsSendsRelationshipsWriteRequest(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotContentType string
	var gotAPIKey string
	var gotRequest WriteRelationshipsRequest

	createRelationship := testRelationship("group-1", "ORG-100")
	deleteRelationship := testRelationship("group-2", "ORG-200")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotAPIKey = r.Header.Get("X-API-Key")
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(WriteRelationshipsResponse{
			Writes: []UpdatedRelationshipTask{{
				Relationship: createRelationship,
				Success:      true,
			}},
			Deletes: []UpdatedRelationshipTask{{
				Relationship: deleteRelationship,
				Success:      true,
			}},
		})
	}))
	defer server.Close()

	client := New(" "+server.URL+"/ ", "secret-key", "X-API-Key")
	result, err := client.WriteRelationships(context.Background(), WriteRelationshipsParameter{
		Tasks: []RelationshipTask{
			{Operator: RelationshipOperationCreate, Relationship: createRelationship},
			{Operator: RelationshipOperationDelete, Relationship: deleteRelationship},
		},
	})
	if err != nil {
		t.Fatalf("WriteRelationships error = %v, want nil", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v1/relationships/write" {
		t.Fatalf("path = %q, want /api/v1/relationships/write", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotAPIKey != "secret-key" {
		t.Fatalf("X-API-Key = %q, want secret-key", gotAPIKey)
	}
	if len(gotRequest.Updates) != 2 {
		t.Fatalf("updates len = %d, want 2", len(gotRequest.Updates))
	}
	if gotRequest.Updates[0].Operation != RelationshipOperationCreate {
		t.Fatalf("first operation = %q, want create", gotRequest.Updates[0].Operation)
	}
	if gotRequest.Updates[1].Operation != RelationshipOperationDelete {
		t.Fatalf("second operation = %q, want delete", gotRequest.Updates[1].Operation)
	}
	if gotRequest.Updates[0].Relationship.Resource.ObjectID != "group-1" {
		t.Fatalf("first relationship resource id = %q, want group-1", gotRequest.Updates[0].Relationship.Resource.ObjectID)
	}
	if len(result.SuccessTasks) != 2 {
		t.Fatalf("success tasks len = %d, want 2", len(result.SuccessTasks))
	}
	if len(result.FailedTasks) != 0 {
		t.Fatalf("failed tasks len = %d, want 0", len(result.FailedTasks))
	}
	if result.SuccessTasks[0].Operator != RelationshipOperationCreate || result.SuccessTasks[1].Operator != RelationshipOperationDelete {
		t.Fatalf("success operators = %#v, want create/delete", result.SuccessTasks)
	}
}

func TestClientWriteRelationshipsMapsFailedTasks(t *testing.T) {
	createRelationship := testRelationship("group-1", "ORG-100")
	deleteRelationship := testRelationship("group-2", "ORG-200")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(WriteRelationshipsResponse{
			Writes: []UpdatedRelationshipTask{{
				Relationship: createRelationship,
				Success:      false,
				Error:        "relationship already exists",
			}},
			Deletes: []UpdatedRelationshipTask{{
				Relationship: deleteRelationship,
				Success:      true,
			}},
		})
	}))
	defer server.Close()

	client := New(server.URL, "secret-key", "X-API-Key")
	result, err := client.WriteRelationships(context.Background(), WriteRelationshipsParameter{
		Tasks: []RelationshipTask{
			{Operator: RelationshipOperationCreate, Relationship: createRelationship},
			{Operator: RelationshipOperationDelete, Relationship: deleteRelationship},
		},
	})
	if err != nil {
		t.Fatalf("WriteRelationships error = %v, want nil", err)
	}
	if len(result.SuccessTasks) != 1 || result.SuccessTasks[0].Operator != RelationshipOperationDelete {
		t.Fatalf("success tasks = %#v, want delete task", result.SuccessTasks)
	}
	if len(result.FailedTasks) != 1 {
		t.Fatalf("failed tasks len = %d, want 1", len(result.FailedTasks))
	}
	if result.FailedTasks[0].Operator != RelationshipOperationCreate {
		t.Fatalf("failed operator = %q, want create", result.FailedTasks[0].Operator)
	}
	if result.FailedTasks[0].Error != "relationship already exists" {
		t.Fatalf("failed error = %q, want relationship already exists", result.FailedTasks[0].Error)
	}
	if result.FailedTasks[0].Relationship.Resource.ObjectID != "group-1" {
		t.Fatalf("failed relationship resource id = %q, want group-1", result.FailedTasks[0].Relationship.Resource.ObjectID)
	}
}

func TestClientWriteRelationshipsReturnsAPIErrorForNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Code:    http.StatusBadGateway,
			Error:   "upstream_unavailable",
			Message: "Permission API unavailable",
		})
	}))
	defer server.Close()

	client := New(server.URL, "secret-key", "X-API-Key")
	_, err := client.WriteRelationships(context.Background(), WriteRelationshipsParameter{
		Tasks: []RelationshipTask{{Operator: RelationshipOperationCreate, Relationship: testRelationship("group-1", "ORG-100")}},
	})
	if err == nil {
		t.Fatal("WriteRelationships error = nil, want error")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if apiErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", apiErr.StatusCode)
	}
	if apiErr.Response.Error != "upstream_unavailable" {
		t.Fatalf("response = %+v, want upstream_unavailable", apiErr.Response)
	}
}

func TestClientWriteRelationshipsReturnsDecodeErrorForMalformedSuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client := New(server.URL, "secret-key", "X-API-Key")
	_, err := client.WriteRelationships(context.Background(), WriteRelationshipsParameter{
		Tasks: []RelationshipTask{{Operator: RelationshipOperationCreate, Relationship: testRelationship("group-1", "ORG-100")}},
	})
	if err == nil {
		t.Fatal("WriteRelationships error = nil, want error")
	}
	if !strings.Contains(err.Error(), "decode permission API relationships write response") {
		t.Fatalf("error = %q, want decode context", err.Error())
	}
}

func testRelationship(groupID string, organizationID string) permissionrelationship.Relationship {
	return *permissionrelationship.New(
		permissionrelation.HRMemberRelation,
		*permissionobject.NewGroup(groupID),
		*permissionsubject.New(
			*permissionobject.NewOrganization(organizationID),
			permissionsubject.WithRelation(permissionrelation.MemberRelation),
		),
	)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
