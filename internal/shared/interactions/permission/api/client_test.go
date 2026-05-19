package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)

var _ clientpermission.Client = (*Client)(nil)

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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
