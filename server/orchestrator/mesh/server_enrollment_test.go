package mesh

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestMessageTypeJoinAck_Value(t *testing.T) {
	if MessageTypeJoinAck != 4 {
		t.Errorf("MessageTypeJoinAck = %d, want 4 (firmware MESH_TYPE_JOIN_ACK)", MessageTypeJoinAck)
	}
}

func enrollTestNode(t *testing.T, ms *MeshServer) (macStr string, pubKey [32]byte) {
	t.Helper()
	mac := [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	for i := range pubKey {
		pubKey[i] = byte(i + 1)
	}
	if err := ms.authRegistry.AddPending(mac, pubKey); err != nil {
		t.Fatalf("AddPending failed: %v", err)
	}
	return "aabbccddeeff", pubKey
}

// enrollTestNodeWithMAC adds a pending enrollment for the given MAC address.
func enrollTestNodeWithMAC(t *testing.T, ms *MeshServer, mac [6]byte) (macStr string, pubKey [32]byte) {
	t.Helper()
	for i := range pubKey {
		pubKey[i] = byte(i + 1)
	}
	if err := ms.authRegistry.AddPending(mac, pubKey); err != nil {
		t.Fatalf("AddPending failed: %v", err)
	}
	return fmt.Sprintf("%02x%02x%02x%02x%02x%02x",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]), pubKey
}

func decodeWrittenFrame(t *testing.T, mock *MockSerialPort) *MeshMessage {
	t.Helper()
	data := mock.GetWrittenDataFrom(mock.writeOffset)
	if len(data) < 2 {
		t.Fatalf("no frame written at offset %d: only %d bytes remaining", mock.writeOffset, len(data))
	}
	length := int(binary.LittleEndian.Uint16(data[:2]))
	if len(data) < 2+length {
		t.Fatalf("frame truncated: need %d bytes after header, have %d", length, len(data)-2)
	}
	var msg MeshMessage
	if err := proto.Unmarshal(data[2:2+length], &msg); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}
	mock.writeOffset += 2 + length
	return &msg
}

func TestApproveEnrollment_SendsJoinAckWithPubKey(t *testing.T) {
	ms := newTestMeshServer(t)
	mockPort := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mockPort)

	macStr, wantPubKey := enrollTestNode(t, ms)

	if err := ms.ApproveEnrollment(macStr, ApprovalParams{}); err != nil {
		t.Fatalf("ApproveEnrollment returned error: %v", err)
	}

	// First frame: JOIN_ACK
	joinAck := decodeWrittenFrame(t, mockPort)

	if joinAck.MessageType != 4 {
		t.Errorf("MessageType = %d, want 4", joinAck.MessageType)
	}
	if len(joinAck.PublicKey) != 32 {
		t.Errorf("PublicKey length = %d, want 32", len(joinAck.PublicKey))
	}
	for i, b := range joinAck.PublicKey {
		if b != wantPubKey[i] {
			t.Errorf("PublicKey[%d] = %d, want %d", i, b, wantPubKey[i])
			break
		}
	}

	// TargetMacAddress must carry the enrolling node's MAC — not OriginMacAddress
	wantMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF} // from enrollTestNode helper
	if !bytes.Equal(joinAck.TargetMacAddress, wantMAC) {
		t.Errorf("TargetMacAddress = %x, want %x", joinAck.TargetMacAddress, wantMAC)
	}
	if len(joinAck.OriginMacAddress) != 0 {
		t.Errorf("OriginMacAddress should be absent, got %x", joinAck.OriginMacAddress)
	}

	// Second frame: OP_NODE_ID_SET
	nodeIdMsg := decodeWrittenFrame(t, mockPort)
	if nodeIdMsg.Data[0] != byte(OpNodeIdSet) {
		t.Errorf("second frame opcode = 0x%02x, want 0x%02x", nodeIdMsg.Data[0], OpNodeIdSet)
	}
}

func TestApproveEnrollment_UnknownMAC_ReturnsError(t *testing.T) {
	ms := newTestMeshServer(t)

	if err := ms.ApproveEnrollment("aabbccddeeff", ApprovalParams{}); err == nil {
		t.Fatal("expected error for unknown MAC, got nil")
	}
}

