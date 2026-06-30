package nodeauth

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// TrustStatus represents the enrollment state of a node.
type TrustStatus int

const (
	TrustPending  TrustStatus = iota // Enrollment request received, awaiting admin approval
	TrustApproved                    // Admin approved; node is a valid mesh member
	TrustRejected                    // Admin rejected; node should not join
)

// NodeAuth holds cryptographic identity for one mesh node.
type NodeAuth struct {
	MAC        [6]byte
	MACString  string
	PublicKey  [32]byte // Curve25519 public key
	Status     TrustStatus
	ReceivedAt time.Time
	ApprovedAt time.Time
}

// Registry manages the trust state of all known nodes.
type Registry struct {
	mu    sync.RWMutex
	nodes map[string]*NodeAuth // keyed by MAC string
}

func NewRegistry() *Registry {
	return &Registry{nodes: make(map[string]*NodeAuth)}
}

func (r *Registry) AddPending(mac [6]byte, pubKey [32]byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := macToString(mac)
	if existing, ok := r.nodes[key]; ok && existing.Status == TrustApproved {
		return fmt.Errorf("node %s already approved", key)
	}
	r.nodes[key] = &NodeAuth{
		MAC:        mac,
		MACString:  key,
		PublicKey:  pubKey,
		Status:     TrustPending,
		ReceivedAt: time.Now(),
	}
	return nil
}

func (r *Registry) Approve(macStr string) (*NodeAuth, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Normalize to canonical colon-separated form so callers may pass either format.
	if mac, err := ParseMAC(macStr); err == nil {
		macStr = macToString(mac)
	}
	node, ok := r.nodes[macStr]
	if !ok {
		return nil, fmt.Errorf("node %s not found", macStr)
	}
	node.Status = TrustApproved
	node.ApprovedAt = time.Now()
	return node, nil
}

func (r *Registry) Reject(macStr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Normalize to canonical colon-separated form so callers may pass either format.
	if mac, err := ParseMAC(macStr); err == nil {
		macStr = macToString(mac)
	}
	node, ok := r.nodes[macStr]
	if !ok {
		return fmt.Errorf("node %s not found", macStr)
	}
	node.Status = TrustRejected
	return nil
}

func (r *Registry) GetAll() []*NodeAuth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*NodeAuth, 0, len(r.nodes))
	for _, n := range r.nodes {
		copy := *n
		out = append(out, &copy)
	}
	return out
}

func (r *Registry) GetPending() []*NodeAuth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*NodeAuth
	for _, n := range r.nodes {
		if n.Status == TrustPending {
			copy := *n
			out = append(out, &copy)
		}
	}
	return out
}

func (r *Registry) IsApproved(mac [6]byte) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	node, ok := r.nodes[macToString(mac)]
	return ok && node.Status == TrustApproved
}

func (r *Registry) GetApprovedPublicKey(mac [6]byte) ([32]byte, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	node, ok := r.nodes[macToString(mac)]
	if !ok || node.Status != TrustApproved {
		return [32]byte{}, false
	}
	return node.PublicKey, true
}

func macToString(mac [6]byte) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

func ParseMAC(s string) ([6]byte, error) {
	var mac [6]byte
	// Try colon-separated format first
	n, err := fmt.Sscanf(s, "%02x:%02x:%02x:%02x:%02x:%02x",
		&mac[0], &mac[1], &mac[2], &mac[3], &mac[4], &mac[5])
	if err == nil && n == 6 {
		return mac, nil
	}
	// Fall back to bare hex (for backward compatibility with existing persisted data)
	b, hexErr := hex.DecodeString(s)
	if hexErr != nil || len(b) != 6 {
		return mac, fmt.Errorf("invalid MAC: %s", s)
	}
	copy(mac[:], b)
	return mac, nil
}

func ParsePublicKey(s string) ([32]byte, error) {
	var key [32]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return key, fmt.Errorf("invalid public key: %s", s)
	}
	copy(key[:], b)
	return key, nil
}
