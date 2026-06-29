package mesh

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

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

func TestHandleNodeHealth_RegistersNode(t *testing.T) {
	ms := newTestMeshServer(t)
	mac := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x01}

	data := make([]byte, MaxDataLength)
	data[0] = byte(OpNodeHealth) // 0xB2
	data[1] = byte(AdapterTypePIR)
	copy(data[2:8], mac)
	data[8] = 60 // uptime = 60s
	data[9] = 0
	data[10] = 0
	data[11] = 0

	msg := &MeshMessage{
		ProtoVersion:     2,
		MessageType:      MessageTypeAdapterData,
		DataType:         AdapterTypeSerial,
		Data:             data,
		OriginMacAddress: mac,
		HopCount:         2,
	}

	if err := ms.handleMessage(msg); err != nil {
		t.Fatalf("handleMessage returned error: %v", err)
	}

	node, ok := ms.GetNodeRegistry().GetNode(mac)
	if !ok {
		t.Fatal("node not registered after 0xB2 health report")
	}
	if node.AdapterType != AdapterTypePIR {
		t.Errorf("AdapterType: got %d, want %d", node.AdapterType, AdapterTypePIR)
	}
	if node.Uptime != 60 {
		t.Errorf("Uptime: got %d, want 60", node.Uptime)
	}
	if node.HopCount != 2 {
		t.Errorf("HopCount: got %d, want 2", node.HopCount)
	}
}

func TestMeshServer_PublishesMotionEvent_OnPIRData(t *testing.T) {
	ms := newTestMeshServer(t)
	ch := ms.GetEventBroker().Subscribe()
	defer ms.GetEventBroker().Unsubscribe(ch)

	mac := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x02}
	ms.nodeRegistry.AssignNode(mac, 5, "stage-left", "stage")

	data := make([]byte, MaxDataLength)
	data[0] = byte(AdapterTypePIR)
	copy(data[1:7], mac)
	msg := &MeshMessage{
		ProtoVersion:     2,
		MessageType:      MessageTypeAdapterData,
		DataType:         AdapterTypePIR,
		Data:             data,
		OriginMacAddress: mac,
	}
	if err := ms.handleMessage(msg); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	select {
	case e := <-ch:
		if e.Type != EventMotion {
			t.Errorf("event type: %q, want %q", e.Type, EventMotion)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no motion event within 200ms")
	}
}

func TestMeshServer_PublishesNodeOnline_OnFirstHealthReport(t *testing.T) {
	ms := newTestMeshServer(t)
	ch := ms.GetEventBroker().Subscribe()
	defer ms.GetEventBroker().Unsubscribe(ch)

	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	data := make([]byte, MaxDataLength)
	data[0] = byte(OpHealthReport)
	data[1] = byte(AdapterTypePIR)
	copy(data[2:8], mac)
	msg := &MeshMessage{
		ProtoVersion:     2,
		MessageType:      MessageTypeAdapterData,
		DataType:         AdapterTypeSerial,
		Data:             data,
		OriginMacAddress: mac,
	}
	if err := ms.handleMessage(msg); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var gotOnline bool
	for {
		select {
		case e := <-ch:
			if e.Type == EventNodeOnline {
				gotOnline = true
			}
		case <-time.After(200 * time.Millisecond):
			if !gotOnline {
				t.Error("no node_online event")
			}
			return
		}
		if gotOnline {
			return
		}
	}
}

func TestMeshServer_CheckOfflineNodes_PublishesOfflineEvent(t *testing.T) {
	ms := newTestMeshServer(t)
	ch := ms.GetEventBroker().Subscribe()
	defer ms.GetEventBroker().Unsubscribe(ch)

	mac := []byte{0xCA, 0xFE, 0xBA, 0xBE, 0x00, 0x01}
	ms.nodeRegistry.AssignNode(mac, 3, "old-node", "lobby")
	macStr := macToString(mac)

	// mark as previously online
	ms.mu.Lock()
	ms.nodeOnlineState[macStr] = true
	// set LastSeen 80s ago — exceeds 75s threshold
	ms.nodeRegistry.nodes[macStr].LastSeen = time.Now().Add(-80 * time.Second)
	ms.mu.Unlock()

	ms.checkOfflineNodes()

	select {
	case e := <-ch:
		if e.Type != EventNodeOffline {
			t.Errorf("event type: %q, want %q", e.Type, EventNodeOffline)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no node_offline event")
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

func TestSendNodeData_EmbedsMacInPayload(t *testing.T) {
	ms := newTestMeshServer(t)
	mockPort := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mockPort)

	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	data := make([]byte, MaxDataLength)
	data[0] = 0xD0 // trigger opcode

	if err := ms.SendNodeData(mac, int32(AdapterTypeSerial), data); err != nil {
		t.Fatalf("SendNodeData: %v", err)
	}

	written := mockPort.GetWrittenData()
	if len(written) < 2 {
		t.Fatal("no frame written")
	}
	length := int(binary.LittleEndian.Uint16(written[:2]))
	var msg MeshMessage
	if err := proto.Unmarshal(written[2:2+length], &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if msg.Data[0] != 0xD0 {
		t.Errorf("Data[0] opcode: got 0x%02X, want 0xD0", msg.Data[0])
	}
	if !bytes.Equal(msg.Data[1:7], mac) {
		t.Errorf("Data[1:7] MAC: got %v, want %v", msg.Data[1:7], mac)
	}
}

func TestActiveOutboundComm_ReturnsPrimary_WhenSecondaryNotConfigured(t *testing.T) {
	ms := newTestMeshServer(t)
	mock := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mock)

	comm := ms.activeOutboundComm()
	if comm != ms.serialComm {
		t.Error("activeOutboundComm() must return primary when no secondary configured")
	}
}

func TestActiveOutboundComm_ReturnsPrimary_WhenPrimaryIsRecent(t *testing.T) {
	ms := newTestMeshServer(t)
	primaryMock := NewMockSerialPort()
	secondaryMock := NewMockSerialPort()
	ms.serialComm = NewSerialComm(primaryMock)
	ms.secondarySerialComm = NewSerialComm(secondaryMock)
	// Primary received a frame 10 seconds ago — well within the 75s threshold
	ms.frameTimeMu.Lock()
	ms.primaryLastFrameAt = time.Now().Add(-10 * time.Second)
	ms.frameTimeMu.Unlock()

	comm := ms.activeOutboundComm()
	if comm != ms.serialComm {
		t.Error("activeOutboundComm() must return primary when primary is recent")
	}
}

func TestActiveOutboundComm_FailsOverToSecondary_AfterPrimaryTimeout(t *testing.T) {
	ms := newTestMeshServer(t)
	primaryMock := NewMockSerialPort()
	secondaryMock := NewMockSerialPort()
	ms.serialComm = NewSerialComm(primaryMock)
	ms.secondarySerialComm = NewSerialComm(secondaryMock)
	// Primary last heard 76 seconds ago — over the 75s threshold
	ms.frameTimeMu.Lock()
	ms.primaryLastFrameAt = time.Now().Add(-76 * time.Second)
	ms.frameTimeMu.Unlock()

	comm := ms.activeOutboundComm()
	if comm != ms.secondarySerialComm {
		t.Error("activeOutboundComm() must return secondary after primary timeout")
	}
}
