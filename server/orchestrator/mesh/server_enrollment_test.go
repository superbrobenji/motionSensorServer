package mesh

import (
	"bytes"
	"encoding/binary"
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
