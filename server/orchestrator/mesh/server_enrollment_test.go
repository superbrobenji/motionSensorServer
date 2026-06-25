package mesh

import (
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
	data := mock.GetWrittenData()
	if len(data) < 2 {
		t.Fatalf("no frame written: only %d bytes", len(data))
	}
	length := int(binary.LittleEndian.Uint16(data[:2]))
	if len(data) < 2+length {
		t.Fatalf("frame truncated: need %d bytes after header, have %d", length, len(data)-2)
	}
	var msg MeshMessage
	if err := proto.Unmarshal(data[2:2+length], &msg); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}
	return &msg
}

func TestApproveEnrollment_SendsJoinAckWithPubKey(t *testing.T) {
	ms := newTestMeshServer(t)
	mockPort := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mockPort)

	macStr, wantPubKey := enrollTestNode(t, ms)

	if err := ms.ApproveEnrollment(macStr); err != nil {
		t.Fatalf("ApproveEnrollment returned error: %v", err)
	}

	msg := decodeWrittenFrame(t, mockPort)

	if msg.MessageType != 4 {
		t.Errorf("MessageType = %d, want 4", msg.MessageType)
	}
	if len(msg.PublicKey) != 32 {
		t.Errorf("PublicKey length = %d, want 32", len(msg.PublicKey))
	}
	for i, b := range msg.PublicKey {
		if b != wantPubKey[i] {
			t.Errorf("PublicKey[%d] = %d, want %d", i, b, wantPubKey[i])
			break
		}
	}
}

func TestApproveEnrollment_UnknownMAC_ReturnsError(t *testing.T) {
	ms := newTestMeshServer(t)

	if err := ms.ApproveEnrollment("aabbccddeeff"); err == nil {
		t.Fatal("expected error for unknown MAC, got nil")
	}
}

func TestApproveEnrollment_NilSerialComm_Succeeds(t *testing.T) {
	ms := newTestMeshServer(t)
	macStr, _ := enrollTestNode(t, ms)

	if err := ms.ApproveEnrollment(macStr); err != nil {
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
