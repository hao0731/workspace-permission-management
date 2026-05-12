package workspace

import (
	"errors"
	"testing"
	"time"
)

func TestCreateInputValidateRejectsRequiredFields(t *testing.T) {
	tests := []CreateInput{
		{Description: "desc", OwnerNTAccount: "user1"},
		{Name: "name", OwnerNTAccount: "user1"},
		{Name: "name", Description: "desc"},
		{Name: "name", Description: "desc", OwnerNTAccount: "user1", Documents: &ResourceRequest{}},
	}
	for _, input := range tests {
		if err := input.Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate() error = %v, want ErrInvalidInput", err)
		}
	}
}

func TestCreateInputNormalize(t *testing.T) {
	input := CreateInput{
		Name:           " Project ",
		Description:    " Description ",
		OwnerNTAccount: " user1 ",
		Documents:      &ResourceRequest{ResourceName: " Docs "},
	}
	normalized := input.Normalize()
	if normalized.Name != "Project" || normalized.Description != "Description" || normalized.OwnerNTAccount != "user1" || normalized.Documents.ResourceName != "Docs" {
		t.Fatalf("Normalize() = %+v", normalized)
	}
}

func TestCreateInputValidateAcceptsOmittedResources(t *testing.T) {
	input := CreateInput{Name: "Project", Description: "Description", OwnerNTAccount: "user1"}
	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestResourceCreateCommandValidate(t *testing.T) {
	validTime := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	tests := []ResourceCreateCommand{
		{AppName: "documents", ResourceName: "Docs", ResourceType: "document", EventID: "event-1", EventTime: validTime},
		{WorkspaceID: "workspace-1", ResourceName: "Docs", ResourceType: "document", EventID: "event-1", EventTime: validTime},
		{WorkspaceID: "workspace-1", AppName: "documents", ResourceType: "document", EventID: "event-1", EventTime: validTime},
		{WorkspaceID: "workspace-1", AppName: "documents", ResourceName: "Docs", EventID: "event-1", EventTime: validTime},
		{WorkspaceID: "workspace-1", AppName: "documents", ResourceName: "Docs", ResourceType: "document", EventTime: validTime},
		{WorkspaceID: "workspace-1", AppName: "documents", ResourceName: "Docs", ResourceType: "document", EventID: "event-1"},
	}
	for _, command := range tests {
		if err := command.Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate() error = %v, want ErrInvalidInput for %+v", err, command)
		}
	}
}

func TestResourceCreateCommandSubject(t *testing.T) {
	command := ResourceCreateCommand{AppName: "documents"}
	if command.Subject() != "cmd.app.documents.resource.create" {
		t.Fatalf("Subject() = %q", command.Subject())
	}
}
