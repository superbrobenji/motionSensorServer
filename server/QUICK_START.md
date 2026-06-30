<!--
SPDX-License-Identifier: GPL-3.0-or-later
Copyright (C) 2026 Planetopia Contributors
-->

# Quick Start

## Prerequisites

- Docker and Docker Compose
- (Optional) ESP32 master node connected via USB for node enrollment

## 1. Environment Setup

```bash
cp env.example .env
```

Edit `.env` and set these required variables:

```
API_KEY=<generate with: openssl rand -hex 32>
ADMIN_KEY=<generate with: openssl rand -hex 32>
```

All other variables have working defaults for local development.

## 2. Start the Stack

```bash
docker compose up -d
```

This starts:
| Service | Port | Description |
|---------|------|-------------|
| Orchestrator API | 8080 | REST API v1 and mesh server |
| Artist Portal | 3001 | Artist workspace UI |
| Ops Dashboard | 3002 | Operations & enrollment management |
| Kafka | 9092 | Event stream (internal) |

## 3. Verify Services

```bash
curl http://localhost:8080/api/v1/status
```

Expected response:
```json
{"status":"ok","data":{"running":true,"totalNodes":0,"onlineNodes":0}}
```

## 4. Enroll a Node

After connecting an ESP32 master node via USB:

### Check Pending Enrollments

```bash
curl http://localhost:8080/api/v1/enrollments/pending
```

### Approve via cURL

```bash
curl -X POST http://localhost:8080/api/v1/enrollments/<enrollment_id>/approve \
  -H "X-Admin-Key: $ADMIN_KEY"
```

Or use the **Ops Dashboard** (step 7) for a UI-based approval workflow.

## 5. Artist Portal

Open your browser and navigate to:

```
http://localhost:3001
```

The Artist Portal is where users create and manage installations, configure nodes, and design motion-reactive visuals.

## 6. Ops Dashboard

Open your browser and navigate to:

```
http://localhost:3002
```

You will be prompted for the `ADMIN_KEY` at login. The Ops Dashboard provides:
- Node enrollment and approval workflow
- Real-time system status
- Health monitoring and diagnostics

## 7. Development with Jupyter

To include Jupyter for development and experimentation, use the development compose override:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d
```

Jupyter will be available at `http://localhost:8888`.

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

### macOS

```bash
# Check for connected device
ls /dev/tty.usbserial* /dev/tty.usbmodem*

# Update .env with the device path
SERIAL_PORT=/dev/tty.usbserial-0001
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

## Troubleshooting

### Services not starting

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

### API returns 401/403

Verify your API keys:
```bash
grep API_KEY .env
grep ADMIN_KEY .env
```

Ensure the correct header is used:
- Public endpoints: no auth required
- Protected endpoints: `-H "X-Admin-Key: $ADMIN_KEY"`

### Clean rebuild

```bash
docker compose down
docker compose build --no-cache
docker compose up -d
```

## Next Steps

See [orchestrator/README.md](orchestrator/README.md) for the full API reference, protocol documentation, and advanced configuration options.
