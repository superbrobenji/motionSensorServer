package mesh

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"

	"google.golang.org/protobuf/proto"
)

// SerialPort interface for serial communication
type SerialPort interface {
	io.ReadWriter
	Close() error
}

// SerialComm handles serial communication with framing
type SerialComm struct {
	port SerialPort
}

// NewSerialComm creates a new serial communication handler
func NewSerialComm(port SerialPort) *SerialComm {
	return &SerialComm{port: port}
}

// WriteFrame writes a protobuf message with 2-byte little-endian length prefix
func (s *SerialComm) WriteFrame(msg *MeshMessage) error {
	log.Printf("[SERIAL_TX] Preparing to send message - Type: %d, DataType: %d, Origin: %x, Target: %x, HopCount: %d, DataLen: %d", 
		msg.MessageType, msg.DataType, msg.OriginMacAddress, msg.TargetMacAddress, msg.HopCount, len(msg.Data))
	
	if len(msg.Data) > 0 {
		log.Printf("[SERIAL_TX] Message data: %x", msg.Data)
	}

	// Marshal the protobuf message
	data, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("[SERIAL_TX] Failed to marshal message: %v", err)
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	log.Printf("[SERIAL_TX] Marshaled protobuf data (%d bytes): %x", len(data), data)

	// Create 2-byte little-endian length header
	header := make([]byte, 2)
	binary.LittleEndian.PutUint16(header, uint16(len(data)))

	log.Printf("[SERIAL_TX] Header to send: %02x %02x (length: %d)", header[0], header[1], len(data))

	// Write header
	if _, err := s.port.Write(header); err != nil {
		log.Printf("[SERIAL_TX] Failed to write header: %v", err)
		return fmt.Errorf("failed to write header: %w", err)
	}

	log.Printf("[SERIAL_TX] Header sent successfully")

	// Write data
	if _, err := s.port.Write(data); err != nil {
		log.Printf("[SERIAL_TX] Failed to write data: %v", err)
		return fmt.Errorf("failed to write data: %w", err)
	}

	log.Printf("[SERIAL_TX] Data sent successfully - Total frame size: %d bytes (2-byte header + %d data bytes)", 
		len(header)+len(data), len(data))

	return nil
}

// ReadFrame reads a protobuf message with 2-byte little-endian length prefix
func (s *SerialComm) ReadFrame() (*MeshMessage, error) {
	// Read 2-byte header
	header := make([]byte, 2)
	if _, err := io.ReadFull(s.port, header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	log.Printf("[SERIAL_RX] Header received: %02x %02x", header[0], header[1])

	// Parse length
	length := binary.LittleEndian.Uint16(header)
	if length == 0 {
		log.Printf("[SERIAL_RX] WARNING: Zero-length frame detected - possible frame sync issue")
		return nil, fmt.Errorf("invalid frame length: 0 (header bytes: %02x %02x)", header[0], header[1])
	}

	// Enhanced validation with more detailed logging
	if length > 4096 {
		log.Printf("[SERIAL_RX] CRITICAL: Frame length too large: %d bytes (header: %02x %02x)", length, header[0], header[1])
		log.Printf("[SERIAL_RX] This indicates frame desynchronization - ESP32 may be sending non-framed data")
		log.Printf("[SERIAL_RX] Header as ASCII: '%c%c' (if printable)", 
			func() byte { if header[0] >= 32 && header[0] <= 126 { return header[0] } else { return '.' } }(),
			func() byte { if header[1] >= 32 && header[1] <= 126 { return header[1] } else { return '.' } }())
		
		// Try to recover by reading and discarding some bytes
		discardBuf := make([]byte, 100)
		if n, err := s.port.Read(discardBuf); err == nil {
			log.Printf("[SERIAL_RX] Discarded %d bytes for recovery: %x", n, discardBuf[:n])
		}
		return nil, fmt.Errorf("frame length too large: %d (header bytes: %02x %02x)", length, header[0], header[1])
	}

	// Warn about suspiciously large but valid frames
	if length > 200 {
		log.Printf("[SERIAL_RX] WARNING: Large frame detected: %d bytes - may indicate buffer overflow", length)
	}

	log.Printf("[SERIAL_RX] Frame length: %d bytes", length)

	// Read data
	data := make([]byte, length)
	if _, err := io.ReadFull(s.port, data); err != nil {
		log.Printf("[SERIAL_RX] Failed to read %d bytes of frame data: %v", length, err)
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	log.Printf("[SERIAL_RX] Raw data received (%d bytes): %x", len(data), data)

	// Check if data looks like ASCII (debugging ESP32 text output)
	asciiCount := 0
	for _, b := range data {
		if b >= 32 && b <= 126 {
			asciiCount++
		}
	}
	if asciiCount > len(data)/2 {
		log.Printf("[SERIAL_RX] WARNING: Data appears to be %d%% ASCII text - ESP32 may be sending debug output instead of protobuf", (asciiCount*100)/len(data))
		log.Printf("[SERIAL_RX] Data as ASCII: %q", string(data))
	}

	// Unmarshal protobuf message
	var msg MeshMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		log.Printf("[SERIAL_RX] UNMARSHAL FAILED: %v", err)
		log.Printf("[SERIAL_RX] Failed protobuf data (%d bytes): %x", len(data), data)
		log.Printf("[SERIAL_RX] Data as ASCII (if readable): %q", string(data))
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	log.Printf("[SERIAL_RX] ✓ Successfully parsed message - Type: %d, DataType: %d, Origin: %x, Target: %x, HopCount: %d, DataLen: %d", 
		msg.MessageType, msg.DataType, msg.OriginMacAddress, msg.TargetMacAddress, msg.HopCount, len(msg.Data))
	
	if len(msg.Data) > 0 {
		log.Printf("[SERIAL_RX] Message data: %x", msg.Data)
	}

	return &msg, nil
}

// Close closes the serial port
func (s *SerialComm) Close() error {
	return s.port.Close()
}

// FlushBuffer attempts to clear any buffered data from the serial port
func (s *SerialComm) FlushBuffer() error {
	log.Printf("[SERIAL_FLUSH] Attempting to flush serial buffer")
	
	// Try to read any remaining data with a short timeout
	buffer := make([]byte, 1024)
	totalFlushed := 0
	
	for i := 0; i < 10; i++ { // Try up to 10 times
		n, err := s.port.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break // No more data
			}
			log.Printf("[SERIAL_FLUSH] Error during flush: %v", err)
			break
		}
		if n == 0 {
			break // No data available
		}
		totalFlushed += n
		log.Printf("[SERIAL_FLUSH] Flushed %d bytes: %x", n, buffer[:n])
	}
	
	log.Printf("[SERIAL_FLUSH] Total flushed: %d bytes", totalFlushed)
	return nil
}
