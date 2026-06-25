package eventstore

import (
	"testing"
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
