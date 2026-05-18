package transport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func TestNewSystemResourcesResponse(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	response := NewSystemResourcesResponse([]resource.ResourceDefinition{{
		SystemID:    "todo",
		Type:        resource.ResourceDefinitionKindAction,
		Label:       "Can Edit",
		Key:         "can_edit",
		Description: "Allows editing.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}})

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, `"description":"Allows editing."`) {
		t.Fatalf("response missing description: %s", body)
	}
	if !strings.Contains(body, `"created_at":"2026-05-18T10:00:00Z"`) {
		t.Fatalf("response missing created_at: %s", body)
	}
}

func TestNewSystemResourcesResponseOmitsEmptyDescription(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	response := NewSystemResourcesResponse([]resource.ResourceDefinition{{
		SystemID:  "todo",
		Type:      resource.ResourceDefinitionKindTag,
		Label:     "Private",
		Key:       "private",
		CreatedAt: now,
		UpdatedAt: now,
	}})

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(data), "description") {
		t.Fatalf("response includes empty description: %s", data)
	}
}

func TestNewSystemResourceAttributesResponse(t *testing.T) {
	response := NewSystemResourceAttributesResponse([]resource.ResourceAttribute{
		resource.ResourceAttribute("can_edit_private_repo"),
	})

	if len(response.ResourceAttributes) != 1 || response.ResourceAttributes[0] != "can_edit_private_repo" {
		t.Fatalf("attributes = %#v, want can_edit_private_repo", response.ResourceAttributes)
	}
}
