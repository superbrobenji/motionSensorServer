package mesh

import (
	"bytes"
	"testing"
	"time"
)

// MockSerialPort implements SerialPort for testing
type MockSerialPort struct {
	readBuffer  *bytes.Buffer
	writeBuffer *bytes.Buffer
	writeOffset int // tracks how many bytes have been consumed by decodeWrittenFrame
}

func NewMockSerialPort() *MockSerialPort {
	return &MockSerialPort{
		readBuffer:  &bytes.Buffer{},
		writeBuffer: &bytes.Buffer{},
	}
}

func (m *MockSerialPort) Read(p []byte) (int, error) {
	return m.readBuffer.Read(p)
}

func (m *MockSerialPort) Write(p []byte) (int, error) {
	return m.writeBuffer.Write(p)
}

func (m *MockSerialPort) Close() error {
	return nil
}

func (m *MockSerialPort) AddReadData(data []byte) {
	m.readBuffer.Write(data)
}

func (m *MockSerialPort) GetWrittenData() []byte {
	return m.writeBuffer.Bytes()
}

// GetWrittenDataFrom returns all written bytes starting at offset.
func (m *MockSerialPort) GetWrittenDataFrom(offset int) []byte {
	all := m.writeBuffer.Bytes()
	if offset >= len(all) {
		return nil
	}
	return all[offset:]
}

