package transport

import "github.com/hao0731/workspace-permission-management/internal/domain/resource"

type ResourceListResponse struct {
	Resources []ResourceResponse `json:"resources"`
	PageInfo  PageInfoResponse   `json:"page_info"`
}

type ResourceResponse struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"display_name"`
	Type         string   `json:"type"`
	ResourceTags []string `json:"resource_tags"`
}

type PageInfoResponse struct {
	HasNextPage bool   `json:"has_next_page"`
	NextToken   string `json:"next_token"`
}

func NewResourceListResponse(page resource.Page) (ResourceListResponse, error) {
	resources := make([]ResourceResponse, 0, len(page.Resources))
	for _, item := range page.Resources {
		resources = append(resources, ResourceResponse{
			ID:           item.ID,
			DisplayName:  item.DisplayName,
			Type:         item.Type,
			ResourceTags: append([]string(nil), item.Tags...),
		})
	}
	nextToken, err := EncodeNextToken(page.NextCursor)
	if err != nil {
		return ResourceListResponse{}, err
	}
	return ResourceListResponse{
		Resources: resources,
		PageInfo: PageInfoResponse{
			HasNextPage: page.HasNextPage,
			NextToken:   nextToken,
		},
	}, nil
}
