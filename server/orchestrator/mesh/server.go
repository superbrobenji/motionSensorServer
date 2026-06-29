package mesh

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	EventStore "github.com/superbrobenji/motionServer/eventStore"
	"github.com/superbrobenji/motionServer/nodeauth"
	"go.bug.st/serial"
)

// TX power preset constants
const (
	OpTxPowerSet      = 0xA1
	TxPowerShortRange = 0 // 2dBm  — same room
	TxPowerIndoor     = 1 // 14dBm — through walls
	TxPowerOutdoor    = 2 // 20dBm — outdoor, max range (default)
)

var txPowerPresetNames = map[uint8]string{
	TxPowerShortRange: "short_range",
	TxPowerIndoor:     "indoor",
	TxPowerOutdoor:    "outdoor",
}

// MeshServer manages the mesh network communication
type MeshServer struct {
	serialComm     *SerialComm
	nodeRegistry   *NodeRegistry
	messageBuilder *MessageBuilder
	eventStore     EventStore.EventStoreInterface

	// Auth
	authRegistry *nodeauth.Registry
	replayCache  *nodeauth.ReplayCache
	authPath     string        // Path to persist registry JSON
	stopPersist  chan struct{}

	// Node registry persistence
	nodeRegistryPath string
	stopNodePersist  chan struct{}

	// Configuration
	serialPort    string
	baudRate      int
	healthTimeout time.Duration

	// TX power
	currentTxPreset uint8

	// EventBroker and online state tracking
	eventBroker     *EventBroker
	nodeOnlineState map[string]bool // keyed by MACString; true = was online last check

	// Zone registry
	zoneRegistry     *ZoneRegistry
	zoneRegistryPath string

	// Runtime state
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
	running bool
}

// MeshServerConfig holds configuration for the mesh server
type MeshServerConfig struct {
	SerialPort       string
	BaudRate         int
	HealthTimeout    time.Duration
	EventStore       EventStore.EventStoreInterface
	AuthRegistryPath string // e.g. "data/nodeauth.json"
	NodeRegistryPath string // e.g. "data/nodes.json"
}

