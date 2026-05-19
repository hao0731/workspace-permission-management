package transport

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type SystemResourceSaveRequest struct {
	Resources []SystemResourceRequest `json:"resources"`
}

type SystemResourceRequest struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Key         string `json:"key"`
	Description string `json:"description,omitempty"`
}

func DecodeSystemResourceSaveRequest(body io.Reader) (SystemResourceSaveRequest, error) {
	var request SystemResourceSaveRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return SystemResourceSaveRequest{}, fmt.Errorf("decode system resource request: %w", err)
	}
	return request, nil
}

func (request SystemResourceSaveRequest) ToDomain(systemID string) (resource.ResourceDefinitionSaveInput, error) {
	resources := make([]resource.ResourceDefinitionInput, 0, len(request.Resources))
	for _, item := range request.Resources {
		resources = append(resources, resource.ResourceDefinitionInput{
			Type:        resource.ResourceDefinitionType(item.Type),
			Label:       item.Label,
			Key:         item.Key,
			Description: item.Description,
		})
	}
	input := resource.ResourceDefinitionSaveInput{
		SystemID:  systemID,
		Resources: resources,
	}
	if err := input.Validate(); err != nil {
		return resource.ResourceDefinitionSaveInput{}, err
	}
	return input.Normalize(), nil
}
