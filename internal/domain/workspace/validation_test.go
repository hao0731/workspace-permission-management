package workspace

import (
	"errors"
	"testing"
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
