package mesh

import (
	"context"

	EventStore "github.com/superbrobenji/motionServer/eventStore"
)

// MockEventStore provides a mock implementation for testing.
type MockEventStore struct {
	messages []string
	topics   []string
}

func NewMockEventStore() *MockEventStore {
	return &MockEventStore{
		messages: make([]string, 0),
		topics:   make([]string, 0),
	}
}

func (m *MockEventStore) Connect() error                                        { return nil }
func (m *MockEventStore) Close() error                                          { return nil }
func (m *MockEventStore) SubscribeToEvents(ctx context.Context, topic string) error { return nil }

func (m *MockEventStore) WriteMessage(event string, topic string) error {
	m.messages = append(m.messages, event)
	m.topics = append(m.topics, topic)
	return nil
}

func (m *MockEventStore) GetMessages() []string { return m.messages }
func (m *MockEventStore) GetTopics() []string   { return m.topics }

var _ EventStore.EventStoreInterface = (*MockEventStore)(nil)
