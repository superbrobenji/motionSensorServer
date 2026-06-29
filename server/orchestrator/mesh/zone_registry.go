package mesh

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// ZoneV1 is the artist-facing zone representation.
type ZoneV1 struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ZoneRegistry stores artist-defined zones.
type ZoneRegistry struct {
	mu    sync.RWMutex
	zones map[string]*ZoneV1 // keyed by ID
}

// NewZoneRegistry returns an empty ZoneRegistry.
func NewZoneRegistry() *ZoneRegistry {
	return &ZoneRegistry{zones: make(map[string]*ZoneV1)}
}

// zoneSlug derives the URL-safe ID from a zone name.
func zoneSlug(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "-")
}

// Add creates a zone. Returns an error if the derived ID is already taken.
func (zr *ZoneRegistry) Add(name string) (*ZoneV1, error) {
	id := zoneSlug(name)
	zr.mu.Lock()
	defer zr.mu.Unlock()
	if _, exists := zr.zones[id]; exists {
		return nil, fmt.Errorf("zone %q already exists", id)
	}
	z := &ZoneV1{ID: id, Name: name}
	zr.zones[id] = z
	copy := *z
	return &copy, nil
}

// Get returns a zone by ID.
func (zr *ZoneRegistry) Get(id string) (*ZoneV1, bool) {
	zr.mu.RLock()
	defer zr.mu.RUnlock()
	z, ok := zr.zones[id]
	if !ok {
		return nil, false
	}
	copy := *z
	return &copy, true
}

// List returns all zones (unordered).
func (zr *ZoneRegistry) List() []*ZoneV1 {
	zr.mu.RLock()
	defer zr.mu.RUnlock()
	result := make([]*ZoneV1, 0, len(zr.zones))
	for _, z := range zr.zones {
		copy := *z
		result = append(result, &copy)
	}
	return result
}

// Update renames a zone. The ID never changes.
func (zr *ZoneRegistry) Update(id, newName string) (*ZoneV1, bool) {
	zr.mu.Lock()
	defer zr.mu.Unlock()
	z, ok := zr.zones[id]
	if !ok {
		return nil, false
	}
	z.Name = newName
	copy := *z
	return &copy, true
}

// Delete removes a zone by ID. Returns false if not found.
func (zr *ZoneRegistry) Delete(id string) bool {
	zr.mu.Lock()
	defer zr.mu.Unlock()
	_, ok := zr.zones[id]
	delete(zr.zones, id)
	return ok
}

// Persist writes the registry to a JSON file.
func (zr *ZoneRegistry) Persist(path string) error {
	zr.mu.RLock()
	zones := make([]*ZoneV1, 0, len(zr.zones))
	for _, z := range zr.zones {
		cp := *z
		zones = append(zones, &cp)
	}
	zr.mu.RUnlock()
	data, err := json.MarshalIndent(zones, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load reads zones from a JSON file. Non-existent file is a no-op.
func (zr *ZoneRegistry) Load(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var zones []*ZoneV1
	if err := json.Unmarshal(data, &zones); err != nil {
		return err
	}
	zr.mu.Lock()
	defer zr.mu.Unlock()
	for _, z := range zones {
		zr.zones[z.ID] = z
	}
	return nil
}
