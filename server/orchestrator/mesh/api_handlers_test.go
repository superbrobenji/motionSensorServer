package mesh

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestAPIServer builds a minimal APIServer with no auth and no CORS,
// suitable for testing individual HTTP handlers in isolation.
func newTestAPIServer(t *testing.T) *APIServer {
	t.Helper()
	ms := newTestMeshServer(t)
	return NewAPIServer(ms, "", nil)
}

func TestGetStatus(t *testing.T) {
	api := newTestAPIServer(t)

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success:true")
	}
}

func TestGetNodes_Empty(t *testing.T) {
	api := newTestAPIServer(t)

	req := httptest.NewRequest("GET", "/nodes", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestConfigureNode_InvalidMAC(t *testing.T) {
	api := newTestAPIServer(t)

	body := bytes.NewBufferString(`{"adapterType":0}`)
	req := httptest.NewRequest("POST", "/nodes/invalid-mac/configure", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestConfigureNode_InvalidAdapterType(t *testing.T) {
	api := newTestAPIServer(t)

	body := bytes.NewBufferString(`{"adapterType":99}`)
	req := httptest.NewRequest("POST", "/nodes/aa:bb:cc:dd:ee:ff/configure", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthMiddleware_ProtectsRoutes(t *testing.T) {
	ms := newTestMeshServer(t)
	api := NewAPIServer(ms, "test-key", nil)

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", w.Code)
	}

	req2 := httptest.NewRequest("GET", "/status", nil)
	req2.Header.Set("Authorization", "Bearer test-key")
	w2 := httptest.NewRecorder()
	api.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 with valid auth, got %d", w2.Code)
	}
}

func TestIsValidAdapterType_AllKnownTypes(t *testing.T) {
	valid := []int32{AdapterTypeUnknown, AdapterTypePIR, AdapterTypeWIFI, AdapterTypeLED, AdapterTypeSerial}
	for _, v := range valid {
		if !isValidAdapterType(v) {
			t.Errorf("expected type %d to be valid", v)
		}
	}
}
