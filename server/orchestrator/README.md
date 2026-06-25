# Planetopia Mesh Server

A Go-based server that communicates with ESP32 mesh networks over USB serial, implementing the Planetopia protocol for motion sensor management.

## Features

- **Serial Communication**: USB serial interface with ESP32 master node using protobuf framing
- **Mesh Network Management**: Control and monitor ESP-NOW mesh nodes
- **Node Configuration**: Set adapter types (PIR, LED, etc.) on individual or all nodes
- **Health Monitoring**: Track node status, uptime, and connectivity
- **Kafka Integration**: Log events and messages for monitoring and analytics
- **HTTP API**: RESTful API for remote control and status monitoring
- **Docker Support**: Containerized deployment with Docker Compose

## Architecture

```
┌─────────────────┐    USB Serial    ┌─────────────────┐    ESP-NOW Mesh    ┌─────────────────┐
│   Mesh Server   │ ◄─────────────► │  ESP32 Master   │ ◄─────────────────► │   Mesh Nodes    │
│                 │    115200 baud   │                 │                     │   (PIR, LED)    │
└─────────────────┘                  └─────────────────┘                     └─────────────────┘
         │
         ▼
┌─────────────────┐
│   Kafka Store   │
│   (Events &     │
│    Messages)    │
└─────────────────┘
```

## Protocol

The server implements the Planetopia mesh protocol:

- **Transport**: Serial 115200 8N1 with 2-byte little-endian length framing
- **Encoding**: Protocol Buffers for message serialization
- **Message Types**: Adapter data, master beacons, broadcast commands
- **Control Opcodes**: Node configuration, health requests/reports

### Message Types

| Type | Value | Description |
|------|-------|-------------|
| ADAPTER_DATA | 0 | Normal sensor data from nodes |
| MASTER_BEACON | 1 | Heartbeat from master node |
| SERIAL_CMD_BROADCAST | 3 | Server broadcast commands |

### Adapter Types

| Type | Value | Description |
|------|-------|-------------|
| UNKNOWN | -1 | Unknown/unset adapter |
| PIR | 0 | Motion sensor |
| WIFI | 1 | WiFi adapter (reserved) |
| LED | 2 | LED controller (reserved) |
| SERIAL | 3 | Serial control messages |

## Configuration

### Environment Variables

- `SERIAL_PORT`: Serial port path (default: `/dev/ttyUSB0`)
- `BAUD_RATE`: Serial baud rate (default: `115200`)
- `API_PORT`: HTTP API port (default: `8080`)
- `KAFKA_BROKER`: Kafka broker address (default: `kafka:9094`)

### Command Line Flags

```bash
./main -serial=/dev/ttyUSB0 -baud=115200 -port=8080
```

## HTTP API

### Node Management

- `GET /nodes` - List all known nodes
- `GET /nodes/{mac}` - Get specific node information
- `POST /nodes/{mac}/configure` - Configure node adapter type
- `POST /nodes/configure-all` - Configure all nodes

### Health & Monitoring

- `POST /health/request` - Request health reports from all nodes
- `GET /status` - Get server status and statistics

### Data Broadcasting

- `POST /broadcast` - Broadcast data to all nodes

### Server Control

- `POST /server/start` - Start mesh communication
- `POST /server/stop` - Stop mesh communication

### API Examples

#### Configure a node as PIR sensor:
```bash
curl -X POST http://localhost:8080/nodes/aa:bb:cc:dd:ee:ff/configure \
  -H "Content-Type: application/json" \
  -d '{"adapterType": 0}'
```

#### Request health reports:
```bash
curl -X POST http://localhost:8080/health/request
```

#### Get all nodes:
```bash
curl http://localhost:8080/nodes
```

#### Get server status:
```bash
curl http://localhost:8080/status
```

## Docker Deployment

### Using Docker Compose (Recommended)

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f orchestrator

# Stop services
docker-compose down
```

### Manual Docker Build

```bash
# Build image
docker build -t planetopia-mesh-server .

# Run container
docker run -d \
  --name mesh-server \
  --device /dev/ttyUSB0:/dev/ttyUSB0 \
  --privileged \
  -p 8080:8080 \
  -e SERIAL_PORT=/dev/ttyUSB0 \
  planetopia-mesh-server
```

## Development

### Prerequisites

- Go 1.23+
- USB serial device (ESP32 master node)
- Kafka (for event logging)

### Build

```bash
go mod tidy
go build -o mesh-server .
```

### Run

```bash
./mesh-server -serial=/dev/ttyUSB0
```

## Kafka Topics

The server publishes to these Kafka topics:

- `motion-trigger`: PIR motion detection events
- `mesh-messages`: All mesh protocol messages (debugging)

## Troubleshooting

### Serial Port Issues

- Ensure the serial device exists: `ls -la /dev/ttyUSB*`
- Check permissions: `sudo chmod 666 /dev/ttyUSB0`
- Verify ESP32 connection and baud rate

### Docker Serial Access

- Use `--privileged` flag or add user to `dialout` group
- Map the correct device: `--device /dev/ttyUSB0:/dev/ttyUSB0`

### Kafka Connection Issues

- Verify Kafka is running: `docker-compose ps kafka`
- Check network connectivity: `docker-compose exec orchestrator ping kafka`
- Review Kafka logs: `docker-compose logs kafka`

## Protocol Details

### Frame Format

```
[2 bytes: length (little-endian)] [N bytes: protobuf message]
```

### Health Report Format

```
Byte 0: 0xB1 (OP_HEALTH_REPORT)
Byte 1: adapter type (int8)
Bytes 2-7: MAC address (6 bytes)
Bytes 8-11: uptime seconds (uint32 LE)
```

### Config Set Format

```
Byte 0: 0xA0 (OP_CONFIG_SET)
Bytes 1-6: target MAC address
Byte 7: adapter type (int8)
Bytes 8-11: reserved (0x00)
```

## License

This project is part of the Planetopia motion sensor system.
