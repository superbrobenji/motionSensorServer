package nodeauth

import (
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"time"
)

// persistedEntry is the JSON-serializable form of NodeAuth.
type persistedEntry struct {
	MAC        string `json:"mac"`
	PublicKey  string `json:"publicKey"`
	Status     int    `json:"status"`
	ReceivedAt int64  `json:"receivedAt"`
	ApprovedAt int64  `json:"approvedAt"`
}

// Persist saves the registry to a JSON file at the given path.
func (r *Registry) Persist(path string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]persistedEntry, 0, len(r.nodes))
	for _, n := range r.nodes {
		entries = append(entries, persistedEntry{
			MAC:        n.MACString,
			PublicKey:  hex.EncodeToString(n.PublicKey[:]),
			Status:     int(n.Status),
			ReceivedAt: n.ReceivedAt.Unix(),
			ApprovedAt: n.ApprovedAt.Unix(),
		})
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Load reads a persisted registry from a JSON file. Missing file = empty registry.
func (r *Registry) Load(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		slog.Info("No auth registry file — starting fresh", "path", path)
		return nil
	}
	if err != nil {
		return err
	}

	var entries []persistedEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range entries {
		mac, err := ParseMAC(e.MAC)
		if err != nil {
			slog.Warn("Skipping invalid MAC in auth registry", "mac", e.MAC, "error", err)
			continue
		}
		pubKey, err := ParsePublicKey(e.PublicKey)
		if err != nil {
			slog.Warn("Skipping invalid public key in auth registry", "mac", e.MAC, "error", err)
			continue
		}
		node := &NodeAuth{
			MAC:        mac,
			MACString:  e.MAC,
			PublicKey:  pubKey,
			Status:     TrustStatus(e.Status),
			ReceivedAt: time.Unix(e.ReceivedAt, 0),
			ApprovedAt: time.Unix(e.ApprovedAt, 0),
		}
		r.nodes[e.MAC] = node
	}
	slog.Info("Auth registry loaded", "count", len(r.nodes), "path", path)
	return nil
}

// PersistLoop saves the registry every interval. Run as a goroutine.
func (r *Registry) PersistLoop(path string, interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if err := r.Persist(path); err != nil {
				slog.Warn("Auth registry persist failed", "error", err)
			}
		case <-stop:
			if err := r.Persist(path); err != nil {
				slog.Warn("Auth registry final persist failed", "error", err)
			}
			return
		}
	}
}
