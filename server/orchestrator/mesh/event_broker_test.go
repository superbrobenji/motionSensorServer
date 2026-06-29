package mesh

import (
	"testing"
	"time"
)

func TestEventBroker_SubscribeReceivesPublished(t *testing.T) {
	b := NewEventBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	raw := []byte(`{"nodeId":7}`)
	b.Publish(Event{Type: EventMotion, Data: raw, Timestamp: time.Now()})

	select {
	case e := <-ch:
		if e.Type != EventMotion {
			t.Errorf("Type: got %q, want %q", e.Type, EventMotion)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no event received within 100ms")
	}
}

func TestEventBroker_UnsubscribedChannelClosed(t *testing.T) {
	b := NewEventBroker()
	ch := b.Subscribe()
	b.Unsubscribe(ch)
	_, open := <-ch
	if open {
		t.Error("channel should be closed after Unsubscribe")
	}
}

func TestEventBroker_FullBufferDropsEvent(t *testing.T) {
	b := NewEventBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Fill buffer (size 32) + 1 extra — must not block
	done := make(chan struct{})
	go func() {
		for i := 0; i < 33; i++ {
			b.Publish(Event{Type: EventHealth, Data: []byte(`{}`), Timestamp: time.Now()})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Publish blocked on full buffer")
	}

	if len(ch) != 32 {
		t.Errorf("buffer has %d events, want 32", len(ch))
	}
}

func TestEventBroker_MultipleSubscribers(t *testing.T) {
	b := NewEventBroker()
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	defer b.Unsubscribe(ch1)
	defer b.Unsubscribe(ch2)

	b.Publish(Event{Type: EventNodeOnline, Data: []byte(`{}`), Timestamp: time.Now()})

	for i, ch := range []chan Event{ch1, ch2} {
		select {
		case e := <-ch:
			if e.Type != EventNodeOnline {
				t.Errorf("subscriber %d: type %q, want %q", i, e.Type, EventNodeOnline)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d: no event received", i)
		}
	}
}