// NewMeshServer creates a new mesh server
func NewMeshServer(config MeshServerConfig) *MeshServer {
	ctx, cancel := context.WithCancel(context.Background())

	registry := nodeauth.NewRegistry()
	if config.AuthRegistryPath != "" {
		if err := registry.Load(config.AuthRegistryPath); err != nil {
			slog.Warn("Failed to load auth registry", "error", err)
		}
	}

	nodeRegistry := NewNodeRegistry()
	if config.NodeRegistryPath != "" {
		if err := nodeRegistry.Load(config.NodeRegistryPath); err != nil {
			slog.Warn("Failed to load node registry", "error", err)
		}
	}

	return &MeshServer{
		nodeRegistry:     nodeRegistry,
		messageBuilder:   NewMessageBuilder(),
		eventStore:       config.EventStore,
		authRegistry:     registry,
		replayCache:      nodeauth.NewReplayCache(64),
		authPath:         config.AuthRegistryPath,
		stopPersist:      make(chan struct{}),
		nodeRegistryPath: config.NodeRegistryPath,
		stopNodePersist:  make(chan struct{}),
		serialPort:       config.SerialPort,
		baudRate:         config.BaudRate,
		healthTimeout:    config.HealthTimeout,
		eventBroker:      NewEventBroker(),
		nodeOnlineState:  make(map[string]bool),
		zoneRegistry:     NewZoneRegistry(),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start starts the mesh server
func (ms *MeshServer) Start() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.running {
		return fmt.Errorf("mesh server is already running")
	}

	// Open serial port
	mode := &serial.Mode{
		BaudRate: ms.baudRate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(ms.serialPort, mode)
	if err != nil {
		return fmt.Errorf("failed to open serial port %s: %w", ms.serialPort, err)
	}

	ms.serialComm = NewSerialComm(port)
	ms.running = true
	SetSerialConnected(true)

	// Start message processing goroutine
	ms.wg.Add(1)
	go ms.messageProcessor()

	// Start auth registry persistence loop
	if ms.authPath != "" {
		ms.wg.Add(1)
		go func() {
			defer ms.wg.Done()
			ms.authRegistry.PersistLoop(ms.authPath, 30*time.Second, ms.stopPersist)
		}()
	}

	// Start node registry persistence loop
	if ms.nodeRegistryPath != "" {
		ms.wg.Add(1)
		go func() {
			defer ms.wg.Done()
			ms.nodeRegistry.PersistLoop(ms.nodeRegistryPath, 60*time.Second, ms.stopNodePersist)
		}()
	}

	// Start offline detector goroutine
	ms.wg.Add(1)
	go func() {
		defer ms.wg.Done()
		ms.offlineDetectorLoop()
	}()

	slog.Info("Mesh server started", "port", ms.serialPort, "baud", ms.baudRate)
	return nil
}

// Stop stops the mesh server
func (ms *MeshServer) Stop() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.running {
		return fmt.Errorf("mesh server is not running")
	}

	ms.cancel()
	ms.running = false

	// Stop persistence loops (final save happens inside each PersistLoop)
	close(ms.stopPersist)
	if ms.nodeRegistryPath != "" {
		close(ms.stopNodePersist)
	}

	if ms.zoneRegistryPath != "" {
		_ = ms.zoneRegistry.Persist(ms.zoneRegistryPath)
	}

	if ms.serialComm != nil {
		ms.serialComm.Close()
		SetSerialConnected(false)
	}

	ms.wg.Wait()
	slog.Info("Mesh server stopped")
	return nil
}

// messageProcessor processes incoming messages from the serial port
func (ms *MeshServer) messageProcessor() {
	defer ms.wg.Done()

	consecutiveErrors := 0
	maxConsecutiveErrors := 5

	for {
		select {
		case <-ms.ctx.Done():
			return
		default:
			msg, err := ms.serialComm.ReadFrame()
			if err != nil {
				consecutiveErrors++
				if consecutiveErrors <= maxConsecutiveErrors {
					slog.Warn("Serial frame read error", "count", consecutiveErrors, "error", err)
				} else if consecutiveErrors == maxConsecutiveErrors+1 {
					slog.Error("Serial read suppressed — too many consecutive errors", "count", consecutiveErrors)
					SetSerialConnected(false)
				}

				// After many consecutive errors, try to flush the buffer
				if consecutiveErrors == 10 {
					slog.Warn("Attempting buffer flush after consecutive errors", "count", consecutiveErrors)
					if flushErr := ms.serialComm.FlushBuffer(); flushErr != nil {
						slog.Warn("Buffer flush failed", "error", flushErr)
					}
				}

				// Brief pause to prevent tight error loop
				select {
				case <-ms.ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
				}
				continue
			}

			// Reset error counter on successful read
			if consecutiveErrors > 0 {
				if consecutiveErrors > maxConsecutiveErrors {
					slog.Info("Frame reading recovered", "consecutiveErrors", consecutiveErrors)
				}
				consecutiveErrors = 0
			}

			slog.Debug("Message received", "type", msg.MessageType, "dataType", msg.DataType, "origin", macToString(msg.OriginMacAddress))

			if err := ms.handleMessage(msg); err != nil {
				slog.Error("Message handling failed", "error", err)
			}
		}
	}
}

// handleMessage processes a received mesh message
func (ms *MeshServer) handleMessage(msg *MeshMessage) error {
	// Proto version check — version 0 means legacy (pre-security) node; allow it.
	// Any protoVersion > 0 that is not 2 is an unknown future version; drop it.
	if msg.ProtoVersion > 0 && msg.ProtoVersion != 2 {
		slog.Warn("Unsupported proto version — dropping", "version", msg.ProtoVersion, "origin", fmt.Sprintf("%x", msg.OriginMacAddress))
		return nil
	}

	// Replay check (only for proto v2 messages with epoch/seq set)
	if msg.ProtoVersion == 2 && msg.EpochNum > 0 {
		if len(msg.OriginMacAddress) != 6 {
			slog.Warn("Dropping message: invalid OriginMacAddress length", "len", len(msg.OriginMacAddress))
			return nil
		}
		var mac [6]byte
		copy(mac[:], msg.OriginMacAddress)
		if ms.replayCache.IsDuplicate(mac, msg.EpochNum, msg.SeqNum) {
			slog.Warn("Replayed message dropped", "origin", fmt.Sprintf("%x", mac), "epoch", msg.EpochNum, "seq", msg.SeqNum)
			return nil
		}
	}

	// Log the message to Kafka
	if err := ms.logMessageToKafka(msg, "incoming"); err != nil {
		slog.Warn("Failed to log incoming message to Kafka", "error", err)
	}

	switch msg.MessageType {
	case MessageTypeAdapterData:
		return ms.handleAdapterData(msg)
	case MessageTypeMasterBeacon:
		return ms.handleMasterBeacon(msg)
	case MessageTypeEnrollment:
		return ms.handleEnrollmentRequest(msg)
	case MessageTypeJoinAck:
		// JOIN_ACK originates from master→node; server shouldn't receive it normally.
		// Log and ignore.
		slog.Warn("Unexpected JOIN_ACK received — ignoring", "origin", fmt.Sprintf("%x", msg.OriginMacAddress))
		return nil
	default:
		slog.Warn("Unknown message type", "type", msg.MessageType)
	}

	return nil
}

// handleAdapterData processes adapter data messages
func (ms *MeshServer) handleAdapterData(msg *MeshMessage) error {
	switch msg.DataType {
	case AdapterTypeSerial:
		return ms.handleSerialData(msg)
	case AdapterTypePIR:
		return ms.handlePIRData(msg)
	default:
		slog.Debug("Received adapter data", "type", GetAdapterTypeName(msg.DataType), "origin", macToString(msg.OriginMacAddress), "data", fmt.Sprintf("%x", msg.Data))
	}

	return nil
}

// handleSerialData processes serial control messages
func (ms *MeshServer) handleSerialData(msg *MeshMessage) error {
	if len(msg.Data) == 0 {
		return fmt.Errorf("empty serial data")
	}

	opcode := msg.Data[0]
	switch opcode {
	case OpHealthReport, OpNodeHealth:
		return ms.handleHealthReport(msg)
	default:
		slog.Warn("Unknown serial opcode", "opcode", fmt.Sprintf("0x%02x", opcode))
	}

	return nil
}

// handleHealthReport processes health report messages
func (ms *MeshServer) handleHealthReport(msg *MeshMessage) error {
	healthReport, err := ms.messageBuilder.ParseHealthReport(msg)
	if err != nil {
		return fmt.Errorf("failed to parse health report: %w", err)
	}

	// Update node registry
	ms.nodeRegistry.UpdateNode(
		healthReport.MAC,
		healthReport.AdapterType,
		healthReport.Uptime,
		healthReport.HopCount,
	)

	if node, ok := ms.nodeRegistry.GetNode(healthReport.MAC); ok {
		ms.publishHealthEvent(node)
	}

	slog.Info("Health report", "mac", macToString(healthReport.MAC), "adapterType", GetAdapterTypeName(healthReport.AdapterType), "uptime", healthReport.Uptime, "hops", healthReport.HopCount)

	return nil
}

// handleEnrollmentRequest processes an enrollment request from a new node.
// The MAC and public key are carried in the MeshMessage fields directly.
func (ms *MeshServer) handleEnrollmentRequest(msg *MeshMessage) error {
	if len(msg.OriginMacAddress) < 6 {
		return fmt.Errorf("enrollment request missing origin MAC")
	}
	if len(msg.PublicKey) != 32 {
		return fmt.Errorf("enrollment request has invalid public key length: %d", len(msg.PublicKey))
	}
	var mac [6]byte
	copy(mac[:], msg.OriginMacAddress[:6])
	var pubKey [32]byte
	copy(pubKey[:], msg.PublicKey[:32])

	macStr := fmt.Sprintf("%x", mac)
	slog.Info("Enrollment request received", "mac", macStr, "pubkeyPrefix", fmt.Sprintf("%x", pubKey[:4]))

	if err := ms.authRegistry.AddPending(mac, pubKey); err != nil {
		slog.Warn("Failed to add pending enrollment", "mac", macStr, "error", err)
		return err
	}

	// Persist immediately so the admin sees it even if the server restarts
	if ms.authPath != "" {
		if err := ms.authRegistry.Persist(ms.authPath); err != nil {
			slog.Warn("Failed to persist after enrollment request", "error", err)
		}
	}

	// Publish to Kafka for dashboard notification
	event := map[string]interface{}{
		"type":      "enrollment_request",
		"mac":       macStr,
		"publicKey": fmt.Sprintf("%x", pubKey),
		"timestamp": time.Now().Unix(),
	}
	if ms.eventStore != nil {
		j, _ := json.Marshal(event)
		err := ms.eventStore.WriteMessage(string(j), "mesh-enrollment")
		if err != nil {
			slog.Warn("Failed to write enrollment event to Kafka", "error", err)
		}
		RecordKafkaWrite("mesh-enrollment", err)
	}
	return nil
}

// handlePIRData processes PIR sensor data
func (ms *MeshServer) handlePIRData(msg *MeshMessage) error {
	slog.Info("PIR motion detected", "mac", macToString(msg.OriginMacAddress), "hops", msg.HopCount)

	pirEvent := map[string]interface{}{
		"type":      "pir_motion",
		"mac":       macToString(msg.OriginMacAddress),
		"timestamp": time.Now().Unix(),
		"hopCount":  msg.HopCount,
		"data":      msg.Data,
	}

	eventJSON, err := json.Marshal(pirEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal PIR event: %w", err)
	}

	if ms.eventStore == nil {
		return nil
	}

	writeErr := ms.eventStore.WriteMessage(string(eventJSON), "motion-trigger")
	RecordKafkaWrite("motion-trigger", writeErr)
	if writeErr != nil {
		slog.Warn("Failed to write PIR event to Kafka", "error", writeErr)
	}

	if node, ok := ms.nodeRegistry.GetNode(msg.OriginMacAddress); ok {
		ms.publishMotionEvent(node)
	}

	return nil
}

// handleMasterBeacon processes master beacon messages
func (ms *MeshServer) handleMasterBeacon(msg *MeshMessage) error {
	slog.Debug("Master beacon received", "origin", macToString(msg.OriginMacAddress))
	return nil
}

// ApprovalParams carries optional identity fields for a node being approved.
// NodeID 0 means auto-assign the lowest free ID.
type ApprovalParams struct {
	NodeID         uint8
	Name           string
	Zone           string
	AdapterTypeStr string // "pir", "led", etc. — for SSE enrolled event; empty = "unknown"
}

// ApproveEnrollment approves a pending node enrollment and sends a JOIN_ACK
// frame over serial with the node's Curve25519 public key echoed back,
// followed by an OP_NODE_ID_SET frame that assigns the node its logical ID.
func (ms *MeshServer) ApproveEnrollment(macStr string, params ApprovalParams) error {
	node, err := ms.authRegistry.Approve(macStr)
	if err != nil {
		return err
	}

	// Auto-assign nodeId if not provided
	nodeId := params.NodeID
	if nodeId == 0 {
		nodeId = ms.nodeRegistry.NextFreeNodeID()
		if nodeId == 0 {
			slog.Warn("All node IDs in use; node will have ID 0", "mac", macStr)
		}
	}

	// Assign in node registry (creates entry if not yet seen)
	ms.nodeRegistry.AssignNode(node.MAC[:], nodeId, params.Name, params.Zone)

	if registryNode, ok := ms.nodeRegistry.GetNode(node.MAC[:]); ok {
		typeStr := params.AdapterTypeStr
		if typeStr == "" {
			typeStr = "unknown"
		}
		ms.publishEnrolledEvent(registryNode, typeStr)
	}

	if ms.serialComm != nil {
		// Send JOIN_ACK
		ackMsg := &MeshMessage{
			MessageType:      MessageTypeJoinAck,
			TargetMacAddress: node.MAC[:],
			PublicKey:        node.PublicKey[:],
		}
		if err := ms.serialComm.WriteFrame(ackMsg); err != nil {
			slog.Warn("Failed to send JOIN_ACK", "mac", macStr, "error", err)
		}

		// Send OP_NODE_ID_SET immediately after JOIN_ACK
		if nodeId > 0 {
			payload := make([]byte, MaxDataLength)
			payload[0] = OpNodeIdSet          // 0xC0
			copy(payload[1:7], node.MAC[:])   // target MAC
			payload[7] = nodeId
			idMsg := &MeshMessage{
				MessageType: MessageTypeSerialCmdBroadcast,
				DataType:    AdapterTypeSerial,
				Data:        payload,
			}
			if err := ms.serialComm.WriteFrame(idMsg); err != nil {
				slog.Warn("Failed to send OP_NODE_ID_SET", "mac", macStr, "nodeId", nodeId, "error", err)
			}
		}
	}

	slog.Info("Enrollment approved", "mac", macStr, "nodeId", nodeId, "name", params.Name)
	if ms.authPath != "" {
		return ms.authRegistry.Persist(ms.authPath)
	}
	return nil
}

// RejectEnrollment rejects a pending enrollment request and notifies the master.
// A JOIN_ACK with empty PublicKey signals rejection to the firmware.
func (ms *MeshServer) RejectEnrollment(macStr string) error {
	// Get the node MAC before rejecting (Reject only sets status; node remains in registry).
	mac, err := nodeauth.ParseMAC(macStr)
	if err != nil {
		return fmt.Errorf("invalid MAC string: %w", err)
	}

	if err := ms.authRegistry.Reject(macStr); err != nil {
		return err
	}

	// Send rejection frame: JOIN_ACK with empty PublicKey = rejection signal to firmware.
	if ms.serialComm != nil {
		rejectMsg := &MeshMessage{
			MessageType:      MessageTypeJoinAck,
			TargetMacAddress: mac[:],
			// PublicKey intentionally absent — rejection signal
		}
		if err := ms.serialComm.WriteFrame(rejectMsg); err != nil {
			slog.Warn("Failed to send rejection frame", "mac", macStr, "error", err)
			// best-effort; do not block the rejection
		}
	}

	slog.Info("Enrollment rejected", "mac", macStr)
	if ms.authPath != "" {
		return ms.authRegistry.Persist(ms.authPath)
	}
	return nil
}

// GetPendingEnrollments returns all nodes with TrustPending status.
func (ms *MeshServer) GetPendingEnrollments() []*nodeauth.NodeAuth {
	return ms.authRegistry.GetPending()
}

// GetAuthRegistry returns the underlying auth registry (for HTTP handlers).
func (ms *MeshServer) GetAuthRegistry() *nodeauth.Registry {
	return ms.authRegistry
}

// SendMessage sends a message to the mesh network
func (ms *MeshServer) SendMessage(msg *MeshMessage) error {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if !ms.running {
		return fmt.Errorf("mesh server is not running")
	}

	slog.Debug("Sending message", "type", msg.MessageType, "dataType", msg.DataType)

	// Log the outgoing message
	if err := ms.logMessageToKafka(msg, "outgoing"); err != nil {
		slog.Warn("Failed to log outgoing message to Kafka", "error", err)
	}

	if err := ms.serialComm.WriteFrame(msg); err != nil {
		slog.Error("Failed to send message", "error", err)
		return err
	}

	return nil
}

// ConfigureNode sets the adapter type for a specific node
func (ms *MeshServer) ConfigureNode(targetMAC []byte, adapterType int32) error {
	msg, err := ms.messageBuilder.BuildConfigSetMessage(targetMAC, adapterType)
	if err != nil {
		return fmt.Errorf("failed to build config message: %w", err)
	}

	slog.Info("Configuring node", "mac", macToString(targetMAC), "adapterType", GetAdapterTypeName(adapterType))

	return ms.SendMessage(msg)
}

// ConfigureAllNodes sets the adapter type for all nodes
func (ms *MeshServer) ConfigureAllNodes(adapterType int32) error {
	msg, err := ms.messageBuilder.BuildConfigSetBroadcastMessage(adapterType)
	if err != nil {
		return fmt.Errorf("failed to build broadcast config message: %w", err)
	}

	slog.Info("Configuring all nodes", "adapterType", GetAdapterTypeName(adapterType))

	return ms.SendMessage(msg)
}

// RequestHealthReports requests health reports from all nodes
func (ms *MeshServer) RequestHealthReports() error {
	msg := ms.messageBuilder.BuildHealthRequestMessage()

	slog.Debug("Requesting health reports from all nodes")
	return ms.SendMessage(msg)
}

// BroadcastData broadcasts data to all nodes
func (ms *MeshServer) BroadcastData(dataType int32, data []byte) error {
	msg, err := ms.messageBuilder.BuildBroadcastMessage(dataType, data)
	if err != nil {
		return fmt.Errorf("failed to build broadcast message: %w", err)
	}

	slog.Debug("Broadcasting data", "type", GetAdapterTypeName(dataType), "length", len(data))

	return ms.SendMessage(msg)
}

// GetNodeRegistry returns the node registry
func (ms *MeshServer) GetNodeRegistry() *NodeRegistry {
	return ms.nodeRegistry
}

// GetEventBroker returns the in-process event broker for SSE subscribers.
func (ms *MeshServer) GetEventBroker() *EventBroker { return ms.eventBroker }

// GetHealthTimeout returns the configured health timeout duration.
func (ms *MeshServer) GetHealthTimeout() time.Duration { return ms.healthTimeout }

// GetZoneRegistry returns the zone registry.
func (ms *MeshServer) GetZoneRegistry() *ZoneRegistry { return ms.zoneRegistry }

// SetZoneRegistryPath sets the persistence path and loads existing zones.
func (ms *MeshServer) SetZoneRegistryPath(path string) {
	ms.mu.Lock()
	ms.zoneRegistryPath = path
	ms.mu.Unlock()
	if path != "" {
		_ = ms.zoneRegistry.Load(path)
	}
}

// SendNodeData sends a serial command frame to a specific node MAC.
// The target MAC is embedded at bytes [1:7] of the payload (after the opcode byte at [0]),
// mirroring the OpNodeIdSet pattern used in ApproveEnrollment.
func (ms *MeshServer) SendNodeData(mac []byte, dataType int32, data []byte) error {
	if ms.serialComm == nil {
		return fmt.Errorf("mesh server is not running")
	}

	payload := make([]byte, MaxDataLength)
	copy(payload, data)
	if len(mac) == 6 && len(payload) >= 7 {
		copy(payload[1:7], mac) // embed target MAC after opcode byte (mirrors OpNodeIdSet pattern)
	}
	msg := &MeshMessage{
		ProtoVersion: 2,
		MessageType:  MessageTypeSerialCmdBroadcast,
		DataType:     dataType,
		Data:         payload,
	}
	return ms.serialComm.WriteFrame(msg)
}

// publishEvent marshals data and publishes a typed Event to the broker.
func (ms *MeshServer) publishEvent(eventType EventType, data interface{}) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	ms.eventBroker.Publish(Event{Type: eventType, Data: raw, Timestamp: time.Now()})
}

