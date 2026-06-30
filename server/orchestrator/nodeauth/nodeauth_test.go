package nodeauth

import (
	"os"
	"testing"
	"time"
)

// ---- helper ---------------------------------------------------------------

func makeMAC(b byte) [6]byte { return [6]byte{b, b, b, b, b, b} }

func makePubKey(b byte) [32]byte {
	var k [32]byte
	for i := range k {
		k[i] = b
	}
	return k
}

// ---- Registry tests -------------------------------------------------------

// TestRegistry_AddPendingAndGetPending verifies that AddPending stores a node
// and GetPending returns it with TrustPending status.
func TestRegistry_AddPendingAndGetPending(t *testing.T) {
	r := NewRegistry()
	mac := makeMAC(0xAA)
	pub := makePubKey(0x01)

	if err := r.AddPending(mac, pub); err != nil {
		t.Fatalf("AddPending: %v", err)
	}

	pending := r.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending node, got %d", len(pending))
	}
	if pending[0].MAC != mac {
		t.Errorf("MAC mismatch: got %x, want %x", pending[0].MAC, mac)
	}
	if pending[0].PublicKey != pub {
		t.Errorf("PublicKey mismatch")
	}
	if pending[0].Status != TrustPending {
		t.Errorf("expected TrustPending, got %d", pending[0].Status)
	}
}

// TestRegistry_ApproveMovesToTrusted verifies that Approve transitions the
// node from TrustPending to TrustApproved and moves it out of GetPending.
func TestRegistry_ApproveMovesToTrusted(t *testing.T) {
	r := NewRegistry()
	mac := makeMAC(0xBB)
	pub := makePubKey(0x02)

	if err := r.AddPending(mac, pub); err != nil {
		t.Fatalf("AddPending: %v", err)
	}

	macStr := macToString(mac)
	node, err := r.Approve(macStr)
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if node.Status != TrustApproved {
		t.Errorf("expected TrustApproved after Approve, got %d", node.Status)
	}
	if node.ApprovedAt.IsZero() {
		t.Error("ApprovedAt should be set after approval")
	}

	// Should no longer appear in GetPending
	pending := r.GetPending()
	for _, n := range pending {
		if n.MAC == mac {
			t.Error("approved node still appears in GetPending")
		}
	}

	// IsApproved should now return true
	if !r.IsApproved(mac) {
		t.Error("IsApproved returned false after Approve")
	}

	// GetApprovedPublicKey should return the key
	gotKey, ok := r.GetApprovedPublicKey(mac)
	if !ok {
		t.Fatal("GetApprovedPublicKey returned !ok")
	}
	if gotKey != pub {
		t.Error("GetApprovedPublicKey returned wrong key")
	}
}

// TestRegistry_GetAll returns all nodes regardless of status.
func TestRegistry_GetAll(t *testing.T) {
	r := NewRegistry()
	mac1 := makeMAC(0x01)
	mac2 := makeMAC(0x02)
	_ = r.AddPending(mac1, makePubKey(0x01))
	_ = r.AddPending(mac2, makePubKey(0x02))
	_, _ = r.Approve(macToString(mac1))

	all := r.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(all))
	}
}

// ---- Persistence round-trip -----------------------------------------------

