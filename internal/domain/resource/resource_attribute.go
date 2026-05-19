package resource

import "fmt"

type ResourceAttribute string

func NewResourceAttribute(action, tag, resourceType string) ResourceAttribute {
	return ResourceAttribute(fmt.Sprintf("%s_%s_%s", action, tag, resourceType))
}
