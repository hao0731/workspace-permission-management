package eventbus

import (
	"context"
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func TestValidateConsumerSubject(t *testing.T) {
	tests := []struct {
		name           string
		filterSubject  string
		filterSubjects []string
		configSubject  string
		wantErr        bool
	}{
		{name: "single filter matches", filterSubject: "events.created", configSubject: "events.created"},
		{name: "multi filter contains subject", filterSubjects: []string{"events.created", "events.updated"}, configSubject: "events.updated"},
		{name: "filter mismatch", filterSubject: "events.created", configSubject: "events.updated", wantErr: true},
		{name: "missing consumer filter", configSubject: "events.created", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConsumerSubject(tt.filterSubject, tt.filterSubjects, tt.configSubject)
			if tt.wantErr && err == nil {
				t.Fatal("error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
		})
	}
}

func TestValidateConsumerSubjects(t *testing.T) {
	tests := []struct {
		name           string
		filterSubject  string
		filterSubjects []string
		configSubjects []string
		wantErr        bool
	}{
		{name: "single filter with single subject", filterSubject: "orders.created", configSubjects: []string{"orders.created"}},
		{name: "multi filter exact set", filterSubjects: []string{"orders.created", "orders.updated"}, configSubjects: []string{"orders.updated", "orders.created"}},
		{name: "missing configured subject", filterSubjects: []string{"orders.created"}, configSubjects: []string{"orders.created", "orders.updated"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConsumerSubjects(tt.filterSubject, tt.filterSubjects, tt.configSubjects)
			if tt.wantErr && err == nil {
				t.Fatal("error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
		})
	}
}

type stubPublisher struct {
	subject string
	data    []byte
	err     error
}

func (s *stubPublisher) Publish(context.Context, string, []byte, ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	panic("not used")
}

func (s *stubPublisher) PublishMsg(_ context.Context, msg *nats.Msg, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	s.subject = msg.Subject
	s.data = append([]byte(nil), msg.Data...)
	if s.err != nil {
		return nil, s.err
	}
	return &jetstream.PubAck{}, nil
}

func (s *stubPublisher) PublishAsync(string, []byte, ...jetstream.PublishOpt) (jetstream.PubAckFuture, error) {
	panic("not used")
}

func (s *stubPublisher) PublishMsgAsync(*nats.Msg, ...jetstream.PublishOpt) (jetstream.PubAckFuture, error) {
	panic("not used")
}

func (s *stubPublisher) PublishAsyncPending() int {
	return 0
}

func (s *stubPublisher) PublishAsyncComplete() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (s *stubPublisher) CleanupPublisher() {}

func TestJetStreamPublishSourcePublish(t *testing.T) {
	t.Run("publish success", func(t *testing.T) {
		pub := &stubPublisher{}
		source := &jetStreamPublishSource{publisher: pub}

		err := source.Publish(context.Background(), "events.persisted", []byte(`{"id":"1"}`))
		if err != nil {
			t.Fatalf("Publish error = %v, want nil", err)
		}
		if pub.subject != "events.persisted" {
			t.Fatalf("subject = %q, want events.persisted", pub.subject)
		}
	})

	t.Run("publish error", func(t *testing.T) {
		pub := &stubPublisher{err: errors.New("publish failed")}
		source := &jetStreamPublishSource{publisher: pub}

		err := source.Publish(context.Background(), "events.persisted", []byte(`{"id":"1"}`))
		if err == nil {
			t.Fatal("Publish error = nil, want error")
		}
	})
}
