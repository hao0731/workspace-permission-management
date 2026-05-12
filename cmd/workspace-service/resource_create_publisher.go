package main

import (
	"context"
	"fmt"

	"github.com/hao0731/workspace-permission-management/internal/domain/workspace"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/workspace-service/transport"
)

type messagePublisher interface {
	Publish(ctx context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error
}

type resourceCreatePublisher struct {
	publisher messagePublisher
	opts      []eventbus.PublishOption
}

func newResourceCreatePublisher(publisher messagePublisher, opts ...eventbus.PublishOption) resourceCreatePublisher {
	return resourceCreatePublisher{
		publisher: publisher,
		opts:      append([]eventbus.PublishOption(nil), opts...),
	}
}

func (p resourceCreatePublisher) PublishResourceCreateCommand(ctx context.Context, command workspace.ResourceCreateCommand) error {
	data, err := transport.NewResourceCreateEvent(command)
	if err != nil {
		return fmt.Errorf("build resource create event: %w", err)
	}
	if err := p.publisher.Publish(ctx, command.Subject(), data, p.opts...); err != nil {
		return fmt.Errorf("publish resource create event: %w", err)
	}
	return nil
}
