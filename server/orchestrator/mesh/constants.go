package mesh

import "fmt"

// Message Types
const (
	MessageTypeAdapterData        uint32 = 0 // Normal adapter-originated data
	MessageTypeMasterBeacon       uint32 = 1 // Mesh-internal heartbeat from master
	MessageTypeSerialCmdBroadcast uint32 = 3 // Server→device: serial command to broadcast adapter data
)

// Adapter Types (maps to firmware enum adapter_types)
const (
	AdapterTypeUnknown int32 = -1
	AdapterTypePIR     int32 = 0
	AdapterTypeWIFI    int32 = 1 // reserved
	AdapterTypeLED     int32 = 2 // reserved
	AdapterTypeSerial  int32 = 3 // serial control / health / commands
)

// Serial Control Opcodes (only when dataType = SERIAL)
const (
	OpConfigSet    byte = 0xA0 // Set adapter type on one node or all nodes
	OpHealthReq    byte = 0xB0 // Request health reports
	OpHealthReport byte = 0xB1 // Node → server health status
	OpNodeHealth   byte = 0xB2 // PIR (non-serial) node → server health status; transport: AdapterTypeSerial
	OpNodeIdSet    byte = 0xC0 // Server → node: assign logical ID; data: [C0][6B targetMAC][1B nodeId]
)

// Enrollment Message Type Constants
const (
	MessageTypeEnrollment uint32 = 2 // Node→master: public key announcement
	MessageTypeJoinAck    uint32 = 4 // Server→master→node: enrollment approved (pubkey present) or rejected (pubkey absent)
)

// Broadcast MAC address (all FF bytes)
var broadcastMAC = []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

// BroadcastMACBytes returns a copy of the broadcast MAC address.
func BroadcastMACBytes() []byte {
	cp := make([]byte, MACAddressLength)
	copy(cp, broadcastMAC)
	return cp
}

// MAC address length
const MACAddressLength = 6

// Maximum data payload length
const MaxDataLength = 64

// GetAdapterTypeName returns a human-readable name for an adapter type.
func GetAdapterTypeName(adapterType int32) string {
	switch adapterType {
	case AdapterTypeUnknown:
		return "Unknown"
	case AdapterTypePIR:
		return "PIR"
	case AdapterTypeWIFI:
		return "WiFi"
	case AdapterTypeLED:
		return "LED"
	case AdapterTypeSerial:
		return "Serial"
	default:
		return fmt.Sprintf("Unknown(%d)", adapterType)
	}
}
