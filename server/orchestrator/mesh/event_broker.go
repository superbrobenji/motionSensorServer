package mesh

import (
	"encoding/json"
	"sync"
	"time"
)

// EventType is a named SSE event category.
type EventType string

const (
	EventMotion      EventType = "motion"
	EventNodeOnline  EventType = "node_online"
	EventNodeOffline EventType = "node_offline"
	EventEnrolled    EventType = "enrolled"
	EventHealth      EventType = "health"
)

// Event is a single SSE message sent to subscribers.
type Event struct {
	Type      EventType       `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// EventBroker is an in-process pub/sub bus. Publish is non-blocking; slow
// subscribers lose events silently rather than blocking the mesh message loop.
type EventBroker struct {
	mu      sync.RWMutex
	clients map[chan Event]struct{}
}

// NewEventBroker returns an initialised EventBroker.
func NewEventBroker() *EventBroker {
	return &EventBroker{clients: make(map[chan Event]struct{})}
}

// Subscribe returns a buffered channel that receives future events.
func (b *EventBroker) Subscribe() chan Event {
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes a subscriber channel.
func (b *EventBroker) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

// Publish sends e to all subscribers. Drops the event for any subscriber
// whose buffer is full (non-blocking).
func (b *EventBroker) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- e:
		default:
		}
	}
}
