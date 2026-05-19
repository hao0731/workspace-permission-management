package permission

import (
	"context"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

type Client interface {
	RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error
}