func TestApproveEnrollment_NilSerialComm_Succeeds(t *testing.T) {
	ms := newTestMeshServer(t)
	macStr, _ := enrollTestNode(t, ms)

	if err := ms.ApproveEnrollment(macStr, ApprovalParams{}); err != nil {
		t.Fatalf("ApproveEnrollment with nil serialComm returned error: %v", err)
	}
}

func TestRejectEnrollment_SendsJoinAckToTargetMac(t *testing.T) {
	ms := newTestMeshServer(t)
	mockPort := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mockPort)

	macStr, _ := enrollTestNode(t, ms)

	if err := ms.RejectEnrollment(macStr); err != nil {
		t.Fatalf("RejectEnrollment returned error: %v", err)
	}

	msg := decodeWrittenFrame(t, mockPort)
	wantMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	if !bytes.Equal(msg.TargetMacAddress, wantMAC) {
		t.Errorf("TargetMacAddress = %x, want %x", msg.TargetMacAddress, wantMAC)
	}
	if len(msg.OriginMacAddress) != 0 {
		t.Errorf("OriginMacAddress should be absent in rejection frame, got %x", msg.OriginMacAddress)
	}
	if len(msg.PublicKey) != 0 {
		t.Errorf("PublicKey should be absent (rejection signal), got %x", msg.PublicKey)
	}
}

func TestRejectEnrollment_SendsJoinAckWithEmptyPubKey(t *testing.T) {
	ms := newTestMeshServer(t)
	mockPort := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mockPort)

	macStr, _ := enrollTestNode(t, ms)

	if err := ms.RejectEnrollment(macStr); err != nil {
		t.Fatalf("RejectEnrollment returned error: %v", err)
	}

	msg := decodeWrittenFrame(t, mockPort)

	if msg.MessageType != 4 {
		t.Errorf("MessageType = %d, want 4", msg.MessageType)
	}
	if len(msg.PublicKey) != 0 {
		t.Errorf("PublicKey should be absent for rejection, got %d bytes", len(msg.PublicKey))
	}
}

func TestApproveEnrollment_SendsNodeIdSet_AfterJoinAck(t *testing.T) {
	ms := newTestMeshServer(t)
	mockPort := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mockPort)

	macStr, _ := enrollTestNode(t, ms)

	params := ApprovalParams{NodeID: 7, Name: "test-node", Zone: "lobby"}
	if err := ms.ApproveEnrollment(macStr, params); err != nil {
		t.Fatalf("ApproveEnrollment returned error: %v", err)
	}

	// First frame: JOIN_ACK
	_ = decodeWrittenFrame(t, mockPort)

	// Second frame: OP_NODE_ID_SET
	idMsg := decodeWrittenFrame(t, mockPort)
	if idMsg.Data[0] != byte(OpNodeIdSet) {
		t.Errorf("second frame opcode = 0x%02x, want 0x%02x", idMsg.Data[0], OpNodeIdSet)
	}
	if idMsg.Data[7] != 7 {
		t.Errorf("nodeId in frame = %d, want 7", idMsg.Data[7])
	}
	// target MAC must match the enrolled node's MAC
	wantMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF} // from enrollTestNode helper
	if !bytes.Equal(idMsg.Data[1:7], wantMAC) {
		t.Errorf("OP_NODE_ID_SET target MAC = %x, want %x", idMsg.Data[1:7], wantMAC)
	}
}

func TestApproveEnrollment_AutoAssignsNodeId_WhenZero(t *testing.T) {
	ms := newTestMeshServer(t)
	macStr, _ := enrollTestNode(t, ms)

	params := ApprovalParams{} // NodeID = 0 → auto-assign
	if err := ms.ApproveEnrollment(macStr, params); err != nil {
		t.Fatalf("ApproveEnrollment returned error: %v", err)
	}

	wantMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF} // from enrollTestNode
	node, ok := ms.GetNodeRegistry().GetNode(wantMAC)
	if !ok {
		t.Fatal("node not registered")
	}
	if node.NodeID == 0 {
		t.Error("NodeID should be auto-assigned (>0)")
	}
}

