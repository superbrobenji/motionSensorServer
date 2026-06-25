package mesh

// Message Types
const (
	MessageTypeAdapterData      uint32 = 0 // Normal adapter-originated data
	MessageTypeMasterBeacon     uint32 = 1 // Mesh-internal heartbeat from master
	MessageTypeSerialCmdBroadcast uint32 = 3 // Server→device serial command to broadcast adapter data
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
)

// Enrollment Serial Opcodes (raw frames, not protobuf-encoded)
const (
	OpEnrollmentReq     byte = 0xC0 // New node → master → server: public key announcement
	OpEnrollmentApprove byte = 0xC1 // Server → master → node: enrollment approved
	OpEnrollmentReject  byte = 0xC2 // Server → master → node: enrollment rejected
)

// Enrollment Message Type Constants
const (
	MessageTypeEnrollment uint32 = 2 // New node → master: public key announcement
	MessageTypeJoinAck    uint32 = 3 // Master → new node: enrollment approved
)

// Broadcast MAC address (all FF bytes)
var BroadcastMAC = []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

// MAC address length
const MACAddressLength = 6

// Maximum data payload length
const MaxDataLength = 12
