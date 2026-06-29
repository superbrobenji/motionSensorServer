<!--
SPDX-License-Identifier: GPL-3.0-or-later
Copyright (C) 2026 Planetopia Contributors
-->

# Quick Start

## Prerequisites

- Docker and Docker Compose
- (Optional) ESP32 master node connected via USB

## 1. Configure environment

```bash
cp env.example .env
```

Open `.env` and set at minimum:

```
API_KEY=<generate with: openssl rand -hex 32>
```

All other variables have working defaults for local development.

## 2. Start services

```bash
docker compose up -d
```

This starts:
| Service | Port | Description |
|---------|------|-------------|
| Orchestrator API | 8080 | REST API and mesh server |
| Dashboard | 3000 | Web UI |
| Kafka | 9092 | Event stream |
| Jupyter | 8888 | Notebook environment |

## 3. Verify

```bash
curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/status
```

Expected:
```json
{"success":true,"data":{"running":false,"totalNodes":0,"onlineNodes":0,"timestamp":1704067200}}
```

## 4. Start the mesh server

```bash
curl -X POST -H "Authorization: Bearer $API_KEY" http://localhost:8080/server/start
```

## USB Serial Device Setup

### Standard Linux

```bash
# Find your device
ls /dev/ttyUSB* /dev/ttyACM*

# Grant access
sudo usermod -a -G dialout $USER
# Log out and back in

# Update .env
SERIAL_PORT=/dev/ttyUSB0
```

### Proxmox Container

If running inside a Proxmox LXC with USB passthrough:

```bash
# Find the USB device path
ls /dev/bus/usb/*/

# Example: /dev/bus/usb/003/002
# Update docker-compose.yml devices section:
#   devices:
#     - "/dev/bus/usb/003/002:/dev/ttyUSB0"
```

## Checking nodes

After connecting an ESP32 master node:

```bash
# List enrolled nodes
curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/nodes

# Request health reports from all nodes
curl -X POST -H "Authorization: Bearer $API_KEY" http://localhost:8080/health/request
```

## Troubleshooting

### Service not starting

```bash
docker compose ps
docker compose logs orchestrator
docker compose logs kafka
```

### Serial port errors

```bash
ls -la /dev/ttyUSB0
sudo chmod 666 /dev/ttyUSB0
```

### API returns 401

`API_KEY` in `.env` does not match the header. Verify with:
```bash
grep API_KEY .env
```

### Clean rebuild

```bash
docker compose down
docker compose build --no-cache
docker compose up -d
```

See [orchestrator/README.md](orchestrator/README.md) for full API reference and
protocol documentation.