// TestPersistence_RoundTrip verifies that Persist followed by Load on a fresh
// Registry reproduces MAC, public key, status, and ReceivedAt timestamps.
func TestPersistence_RoundTrip(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "nodeauth-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	path := f.Name()
	_ = f.Close()

	r1 := NewRegistry()
	mac := makeMAC(0xCC)
	pub := makePubKey(0x03)
	before := time.Now().Truncate(time.Second)

	if err := r1.AddPending(mac, pub); err != nil {
		t.Fatalf("AddPending: %v", err)
	}

	macStr := macToString(mac)
	if _, err2 := r1.Approve(macStr); err2 != nil {
		t.Fatalf("Approve: %v", err2)
	}

	if err := r1.Persist(path); err != nil {
		t.Fatalf("Persist: %v", err)
	}

	r2 := NewRegistry()
	if err := r2.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	nodes := r2.GetAll()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node after Load, got %d", len(nodes))
	}
	n := nodes[0]

	if n.MAC != mac {
		t.Errorf("MAC: got %x, want %x", n.MAC, mac)
	}
	if n.PublicKey != pub {
		t.Errorf("PublicKey mismatch after round-trip")
	}
	if n.Status != TrustApproved {
		t.Errorf("Status: got %d, want TrustApproved", n.Status)
	}
	if n.ReceivedAt.Before(before) {
		t.Errorf("ReceivedAt not preserved: got %v", n.ReceivedAt)
	}
	if n.ApprovedAt.IsZero() {
		t.Error("ApprovedAt is zero after round-trip")
	}
}

// ---- Replay cache tests ---------------------------------------------------

// TestReplayCache_DuplicateDetection verifies that the second call with the
// same (MAC, epoch, seq) is detected as a duplicate.
func TestReplayCache_DuplicateDetection(t *testing.T) {
	rc := NewReplayCache(32)
	mac := makeMAC(0xDD)

	if rc.IsDuplicate(mac, 1, 1) {
		t.Fatal("first call should not be a duplicate")
	}
	if !rc.IsDuplicate(mac, 1, 1) {
		t.Fatal("second call with same (epoch, seq) must be detected as duplicate")
	}
}

// TestReplayCache_PerNodeIsolation verifies that two different MACs with
// identical (epoch, seq) values are NOT treated as duplicates of each other.
func TestReplayCache_PerNodeIsolation(t *testing.T) {
	rc := NewReplayCache(32)
	mac1 := makeMAC(0xEE)
	mac2 := makeMAC(0xFF)

	if rc.IsDuplicate(mac1, 5, 10) {
		t.Fatal("mac1 (5,10): unexpected duplicate on first call")
	}
	// Same epoch+seq but different MAC — must NOT be flagged
	if rc.IsDuplicate(mac2, 5, 10) {
		t.Fatal("mac2 (5,10): should not be a duplicate of mac1 — per-node isolation broken")
	}
}

// TestReplayCache_RingWrap verifies FIFO eviction: after 32 entries the 33rd
// write recycles slot 0, so the (epoch, seq) that was at slot 0 is no longer
// tracked and is not flagged as a duplicate.
func TestReplayCache_RingWrap(t *testing.T) {
	rc := NewReplayCache(32)
	mac := makeMAC(0x11)

	// First entry: epoch=1, seq=0 — occupies slot 0.
	if rc.IsDuplicate(mac, 1, 0) {
		t.Fatal("entry 0: unexpected duplicate")
	}

	// Fill the remaining 31 slots with distinct entries.
	for i := uint32(1); i < 32; i++ {
		if rc.IsDuplicate(mac, 1, i) {
			t.Fatalf("entry %d: unexpected duplicate while filling ring", i)
		}
	}

	// At this point the ring is full (32 entries, slots 0–31 used).
	// Verify that entry 0 (epoch=1, seq=0) is still detected as duplicate.
	if !rc.IsDuplicate(mac, 1, 0) {
		t.Fatal("entry 0 should still be in ring before wrap")
	}
	// The previous IsDuplicate call for (1,0) returned true — it was not written
	// again. The write index is still at 0 (next write will go to slot 0).

	// Write the 33rd distinct entry — this wraps idx to slot 0, evicting (1,0).
	if rc.IsDuplicate(mac, 2, 0) {
		t.Fatal("entry 32 (epoch=2,seq=0): unexpected duplicate")
	}

	// Now (epoch=1, seq=0) should have been evicted from slot 0.
	// It must no longer be flagged as a duplicate.
	if rc.IsDuplicate(mac, 1, 0) {
		t.Fatal("entry (1,0) should have been evicted after ring wrap, but was still detected as duplicate")
	}
}