// publishMotionEvent publishes a motion event for the given node.
// Called from handlePIRData after the Kafka publish.
func (ms *MeshServer) publishMotionEvent(node *NodeInfo) {
	ms.publishEvent(EventMotion, map[string]interface{}{
		"nodeId":    node.NodeID,
		"name":      node.Name,
		"zone":      node.Zone,
		"hopCount":  node.HopCount,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// publishHealthEvent publishes a health event and, on first-seen, a node_online event.
// Called from handleHealthReport after UpdateNode.
func (ms *MeshServer) publishHealthEvent(node *NodeInfo) {
	online := time.Since(node.LastSeen) <= ms.healthTimeout
	ms.publishEvent(EventHealth, map[string]interface{}{
		"nodeId":   node.NodeID,
		"name":     node.Name,
		"online":   online,
		"uptime":   node.Uptime,
		"hopCount": node.HopCount,
	})
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if !ms.nodeOnlineState[node.MACString] {
		ms.nodeOnlineState[node.MACString] = true
		ms.publishEvent(EventNodeOnline, map[string]interface{}{
			"nodeId": node.NodeID,
			"name":   node.Name,
		})
	}
}

// publishEnrolledEvent publishes an enrolled event for the given node.
// Called from ApproveEnrollment after AssignNode.
func (ms *MeshServer) publishEnrolledEvent(node *NodeInfo, adapterTypeStr string) {
	ms.publishEvent(EventEnrolled, map[string]interface{}{
		"nodeId": node.NodeID,
		"name":   node.Name,
		"type":   adapterTypeStr,
	})
}

// offlineDetectorLoop runs on a 30-second ticker and detects nodes that have
// gone offline since the last health report.
func (ms *MeshServer) offlineDetectorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ms.checkOfflineNodes()
		case <-ms.ctx.Done():
			return
		}
	}
}

