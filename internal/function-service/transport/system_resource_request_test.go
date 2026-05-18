package transport

import (
	"errors"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func validSystemResourceRequestJSON() string {
	return `{
		"resources": [
			{"type": "action", "label": "Can Edit", "key": "can_edit", "description": "Allows editing."},
			{"type": "tag", "label": "Private", "key": "private"},
			{"type": "type", "label": "Repository", "key": "repo"}
		]
	}`
}

func TestDecodeSystemResourceSaveRequest(t *testing.T) {
	req, err := DecodeSystemResourceSaveRequest(strings.NewReader(validSystemResourceRequestJSON()))
	if err != nil {
		t.Fatalf("DecodeSystemResourceSaveRequest error = %v, want nil", err)
	}
	if len(req.Resources) != 3 {
		t.Fatalf("resources len = %d, want 3", len(req.Resources))
	}
	if req.Resources[0].Type != "action" || req.Resources[0].Key != "can_edit" {
		t.Fatalf("first resource = %+v, want action/can_edit", req.Resources[0])
	}
}

func TestDecodeSystemResourceSaveRequestRejectsInvalidJSON(t *testing.T) {
	if _, err := DecodeSystemResourceSaveRequest(strings.NewReader(`{"resources":`)); err == nil {
		t.Fatal("DecodeSystemResourceSaveRequest error = nil, want error")
	}
}

func TestSystemResourceSaveRequestToDomain(t *testing.T) {
	req, err := DecodeSystemResourceSaveRequest(strings.NewReader(validSystemResourceRequestJSON()))
	if err != nil {
		t.Fatalf("decode request: %v", err)
	}

	input, err := req.ToDomain("todo")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.SystemID != "todo" {
		t.Fatalf("SystemID = %q, want todo", input.SystemID)
	}
	if input.Resources[0].Type != resource.ResourceDefinitionKindAction {
		t.Fatalf("type = %q, want action", input.Resources[0].Type)
	}
	if input.Resources[0].Description != "Allows editing." {
		t.Fatalf("description = %q, want Allows editing.", input.Resources[0].Description)
	}
}

func TestSystemResourceSaveRequestToDomainReturnsDomainValidationError(t *testing.T) {
	req := SystemResourceSaveRequest{
		Resources: []SystemResourceRequest{{Type: "ACTION", Label: "Can Edit", Key: "can_edit"}},
	}

	_, err := req.ToDomain("todo")
	if !errors.Is(err, resource.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}
