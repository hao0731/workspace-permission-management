package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hao0731/workspace-permission-management/internal/group-expiry-scheduler/transport"
	"github.com/hao0731/workspace-permission-management/internal/shared/eventbus"
	"github.com/hao0731/workspace-permission-management/internal/shared/repositories/expiry"
)

type messagePublisher interface {
	Publish(ctx context.Context, subject string, data []byte, opts ...eventbus.PublishOption) error
}

type expiryCommandPublisher struct {
	publisher               messagePublisher
	groupSubject            string
	individualMemberSubject string
	now                     func() time.Time
	idGenerator             func() string
	opts                    []eventbus.PublishOption
}

type publisherOption func(*expiryCommandPublisher)

func withPublisherClock(clock func() time.Time) publisherOption {
	return func(p *expiryCommandPublisher) {
		if clock != nil {
			p.now = clock
		}
	}
}

func withPublisherIDGenerator(generator func() string) publisherOption {
	return func(p *expiryCommandPublisher) {
		if generator != nil {
			p.idGenerator = generator
		}
	}
}

func withPublisherPublishOptions(opts ...eventbus.PublishOption) publisherOption {
	return func(p *expiryCommandPublisher) {
		p.opts = append([]eventbus.PublishOption(nil), opts...)
	}
}

func newExpiryCommandPublisher(publisher messagePublisher, groupSubject string, individualMemberSubject string, opts ...publisherOption) expiryCommandPublisher {
	p := expiryCommandPublisher{
		publisher:               publisher,
		groupSubject:            groupSubject,
		individualMemberSubject: individualMemberSubject,
		now: func() time.Time {
			return time.Now().UTC()
		},
		idGenerator: uuid.NewString,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&p)
		}
	}
	return p
}

func (p expiryCommandPublisher) PublishGroupExpiryCommand(ctx context.Context, task expiry.GroupTask) error {
	data, err := transport.NewGroupExpiryCommandEvent(transport.GroupExpiryCommand{
		TaskID:           task.ID,
		WorkspaceID:      task.WorkspaceID,
		GroupID:          task.GroupID,
		ExpirationBucket: task.ExpirationBucket,
	}, p.groupSubject, p.idGenerator(), p.now().UTC())
	if err != nil {
		return fmt.Errorf("build group expiry command event: %w", err)
	}
	if err := p.publisher.Publish(ctx, p.groupSubject, data, p.opts...); err != nil {
		return fmt.Errorf("publish group expiry command event: %w", err)
	}
	return nil
}

func (p expiryCommandPublisher) PublishIndividualMemberExpiryCommand(ctx context.Context, task expiry.IndividualMemberTask) error {
	data, err := transport.NewIndividualMemberExpiryCommandEvent(transport.IndividualMemberExpiryCommand{
		TaskID:           task.ID,
		GroupID:          task.GroupID,
		NTAccount:        task.NTAccount,
		ExpirationBucket: task.ExpirationBucket,
	}, p.individualMemberSubject, p.idGenerator(), p.now().UTC())
	if err != nil {
		return fmt.Errorf("build individual member expiry command event: %w", err)
	}
	if err := p.publisher.Publish(ctx, p.individualMemberSubject, data, p.opts...); err != nil {
		return fmt.Errorf("publish individual member expiry command event: %w", err)
	}
	return nil
}
