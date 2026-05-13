package transport

import (
	"strings"
	"testing"
)

func TestDecodeWorkspaceCreateRequestWithAllResources(t *testing.T) {
	body := strings.NewReader(`{
		"name":" Workspace ",
		"description":" Description ",
		"owner":" user1 ",
		"documents":{"resource_name":" Docs "},
		"tasks":{"resource_name":" Tasks "},
		"drive":{"resource_name":" Drive "}
	}`)
	request, err := DecodeWorkspaceCreateRequest(body)
	if err != nil {
		t.Fatalf("DecodeWorkspaceCreateRequest() error = %v", err)
	}
	input, err := request.ToDomain()
	if err != nil {
		t.Fatalf("ToDomain() error = %v", err)
	}
	if input.Name != "Workspace" || input.OwnerNTAccount != "user1" {
		t.Fatalf("input = %+v", input)
	}
	if input.Documents == nil || input.Tasks == nil || input.Drive == nil {
		t.Fatalf("resources = documents:%v tasks:%v drive:%v", input.Documents, input.Tasks, input.Drive)
	}
	if input.Documents.ResourceName != "Docs" || input.Tasks.ResourceName != "Tasks" || input.Drive.ResourceName != "Drive" {
		t.Fatalf("resource names = %+v %+v %+v", input.Documents, input.Tasks, input.Drive)
	}
}

func TestDecodeWorkspaceCreateRequestRejectsMalformedJSON(t *testing.T) {
	_, err := DecodeWorkspaceCreateRequest(strings.NewReader(`{"name":`))
	if err == nil {
		t.Fatal("DecodeWorkspaceCreateRequest() error = nil, want error")
	}
}

func TestWorkspaceCreateRequestToDomainRejectsInvalidInput(t *testing.T) {
	request := WorkspaceCreateRequest{Name: "Project", Description: "Description"}
	if _, err := request.ToDomain(); err == nil {
		t.Fatal("ToDomain() error = nil, want error")
	}
}
