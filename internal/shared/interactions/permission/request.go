package permission

import "github.com/hao0731/workspace-permission-management/internal/domain/resource"

const dynamicContextCondition = "enable_dynamic_context"

type RegisterResourceAttributesRelationRequest struct {
	ResourceAttribute resource.ResourceAttribute `json:"resAttr"`
	Condition         string                     `json:"condition"`
	IsPublic          bool                       `json:"isPublic"`
}

type RegisterResourceAttributesRequest struct {
	Definition string                                      `json:"definition"`
	Relations  []RegisterResourceAttributesRelationRequest `json:"relations"`
}

func newRegisterResourceAttributesRequest(systemID string, resourceAttributes []resource.ResourceAttribute) RegisterResourceAttributesRequest {
	relations := make([]RegisterResourceAttributesRelationRequest, 0, len(resourceAttributes))
	for _, attribute := range resourceAttributes {
		relations = append(relations, RegisterResourceAttributesRelationRequest{
			ResourceAttribute: attribute,
			Condition:         dynamicContextCondition,
			IsPublic:          false,
		})
	}
	return RegisterResourceAttributesRequest{
		Definition: systemID,
		Relations:  relations,
	}
}
