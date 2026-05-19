package transport

import (
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type SystemResourcesResponse struct {
	Resources []SystemResourceResponse `json:"resources"`
}

type SystemResourceResponse struct {
	Type        resource.ResourceDefinitionType `json:"type"`
	Label       string                          `json:"label"`
	Key         string                          `json:"key"`
	Description string                          `json:"description,omitempty"`
	CreatedAt   time.Time                       `json:"created_at"`
	UpdatedAt   time.Time                       `json:"updated_at"`
}

type SystemResourceAttributesResponse struct {
	ResourceAttributes []string `json:"resource_attributes"`
}

func NewSystemResourcesResponse(definitions []resource.ResourceDefinition) SystemResourcesResponse {
	resources := make([]SystemResourceResponse, 0, len(definitions))
	for _, definition := range definitions {
		resources = append(resources, SystemResourceResponse{
			Type:        definition.Type,
			Label:       definition.Label,
			Key:         definition.Key,
			Description: definition.Description,
			CreatedAt:   definition.CreatedAt,
			UpdatedAt:   definition.UpdatedAt,
		})
	}
	return SystemResourcesResponse{Resources: resources}
}

func NewSystemResourceAttributesResponse(attributes []resource.ResourceAttribute) SystemResourceAttributesResponse {
	values := make([]string, 0, len(attributes))
	for _, attribute := range attributes {
		values = append(values, string(attribute))
	}
	return SystemResourceAttributesResponse{ResourceAttributes: values}
}
