package mesh

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	EventStore "github.com/superbrobenji/motionServer/eventStore"
	"github.com/superbrobenji/motionServer/nodeauth"
	"go.bug.st/serial"
)

// MeshServer manages the mesh network communication
type MeshServer struct {
	serialComm     *SerialComm
	nodeRegistry   *NodeRegistry
	messageBuilder *MessageBuilder
	eventStore     EventStore.EventStore_interface

	// Auth
	authRegistry *nodeauth.Registry
	replayCache  *nodeauth.ReplayCache
	authPath     string        // Path to persist registry JSON
	stopPersist  chan struct{}

	// Configuration
	serialPort    string
	baudRate      int
	healthTimeout time.Duration

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
	EventStore       EventStore.EventStore_interface
	AuthRegistryPath string // e.g. "data/nodeauth.json"
}

// NewMeshServer creates a new mesh server
func NewMeshServer(config MeshServerConfig) *MeshServer {
	ctx, cancel := context.WithCancel(context.Background())

	registry := nodeauth.NewRegistry()
	if config.AuthRegistryPath != "" {
		if err := registry.Load(config.AuthRegistryPath); err != nil {
			log.Printf("[AUTH] Failed to load auth registry: %v", err)
		}
	}

	return &MeshServer{
		nodeRegistry:   NewNodeRegistry(),
		messageBuilder: NewMessageBuilder(),
		eventStore:     config.EventStore,
		authRegistry:   registry,
		replayCache:    nodeauth.NewReplayCache(64),
		authPath:       config.AuthRegistryPath,
		stopPersist:    make(chan struct{}),
		serialPort:     config.SerialPort,
		baudRate:       config.BaudRate,
		healthTimeout:  config.HealthTimeout,
		ctx:            ctx,
		cancel:         cancel,
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

	// Start message processing goroutine
	ms.wg.Add(1)
	go ms.messageProcessor()

	// Start auth registry persistence loop
	if ms.authPath != "" {
		go ms.authRegistry.PersistLoop(ms.authPath, 30*time.Second, ms.stopPersist)
	}

	log.Printf("Mesh server started on serial port %s at %d baud", ms.serialPort, ms.baudRate)
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

	// Stop persistence loop (final save happens inside PersistLoop)
	close(ms.stopPersist)

	if ms.serialComm != nil {
		ms.serialComm.Close()
	}

	ms.wg.Wait()
	log.Printf("Mesh server stopped")
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
					log.Printf("[MSG_PROCESSOR] Error reading frame (#%d): %v", consecutiveErrors, err)
				} else if consecutiveErrors == maxConsecutiveErrors+1 {
					log.Printf("[MSG_PROCESSOR] Too many consecutive frame errors (%d), suppressing further error messages. Last error: %v", consecutiveErrors, err)
					log.Printf("[MSG_PROCESSOR] Note: If you see 'frame length too large' with ASCII characters (like 'un', 't:', '--'), the ESP32 might be sending text data instead of binary protobuf frames.")
					log.Printf("[MSG_PROCESSOR] Check ESP32 firmware and ensure it's configured for mesh protocol, not debug output.")
					log.Printf("[MSG_PROCESSOR] Consider restarting ESP32 or checking serial connection.")
				}

				// After many consecutive errors, try to flush the buffer
				if consecutiveErrors == 10 {
					log.Printf("[MSG_PROCESSOR] Attempting buffer flush after %d consecutive errors", consecutiveErrors)
					if flushErr := ms.serialComm.FlushBuffer(); flushErr != nil {
						log.Printf("[MSG_PROCESSOR] Buffer flush failed: %v", flushErr)
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
					log.Printf("Frame reading recovered after %d consecutive errors", consecutiveErrors)
				}
				consecutiveErrors = 0
			}

			log.Printf("[MSG_PROCESSOR] Successfully received message from serial port - Type: %d, DataType: %d, Origin: %s",
				msg.MessageType, msg.DataType, macToString(msg.OriginMacAddress))

			if err := ms.handleMessage(msg); err != nil {
				log.Printf("[MSG_PROCESSOR] Error handling message: %v", err)
			} else {
				log.Printf("[MSG_PROCESSOR] Message processed successfully")
			}
		}
	}
}

