package mesh

import (
	"encoding/binary"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestSetTxPowerPreset_SendsProtoFrame(t *testing.T) {
	ms := newTestMeshServer(t)
	mockPort := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mockPort)

	if err := ms.SetTxPowerPreset(1); err != nil {
		t.Fatalf("SetTxPowerPreset(1) returned error: %v", err)
	}

	data := mockPort.GetWrittenData()
	if len(data) < 2 {
		t.Fatalf("no frame written: only %d bytes", len(data))
	}
	length := int(binary.LittleEndian.Uint16(data[:2]))
	if len(data) < 2+length {
		t.Fatalf("frame truncated: need %d bytes after header, have %d", length, len(data)-2)
	}

	var msg MeshMessage
	if err := proto.Unmarshal(data[2:2+length], &msg); err != nil {
		t.Fatalf("frame is not valid protobuf: %v — WriteRaw was used instead of WriteFrame", err)
	}

	if msg.MessageType != MessageTypeAdapterData {
		t.Errorf("MessageType = %d, want %d (MessageTypeAdapterData)", msg.MessageType, MessageTypeAdapterData)
	}
	if msg.DataType != AdapterTypeSerial {
		t.Errorf("DataType = %d, want %d (AdapterTypeSerial)", msg.DataType, AdapterTypeSerial)
	}
	if len(msg.Data) == 0 || msg.Data[0] != OpTxPowerSet {
		t.Errorf("Data[0] = %d, want %d (OpTxPowerSet)", msg.Data[0], OpTxPowerSet)
	}
	if len(msg.Data) < 2 || msg.Data[1] != 1 {
		t.Errorf("Data[1] = %d, want 1 (preset)", msg.Data[1])
	}
}

func TestSetTxPowerPreset_InvalidPreset_ReturnsError(t *testing.T) {
	ms := newTestMeshServer(t)
	if err := ms.SetTxPowerPreset(3); err == nil {
		t.Error("expected error for preset=3, got nil")
	}
}

func TestHandleMessage_ProtoVersionGuard(t *testing.T) {
	ms := newTestMeshServer(t)
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}

	// Build a health report payload that would register a node if processed.
	healthData := make([]byte, 12)
	healthData[0] = byte(OpHealthReport)
	healthData[1] = byte(AdapterTypePIR)
	copy(healthData[2:8], mac)
	// bytes 8-11: uptime = 0 (zero value)

	// v1 message: current guard accepts v1 — after fix it must drop v1.
	v1msg := &MeshMessage{
		ProtoVersion:     1,
		MessageType:      MessageTypeAdapterData,
		DataType:         AdapterTypeSerial,
		Data:             healthData,
		OriginMacAddress: mac,
	}
	if err := ms.handleMessage(v1msg); err != nil {
		t.Fatalf("handleMessage(v1) returned unexpected error: %v", err)
	}
	if _, ok := ms.GetNodeRegistry().GetNode(mac); ok {
		t.Error("v1 message should be dropped — node must not be registered")
	}

	// v2 message: must be accepted and processed.
	v2msg := &MeshMessage{
		ProtoVersion:     2,
		MessageType:      MessageTypeAdapterData,
		DataType:         AdapterTypeSerial,
		Data:             healthData,
		OriginMacAddress: mac,
	}
	if err := ms.handleMessage(v2msg); err != nil {
		t.Fatalf("handleMessage(v2) returned unexpected error: %v", err)
	}
	if _, ok := ms.GetNodeRegistry().GetNode(mac); !ok {
		t.Error("v2 message must be processed — node should be registered after health report")
	}
}
