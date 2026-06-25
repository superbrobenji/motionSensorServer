# Planetopia Mesh Server API Examples

This document provides examples of how to interact with the Planetopia Mesh Server HTTP API.

## Base URL

```
http://localhost:8080
```

## Authentication

Currently, the API does not require authentication. For production use, consider adding authentication middleware.

## API Endpoints

### 1. Server Status

Get the current status of the mesh server:

```bash
curl -X GET http://localhost:8080/status
```

Response:
```json
{
  "success": true,
  "data": {
    "running": true,
    "totalNodes": 5,
    "onlineNodes": 3,
    "timestamp": 1704067200
  }
}
```

### 2. List All Nodes

Get information about all known mesh nodes:

```bash
curl -X GET http://localhost:8080/nodes
```

Response:
```json
{
  "success": true,
  "data": [
    {
      "mac": "aa:bb:cc:dd:ee:ff",
      "macString": "aa:bb:cc:dd:ee:ff",
      "adapterType": 0,
      "uptime": 3600,
      "lastSeen": "2024-01-01T12:00:00Z",
      "hopCount": 1
    },
    {
      "mac": "11:22:33:44:55:66",
      "macString": "11:22:33:44:55:66",
      "adapterType": 0,
      "uptime": 7200,
      "lastSeen": "2024-01-01T12:05:00Z",
      "hopCount": 2
    }
  ]
}
```

### 3. Get Specific Node

Get information about a specific node by MAC address:

```bash
curl -X GET http://localhost:8080/nodes/aa:bb:cc:dd:ee:ff
```

Response:
```json
{
  "success": true,
  "data": {
    "mac": "aa:bb:cc:dd:ee:ff",
    "macString": "aa:bb:cc:dd:ee:ff",
    "adapterType": 0,
    "uptime": 3600,
    "lastSeen": "2024-01-01T12:00:00Z",
    "hopCount": 1
  }
}
```

### 4. Configure Single Node

Configure a specific node's adapter type:

```bash
curl -X POST http://localhost:8080/nodes/aa:bb:cc:dd:ee:ff/configure \
  -H "Content-Type: application/json" \
  -d '{
    "adapterType": 0
  }'
```

Adapter Types:
- `-1`: Unknown
- `0`: PIR (Motion Sensor)
- `1`: WiFi (Reserved)
- `2`: LED (Reserved)
- `3`: Serial (Control)

Response:
```json
{
  "success": true,
  "message": "Node aa:bb:cc:dd:ee:ff configured to adapter type PIR"
}
```

### 5. Configure All Nodes

Configure all nodes in the mesh to the same adapter type:

```bash
curl -X POST http://localhost:8080/nodes/configure-all \
  -H "Content-Type: application/json" \
  -d '{
    "adapterType": 0
  }'
```

Response:
```json
{
  "success": true,
  "message": "All nodes configured to adapter type PIR"
}
```

### 6. Request Health Reports

Request immediate health reports from all nodes:

```bash
curl -X POST http://localhost:8080/health/request
```

Response:
```json
{
  "success": true,
  "message": "Health reports requested"
}
```

### 7. Broadcast Data

Broadcast custom data to all nodes in the mesh:

```bash
curl -X POST http://localhost:8080/broadcast \
  -H "Content-Type: application/json" \
  -d '{
    "dataType": 0,
    "data": [1, 2, 3, 4, 5]
  }'
```

Response:
```json
{
  "success": true,
  "message": "Data broadcasted to all nodes (type: PIR, length: 5)"
}
```

### 8. Start Server

Start the mesh communication (if stopped):

```bash
curl -X POST http://localhost:8080/server/start
```

Response:
```json
{
  "success": true,
  "message": "Mesh server started"
}
```

### 9. Stop Server

Stop the mesh communication:

```bash
curl -X POST http://localhost:8080/server/stop
```

Response:
```json
{
  "success": true,
  "message": "Mesh server stopped"
}
```

## Error Responses

All error responses follow this format:

```json
{
  "success": false,
  "error": "Error message describing what went wrong"
}
```

Common HTTP status codes:
- `200`: Success
- `400`: Bad Request (invalid input)
- `404`: Not Found (node not found)
- `409`: Conflict (server already running/stopped)
- `500`: Internal Server Error

## Example Workflow

Here's a typical workflow for setting up and monitoring a mesh network:

### 1. Check Server Status
```bash
curl http://localhost:8080/status
```

### 2. Request Health Reports
```bash
curl -X POST http://localhost:8080/health/request
```

### 3. Wait a few seconds, then check nodes
```bash
curl http://localhost:8080/nodes
```

### 4. Configure all nodes as PIR sensors
```bash
curl -X POST http://localhost:8080/nodes/configure-all \
  -H "Content-Type: application/json" \
  -d '{"adapterType": 0}'
```

### 5. Monitor specific node
```bash
curl http://localhost:8080/nodes/aa:bb:cc:dd:ee:ff
```

## Integration with Other Systems

### Webhook Integration

You can create a simple monitoring script that polls the API:

```bash
#!/bin/bash
# monitor_nodes.sh

while true; do
  echo "Checking node status at $(date)"
  curl -s http://localhost:8080/nodes | jq '.data[] | {mac: .macString, type: .adapterType, uptime: .uptime}'
  sleep 30
done
```

### Python Integration

```python
import requests
import time

class MeshServerClient:
    def __init__(self, base_url="http://localhost:8080"):
        self.base_url = base_url
    
    def get_status(self):
        response = requests.get(f"{self.base_url}/status")
        return response.json()
    
    def get_nodes(self):
        response = requests.get(f"{self.base_url}/nodes")
        return response.json()
    
    def configure_node(self, mac, adapter_type):
        data = {"adapterType": adapter_type}
        response = requests.post(
            f"{self.base_url}/nodes/{mac}/configure",
            json=data
        )
        return response.json()
    
    def request_health(self):
        response = requests.post(f"{self.base_url}/health/request")
        return response.json()

# Usage example
client = MeshServerClient()
status = client.get_status()
print(f"Server running: {status['data']['running']}")

nodes = client.get_nodes()
print(f"Total nodes: {len(nodes['data'])}")
```

## Troubleshooting

### Common Issues

1. **Node not responding**: Check if the node is powered and within range
2. **Configuration not applied**: Nodes reboot after configuration; wait 10-15 seconds
3. **Health reports missing**: Request health reports manually with `/health/request`
4. **Serial connection issues**: Check Docker container has access to `/dev/ttyUSB0`

### Debugging

Enable verbose logging by checking Docker logs:

```bash
docker-compose logs -f orchestrator
```

Monitor Kafka messages for debugging:

```bash
# Connect to Kafka container
docker-compose exec kafka kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic mesh-messages \
  --from-beginning
```
