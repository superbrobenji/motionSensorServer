package mesh

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestV1Status_SingleMaster_SerialBlockShowsPrimaryOnly(t *testing.T) {
	api, ms := newV1TestServer(t)
	mock := NewMockSerialPort()
	ms.serialComm = NewSerialComm(mock)
	// secondarySerialComm remains nil — single master mode

	w := v1Request(t, api, "GET", "/api/v1/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status returned %d", w.Code)
	}

	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := json.Marshal(resp.Data)
	var status struct {
		Serial struct {
			Primary   string `json:"primary"`
			Secondary string `json:"secondary"`
		} `json:"serial"`
	}
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if status.Serial.Primary != "connected" {
		t.Errorf("serial.primary = %q, want %q", status.Serial.Primary, "connected")
	}
	if status.Serial.Secondary != "not_configured" {
		t.Errorf("serial.secondary = %q, want %q", status.Serial.Secondary, "not_configured")
	}
}

func TestV1Status_DualMaster_SecondaryConnected(t *testing.T) {
	api, ms := newV1TestServer(t)
	primaryMock := NewMockSerialPort()
	secondaryMock := NewMockSerialPort()
	ms.serialComm = NewSerialComm(primaryMock)
	ms.secondarySerialComm = NewSerialComm(secondaryMock)
	ms.secondaryConnected = true

	w := v1Request(t, api, "GET", "/api/v1/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status returned %d", w.Code)
	}

	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := json.Marshal(resp.Data)
	var status struct {
		Serial struct {
			Primary   string `json:"primary"`
			Secondary string `json:"secondary"`
		} `json:"serial"`
	}
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if status.Serial.Primary != "connected" {
		t.Errorf("serial.primary = %q, want %q", status.Serial.Primary, "connected")
	}
	if status.Serial.Secondary != "connected" {
		t.Errorf("serial.secondary = %q, want %q", status.Serial.Secondary, "connected")
	}
}

func TestV1Status_DualMaster_SecondaryDisconnected(t *testing.T) {
	api, ms := newV1TestServer(t)
	primaryMock := NewMockSerialPort()
	ms.serialComm = NewSerialComm(primaryMock)
	// secondarySerialComm is nil but secondaryPort is set — secondary configured but failed to open
	ms.secondaryPort = "/dev/ttyUSB1"
	ms.secondaryConnected = false

	w := v1Request(t, api, "GET", "/api/v1/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status returned %d", w.Code)
	}

	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := json.Marshal(resp.Data)
	var status struct {
		Serial struct {
			Primary   string `json:"primary"`
			Secondary string `json:"secondary"`
		} `json:"serial"`
	}
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if status.Serial.Secondary != "disconnected" {
		t.Errorf("serial.secondary = %q, want %q", status.Serial.Secondary, "disconnected")
	}
}
