package mesh

import (
	"testing"
)

func TestMessageTypeJoinAck_Value(t *testing.T) {
	if MessageTypeJoinAck != 4 {
		t.Errorf("MessageTypeJoinAck = %d, want 4 (firmware MESH_TYPE_JOIN_ACK)", MessageTypeJoinAck)
	}
}
