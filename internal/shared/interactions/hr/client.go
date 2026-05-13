package hr

import (
	"context"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
)

type Client interface {
	Get(ctx context.Context, ntAccount string) (domainhr.User, error)
	BatchGet(ctx context.Context, ntAccounts []string) ([]domainhr.User, error)
}
