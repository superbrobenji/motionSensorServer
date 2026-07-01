package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	EventStore "github.com/superbrobenji/lattice-hub/eventStore"
	"github.com/superbrobenji/lattice-hub/mesh"
)

func main() {
	broker := envOrDefault("KAFKA_BROKER", "kafka:9092")
	groupId := envOrDefault("KAFKA_GROUP_ID", "1")
	authRegistryPath := envOrDefault("AUTH_REGISTRY_PATH", "data/nodeauth.json")
	nodeRegistryPath := envOrDefault("NODE_REGISTRY_PATH", "data/nodes.json")
	zoneRegistryPath := envOrDefault("ZONE_REGISTRY_PATH", "data/zones.json")
	logLevel := envOrDefault("LOG_LEVEL", "INFO")

	// Configure slog before anything else
	var slogLevel slog.Level
	switch logLevel {
	case "DEBUG":
		slogLevel = slog.LevelDebug
	case "WARN":
		slogLevel = slog.LevelWarn
	case "ERROR":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})))

	// Command line flags
	serialPort := flag.String("serial", envOrDefault("SERIAL_PORT", "/dev/ttyUSB0"), "Serial port for mesh communication")
	baudRate := flag.Int("baud", envOrDefaultInt("BAUD_RATE", 115200), "Serial baud rate")
	serialPortSecondary := envOrDefault("SERIAL_PORT_SECONDARY", "")
	dualMasterEnabled := os.Getenv("DUAL_MASTER_ENABLED") == "true"
	apiPort := flag.Int("port", envOrDefaultInt("API_PORT", 8080), "HTTP API port")
	authRegistry := flag.String("auth-registry", authRegistryPath, "Path to node auth registry JSON")
	nodeRegistry := flag.String("node-registry", nodeRegistryPath, "Path to node registry JSON")
	txPowerPreset := flag.Uint("tx-power", 2, "TX power preset: 0=short_range, 1=indoor, 2=outdoor")
	flag.Parse()

	// Ensure data directory exists for auth registry
	if err := os.MkdirAll("data", 0750); err != nil {
		slog.Warn("Failed to create data directory", "error", err)
	}

	slog.Info("Starting Lattice Motion Sensor Server")
	slog.Info("Serial", "port", *serialPort, "baud", *baudRate)
	if dualMasterEnabled && serialPortSecondary != "" {
		slog.Info("Secondary serial port", "port", serialPortSecondary)
	}
	slog.Info("API Port", "port", *apiPort)
	slog.Info("Kafka Broker", "broker", broker)

	// Setup event store with retry logic
	var eventStore EventStore.EventStoreInterface
	maxRetries := 3
	retryDelay := 1 * time.Second

	slog.Info("Attempting to connect to Kafka", "maxRetries", maxRetries)
	for i := 0; i < maxRetries; i++ {
		eventStore = EventStore.New(broker, groupId)
		err := eventStore.Connect()
		if err == nil {
			slog.Info("Connected to Kafka successfully", "attempt", i+1)
			break
		}

		slog.Warn("Kafka connection attempt failed", "attempt", i+1, "error", err)
		if i < maxRetries-1 {
			slog.Warn("Retrying Kafka connection", "delay", retryDelay)
			time.Sleep(retryDelay)
			eventStore = nil
		}
	}

	if eventStore == nil {
		slog.Warn("Failed to connect to Kafka after all attempts — continuing without Kafka integration", "maxRetries", maxRetries)
	}

	// Setup mesh server
	meshConfig := mesh.MeshServerConfig{
		SerialPort: *serialPort,
		SerialPortSecondary: func() string {
			if dualMasterEnabled && serialPortSecondary != "" {
				return serialPortSecondary
			}
			return ""
		}(),
		BaudRate:         *baudRate,
		HealthTimeout:    75 * time.Second,
		EventStore:       eventStore,
		AuthRegistryPath: *authRegistry,
		NodeRegistryPath: *nodeRegistry,
		ZoneRegistryPath: zoneRegistryPath,
	}

	meshServer := mesh.NewMeshServer(meshConfig)

	// Start mesh server
	if err := meshServer.Start(); err != nil {
		slog.Warn("Failed to start mesh server — mesh functionality will be disabled", "error", err)
	} else {
		slog.Info("Mesh server started successfully")

		// Apply initial TX power preset
		if err := meshServer.SetTxPowerPreset(uint8(*txPowerPreset)); err != nil {
			slog.Warn("Failed to set initial TX power preset", "error", err)
		}

		// Request initial health reports
		time.AfterFunc(2*time.Second, func() {
			if err := meshServer.RequestHealthReports(); err != nil {
				slog.Warn("Failed to request initial health reports", "error", err)
			}
		})
	}

	// Read API key from environment
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		slog.Warn("API_KEY is not set — HTTP API will run without authentication")
	}

	// Read admin key from environment
	adminKey := os.Getenv("ADMIN_KEY")
	if adminKey == "" {
		slog.Warn("ADMIN_KEY is not set — admin endpoints will run without extra authentication")
	}

	// Read allowed CORS origins from environment
	var allowedOrigins []string
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if o != "" {
			allowedOrigins = append(allowedOrigins, o)
		}
	}

	// Start HTTP API server
	shutdownAPI, err := mesh.StartAPIServer(meshServer, *apiPort, apiKey, adminKey, allowedOrigins)
	if err != nil {
		slog.Error("Failed to start API server", "error", err)
		os.Exit(1)
	}

	// Setup graceful shutdown

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	slog.Info("Server started successfully. Press Ctrl+C to shutdown.")

	// Wait for shutdown signal
	<-sigChan
	slog.Info("Shutdown signal received, stopping services...")

	// 1. Gracefully shut down HTTP API server first so in-flight requests
	//    can complete before the underlying mesh server is torn down.
	slog.Info("Shutting down API server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := shutdownAPI(shutdownCtx); err != nil {
		slog.Warn("API server shutdown error", "error", err)
	}

	// 2. Stop mesh server after HTTP is no longer accepting requests.
	if meshServer.IsRunning() {
		if err := meshServer.Stop(); err != nil {
			slog.Warn("Error stopping mesh server", "error", err)
		}
	}

	// 3. Close event store last so any pending Kafka flushes complete.
	if eventStore != nil {
		if err := eventStore.Close(); err != nil {
			slog.Warn("Error closing event store", "error", err)
		}
	}

	slog.Info("Server shutdown complete")
}
