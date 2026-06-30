package mesh

import "time"

// NodeV1 is the artist-facing node representation.
type NodeV1 struct {
	ID       uint8     `json:"id"`
	Name     string    `json:"name"`
	Zone     string    `json:"zone"`
	Type     string    `json:"type"`
	Online   bool      `json:"online"`
	HopCount uint32    `json:"hopCount"`
	Uptime   uint32    `json:"uptime"`
	LastSeen time.Time `json:"lastSeen"`
}

// adapterTypeToString converts an internal adapter type to the artist-facing string.
func adapterTypeToString(t int32) string {
	switch t {
	case AdapterTypePIR:
		return "pir"
	case AdapterTypeLED:
		return "led"
	case AdapterTypeRelay:
		return "relay"
	case AdapterTypeSerial:
		return "serial"
	default:
		return "unknown"
	}
}

// adapterTypeFromString converts an artist-facing type string to an internal adapter type.
// Returns false if the string is not a writable type.
func adapterTypeFromString(s string) (int32, bool) {
	switch s {
	case "pir":
		return AdapterTypePIR, true
	case "led":
		return AdapterTypeLED, true
	case "relay":
		return AdapterTypeRelay, true
	default:
		return 0, false
	}
}

// nodeToV1 converts an internal NodeInfo to the artist-facing NodeV1.
func nodeToV1(n *NodeInfo, timeout time.Duration) NodeV1 {
	return NodeV1{
		ID:       n.NodeID,
		Name:     n.Name,
		Zone:     n.Zone,
		Type:     adapterTypeToString(n.AdapterType),
		Online:   time.Since(n.LastSeen) <= timeout,
		HopCount: n.HopCount,
		Uptime:   n.Uptime,
		LastSeen: n.LastSeen,
	}
}