func TestMessageBuilder(t *testing.T) {
	builder := NewMessageBuilder()

	t.Run("BuildConfigSetMessage", func(t *testing.T) {
		mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
		msg, err := builder.BuildConfigSetMessage(mac, AdapterTypePIR)
		
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if msg.MessageType != MessageTypeAdapterData {
			t.Errorf("Expected MessageType %d, got %d", MessageTypeAdapterData, msg.MessageType)
		}

		if msg.DataType != AdapterTypeSerial {
			t.Errorf("Expected DataType %d, got %d", AdapterTypeSerial, msg.DataType)
		}

		if msg.Data[0] != OpConfigSet {
			t.Errorf("Expected opcode %02x, got %02x", OpConfigSet, msg.Data[0])
		}

		if !bytes.Equal(msg.Data[1:7], mac) {
			t.Errorf("Expected MAC %x, got %x", mac, msg.Data[1:7])
		}

		if msg.Data[7] != byte(AdapterTypePIR) {
			t.Errorf("Expected adapter type %d, got %d", AdapterTypePIR, msg.Data[7])
		}
	})

	t.Run("BuildHealthRequestMessage", func(t *testing.T) {
		msg := builder.BuildHealthRequestMessage()
		
		if msg.MessageType != MessageTypeAdapterData {
			t.Errorf("Expected MessageType %d, got %d", MessageTypeAdapterData, msg.MessageType)
		}

		if msg.DataType != AdapterTypeSerial {
			t.Errorf("Expected DataType %d, got %d", AdapterTypeSerial, msg.DataType)
		}

		if msg.Data[0] != OpHealthReq {
			t.Errorf("Expected opcode %02x, got %02x", OpHealthReq, msg.Data[0])
		}
	})

	t.Run("ParseHealthReport", func(t *testing.T) {
		// Create a mock health report message
		data := make([]byte, MaxDataLength)
		data[0] = OpHealthReport
		data[1] = byte(AdapterTypePIR)
		// MAC address
		mac := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
		copy(data[2:8], mac)
		// Uptime (little-endian)
		data[8] = 0x10  // 4112 seconds
		data[9] = 0x10
		data[10] = 0x00
		data[11] = 0x00

		msg := &MeshMessage{
			MessageType: MessageTypeAdapterData,
			DataType:    AdapterTypeSerial,
			Data:        data,
			HopCount:    2,
		}

		report, err := builder.ParseHealthReport(msg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if !bytes.Equal(report.MAC, mac) {
			t.Errorf("Expected MAC %x, got %x", mac, report.MAC)
		}

		if report.AdapterType != AdapterTypePIR {
			t.Errorf("Expected adapter type %d, got %d", AdapterTypePIR, report.AdapterType)
		}

		if report.Uptime != 4112 {
			t.Errorf("Expected uptime 4112, got %d", report.Uptime)
		}

		if report.HopCount != 2 {
			t.Errorf("Expected hop count 2, got %d", report.HopCount)
		}
	})

	t.Run("ParseHealthReport_AcceptsNodeHealth_0xB2", func(t *testing.T) {
		data := make([]byte, MaxDataLength)
		data[0] = OpNodeHealth // 0xB2
		data[1] = byte(AdapterTypePIR)
		mac := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
		copy(data[2:8], mac)
		data[8] = 0x1E // 30 seconds
		data[9] = 0x00
		data[10] = 0x00
		data[11] = 0x00

		msg := &MeshMessage{
			MessageType: MessageTypeAdapterData,
			DataType:    AdapterTypeSerial,
			Data:        data,
			HopCount:    3,
		}

		report, err := builder.ParseHealthReport(msg)
		if err != nil {
			t.Fatalf("Expected no error for 0xB2, got %v", err)
		}
		if !bytes.Equal(report.MAC, mac) {
			t.Errorf("MAC mismatch: got %x", report.MAC)
		}
		if report.AdapterType != AdapterTypePIR {
			t.Errorf("AdapterType: got %d, want %d", report.AdapterType, AdapterTypePIR)
		}
		if report.Uptime != 30 {
			t.Errorf("Uptime: got %d, want 30", report.Uptime)
		}
		if report.HopCount != 3 {
			t.Errorf("HopCount: got %d, want 3", report.HopCount)
		}
	})

	t.Run("IsHealthReport_TrueFor_0xB2", func(t *testing.T) {
		data := make([]byte, MaxDataLength)
		data[0] = OpNodeHealth
		msg := &MeshMessage{DataType: AdapterTypeSerial, Data: data}
		if !builder.IsHealthReport(msg) {
			t.Error("IsHealthReport should return true for 0xB2")
		}
	})
}

func TestNodeRegistry(t *testing.T) {
	registry := NewNodeRegistry()

	mac1 := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	mac2 := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}

	t.Run("UpdateAndGetNode", func(t *testing.T) {
		// Update node
		registry.UpdateNode(mac1, AdapterTypePIR, 1000, 1)

		// Get node
		node, exists := registry.GetNode(mac1)
		if !exists {
			t.Fatal("Expected node to exist")
		}

		if !bytes.Equal(node.MAC, mac1) {
			t.Errorf("Expected MAC %x, got %x", mac1, node.MAC)
		}

		if node.AdapterType != AdapterTypePIR {
			t.Errorf("Expected adapter type %d, got %d", AdapterTypePIR, node.AdapterType)
		}

		if node.Uptime != 1000 {
			t.Errorf("Expected uptime 1000, got %d", node.Uptime)
		}
	})

	t.Run("GetAllNodes", func(t *testing.T) {
		// Add second node
		registry.UpdateNode(mac2, AdapterTypeLED, 2000, 2)

		nodes := registry.GetAllNodes()
		if len(nodes) != 2 {
			t.Errorf("Expected 2 nodes, got %d", len(nodes))
		}
	})

	t.Run("NodeCount", func(t *testing.T) {
		count := registry.NodeCount()
		if count != 2 {
			t.Errorf("Expected count 2, got %d", count)
		}
	})

	t.Run("AssignNode_SetsIdentityFields", func(t *testing.T) {
		registry := NewNodeRegistry()
		mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
		registry.AssignNode(mac, 7, "entrance-left", "lobby")
		node, ok := registry.GetNode(mac)
		if !ok {
			t.Fatal("node not found after AssignNode")
		}
		if node.NodeID != 7 {
			t.Errorf("NodeID: got %d, want 7", node.NodeID)
		}
		if node.Name != "entrance-left" {
			t.Errorf("Name: got %q", node.Name)
		}
		if node.Zone != "lobby" {
			t.Errorf("Zone: got %q", node.Zone)
		}
	})

	t.Run("UpdateNode_DoesNotOverwriteAssignedFields", func(t *testing.T) {
		registry := NewNodeRegistry()
		mac := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
		registry.AssignNode(mac, 3, "stage-left", "main")
		registry.UpdateNode(mac, AdapterTypePIR, 1000, 2)
		node, ok := registry.GetNode(mac)
		if !ok {
			t.Fatal("node not found")
		}
		if node.NodeID != 3 {
			t.Errorf("NodeID overwritten: got %d, want 3", node.NodeID)
		}
		if node.Name != "stage-left" {
			t.Errorf("Name overwritten: got %q", node.Name)
		}
	})

	t.Run("NextFreeNodeID_ReturnsOne_WhenEmpty", func(t *testing.T) {
		registry := NewNodeRegistry()
		if id := registry.NextFreeNodeID(); id != 1 {
			t.Errorf("NextFreeNodeID: got %d, want 1", id)
		}
	})

	t.Run("NextFreeNodeID_SkipsUsedIds", func(t *testing.T) {
		registry := NewNodeRegistry()
		registry.AssignNode([]byte{0x01, 0, 0, 0, 0, 0}, 1, "", "")
		registry.AssignNode([]byte{0x02, 0, 0, 0, 0, 0}, 2, "", "")
		if id := registry.NextFreeNodeID(); id != 3 {
			t.Errorf("NextFreeNodeID: got %d, want 3", id)
		}
	})

	t.Run("GetNodeByID_ReturnsNode_WhenExists", func(t *testing.T) {
		registry := NewNodeRegistry()
		mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
		registry.AssignNode(mac, 7, "entrance-left", "lobby")
		node, ok := registry.GetNodeByID(7)
		if !ok {
			t.Fatal("expected node, got nothing")
		}
		if node.NodeID != 7 {
			t.Errorf("NodeID: %d, want 7", node.NodeID)
		}
		if node.Name != "entrance-left" {
			t.Errorf("Name: %q", node.Name)
		}
	})

	t.Run("GetNodeByID_ReturnsFalse_WhenMissing", func(t *testing.T) {
		registry := NewNodeRegistry()
		if _, ok := registry.GetNodeByID(99); ok {
			t.Error("expected false for unknown ID")
		}
	})

	t.Run("GetNodesByZone_ReturnsOnlyZoneNodes", func(t *testing.T) {
		registry := NewNodeRegistry()
		registry.AssignNode([]byte{0x01, 0, 0, 0, 0, 0}, 1, "a", "lobby")
		registry.AssignNode([]byte{0x02, 0, 0, 0, 0, 0}, 2, "b", "lobby")
		registry.AssignNode([]byte{0x03, 0, 0, 0, 0, 0}, 3, "c", "stage")
		nodes := registry.GetNodesByZone("lobby")
		if len(nodes) != 2 {
			t.Errorf("len: %d, want 2", len(nodes))
		}
		for _, n := range nodes {
			if n.Zone != "lobby" {
				t.Errorf("unexpected zone %q", n.Zone)
			}
		}
	})

	t.Run("GetNodesByZone_ReturnsEmpty_WhenNoMatch", func(t *testing.T) {
		registry := NewNodeRegistry()
		registry.AssignNode([]byte{0x01, 0, 0, 0, 0, 0}, 1, "a", "lobby")
		nodes := registry.GetNodesByZone("nowhere")
		if len(nodes) != 0 {
			t.Errorf("len: %d, want 0", len(nodes))
		}
	})
}

