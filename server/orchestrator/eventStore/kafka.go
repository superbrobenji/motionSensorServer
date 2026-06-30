package eventstore

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/segmentio/kafka-go"
)

type store struct {
	broker  string
	groupId string
	writer  *kafka.Writer
	reader  *kafka.Reader
}

func New(broker string, groupId string) EventStoreInterface {
	return &store{broker: broker, groupId: groupId}
}

func (s *store) Connect() error {
	slog.Info("Connecting to Kafka", "broker", s.broker)

	conn, err := net.DialTimeout("tcp", s.broker, 5*time.Second)
	if err != nil {
		return fmt.Errorf("kafka broker unreachable at %s: %w", s.broker, err)
	}
	_ = conn.Close()

	s.writer = &kafka.Writer{
		Addr:     kafka.TCP(s.broker),
		Balancer: &kafka.LeastBytes{},
	}

	slog.Info("Connected to Kafka", "broker", s.broker)
	return nil
}

func (s *store) WriteMessage(event string, topic string) error {
	if s.writer == nil {
		return fmt.Errorf("not connected")
	}
	slog.Debug("Delivering message", "topic", topic)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := s.writer.WriteMessages(ctx,
		kafka.Message{Topic: topic, Value: []byte(event)},
	)
	if err != nil {
		slog.Error("Kafka delivery failed", "topic", topic, "error", err)
		return err
	}
	slog.Debug("Delivered message", "topic", topic)
	return nil
}

func (s *store) SubscribeToEvents(ctx context.Context, topic string) error {
	if s.reader != nil {
		s.reader.Close()
	}
	s.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{s.broker},
		Topic:   topic,
		GroupID: s.groupId,
	})
	slog.Info("Subscribed to Kafka topic", "topic", topic)
	for {
		msg, err := s.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			slog.Error("Kafka consumer error", "topic", topic, "error", err)
			s.reader.Close()
			return err
		}
		slog.Debug("Kafka message received", "topic", msg.Topic, "value", string(msg.Value))
	}
}

func (s *store) Close() error {
	if s.reader != nil {
		if err := s.reader.Close(); err != nil {
			return fmt.Errorf("closing reader: %w", err)
		}
	}
	if s.writer != nil {
		if err := s.writer.Close(); err != nil {
			return fmt.Errorf("closing writer: %w", err)
		}
	}
	return nil
}
