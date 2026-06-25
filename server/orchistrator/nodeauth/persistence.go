package nodeauth

import (
	"encoding/hex"
	"encoding/json"
	"log"
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
		log.Printf("[nodeauth] No registry file at %s — starting fresh", path)
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
			log.Printf("[nodeauth] Skipping invalid MAC %s: %v", e.MAC, err)
			continue
		}
		pubKey, err := ParsePublicKey(e.PublicKey)
		if err != nil {
			log.Printf("[nodeauth] Skipping invalid pubkey for %s: %v", e.MAC, err)
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
	log.Printf("[nodeauth] Loaded %d node entries from %s", len(r.nodes), path)
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
				log.Printf("[nodeauth] Failed to persist registry: %v", err)
			}
		case <-stop:
			_ = r.Persist(path) // Final save on shutdown
			return
		}
	}
}
