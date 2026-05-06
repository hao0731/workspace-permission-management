package main

import (
	"context"
	"fmt"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"github.com/hao0731/workspace-permission-management/internal/function-service/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type messagePublisher interface {
	Publish(ctx context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error
}

type resourceDeletedPublisher struct {
	publisher messagePublisher
	subject   string
}

func newResourceDeletedPublisher(publisher messagePublisher, subject string) resourceDeletedPublisher {
	return resourceDeletedPublisher{
		publisher: publisher,
		subject:   subject,
	}
}

func (p resourceDeletedPublisher) PublishResourceDeleted(ctx context.Context, event resource.DeletedEvent) error {
	data, err := transport.NewResourceDeletedEvent(event, p.subject)
	if err != nil {
		return fmt.Errorf("build resource deleted event: %w", err)
	}
	if err := p.publisher.Publish(ctx, p.subject, data); err != nil {
		return fmt.Errorf("publish resource deleted event: %w", err)
	}
	return nil
}
