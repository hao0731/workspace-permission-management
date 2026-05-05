package eventbus

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type fakeSource struct {
	messages   []receivedMessage
	err        error
	errs       []error
	calls      int
	afterFetch func()
}

func (f *fakeSource) Fetch(context.Context, int, time.Duration) ([]receivedMessage, error) {
	f.calls++
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		if f.afterFetch != nil {
			f.afterFetch()
		}
		return nil, err
	}
	if f.err != nil {
		if f.afterFetch != nil {
			f.afterFetch()
		}
		return nil, f.err
	}
	messages := f.messages
	f.messages = nil
	if f.afterFetch != nil {
		f.afterFetch()
	}
	return messages, nil
}

type fakeHandler struct {
	result HandleResult
}

func (f fakeHandler) Handle(context.Context, Message) HandleResult {
	return f.result
}

func TestConsumerMapsHandleResultToAckOperation(t *testing.T) {
	tests := []struct {
		name       string
		result     HandleResult
		wantAck    int
		wantNak    int
		wantTerm   int
		wantRunErr bool
	}{
		{name: "ack", result: HandleResultAck, wantAck: 1},
		{name: "retry", result: HandleResultRetry, wantNak: 1},
		{name: "terminate", result: HandleResultTerminate, wantTerm: 1},
		{name: "unknown result terminates", result: HandleResult("bad"), wantTerm: 1, wantRunErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			msg := &fakeReceivedMessage{subject: "events.created", data: []byte("{}")}
			source := &fakeSource{
				messages:   []receivedMessage{msg},
				afterFetch: cancel,
			}
			consumer := newConsumer(source, fakeHandler{result: tt.result}, Config{
				BatchSize: 1,
				MaxWait:   time.Millisecond,
			}, slog.Default())

			err := consumer.Run(ctx)

			if tt.wantRunErr && err == nil {
				t.Fatal("Run error = nil, want error")
			}
			if !tt.wantRunErr && err != nil {
				t.Fatalf("Run error = %v, want nil", err)
			}
			if msg.ackCalls != tt.wantAck || msg.nakCalls != tt.wantNak || msg.termCalls != tt.wantTerm {
				t.Fatalf("ack/nak/term = %d/%d/%d, want %d/%d/%d",
					msg.ackCalls, msg.nakCalls, msg.termCalls, tt.wantAck, tt.wantNak, tt.wantTerm)
			}
		})
	}
}

func TestConsumerStopsWhenContextCanceled(t *testing.T) {
	source := &fakeSource{}
	consumer := newConsumer(source, fakeHandler{result: HandleResultAck}, Config{
		BatchSize: 1,
		MaxWait:   time.Millisecond,
	}, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	if source.calls != 0 {
		t.Fatalf("Fetch calls = %d, want 0", source.calls)
	}
}

func TestConsumerRetriesFetchErrorAndContinues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := &fakeReceivedMessage{subject: "events.created", data: []byte("{}")}
	source := &fakeSource{
		errs:     []error{context.DeadlineExceeded},
		messages: []receivedMessage{msg},
	}
	source.afterFetch = func() {
		if len(source.errs) == 0 && len(source.messages) == 0 {
			cancel()
		}
	}

	consumer := newConsumer(source, fakeHandler{result: HandleResultAck}, Config{
		BatchSize: 1,
		MaxWait:   time.Millisecond,
	}, slog.Default())

	err := consumer.Run(ctx)
	if err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	if source.calls < 2 {
		t.Fatalf("Fetch calls = %d, want at least 2", source.calls)
	}
	if msg.ackCalls != 1 {
		t.Fatalf("Ack calls = %d, want 1", msg.ackCalls)
	}
}

func TestConsumerKeepsRetryingFetchErrorsUntilCanceled(t *testing.T) {
	wantErr := errors.New("fetch failed")
	ctx := context.Background()
	source := &fakeSource{err: wantErr}

	consumer := newConsumer(source, fakeHandler{result: HandleResultAck}, Config{
		BatchSize: 1,
		MaxWait:   time.Millisecond,
	}, slog.Default())

	err := consumer.Run(ctx)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v, want %v", err, wantErr)
	}
	if source.calls != 1 {
		t.Fatalf("Fetch calls = %d, want 1", source.calls)
	}
}

type fakeReceivedMessage struct {
	subject   string
	data      []byte
	headers   Headers
	ackCalls  int
	nakCalls  int
	termCalls int
}

func (f *fakeReceivedMessage) Message() Message {
	return Message{Subject: f.subject, Data: f.data, Headers: f.headers}
}

func (f *fakeReceivedMessage) Ack() error {
	f.ackCalls++
	return nil
}

func (f *fakeReceivedMessage) Nak() error {
	f.nakCalls++
	return nil
}

func (f *fakeReceivedMessage) Term() error {
	f.termCalls++
	return nil
}