func TestApproveEnrollment_HotswapInheritsNameZoneAndMarksReplaced(t *testing.T) {
	ms := newTestMeshServer(t)

	// Old node: assigned with identity, adapter type known from health report
	oldMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	ms.nodeRegistry.AssignNode(oldMAC, 7, "entrance-left", "lobby")
	ms.nodeRegistry.UpdateNode(oldMAC, AdapterTypePIR, 3600, 1)

	// New node sends enrollment request
	newMacStr, _ := enrollTestNodeWithMAC(t, ms, [6]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66})

	// Approve with same nodeId, no explicit overrides
	if err := ms.ApproveEnrollment(newMacStr, ApprovalParams{NodeID: 7}); err != nil {
		t.Fatalf("ApproveEnrollment: %v", err)
	}

	// New node should inherit name and zone from old node
	newNode, ok := ms.nodeRegistry.GetNodeByID(7)
	if !ok {
		t.Fatal("GetNodeByID(7) must return the new node after hotswap")
	}
	if newNode.Name != "entrance-left" {
		t.Errorf("Name = %q, want %q (inherited from old node)", newNode.Name, "entrance-left")
	}
	if newNode.Zone != "lobby" {
		t.Errorf("Zone = %q, want %q (inherited from old node)", newNode.Zone, "lobby")
	}

	// Old node must be marked replaced and its NodeID cleared
	oldNode, ok := ms.nodeRegistry.GetNode(oldMAC)
	if !ok {
		t.Fatal("old node must still exist in registry after hotswap")
	}
	if oldNode.Status != "replaced" {
		t.Errorf("old node Status = %q, want %q", oldNode.Status, "replaced")
	}
	if oldNode.NodeID != 0 {
		t.Errorf("old node NodeID = %d, want 0 after replacement", oldNode.NodeID)
	}
}

func TestApproveEnrollment_HotswapSendsConfigSet(t *testing.T) {
	ms := newTestMeshServer(t)
	mockPort := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mockPort)

	// Old node with PIR adapter type
	oldMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	ms.nodeRegistry.AssignNode(oldMAC, 7, "entrance-left", "lobby")
	ms.nodeRegistry.UpdateNode(oldMAC, AdapterTypePIR, 3600, 1)

	newMacStr, _ := enrollTestNodeWithMAC(t, ms, [6]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66})

	if err := ms.ApproveEnrollment(newMacStr, ApprovalParams{NodeID: 7}); err != nil {
		t.Fatalf("ApproveEnrollment: %v", err)
	}

	_ = decodeWrittenFrame(t, mockPort) // JOIN_ACK
	_ = decodeWrittenFrame(t, mockPort) // OP_NODE_ID_SET
	configMsg := decodeWrittenFrame(t, mockPort) // OP_CONFIG_SET

	if len(configMsg.Data) == 0 || configMsg.Data[0] != byte(OpConfigSet) {
		t.Errorf("3rd frame opcode = 0x%02x, want 0x%02x (OP_CONFIG_SET)",
			configMsg.Data[0], OpConfigSet)
	}
	if configMsg.Data[7] != byte(AdapterTypePIR) {
		t.Errorf("OP_CONFIG_SET adapter type = %d, want %d (AdapterTypePIR)",
			configMsg.Data[7], byte(AdapterTypePIR))
	}
}

func TestApproveEnrollment_HotswapExplicitOverrideNotInherited(t *testing.T) {
	ms := newTestMeshServer(t)

	// Old node with known name, zone, type
	oldMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	ms.nodeRegistry.AssignNode(oldMAC, 7, "entrance-left", "lobby")
	ms.nodeRegistry.UpdateNode(oldMAC, AdapterTypePIR, 3600, 1)

	newMacStr, _ := enrollTestNodeWithMAC(t, ms, [6]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66})

	// Explicit overrides provided — should NOT inherit from old node
	if err := ms.ApproveEnrollment(newMacStr, ApprovalParams{
		NodeID:         7,
		Name:           "stage-right",
		Zone:           "stage",
		AdapterTypeStr: "led",
	}); err != nil {
		t.Fatalf("ApproveEnrollment: %v", err)
	}

	newNode, ok := ms.nodeRegistry.GetNodeByID(7)
	if !ok {
		t.Fatal("GetNodeByID(7) must return new node")
	}
	if newNode.Name != "stage-right" {
		t.Errorf("Name = %q, want %q (explicit override)", newNode.Name, "stage-right")
	}
	if newNode.Zone != "stage" {
		t.Errorf("Zone = %q, want %q (explicit override)", newNode.Zone, "stage")
	}
}
