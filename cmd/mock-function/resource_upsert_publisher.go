package main

import (
	"context"
	"fmt"

	"github.com/hao0731/workspace-permission-management/internal/domain/mockfunction"
	"github.com/hao0731/workspace-permission-management/internal/mock-function/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
)

type messagePublisher interface {
	Publish(ctx context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error
}

type resourceUpsertPublisher struct {
	publisher messagePublisher
	opts      []eventbus.PublishOption
}

func newResourceUpsertPublisher(publisher messagePublisher, opts ...eventbus.PublishOption) resourceUpsertPublisher {
	return resourceUpsertPublisher{
		publisher: publisher,
		opts:      append([]eventbus.PublishOption(nil), opts...),
	}
}

func (p resourceUpsertPublisher) PublishResourceUpsert(ctx context.Context, event mockfunction.ResourceUpsertEvent) error {
	data, subject, err := transport.NewResourceUpsertEvent(event)
	if err != nil {
		return fmt.Errorf("build resource upsert event: %w", err)
	}
	if err := p.publisher.Publish(ctx, subject, data, p.opts...); err != nil {
		return fmt.Errorf("publish resource upsert event: %w", err)
	}
	return nil
}
