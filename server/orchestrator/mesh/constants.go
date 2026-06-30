package mesh

import (
	"fmt"

	"github.com/superbrobenji/planetopia-protocol/adapter"
	"github.com/superbrobenji/planetopia-protocol/opcodes"
)

// Message Types
const (
	MessageTypeAdapterData        uint32 = 0 // Normal adapter-originated data
	MessageTypeMasterBeacon       uint32 = 1 // Mesh-internal heartbeat from master
	MessageTypeSerialCmdBroadcast uint32 = 3 // Server→device: serial command to broadcast adapter data
)

// Adapter type aliases — use shared protocol constants.
// Values: TypeUnknown=0, TypeSerial=1, TypePIR=2, TypeLED=3, TypeRelay=4.
const (
	AdapterTypeUnknown = adapter.TypeUnknown // 0
	AdapterTypeSerial  = adapter.TypeSerial  // 1 — serial management (internal)
	AdapterTypePIR     = adapter.TypePIR     // 2 — passive infrared motion sensor (INPUT)
	AdapterTypeLED     = adapter.TypeLED     // 3 — LED strip (OUTPUT)
	AdapterTypeRelay   = adapter.TypeRelay   // 4 — relay switch (OUTPUT)

	// AdapterTypeWIFI is reserved locally; not part of the shared protocol.
	AdapterTypeWIFI int32 = 5 // reserved
)

// Serial Control Opcodes (only when dataType = SERIAL)
// Shared opcodes are imported from the protocol package.
const (
	OpNodeIdSet  = opcodes.OpNodeIdSet  // 0xC0 — Server → node: assign logical node ID
	OpConfigSet  = opcodes.OpConfigSet  // 0xC1 — Server → node: set adapter type and config
	OpTxPowerSet = opcodes.OpTxPowerSet // 0xC2 — Server → node: set TX power preset

	// Health opcodes remain local (not yet in shared protocol).
	OpHealthReq    byte = 0xB0 // Request health reports
	OpHealthReport byte = 0xB1 // Node → server health status
	OpNodeHealth   byte = 0xB2 // PIR (non-serial) node → server health status; transport: AdapterTypeSerial
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
	case AdapterTypeSerial:
		return "Serial"
	case AdapterTypePIR:
		return "PIR"
	case AdapterTypeLED:
		return "LED"
	case AdapterTypeRelay:
		return "Relay"
	case AdapterTypeWIFI:
		return "WiFi"
	default:
		return fmt.Sprintf("Unknown(%d)", adapterType)
	}
}
