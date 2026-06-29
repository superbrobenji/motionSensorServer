package mesh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testFlusher wraps httptest.ResponseRecorder and implements http.Flusher.
type testFlusher struct {
	*httptest.ResponseRecorder
}

func (tf *testFlusher) Flush() {}

func TestV1Events_StreamsEventToClient(t *testing.T) {
	api, ms := newV1TestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/api/v1/events", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer test-key")
	tf := &testFlusher{httptest.NewRecorder()}

	done := make(chan struct{})
	go func() {
		defer close(done)
		api.v1Events(tf, req)
	}()

	// Give handler time to subscribe
	time.Sleep(10 * time.Millisecond)

	raw, _ := json.Marshal(map[string]interface{}{"nodeId": 7})
	ms.GetEventBroker().Publish(Event{Type: EventMotion, Data: raw, Timestamp: time.Now()})

	// Give handler time to write
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := tf.Body.String()
	if !strings.Contains(body, "event: motion") {
		t.Errorf("body missing event line; got: %q", body)
	}
	if !strings.Contains(body, "data: ") {
		t.Errorf("body missing data line; got: %q", body)
	}
}

// nonFlusherWriter is a minimal http.ResponseWriter that intentionally
// does NOT implement http.Flusher, to test the streaming-not-supported path.
type nonFlusherWriter struct {
	header http.Header
	code   int
	body   []byte
}

func (n *nonFlusherWriter) Header() http.Header {
	if n.header == nil {
		n.header = make(http.Header)
	}
	return n.header
}

func (n *nonFlusherWriter) WriteHeader(code int) { n.code = code }
func (n *nonFlusherWriter) Write(b []byte) (int, error) {
	n.body = append(n.body, b...)
	return len(b), nil
}

func TestV1Events_NonFlusher_Returns500(t *testing.T) {
	api, _ := newV1TestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := &nonFlusherWriter{}
	api.v1Events(w, req)
	if w.code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", w.code)
	}
}

func TestV1Status_ReturnsStructuredStatus(t *testing.T) {
	api, ms := newV1TestServer(t)
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	ms.nodeRegistry.AssignNode(mac, 1, "test-node", "lobby")
	ms.nodeRegistry.UpdateNode(mac, AdapterTypePIR, 100, 1)

	w := v1Request(t, api, "GET", "/api/v1/status", nil)
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

func TestV1Enrollments_GetPending_ReturnsOK(t *testing.T) {
	api, _ := newV1TestServer(t)
	w := v1Request(t, api, "GET", "/api/v1/enrollments/pending", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
}

func TestV1Enrollments_GetAll_ReturnsOK(t *testing.T) {
	api, _ := newV1TestServer(t)
	w := v1Request(t, api, "GET", "/api/v1/enrollments", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
}
