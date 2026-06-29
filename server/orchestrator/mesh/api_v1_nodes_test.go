package mesh

import (
	"encoding/json"
	"net/http"
	"testing"
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

func TestV1Nodes_Command_UnsupportedAction_Returns501(t *testing.T) {
	api, ms := newV1TestServer(t)
	setupNodeForV1Test(t, ms)
	w := v1Request(t, api, "POST", "/api/v1/nodes/7/command",
		map[string]interface{}{"action": "explode", "params": map[string]interface{}{}})
	if w.Code != http.StatusNotImplemented {
		t.Errorf("got %d, want 501", w.Code)
	}
}
