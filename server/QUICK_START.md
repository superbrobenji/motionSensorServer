# Quick Start Guide

## Linux/Unix Setup Guide

## Fixed Docker Compose Issues

The Docker Compose configuration has been updated to fix the `jupyter` image issue and make the setup more flexible.

## Getting Started

### 1. Basic Setup (No Serial Device)

If you just want to test the API and Kafka integration without a physical ESP32:

```bash
cd server
docker-compose up -d
```

This will start:
- **Kafka** on port 9092
- **Jupyter Notebook** on port 8888
- **Mesh Server API** on port 8080

### 2. With USB Serial Device

If you have an ESP32 connected via USB:

#### Standard Linux Setup:
1. **Find your serial port:**
   ```bash
   ls /dev/ttyUSB* /dev/ttyACM*
   ```

2. **Set permissions:**
   ```bash
   sudo chmod 666 /dev/ttyUSB0
   # or add yourself to dialout group
   sudo usermod -a -G dialout $USER
   ```

3. **Start services:**
   ```bash
   cd server
   docker-compose up -d
   ```

#### Proxmox Container Setup:
If running in a Proxmox container with USB passthrough:

1. **Find your USB device:**
   ```bash
   ls /dev/bus/usb/*/
   # Example output: /dev/bus/usb/003/002
   ```

2. **Check device permissions:**
   ```bash
   ls -la /dev/bus/usb/003/002
   # Should show: crw-rw-r-- 1 root root 189, 257 ...
   ```

3. **Update docker-compose.yml:**
   The docker-compose.yml file maps your USB device to `/dev/ttyUSB0` inside the container:
   ```yaml
   devices:
     - "/dev/bus/usb/003/002:/dev/ttyUSB0"  # Your USB device path
   user: "0:0"  # Run as root for USB access
   privileged: true
   ```

4. **Start services:**
   ```bash
   cd server
   docker-compose up -d
   ```

5. **If you still get permission errors:**
   ```bash
   # Option 1: Make device world-writable (temporary fix)
   sudo chmod 666 /dev/bus/usb/003/002
   
   # Option 2: Add your user to dialout group
   sudo usermod -a -G dialout $USER
   # Then logout and login again
   ```

### 3. Environment Variables

Create a `.env` file in the `server` directory:
```bash
cp env.example .env
# Edit .env with your settings
```

Example `.env`:
```
SERIAL_PORT=/dev/ttyUSB0
BAUD_RATE=115200
API_PORT=8080
```

## Testing the Setup

### 1. Check if services are running:
```bash
docker-compose ps
```

### 2. Test the API:
```bash
curl http://localhost:8080/status
```

Expected response:
```json
{
  "success": true,
  "data": {
    "running": false,
    "totalNodes": 0,
    "onlineNodes": 0,
    "timestamp": 1704067200
  }
}
```

### 3. Start the mesh server:
```bash
curl -X POST http://localhost:8080/server/start
```

### 4. Check nodes (after connecting ESP32):
```bash
curl http://localhost:8080/nodes
```

### 5. Request health reports:
```bash
curl -X POST http://localhost:8080/health/request
```

## Accessing Services

- **Mesh Server API**: http://localhost:8080
- **Jupyter Notebook**: http://localhost:8888
- **Kafka**: localhost:9092 (internal)

## Troubleshooting

### Docker Issues
```bash
# View logs
docker-compose logs -f orchestrator

# Restart services
docker-compose restart

# Clean rebuild
docker-compose down
docker-compose build --no-cache
docker-compose up -d
```

### Serial Port Issues
```bash
# Check if device exists
ls -la /dev/ttyUSB*

# Check permissions
ls -la /dev/ttyUSB0

# Fix permissions
sudo chmod 666 /dev/ttyUSB0
```

### API Not Responding
```bash
# Check if container is running
docker-compose ps orchestrator

# Check container logs
docker-compose logs orchestrator

# Test from inside container
docker-compose exec orchestrator curl localhost:8080/status
```

## Next Steps

1. Connect your ESP32 master node via USB
2. Configure nodes using the API
3. Monitor PIR events in Kafka
4. Use Jupyter notebooks for data analysis

See `orchestrator/README.md` for complete documentation.