// checkOfflineNodes scans all registered nodes and publishes EventNodeOffline
// for any that were previously online but haven't reported within healthTimeout.
func (ms *MeshServer) checkOfflineNodes() {
	nodes := ms.nodeRegistry.GetAllNodes()
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, node := range nodes {
		if time.Since(node.LastSeen) > ms.healthTimeout {
			if ms.nodeOnlineState[node.MACString] {
				ms.nodeOnlineState[node.MACString] = false
				ms.publishEvent(EventNodeOffline, map[string]interface{}{
					"nodeId":   node.NodeID,
					"name":     node.Name,
					"lastSeen": node.LastSeen.UTC().Format(time.RFC3339),
				})
			}
		}
	}
}

// IsRunning returns whether the server is running
func (ms *MeshServer) IsRunning() bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.running
}

// SetTxPowerPreset sends OP_TX_POWER_SET to master via serial.
// Master applies locally and broadcasts to all enrolled nodes.
func (ms *MeshServer) SetTxPowerPreset(preset uint8) error {
	if preset > 2 {
		return fmt.Errorf("invalid TX power preset %d: must be 0 (short), 1 (indoor), or 2 (outdoor)", preset)
	}
	if ms.serialComm == nil {
		return fmt.Errorf("mesh server is not running")
	}

	payload := make([]byte, MaxDataLength)
	payload[0] = OpTxPowerSet
	payload[1] = preset
	msg := &MeshMessage{
		MessageType: MessageTypeAdapterData,
		DataType:    AdapterTypeSerial,
		Data:        payload,
	}
	if err := ms.serialComm.WriteFrame(msg); err != nil {
		return fmt.Errorf("failed to send TX power preset: %w", err)
	}

	ms.mu.Lock()
	ms.currentTxPreset = preset
	ms.mu.Unlock()
	slog.Info("TX power preset set", "name", txPowerPresetNames[preset], "preset", preset)
	return nil
}

