package mesh

import (
	"encoding/binary"
	"fmt"
)

// MessageBuilder provides utilities for constructing mesh messages
type MessageBuilder struct{}

// NewMessageBuilder creates a new message builder
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{}
}

// BuildConfigSetMessage creates a message to set adapter type on a node
func (mb *MessageBuilder) BuildConfigSetMessage(targetMAC []byte, adapterType int32) (*MeshMessage, error) {
	if len(targetMAC) != MACAddressLength {
		return nil, fmt.Errorf("invalid MAC address length: %d, expected %d", len(targetMAC), MACAddressLength)
	}

	payload := make([]byte, MaxDataLength)
	payload[0] = OpConfigSet
	copy(payload[1:7], targetMAC)
	payload[7] = byte(adapterType)
	// Bytes 8-11 are reserved (already zero from make)

	return &MeshMessage{
		MessageType:      MessageTypeAdapterData,
		DataType:         AdapterTypeSerial,
		TargetMacAddress: targetMAC,
		Data:             payload,
	}, nil
}

// BuildConfigSetBroadcastMessage creates a broadcast message to set adapter type on all nodes
func (mb *MessageBuilder) BuildConfigSetBroadcastMessage(adapterType int32) (*MeshMessage, error) {
	return mb.BuildConfigSetMessage(BroadcastMACBytes(), adapterType)
}

// BuildHealthRequestMessage creates a message to request health reports
func (mb *MessageBuilder) BuildHealthRequestMessage() *MeshMessage {
	payload := make([]byte, MaxDataLength)
	payload[0] = OpHealthReq
	// Remaining bytes are zero

	return &MeshMessage{
		MessageType: MessageTypeAdapterData,
		DataType:    AdapterTypeSerial,
		Data:        payload,
	}
}

// BuildBroadcastMessage creates a broadcast message with custom data
func (mb *MessageBuilder) BuildBroadcastMessage(dataType int32, data []byte) (*MeshMessage, error) {
	if len(data) > MaxDataLength {
		return nil, fmt.Errorf("data length %d exceeds maximum %d", len(data), MaxDataLength)
	}

	payload := make([]byte, MaxDataLength)
	copy(payload, data)

	return &MeshMessage{
		MessageType: MessageTypeSerialCmdBroadcast,
		DataType:    dataType,
		Data:        payload,
	}, nil
}

// BuildAdapterDataMessage creates a targeted adapter data message
func (mb *MessageBuilder) BuildAdapterDataMessage(targetMAC []byte, dataType int32, data []byte) (*MeshMessage, error) {
	if len(targetMAC) != MACAddressLength {
		return nil, fmt.Errorf("invalid MAC address length: %d, expected %d", len(targetMAC), MACAddressLength)
	}
	
	if len(data) > MaxDataLength {
		return nil, fmt.Errorf("data length %d exceeds maximum %d", len(data), MaxDataLength)
	}

	payload := make([]byte, MaxDataLength)
	copy(payload, data)

	return &MeshMessage{
		MessageType:      MessageTypeAdapterData,
		DataType:         dataType,
		TargetMacAddress: targetMAC,
		Data:             payload,
	}, nil
}

// ParseHealthReport extracts health information from a health report message
func (mb *MessageBuilder) ParseHealthReport(msg *MeshMessage) (*HealthReport, error) {
	if msg.DataType != AdapterTypeSerial {
		return nil, fmt.Errorf("message is not a serial message")
	}

	if len(msg.Data) < 12 {
		return nil, fmt.Errorf("insufficient data length for health report: %d", len(msg.Data))
	}

	if msg.Data[0] != OpHealthReport {
		return nil, fmt.Errorf("message is not a health report, opcode: 0x%02x", msg.Data[0])
	}

	adapterType := int32(int8(msg.Data[1])) // Convert to signed int8 first, then to int32
	mac := make([]byte, MACAddressLength)
	copy(mac, msg.Data[2:8])
	uptime := binary.LittleEndian.Uint32(msg.Data[8:12])

	return &HealthReport{
		MAC:         mac,
		AdapterType: adapterType,
		Uptime:      uptime,
		HopCount:    msg.HopCount,
		OriginMAC:   msg.OriginMacAddress,
	}, nil
}

// HealthReport represents parsed health report data
type HealthReport struct {
	MAC         []byte
	AdapterType int32
	Uptime      uint32
	HopCount    uint32
	OriginMAC   []byte
}

// IsHealthReport checks if a message is a health report
func (mb *MessageBuilder) IsHealthReport(msg *MeshMessage) bool {
	return msg.DataType == AdapterTypeSerial &&
		len(msg.Data) >= 1 &&
		msg.Data[0] == OpHealthReport
}

// IsMasterBeacon checks if a message is a master beacon
func (mb *MessageBuilder) IsMasterBeacon(msg *MeshMessage) bool {
	return msg.MessageType == MessageTypeMasterBeacon
}