// handleMessage processes a received mesh message
func (ms *MeshServer) handleMessage(msg *MeshMessage) error {
	// Proto version check — version 0 means legacy (pre-security) node; allow it.
	// Any version other than 0 or 1 is unknown and must be dropped.
	if msg.ProtoVersion != 0 && msg.ProtoVersion != 1 {
		log.Printf("[MSG] Unsupported proto version %d from %x — dropping", msg.ProtoVersion, msg.OriginMacAddress)
		return nil
	}

	// Replay check (only for proto v1 messages with epoch/seq set)
	if msg.ProtoVersion == 1 && msg.EpochNum > 0 {
		var mac [6]byte
		copy(mac[:], msg.OriginMacAddress)
		if ms.replayCache.IsDuplicate(mac, msg.EpochNum, msg.SeqNum) {
			log.Printf("[MSG] Replayed message dropped from %x (epoch=%d seq=%d)", mac, msg.EpochNum, msg.SeqNum)
			return nil
		}
	}

	// Log the message to Kafka
	if err := ms.logMessageToKafka(msg, "incoming"); err != nil {
		log.Printf("Failed to log incoming message to Kafka: %v", err)
	}

	switch msg.MessageType {
	case MessageTypeAdapterData:
		return ms.handleAdapterData(msg)
	case MessageTypeMasterBeacon:
		return ms.handleMasterBeacon(msg)
	default:
		log.Printf("Unknown message type: %d", msg.MessageType)
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
		log.Printf("Received adapter data - Type: %s, Origin: %s, Data: %x",
			GetAdapterTypeName(msg.DataType),
			macToString(msg.OriginMacAddress),
			msg.Data)
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
	case OpEnrollmentReq:
		return ms.handleEnrollmentRequest(msg.Data)
	default:
		log.Printf("Unknown serial opcode: 0x%02x", opcode)
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

	log.Printf("Health report from %s: Type=%s, Uptime=%ds, Hops=%d",
		macToString(healthReport.MAC),
		GetAdapterTypeName(healthReport.AdapterType),
		healthReport.Uptime,
		healthReport.HopCount)

	return nil
}

// handleEnrollmentRequest processes an enrollment request from a new node.
// Format: [0xC0][6B mac][32B pubkey] = 39 bytes total.
func (ms *MeshServer) handleEnrollmentRequest(data []byte) error {
	// data[0] is the opcode (0xC0); mac starts at data[1]
	if len(data) < 39 {
		return fmt.Errorf("enrollment request too short: %d bytes", len(data))
	}
	var mac [6]byte
	copy(mac[:], data[1:7])
	var pubKey [32]byte
	copy(pubKey[:], data[7:39])

	macStr := fmt.Sprintf("%x", mac)
	log.Printf("[AUTH] Enrollment request from %s (pubkey: %x...)", macStr, pubKey[:4])

	if err := ms.authRegistry.AddPending(mac, pubKey); err != nil {
		log.Printf("[AUTH] Failed to add pending enrollment for %s: %v", macStr, err)
		return err
	}

	// Persist immediately so the admin sees it even if the server restarts
	if ms.authPath != "" {
		if err := ms.authRegistry.Persist(ms.authPath); err != nil {
			log.Printf("[AUTH] Failed to persist after enrollment request: %v", err)
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
		ms.eventStore.WriteMessage(string(j), "mesh-enrollment")
	}
	return nil
}

// handlePIRData processes PIR sensor data
func (ms *MeshServer) handlePIRData(msg *MeshMessage) error {
	log.Printf("PIR motion detected from %s (hops: %d)",
		macToString(msg.OriginMacAddress),
		msg.HopCount)

	// Log PIR event to Kafka with more specific topic
	pirEvent := map[string]interface{}{
		"type":      "pir_motion",
		"mac":       macToString(msg.OriginMacAddress),
		"timestamp": time.Now().Unix(),
		"hopCount":  msg.HopCount,
		"data":      msg.Data,
	}

	eventJSON, _ := json.Marshal(pirEvent)
	if err := ms.eventStore.WriteMessage(string(eventJSON), "motion-trigger"); err != nil {
		log.Printf("Failed to log PIR event to Kafka: %v", err)
	}

	return nil
}

// handleMasterBeacon processes master beacon messages
func (ms *MeshServer) handleMasterBeacon(msg *MeshMessage) error {
	log.Printf("Master beacon from %s", macToString(msg.OriginMacAddress))
	return nil
}

// ApproveEnrollment approves a pending node and sends OP_ENROLLMENT_APPROVE to master via serial.
func (ms *MeshServer) ApproveEnrollment(macStr string) error {
	node, err := ms.authRegistry.Approve(macStr)
	if err != nil {
		return err
	}

	// Build OP_ENROLLMENT_APPROVE frame: [0xC1][6B mac][32B pubkey] = 39 bytes payload
	frame := make([]byte, 39)
	frame[0] = OpEnrollmentApprove
	copy(frame[1:7], node.MAC[:])
	copy(frame[7:39], node.PublicKey[:])

	// Prepend 2-byte LE length header
	header := []byte{byte(len(frame) & 0xFF), byte((len(frame) >> 8) & 0xFF)}
	combined := append(header, frame...)

	if err := ms.serialComm.WriteRaw(combined); err != nil {
		return fmt.Errorf("failed to send enrollment approval: %w", err)
	}

	log.Printf("[AUTH] Enrollment approved and sent to master: %s", macStr)
	if ms.authPath != "" {
		if err := ms.authRegistry.Persist(ms.authPath); err != nil {
			log.Printf("[AUTH] Failed to persist after approval: %v", err)
		}
	}
	return nil
}

// RejectEnrollment rejects a pending enrollment request.
func (ms *MeshServer) RejectEnrollment(macStr string) error {
	if err := ms.authRegistry.Reject(macStr); err != nil {
		return err
	}
	log.Printf("[AUTH] Enrollment rejected: %s", macStr)
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

	log.Printf("[SEND_MESSAGE] Attempting to send message - Type: %d, DataType: %d, Origin: %s, Target: %s",
		msg.MessageType, msg.DataType, macToString(msg.OriginMacAddress), macToString(msg.TargetMacAddress))

	// Log the outgoing message
	if err := ms.logMessageToKafka(msg, "outgoing"); err != nil {
		log.Printf("Failed to log outgoing message to Kafka: %v", err)
	}

	if err := ms.serialComm.WriteFrame(msg); err != nil {
		log.Printf("[SEND_MESSAGE] Failed to send message: %v", err)
		return err
	}

	log.Printf("[SEND_MESSAGE] Message sent successfully via serial port")
	return nil
}

// ConfigureNode sets the adapter type for a specific node
func (ms *MeshServer) ConfigureNode(targetMAC []byte, adapterType int32) error {
	msg, err := ms.messageBuilder.BuildConfigSetMessage(targetMAC, adapterType)
	if err != nil {
		return fmt.Errorf("failed to build config message: %w", err)
	}

	log.Printf("Configuring node %s to adapter type %s",
		macToString(targetMAC),
		GetAdapterTypeName(adapterType))

	return ms.SendMessage(msg)
}

// ConfigureAllNodes sets the adapter type for all nodes
func (ms *MeshServer) ConfigureAllNodes(adapterType int32) error {
	msg, err := ms.messageBuilder.BuildConfigSetBroadcastMessage(adapterType)
	if err != nil {
		return fmt.Errorf("failed to build broadcast config message: %w", err)
	}

	log.Printf("Configuring all nodes to adapter type %s",
		GetAdapterTypeName(adapterType))

	return ms.SendMessage(msg)
}

// RequestHealthReports requests health reports from all nodes
func (ms *MeshServer) RequestHealthReports() error {
	msg := ms.messageBuilder.BuildHealthRequestMessage()

	log.Printf("Requesting health reports from all nodes")
	return ms.SendMessage(msg)
}

// BroadcastData broadcasts data to all nodes
func (ms *MeshServer) BroadcastData(dataType int32, data []byte) error {
	msg, err := ms.messageBuilder.BuildBroadcastMessage(dataType, data)
	if err != nil {
		return fmt.Errorf("failed to build broadcast message: %w", err)
	}

	log.Printf("Broadcasting data: Type=%s, Length=%d",
		GetAdapterTypeName(dataType),
		len(data))

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

	return ms.eventStore.WriteMessage(string(logJSON), "mesh-messages")
}
