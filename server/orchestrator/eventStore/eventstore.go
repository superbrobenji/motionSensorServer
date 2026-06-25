package eventstore

import "context"

// EventStoreInterface defines the event storage contract.
type EventStoreInterface interface {
	Connect() error
	WriteMessage(event string, topic string) error
	SubscribeToEvents(ctx context.Context, topic string) error
	Close() error
}