func TestGetOnlineNodes_ThresholdBoundary(t *testing.T) {
	registry := NewNodeRegistry()
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	macStr := macToString(mac)

	registry.UpdateNode(mac, AdapterTypePIR, 1000, 1)

	// Backdate LastSeen to 45 seconds ago: within 75s threshold but outside 30s threshold
	registry.mu.Lock()
	registry.nodes[macStr].LastSeen = time.Now().Add(-45 * time.Second)
	registry.mu.Unlock()

	if got := registry.GetOnlineNodes(30 * time.Second); len(got) != 0 {
		t.Errorf("GetOnlineNodes(30s): expected 0 nodes for a 45s-old node, got %d", len(got))
	}
	if got := registry.GetOnlineNodes(75 * time.Second); len(got) != 1 {
		t.Errorf("GetOnlineNodes(75s): expected 1 node for a 45s-old node, got %d", len(got))
	}
}

func TestMarkReplaced_SetsStatusAndClearsNodeID(t *testing.T) {
	nr := NewNodeRegistry()
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	nr.AssignNode(mac, 7, "entrance-left", "lobby")

	nr.MarkReplaced(mac, "11:22:33:44:55:66")

	node, ok := nr.GetNode(mac)
	if !ok {
		t.Fatal("node must still exist in registry after MarkReplaced")
	}
	if node.Status != "replaced" {
		t.Errorf("Status = %q, want %q", node.Status, "replaced")
	}
	if node.ReplacedBy != "11:22:33:44:55:66" {
		t.Errorf("ReplacedBy = %q, want %q", node.ReplacedBy, "11:22:33:44:55:66")
	}
	if node.NodeID != 0 {
		t.Errorf("NodeID = %d, want 0 after replacement", node.NodeID)
	}
}

