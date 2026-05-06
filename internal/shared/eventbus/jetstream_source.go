package eventbus

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type jetStreamSource struct {
	consumer jetstream.Consumer
}

type jetStreamPublishSource struct {
	publisher jetstream.Publisher
}

func NewJetStreamProducer(_ context.Context, nc *nats.Conn, logger *slog.Logger) (*Producer, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("create jetstream context: %w", err)
	}
	return newProducer(&jetStreamPublishSource{publisher: js}, logger), nil
}

func NewJetStreamConsumer(ctx context.Context, nc *nats.Conn, config Config, handler Handler, logger *slog.Logger) (*Consumer, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("create jetstream context: %w", err)
	}

	stream, err := js.Stream(ctx, config.Stream)
	if err != nil {
		return nil, fmt.Errorf("bind jetstream stream %q: %w", config.Stream, err)
	}

	consumer, err := stream.Consumer(ctx, config.Durable)
	if err != nil {
		return nil, fmt.Errorf("bind jetstream durable consumer %q: %w", config.Durable, err)
	}

	info, err := consumer.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch jetstream consumer info %q: %w", config.Durable, err)
	}
	if err := validateConsumerSubjects(info.Config.FilterSubject, info.Config.FilterSubjects, config.Subjects); err != nil {
		return nil, err
	}

	return newConsumer(&jetStreamSource{consumer: consumer}, handler, config, logger), nil
}

func (s *jetStreamPublishSource) Publish(ctx context.Context, subject string, data []byte) error {
	_, err := s.publisher.PublishMsg(ctx, &nats.Msg{
		Subject: subject,
		Data:    data,
	})
	if err != nil {
		return fmt.Errorf("jetstream publish: %w", err)
	}
	return nil
}

func (s *jetStreamSource) Fetch(ctx context.Context, batchSize int, maxWait time.Duration) ([]receivedMessage, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	batch, err := s.consumer.Fetch(batchSize, jetstream.FetchContext(fetchCtx))
	if err != nil {
		return nil, err
	}

	var messages []receivedMessage
	for msg := range batch.Messages() {
		select {
		case <-ctx.Done():
			return messages, nil
		default:
		}
		messages = append(messages, jetStreamMessage{msg: msg})
	}
	if err := batch.Error(); err != nil {
		return messages, err
	}
	return messages, nil
}

type jetStreamMessage struct {
	msg jetstream.Msg
}

func (m jetStreamMessage) Message() Message {
	return Message{
		Subject: m.msg.Subject(),
		Data:    m.msg.Data(),
		Headers: cloneHeaders(m.msg.Headers()),
	}
}

func (m jetStreamMessage) Ack() error {
	return m.msg.Ack()
}

func (m jetStreamMessage) Nak() error {
	return m.msg.Nak()
}

func (m jetStreamMessage) Term() error {
	return m.msg.Term()
}

func validateConsumerSubject(filterSubject string, filterSubjects []string, configSubject string) error {
	if filterSubject == configSubject {
		return nil
	}
	if slices.Contains(filterSubjects, configSubject) {
		return nil
	}
	return fmt.Errorf("jetstream consumer subject filter does not match configured subject %q", configSubject)
}

func validateConsumerSubjects(filterSubject string, filterSubjects []string, configSubjects []string) error {
	for _, subject := range configSubjects {
		if err := validateConsumerSubject(filterSubject, filterSubjects, subject); err != nil {
			return err
		}
	}
	return nil
}
