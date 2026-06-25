package mesh

import (
	"context"
	"encoding/json"
	"errors"
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
	// Any protoVersion > 0 that is not 1 is an unknown future version; drop it.
	if msg.ProtoVersion > 0 && msg.ProtoVersion != 1 {
		slog.Warn("Unsupported proto version — dropping", "version", msg.ProtoVersion, "origin", fmt.Sprintf("%x", msg.OriginMacAddress))
		return nil
	}

	// Replay check (only for proto v1 messages with epoch/seq set)
	if msg.ProtoVersion == 1 && msg.EpochNum > 0 {
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
	case OpHealthReport:
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

	return nil
}

// handleMasterBeacon processes master beacon messages
func (ms *MeshServer) handleMasterBeacon(msg *MeshMessage) error {
	slog.Debug("Master beacon received", "origin", macToString(msg.OriginMacAddress))
	return nil
}

// ApproveEnrollment approves a pending node enrollment.
// NOTE: Sending a JOIN_ACK to the node requires a server Curve25519 keypair (for ECDH).
// The server keypair is not yet implemented (deferred to a later task). Until it is,
// this function returns an error immediately without sending any JoinAck frame,
// making the incompleteness explicit and preventing a silently broken ECDH exchange.
// The HTTP API (Task 2) will surface this error to the operator.
func (ms *MeshServer) ApproveEnrollment(macStr string) error {
	return errors.New("server keypair not initialized: enrollment approval not yet supported")
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
			OriginMacAddress: mac[:],
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

	// Frame: [2-byte LE length][A1][preset]
	payload := []byte{OpTxPowerSet, preset}
	header := []byte{byte(len(payload) & 0xFF), byte((len(payload) >> 8) & 0xFF)}
	frame := append(header, payload...)

	if err := ms.serialComm.WriteRaw(frame); err != nil {
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