func TestMarkReplaced_ReplacedNodeNotReturnedByGetNodeByID(t *testing.T) {
	nr := NewNodeRegistry()
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	nr.AssignNode(mac, 7, "entrance-left", "lobby")

	nr.MarkReplaced(mac, "11:22:33:44:55:66")

	_, ok := nr.GetNodeByID(7)
	if ok {
		t.Error("GetNodeByID(7) must not return a replaced node")
	}

	if _, ok := nr.GetNodeByID(0); ok {
		t.Error("GetNodeByID(0) must not return a replaced node")
	}
}

func TestMarkReplaced_PersistsAndLoadsCorrectly(t *testing.T) {
	nr := NewNodeRegistry()
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	nr.AssignNode(mac, 7, "entrance-left", "lobby")
	nr.MarkReplaced(mac, "11:22:33:44:55:66")

	path := t.TempDir() + "/nodes.json"
	if err := nr.Persist(path); err != nil {
		t.Fatalf("Persist: %v", err)
	}

	nr2 := NewNodeRegistry()
	if err := nr2.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	node, ok := nr2.GetNode(mac)
	if !ok {
		t.Fatal("replaced node must survive Persist/Load round-trip")
	}
	if node.Status != "replaced" {
		t.Errorf("Status after load = %q, want %q", node.Status, "replaced")
	}
	if node.ReplacedBy != "11:22:33:44:55:66" {
		t.Errorf("ReplacedBy after load = %q, want %q", node.ReplacedBy, "11:22:33:44:55:66")
	}
}

func TestSerialComm(t *testing.T) {
	mockPort := NewMockSerialPort()
	comm := NewSerialComm(mockPort)

	t.Run("WriteAndReadFrame", func(t *testing.T) {
		// Create test message
		originalMsg := &MeshMessage{
			MessageType: MessageTypeAdapterData,
			DataType:    AdapterTypePIR,
			Data:        []byte{0x01, 0x02, 0x03, 0x04},
		}

		// Write frame
		err := comm.WriteFrame(originalMsg)
		if err != nil {
			t.Fatalf("Expected no error writing frame, got %v", err)
		}

		// Get written data and add it to read buffer for testing
		writtenData := mockPort.GetWrittenData()
		mockPort.AddReadData(writtenData)

		// Read frame back
		readMsg, err := comm.ReadFrame()
		if err != nil {
			t.Fatalf("Expected no error reading frame, got %v", err)
		}

		// Compare messages
		if readMsg.MessageType != originalMsg.MessageType {
			t.Errorf("Expected MessageType %d, got %d", originalMsg.MessageType, readMsg.MessageType)
		}

		if readMsg.DataType != originalMsg.DataType {
			t.Errorf("Expected DataType %d, got %d", originalMsg.DataType, readMsg.DataType)
		}

		if !bytes.Equal(readMsg.Data, originalMsg.Data) {
			t.Errorf("Expected Data %x, got %x", originalMsg.Data, readMsg.Data)
		}
	})
}

