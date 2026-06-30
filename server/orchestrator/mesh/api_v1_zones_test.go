package mesh

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newV1TestServer(t *testing.T) (*APIServer, *MeshServer) {
	t.Helper()
	ms := newTestMeshServer(t)
	api := NewAPIServer(ms, "test-key", "", nil)
	return api, ms
}

func v1Request(t *testing.T, api *APIServer, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test-key")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	return w
}

func TestV1Zones_CreateAndList(t *testing.T) {
	api, _ := newV1TestServer(t)

	w := v1Request(t, api, "POST", "/api/v1/zones", map[string]string{"name": "lobby"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d, want 201", w.Code)
	}

	w = v1Request(t, api, "GET", "/api/v1/zones", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success:true")
	}
}

func TestV1Zones_CreateDuplicate_Returns409(t *testing.T) {
	api, _ := newV1TestServer(t)
	v1Request(t, api, "POST", "/api/v1/zones", map[string]string{"name": "lobby"})
	w := v1Request(t, api, "POST", "/api/v1/zones", map[string]string{"name": "lobby"})
	if w.Code != http.StatusConflict {
		t.Errorf("got %d, want 409", w.Code)
	}
}

func TestV1Zones_Update(t *testing.T) {
	api, _ := newV1TestServer(t)
	v1Request(t, api, "POST", "/api/v1/zones", map[string]string{"name": "lobby"})
	w := v1Request(t, api, "PATCH", "/api/v1/zones/lobby", map[string]string{"name": "Lobby Area"})
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d", w.Code)
	}
}

func TestV1Zones_Delete(t *testing.T) {
	api, _ := newV1TestServer(t)
	v1Request(t, api, "POST", "/api/v1/zones", map[string]string{"name": "lobby"})
	w := v1Request(t, api, "DELETE", "/api/v1/zones/lobby", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d", w.Code)
	}
	w = v1Request(t, api, "DELETE", "/api/v1/zones/lobby", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("second delete: %d, want 404", w.Code)
	}
}

func TestV1Zones_Command_UnknownZone_Returns404(t *testing.T) {
	api, _ := newV1TestServer(t)
	w := v1Request(t, api, "POST", "/api/v1/zones/nowhere/command",
		map[string]interface{}{"action": "trigger", "params": map[string]interface{}{}})
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestV1Zones_Command_UnknownAction_Returns400(t *testing.T) {
	api, _ := newV1TestServer(t)
	v1Request(t, api, "POST", "/api/v1/zones", map[string]string{"name": "lobby"})
	w := v1Request(t, api, "POST", "/api/v1/zones/lobby/command",
		map[string]interface{}{"action": "explode", "params": map[string]interface{}{}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}
