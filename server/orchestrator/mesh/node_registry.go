package mesh

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// NodeInfo holds information about a mesh node
type NodeInfo struct {
	MAC         []byte    `json:"mac"`
	MACString   string    `json:"macString"`
	AdapterType int32     `json:"adapterType"`
	Uptime      uint32    `json:"uptime"`
	LastSeen    time.Time `json:"lastSeen"`
	HopCount    uint32    `json:"hopCount"`
}

// NodeRegistry manages the state of all known mesh nodes
type NodeRegistry struct {
	mu    sync.RWMutex
	nodes map[string]*NodeInfo
}

// NewNodeRegistry creates a new node registry
func NewNodeRegistry() *NodeRegistry {
	return &NodeRegistry{
		nodes: make(map[string]*NodeInfo),
	}
}

// UpdateNode updates or creates a node entry from a health report
func (nr *NodeRegistry) UpdateNode(mac []byte, adapterType int32, uptime uint32, hopCount uint32) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	macStr := macToString(mac)
	
	node, exists := nr.nodes[macStr]
	if !exists {
		node = &NodeInfo{
			MAC:       make([]byte, len(mac)),
			MACString: macStr,
		}
		copy(node.MAC, mac)
		nr.nodes[macStr] = node
	}

	node.AdapterType = adapterType
	node.Uptime = uptime
	node.LastSeen = time.Now()
	node.HopCount = hopCount
}

// GetNode returns information about a specific node
func (nr *NodeRegistry) GetNode(mac []byte) (*NodeInfo, bool) {
	nr.mu.RLock()
	defer nr.mu.RUnlock()

	macStr := macToString(mac)
	node, exists := nr.nodes[macStr]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	nodeCopy := *node
	nodeCopy.MAC = make([]byte, len(node.MAC))
	copy(nodeCopy.MAC, node.MAC)
	
	return &nodeCopy, true
}

// GetAllNodes returns all known nodes
func (nr *NodeRegistry) GetAllNodes() []*NodeInfo {
	nr.mu.RLock()
	defer nr.mu.RUnlock()

	nodes := make([]*NodeInfo, 0, len(nr.nodes))
	for _, node := range nr.nodes {
		// Return copies to avoid race conditions
		nodeCopy := *node
		nodeCopy.MAC = make([]byte, len(node.MAC))
		copy(nodeCopy.MAC, node.MAC)
		nodes = append(nodes, &nodeCopy)
	}

	return nodes
}

// GetOnlineNodes returns nodes that have been seen recently (within timeout)
func (nr *NodeRegistry) GetOnlineNodes(timeout time.Duration) []*NodeInfo {
	nr.mu.RLock()
	defer nr.mu.RUnlock()

	cutoff := time.Now().Add(-timeout)
	nodes := make([]*NodeInfo, 0)
	
	for _, node := range nr.nodes {
		if node.LastSeen.After(cutoff) {
			// Return a copy to avoid race conditions
			nodeCopy := *node
			nodeCopy.MAC = make([]byte, len(node.MAC))
			copy(nodeCopy.MAC, node.MAC)
			nodes = append(nodes, &nodeCopy)
		}
	}

	return nodes
}

// RemoveNode removes a node from the registry
func (nr *NodeRegistry) RemoveNode(mac []byte) bool {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	macStr := macToString(mac)
	_, exists := nr.nodes[macStr]
	if exists {
		delete(nr.nodes, macStr)
	}
	return exists
}

// NodeCount returns the total number of registered nodes
func (nr *NodeRegistry) NodeCount() int {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return len(nr.nodes)
}

type persistedNode struct {
	MAC         string    `json:"mac"`
	AdapterType int32     `json:"adapterType"`
	Uptime      uint32    `json:"uptime"`
	LastSeen    time.Time `json:"lastSeen"`
	HopCount    uint32    `json:"hopCount"`
}

// Persist saves the registry to a JSON file at path.
func (nr *NodeRegistry) Persist(path string) error {
	nr.mu.RLock()
	entries := make([]persistedNode, 0, len(nr.nodes))
	for _, n := range nr.nodes {
		entries = append(entries, persistedNode{
			MAC:         n.MACString,
			AdapterType: n.AdapterType,
			Uptime:      n.Uptime,
			LastSeen:    n.LastSeen,
			HopCount:    n.HopCount,
		})
	}
	nr.mu.RUnlock()

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal nodes: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dirs for %s: %w", path, err)
	}
	return os.WriteFile(path, data, 0600)
}

// Load reads a persisted registry from a JSON file. Missing file = empty registry (not an error).
func (nr *NodeRegistry) Load(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var entries []persistedNode
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("unmarshal nodes: %w", err)
	}

	nr.mu.Lock()
	defer nr.mu.Unlock()
	for _, e := range entries {
		mac, err := StringToMAC(e.MAC)
		if err != nil {
			slog.Warn("Skipping invalid MAC in node registry", "mac", e.MAC, "error", err)
			continue
		}
		nr.nodes[e.MAC] = &NodeInfo{
			MAC:         mac,
			MACString:   e.MAC,
			AdapterType: e.AdapterType,
			Uptime:      e.Uptime,
			LastSeen:    e.LastSeen,
			HopCount:    e.HopCount,
		}
	}
	slog.Info("Node registry loaded", "count", len(entries), "path", path)
	return nil
}

// PersistLoop saves the registry every interval. Run as a goroutine.
func (nr *NodeRegistry) PersistLoop(path string, interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if err := nr.Persist(path); err != nil {
				slog.Warn("Node registry persist failed", "error", err)
			}
		case <-stop:
			if err := nr.Persist(path); err != nil {
				slog.Warn("Node registry final persist failed", "error", err)
			}
			return
		}
	}
}

// macToString converts a MAC address byte slice to a string representation
func macToString(mac []byte) string {
	if len(mac) != MACAddressLength {
		return hex.EncodeToString(mac)
	}
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", 
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

// StringToMAC converts a MAC address string to byte slice
func StringToMAC(macStr string) ([]byte, error) {
	// Try hex format first
	if mac, err := hex.DecodeString(macStr); err == nil {
		if len(mac) == MACAddressLength {
			return mac, nil
		}
	}

	// Try colon-separated format
	var mac [MACAddressLength]byte
	n, err := fmt.Sscanf(macStr, "%02x:%02x:%02x:%02x:%02x:%02x",
		&mac[0], &mac[1], &mac[2], &mac[3], &mac[4], &mac[5])
	if err != nil || n != MACAddressLength {
		return nil, fmt.Errorf("invalid MAC address format: %s", macStr)
	}

	return mac[:], nil
}
