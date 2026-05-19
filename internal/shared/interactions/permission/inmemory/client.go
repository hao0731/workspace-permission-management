package inmemory

import (
	"context"
	"log/slog"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)

type Client struct {
	logger *slog.Logger
}

type Option func(*Client)

func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) {
		if logger != nil {
			c.logger = logger
		}
	}
}

func New(opts ...Option) clientpermission.Client {
	client := &Client{logger: slog.Default()}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client
}

func (c *Client) RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error {
	c.logger.DebugContext(ctx, "register resource attributes with in-memory permission client",
		"system_id", systemID,
		"resource_attribute_count", len(resourceAttributes),
		"resource_attributes", resourceAttributes,
	)
	return nil
}
