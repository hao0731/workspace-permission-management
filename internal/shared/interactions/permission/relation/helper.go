package relation

import (
	"fmt"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

func ResourceAttributeToRelation(resourceAttribute resource.ResourceAttribute) Relation {
	return Relation(fmt.Sprintf("rel_%s", resourceAttribute))
}
