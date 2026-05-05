package eventbus

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type stubPublishSource struct {
	subject string
	data    []byte
	err     error
}

func (s *stubPublishSource) Publish(_ context.Context, subject string, data []byte) error {
	s.subject = subject
	s.data = append([]byte(nil), data...)
	return s.err
}

func TestProducerPublish(t *testing.T) {
	t.Run("publish success", func(t *testing.T) {
		src := &stubPublishSource{}
		p := newProducer(src, slog.Default())

		err := p.Publish(context.Background(), "events.persisted", []byte(`{"id":"1"}`), WithPublishTimeout(time.Second))
		if err != nil {
			t.Fatalf("Publish error = %v, want nil", err)
		}
		if src.subject != "events.persisted" {
			t.Fatalf("subject = %q, want events.persisted", src.subject)
		}
	})

	t.Run("source publish error", func(t *testing.T) {
		src := &stubPublishSource{err: errors.New("publish failed")}
		p := newProducer(src, slog.Default())

		err := p.Publish(context.Background(), "events.persisted", []byte(`{}`), WithPublishTimeout(time.Second))
		if err == nil {
			t.Fatal("Publish error = nil, want error")
		}
	})

	t.Run("default timeout when option omitted", func(t *testing.T) {
		src := &stubPublishSource{}
		p := newProducer(src, slog.Default())
		err := p.Publish(context.Background(), "events.persisted", []byte(`{}`))
		if err != nil {
			t.Fatalf("Publish error = %v, want nil", err)
		}
	})

	t.Run("invalid timeout option", func(t *testing.T) {
		src := &stubPublishSource{}
		p := newProducer(src, slog.Default())
		err := p.Publish(context.Background(), "events.persisted", []byte(`{}`), WithPublishTimeout(0))
		if err == nil {
			t.Fatal("Publish error = nil, want error")
		}
	})
}
