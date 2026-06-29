# Planetopia Orchestrator

Go service that communicates with an ESP32 mesh network over USB serial,
implementing the Planetopia protocol for motion sensor management.

SPDX-License-Identifier: GPL-3.0-or-later

## Features

- USB serial interface to ESP32 master node (protobuf framing, 115200 baud)
- ESP-NOW mesh node management — configure, monitor, and broadcast
- Node health monitoring with configurable online/offline timeout
- Kafka event logging (`motion-trigger`, `mesh-messages` topics)
- Prometheus metrics endpoint
- RESTful HTTP API
- Node authentication with replay-protection persistence

## Architecture

```
┌─────────────────┐    USB Serial    ┌─────────────────┐    ESP-NOW Mesh    ┌─────────────────┐
│   Orchestrator  │ ◄──────────────► │  ESP32 Master   │ ◄─────────────────► │   Mesh Nodes    │
│                 │    115200 baud   │                 │                     │   (PIR, LED)    │
└─────────────────┘                  └─────────────────┘                     └─────────────────┘
         │
         ▼
┌─────────────────┐
│   Kafka Store   │
│  motion-trigger │
│  mesh-messages  │
└─────────────────┘
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERIAL_PORT` | `/dev/ttyUSB0` | Serial port path |
| `BAUD_RATE` | `115200` | Serial baud rate |
| `API_PORT` | `8080` | HTTP API port |
| `KAFKA_BROKER` | `kafka:9092` | Kafka broker address |
| `KAFKA_GROUP_ID` | `1` | Kafka consumer group ID |
| `NODE_REGISTRY_PATH` | `data/nodeauth.json` | Node registry persistence file |
| `API_KEY` | *(required)* | API authentication key — generate with `openssl rand -hex 32` |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | Comma-separated CORS origins |

### Command Line Flags

```bash
./mesh-server -serial=/dev/ttyUSB0 -baud=115200 -port=8080
```

## HTTP API

### Node Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/nodes` | List all known nodes |
| `GET` | `/nodes/{mac}` | Get specific node |
| `POST` | `/nodes/{mac}/configure` | Configure node adapter type |
| `POST` | `/nodes/configure-all` | Configure all nodes |

### Health & Monitoring

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/health/request` | Request health reports from all nodes |
| `GET` | `/status` | Server status and statistics |
| `GET` | `/metrics` | Prometheus metrics |

### Data & Control

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/broadcast` | Broadcast data to all nodes |
| `POST` | `/server/start` | Start mesh communication |
| `POST` | `/server/stop` | Stop mesh communication |

### Authentication

All endpoints require an `Authorization: Bearer` header matching the `API_KEY`
environment variable. Leave `API_KEY` empty only for local development without
Docker.

### Example requests

```bash
# Server status
curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/status

# List nodes
curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/nodes

# Configure a node as PIR sensor (adapterType 0)
curl -X POST -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" \
  -d '{"adapterType": 0}' \
  http://localhost:8080/nodes/aa:bb:cc:dd:ee:ff/configure

# Request health reports
curl -X POST -H "Authorization: Bearer $API_KEY" http://localhost:8080/health/request
```

## Protocol

### Transport

Serial 115200 8N1 with 2-byte little-endian length framing:

```
[2 bytes: length (little-endian)] [N bytes: protobuf message]
```

### Message Types

| Type | Value | Description |
|------|-------|-------------|
| `ADAPTER_DATA` | 0 | Sensor data from mesh nodes |
| `MASTER_BEACON` | 1 | Heartbeat from master node |
| `SERIAL_CMD_BROADCAST` | 3 | Server broadcast commands |

### Adapter Types

| Type | Value | Description |
|------|-------|-------------|
| `UNKNOWN` | -1 | Unknown/unset |
| `PIR` | 0 | Motion sensor |
| `WIFI` | 1 | WiFi adapter (reserved) |
| `LED` | 2 | LED controller (reserved) |
| `SERIAL` | 3 | Serial control messages |

### Control Opcodes

| Opcode | Hex | Format |
|--------|-----|--------|
| `OP_CONFIG_SET` | `0xA0` | `[0xA0][6-byte MAC][adapter type][4 reserved bytes]` |
| `OP_HEALTH_REQ` | `0xB0` | `[0xB0]` |
| `OP_HEALTH_REPORT` | `0xB1` | `[0xB1][adapter type][6-byte MAC][4-byte uptime LE]` |

## Kafka Topics

| Topic | Events |
|-------|--------|
| `motion-trigger` | PIR motion detection events (JSON) |
| `mesh-messages` | All mesh protocol messages for debugging (JSON) |

## Docker Deployment

```bash
# Start all services (from server/)
docker compose up -d

# View orchestrator logs
docker compose logs -f orchestrator

# Stop
docker compose down
```

## Development

### Prerequisites

- Go 1.23+
- Docker (for Kafka dependency)

### Build and test

```bash
cd server/orchestrator
go mod tidy
go test ./...
go vet ./...
go build -o mesh-server .
```

### Serial port permissions (Linux)

```bash
sudo usermod -a -G dialout $USER
# Log out and back in, then:
docker compose up -d
```

## Troubleshooting

**Serial port not found:**
```bash
ls /dev/ttyUSB* /dev/ttyACM*
sudo chmod 666 /dev/ttyUSB0
```

**Kafka connection refused:**
```bash
docker compose ps kafka
docker compose logs kafka
```

**API returns 401:**
Check that `API_KEY` in `server/.env` matches the header value you are sending.

## License

Copyright (C) 2026 Planetopia Contributors.
This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version. See the root [LICENSE](../../LICENSE) file for the full text.