// GetTxPowerPreset returns the current TX power preset value and name.
func (ms *MeshServer) GetTxPowerPreset() (uint8, string) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.currentTxPreset, txPowerPresetNames[ms.currentTxPreset]
}

// logMessageToKafka logs messages to Kafka for debugging and monitoring
func (ms *MeshServer) logMessageToKafka(msg *MeshMessage, direction string) error {
	if ms.eventStore == nil {
		return nil // Event store not configured
	}

	logEntry := map[string]interface{}{
		"timestamp":   time.Now().Unix(),
		"direction":   direction,
		"messageType": msg.MessageType,
		"dataType":    msg.DataType,
		"origin":      macToString(msg.OriginMacAddress),
		"target":      macToString(msg.TargetMacAddress),
		"lastHop":     macToString(msg.LastHopMacAddress),
		"hopCount":    msg.HopCount,
		"dataLength":  len(msg.Data),
	}

	// Add specific fields for health reports
	if ms.messageBuilder.IsHealthReport(msg) {
		if healthReport, err := ms.messageBuilder.ParseHealthReport(msg); err == nil {
			logEntry["healthReport"] = map[string]interface{}{
				"mac":         macToString(healthReport.MAC),
				"adapterType": GetAdapterTypeName(healthReport.AdapterType),
				"uptime":      healthReport.Uptime,
			}
		}
	}

	logJSON, err := json.Marshal(logEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	err = ms.eventStore.WriteMessage(string(logJSON), "mesh-messages")
	RecordKafkaWrite("mesh-messages", err)
	return err
}
