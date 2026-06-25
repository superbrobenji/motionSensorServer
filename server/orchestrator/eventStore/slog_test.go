package eventstore

import (
	"context"
	"testing"
)

// TestSubscribeToEvents_AcceptsContext verifies that SubscribeToEvents accepts a
// context.Context as its first parameter. This test will fail to compile until
// the interface and implementation are updated.
func TestSubscribeToEvents_AcceptsContext(t *testing.T) {
	store := New("localhost:19999", "test-group")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately so the call returns at once.

	// SubscribeToEvents(ctx, topic) must accept a context. If it does not compile,
	// the interface signature has not been updated yet.
	err := store.SubscribeToEvents(ctx, "test-topic")
	// We expect nil (clean shutdown) because the context is already cancelled.
	if err != nil {
		t.Errorf("expected nil on cancelled context, got: %v", err)
	}
}
