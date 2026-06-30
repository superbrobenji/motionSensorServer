package mesh

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func setupNodeForV1Test(t *testing.T, ms *MeshServer) {
	t.Helper()
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	ms.nodeRegistry.AssignNode(mac, 7, "entrance-left", "lobby")
	ms.nodeRegistry.UpdateNode(mac, AdapterTypePIR, 3600, 2)
}

func TestV1Nodes_GetAll_ReturnsAssignedNodes(t *testing.T) {
	api, ms := newV1TestServer(t)
	setupNodeForV1Test(t, ms)
	w := v1Request(t, api, "GET", "/api/v1/nodes", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
}

func TestV1Nodes_GetAll_ExcludesUnassignedNodes(t *testing.T) {
	api, ms := newV1TestServer(t)
	// Register node with UpdateNode only — no NodeID assigned
	mac := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	ms.nodeRegistry.UpdateNode(mac, AdapterTypePIR, 100, 1)
	w := v1Request(t, api, "GET", "/api/v1/nodes", nil)
	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, _ := json.Marshal(resp.Data)
	var nodes []NodeV1
	if err := json.Unmarshal(data, &nodes); err != nil {
		t.Fatalf("unmarshal nodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("got %d nodes, want 0 (unassigned excluded)", len(nodes))
	}
}

func TestV1Nodes_GetByID_ReturnsNode(t *testing.T) {
	api, ms := newV1TestServer(t)
	setupNodeForV1Test(t, ms)
	w := v1Request(t, api, "GET", "/api/v1/nodes/7", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestV1Nodes_GetByID_Returns404_WhenMissing(t *testing.T) {
	api, _ := newV1TestServer(t)
	w := v1Request(t, api, "GET", "/api/v1/nodes/99", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestV1Nodes_Update_ChangesNameAndZone(t *testing.T) {
	api, ms := newV1TestServer(t)
	setupNodeForV1Test(t, ms)
	w := v1Request(t, api, "PATCH", "/api/v1/nodes/7",
		map[string]string{"name": "stage-right", "zone": "stage"})
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	node, _ := ms.nodeRegistry.GetNodeByID(7)
	if node.Name != "stage-right" {
		t.Errorf("Name: %q", node.Name)
	}
	if node.Zone != "stage" {
		t.Errorf("Zone: %q", node.Zone)
	}
}

func TestV1Nodes_Update_UnknownType_Returns400(t *testing.T) {
	api, ms := newV1TestServer(t)
	setupNodeForV1Test(t, ms)
	w := v1Request(t, api, "PATCH", "/api/v1/nodes/7", map[string]string{"type": "toaster"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestV1Nodes_Delete_RemovesNode(t *testing.T) {
	api, ms := newV1TestServer(t)
	setupNodeForV1Test(t, ms)
	w := v1Request(t, api, "DELETE", "/api/v1/nodes/7", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d", w.Code)
	}
	if _, ok := ms.nodeRegistry.GetNodeByID(7); ok {
		t.Error("node still exists after delete")
	}
}

func TestV1Nodes_Command_UnknownAction_Returns400(t *testing.T) {
	api, ms := newV1TestServer(t)
	// Use an LED node so the adapter-type check passes and we reach the action switch.
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	ms.nodeRegistry.AssignNode(mac, 7, "entrance-left", "lobby")
	ms.nodeRegistry.UpdateNode(mac, AdapterTypeLED, 3600, 2)
	w := v1Request(t, api, "POST", "/api/v1/nodes/7/command",
		map[string]interface{}{"action": "explode", "params": map[string]interface{}{}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestV1Nodes_Hotswap_OldNodeExcludedNewNodePresent(t *testing.T) {
	api, ms := newV1TestServer(t)

	// Old node enrolled and assigned
	oldMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	ms.nodeRegistry.AssignNode(oldMAC, 7, "entrance-left", "lobby")
	ms.nodeRegistry.UpdateNode(oldMAC, AdapterTypePIR, 3600, 1)

	// New node sends enrollment
	newMAC := [6]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	var newPubKey [32]byte
	for i := range newPubKey {
		newPubKey[i] = byte(i + 1)
	}
	if err := ms.authRegistry.AddPending(newMAC, newPubKey); err != nil {
		t.Fatalf("AddPending: %v", err)
	}

	// Approve hotswap via API
	w := v1Request(t, api, "POST", "/api/v1/enrollments/11:22:33:44:55:66/approve",
		map[string]interface{}{"nodeId": 7})
	if w.Code != http.StatusOK {
		t.Fatalf("approve returned %d, want 200", w.Code)
	}

	// GET /api/v1/nodes — must return exactly one node with id=7
	w = v1Request(t, api, "GET", "/api/v1/nodes", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/nodes returned %d", w.Code)
	}
	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := json.Marshal(resp.Data)
	var nodes []NodeV1
	if err := json.Unmarshal(data, &nodes); err != nil {
		t.Fatalf("unmarshal nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1 (replaced node must be excluded)", len(nodes))
	}
	if nodes[0].ID != 7 {
		t.Errorf("node ID = %d, want 7", nodes[0].ID)
	}
	if nodes[0].Name != "entrance-left" {
		t.Errorf("Name = %q, want %q (inherited)", nodes[0].Name, "entrance-left")
	}
}

func TestV1NodeCommand_ReturnsCommandId(t *testing.T) {
	ms := newTestMeshServer(t)
	mockPort := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mockPort)
	mac := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	ms.nodeRegistry.AssignNode(mac, 1, "led-node", "stage")
	ms.nodeRegistry.UpdateNode(mac, AdapterTypeLED, 0, 1)

	api := NewAPIServer(ms, "", "", nil)
	req := httptest.NewRequest("POST", "/api/v1/nodes/1/command", strings.NewReader(`{"action":"led_off"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	api.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			CommandID string `json:"commandId"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.CommandID == "" {
		t.Error("commandId is empty, want a UUID")
	}
	if _, err := uuid.Parse(resp.Data.CommandID); err != nil {
		t.Errorf("commandId %q is not a valid UUID: %v", resp.Data.CommandID, err)
	}
}

func TestV1GetCommandStatus_PendingAndAcked(t *testing.T) {
	ms := newTestMeshServer(t)
	cmdID := "test-command-id-1234"
	ms.commandStore.Add(&PendingCommand{
		ID:     cmdID,
		NodeID: 2,
		Action: "led_off",
		SentAt: time.Now(),
		Status: CommandStatusPending,
	})

	api := NewAPIServer(ms, "", "", nil)
	req := httptest.NewRequest("GET", "/api/v1/nodes/2/command/"+cmdID, nil)
	rr := httptest.NewRecorder()
	api.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Status != "pending" {
		t.Errorf("status = %q, want pending", resp.Data.Status)
	}
}

func TestV1GetCommandStatus_NotFound(t *testing.T) {
	ms := newTestMeshServer(t)
	api := NewAPIServer(ms, "", "", nil)
	req := httptest.NewRequest("GET", "/api/v1/nodes/1/command/no-such-id", nil)
	rr := httptest.NewRecorder()
	api.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}