func TestStringToMAC(t *testing.T) {
	testCases := []struct {
		input    string
		expected []byte
		hasError bool
	}{
		{"aa:bb:cc:dd:ee:ff", []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}, false},
		{"11:22:33:44:55:66", []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}, false},
		{"aabbccddeeff", []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}, false},
		{"invalid", nil, true},
		{"aa:bb:cc:dd:ee", nil, true}, // Too short
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := StringToMAC(tc.input)
			
			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for input %s", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %s: %v", tc.input, err)
				}
				
				if !bytes.Equal(result, tc.expected) {
					t.Errorf("Expected %x, got %x for input %s", tc.expected, result, tc.input)
				}
			}
		})
	}
}

func TestGetAdapterTypeName(t *testing.T) {
	testCases := []struct {
		adapterType int32
		expected    string
	}{
		{AdapterTypeUnknown, "Unknown"},
		{AdapterTypePIR, "PIR"},
		{AdapterTypeWIFI, "WiFi"},
		{AdapterTypeLED, "LED"},
		{AdapterTypeSerial, "Serial"},
		{99, "Unknown(99)"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := GetAdapterTypeName(tc.adapterType)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestNodeRegistryPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/nodes.json"

	registry := NewNodeRegistry()
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	registry.UpdateNode(mac, AdapterTypePIR, 1000, 1)

	if err := registry.Persist(path); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	registry2 := NewNodeRegistry()
	if err := registry2.Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	node, exists := registry2.GetNode(mac)
	if !exists {
		t.Fatal("expected node to exist after load")
	}
	if node.AdapterType != AdapterTypePIR {
		t.Errorf("expected AdapterTypePIR, got %d", node.AdapterType)
	}
	if node.Uptime != 1000 {
		t.Errorf("expected uptime 1000, got %d", node.Uptime)
	}
}

func TestNodeRegistryLoad_MissingFile(t *testing.T) {
	registry := NewNodeRegistry()
	err := registry.Load("/tmp/does-not-exist-xyzzy.json")
	if err != nil {
		t.Errorf("expected no error for missing file, got %v", err)
	}
}

func TestZoneRegistry_AddAndGet(t *testing.T) {
	zr := NewZoneRegistry()
	zone, err := zr.Add("Main Hall")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if zone.ID != "main-hall" {
		t.Errorf("ID: %q, want %q", zone.ID, "main-hall")
	}
	if zone.Name != "Main Hall" {
		t.Errorf("Name: %q", zone.Name)
	}
}

func TestZoneRegistry_Add_DuplicateReturnsError(t *testing.T) {
	zr := NewZoneRegistry()
	if _, err := zr.Add("lobby"); err != nil {
		t.Fatal(err)
	}
	if _, err := zr.Add("lobby"); err == nil {
		t.Error("expected error for duplicate")
	}
	// "Lobby" and "lobby" both map to "lobby" — also duplicate
	if _, err := zr.Add("Lobby"); err == nil {
		t.Error("expected error for case-variant duplicate")
	}
}

func TestZoneRegistry_List(t *testing.T) {
	zr := NewZoneRegistry()
	if _, err := zr.Add("lobby"); err != nil {
		t.Fatal(err)
	}
	if _, err := zr.Add("stage"); err != nil {
		t.Fatal(err)
	}
	zones := zr.List()
	if len(zones) != 2 {
		t.Errorf("len: %d, want 2", len(zones))
	}
}

func TestZoneRegistry_Update(t *testing.T) {
	zr := NewZoneRegistry()
	if _, err := zr.Add("lobby"); err != nil {
		t.Fatal(err)
	}
	z, ok := zr.Update("lobby", "Lobby Area")
	if !ok {
		t.Fatal("Update returned false")
	}
	if z.Name != "Lobby Area" {
		t.Errorf("Name: %q", z.Name)
	}
	if z.ID != "lobby" {
		t.Errorf("ID changed: %q", z.ID)
	}
}

func TestZoneRegistry_Delete(t *testing.T) {
	zr := NewZoneRegistry()
	if _, err := zr.Add("lobby"); err != nil {
		t.Fatal(err)
	}
	if !zr.Delete("lobby") {
		t.Error("Delete returned false")
	}
	if _, ok := zr.Get("lobby"); ok {
		t.Error("zone still present after delete")
	}
	if zr.Delete("lobby") {
		t.Error("second delete should return false")
	}
}

func TestZoneRegistry_PersistAndLoad(t *testing.T) {
	path := t.TempDir() + "/zones.json"
	zr := NewZoneRegistry()
	if _, err := zr.Add("lobby"); err != nil {
		t.Fatal(err)
	}
	if _, err := zr.Add("stage"); err != nil {
		t.Fatal(err)
	}
	if err := zr.Persist(path); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	zr2 := NewZoneRegistry()
	if err := zr2.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	zones := zr2.List()
	if len(zones) != 2 {
		t.Errorf("len after load: %d, want 2", len(zones))
	}
}

func TestZoneRegistry_Load_MissingFile(t *testing.T) {
	zr := NewZoneRegistry()
	err := zr.Load("/tmp/does-not-exist-zone-registry-test.json")
	if err != nil {
		t.Errorf("Load on missing file should be no-op, got error: %v", err)
	}
	if len(zr.List()) != 0 {
		t.Error("registry should be empty after loading missing file")
	}
}

func TestAdapterTypeTranslation(t *testing.T) {
	cases := []struct {
		t int32
		s string
	}{
		{AdapterTypePIR, "pir"},
		{AdapterTypeLED, "led"},
		{AdapterTypeSerial, "serial"},
		{AdapterTypeUnknown, "unknown"},
		{999, "unknown"},
	}
	for _, c := range cases {
		if got := adapterTypeToString(c.t); got != c.s {
			t.Errorf("adapterTypeToString(%d) = %q, want %q", c.t, got, c.s)
		}
	}
	if v, ok := adapterTypeFromString("pir"); !ok || v != AdapterTypePIR {
		t.Errorf("adapterTypeFromString(pir): got %d,%v", v, ok)
	}
	if v, ok := adapterTypeFromString("led"); !ok || v != AdapterTypeLED {
		t.Errorf("adapterTypeFromString(led): got %d,%v", v, ok)
	}
	if _, ok := adapterTypeFromString("serial"); ok {
		t.Error("serial should not be writable via type string")
	}
	if _, ok := adapterTypeFromString("unknown"); ok {
		t.Error("unknown should not be writable")
	}
}

func TestHandlePIRData_KafkaWriteError(t *testing.T) {
	mockStore := NewMockEventStore()
	registry := NewNodeRegistry()
	builder := NewMessageBuilder()

	server := &MeshServer{
		nodeRegistry:   registry,
		messageBuilder: builder,
		eventStore:     mockStore,
	}

	msg := &MeshMessage{
		OriginMacAddress: []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66},
		HopCount:         1,
	}

	err := server.handlePIRData(msg)
	if err != nil {
		t.Errorf("handlePIRData should not return error for valid message, got %v", err)
	}

	if len(mockStore.GetMessages()) != 1 {
		t.Errorf("expected 1 Kafka message written, got %d", len(mockStore.GetMessages()))
	}
}
