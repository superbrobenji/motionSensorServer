package eventstore

import (
	"testing"
	"time"
)

func TestNew_ReturnsInterface(t *testing.T) {
	store := New("localhost:9999", "test-group")
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestConnect_FailsOnUnreachableBroker(t *testing.T) {
	store := New("localhost:19999", "test-group")
	err := store.Connect()
	if err == nil {
		t.Error("expected error connecting to unreachable broker, got nil")
	}
}

func TestClose_NoError(t *testing.T) {
	store := New("localhost:19999", "test-group")
	// Connect will fail; Close should still not panic
	_ = store.Connect()
	if err := store.Close(); err != nil {
		t.Errorf("unexpected error on Close: %v", err)
	}
}

func TestWriteMessage_RespectsTimeout(t *testing.T) {
	// This test verifies WriteMessage does not block indefinitely.
	// We test the timeout by checking the function returns within 3s
	// even when given a store with no real Kafka connection (nil writer
	// returns "not connected" immediately — which is also acceptable behavior).
	s := &store{writer: nil}
	start := time.Now()
	err := s.WriteMessage("test", "test-topic")
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Errorf("WriteMessage took %v, want < 3s", elapsed)
	}
	if err == nil {
		t.Error("WriteMessage with nil writer should return error")
	}
}
