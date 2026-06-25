package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	EventStore "github.com/superbrobenji/motionServer/eventStore"
	"github.com/superbrobenji/motionServer/mesh"
)

var (
	broker  = "kafka:9092" // Use docker compose service name - internal port
	groupId = "1"
)

func main() {
	// Command line flags
	serialPort := flag.String("serial", "/dev/ttyUSB0", "Serial port for mesh communication")
	baudRate := flag.Int("baud", 115200, "Serial baud rate")
	apiPort := flag.Int("port", 8080, "HTTP API port")
	authRegistry := flag.String("auth-registry", "data/nodeauth.json", "Path to node auth registry JSON")
	flag.Parse()

	// Ensure data directory exists for auth registry
	if err := os.MkdirAll("data", 0750); err != nil {
		log.Printf("Warning: Failed to create data directory: %v", err)
	}

	log.Printf("Starting Planetopia Motion Sensor Server")
	log.Printf("Serial: %s @ %d baud", *serialPort, *baudRate)
	log.Printf("API Port: %d", *apiPort)
	log.Printf("Kafka Broker: %s", broker)

	// Setup event store with retry logic
	var eventStore EventStore.EventStore_interface
	maxRetries := 3
	retryDelay := 1 * time.Second
	
	log.Printf("Attempting to connect to Kafka with %d retries...", maxRetries)
	for i := 0; i < maxRetries; i++ {
		eventStore = EventStore.New(broker, groupId)
		err := eventStore.Connect()
		if err == nil {
			log.Printf("Connected to Kafka successfully on attempt %d", i+1)
			break
		}
		
		log.Printf("Kafka connection attempt %d failed: %v", i+1, err)
		if i < maxRetries-1 {
			log.Printf("Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
			eventStore = nil
		}
	}
	
	if eventStore == nil {
		log.Printf("Warning: Failed to connect to Kafka after %d attempts", maxRetries)
		log.Printf("Continuing without Kafka integration...")
	}

	// Setup mesh server
	meshConfig := mesh.MeshServerConfig{
		SerialPort:       *serialPort,
		BaudRate:         *baudRate,
		HealthTimeout:    30 * time.Second,
		EventStore:       eventStore,
		AuthRegistryPath: *authRegistry,
	}

	meshServer := mesh.NewMeshServer(meshConfig)

	// Start mesh server
	if err := meshServer.Start(); err != nil {
		log.Printf("Warning: Failed to start mesh server: %v", err)
		log.Printf("Mesh functionality will be disabled")
	} else {
		log.Printf("Mesh server started successfully")
		
		// Request initial health reports
		time.AfterFunc(2*time.Second, func() {
			if err := meshServer.RequestHealthReports(); err != nil {
				log.Printf("Failed to request initial health reports: %v", err)
			}
		})
	}

	// Start HTTP API server
	go func() {
		if err := mesh.StartAPIServer(meshServer, *apiPort); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	// Setup graceful shutdown

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Server started successfully. Press Ctrl+C to shutdown.")
	
	// Wait for shutdown signal
	<-sigChan
	log.Printf("Shutdown signal received, stopping services...")

	// Stop mesh server
	if meshServer.IsRunning() {
		if err := meshServer.Stop(); err != nil {
			log.Printf("Error stopping mesh server: %v", err)
		}
	}

	log.Printf("Server shutdown complete")
}
