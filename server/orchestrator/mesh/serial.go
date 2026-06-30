package mesh

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"

	"go.bug.st/serial"
	"google.golang.org/protobuf/proto"
)

// SerialPort interface for serial communication
type SerialPort interface {
	io.ReadWriter
	Close() error
	Flush() error
}

// realSerialPort wraps go.bug.st/serial Port to satisfy SerialPort interface.
type realSerialPort struct {
	serial.Port
}

func (p *realSerialPort) Flush() error {
	return p.ResetInputBuffer()
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
	slog.Debug("Serial TX", "type", msg.MessageType, "dataType", msg.DataType, "dataLen", len(msg.Data))

	// Marshal the protobuf message
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create 2-byte little-endian length header
	header := make([]byte, 2)
	binary.LittleEndian.PutUint16(header, uint16(len(data)))

	// Write header
	if _, err := s.port.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write data
	if _, err := s.port.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}

// WriteRaw writes raw bytes directly to the serial port without framing.
// Used for non-protobuf control frames such as OP_TX_POWER_SET.
func (s *SerialComm) WriteRaw(data []byte) error {
	_, err := s.port.Write(data)
	return err
}

// ReadFrame reads a protobuf message with 2-byte little-endian length prefix
func (s *SerialComm) ReadFrame() (*MeshMessage, error) {
	// Read 2-byte header
	header := make([]byte, 2)
	if _, err := io.ReadFull(s.port, header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Parse length
	length := binary.LittleEndian.Uint16(header)
	if length == 0 {
		return nil, fmt.Errorf("invalid frame length: 0 (header bytes: %02x %02x)", header[0], header[1])
	}

	if length > 4096 {
		slog.Warn("Frame length too large — possible desync", "length", length, "header", fmt.Sprintf("%02x %02x", header[0], header[1]))
		// Try to recover by reading and discarding some bytes
		discardBuf := make([]byte, 100)
		if n, err := s.port.Read(discardBuf); err == nil {
			slog.Debug("Discarded bytes for recovery", "count", n)
		}
		return nil, fmt.Errorf("frame length too large: %d (header bytes: %02x %02x)", length, header[0], header[1])
	}

	if length > 200 {
		slog.Debug("Large frame detected", "length", length)
	}

	slog.Debug("Serial RX frame", "length", length)

	// Read data
	data := make([]byte, length)
	if _, err := io.ReadFull(s.port, data); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	// Unmarshal protobuf message
	var msg MeshMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	slog.Debug("Serial RX parsed", "type", msg.MessageType, "dataType", msg.DataType, "hops", msg.HopCount)

	return &msg, nil
}

// Close closes the serial port
func (s *SerialComm) Close() error {
	return s.port.Close()
}

// FlushBuffer attempts to clear any buffered data from the serial port
func (s *SerialComm) FlushBuffer() error {
	slog.Debug("Flushing serial buffer")
	if err := s.port.Flush(); err != nil {
		slog.Debug("Flush failed", "error", err)
	}
	return nil
}
