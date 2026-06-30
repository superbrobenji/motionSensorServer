# API Examples

All examples use the v1 API at `/api/v1/`. Replace `localhost:8080` with your deployment host.

## Authentication

Public endpoints (read-only node and zone data, event stream) require no authentication.

Admin endpoints (enrollment approval, node deletion, zone deletion) require:
```
Authorization: Bearer <ADMIN_KEY>
```

---

## System Status

```bash
curl http://localhost:8080/api/v1/status
```

Response:
```json
{
  "success": true,
  "data": {
    "serial": { "primary": "connected", "secondary": "not_configured" },
    "nodes": { "total": 3, "online": 2, "offline": 1 },
    "mesh": { "masterOnline": true }
  }
}
```

---

## Nodes

### List all nodes
```bash
curl http://localhost:8080/api/v1/nodes
```

### Get a single node
```bash
curl http://localhost:8080/api/v1/nodes/3
```

Response:
```json
{
  "success": true,
  "data": {
    "id": 3,
    "name": "entrance-left",
    "zone": "lobby",
    "online": true,
    "adapterType": "pir",
    "uptime": 3600,
    "hopCount": 1,
    "lastSeen": "2026-06-30T12:00:00Z"
  }
}
```

### Update a node
```bash
curl -X PATCH http://localhost:8080/api/v1/nodes/3 \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "entrance-right", "zone": "lobby"}'
```

### Send command to output node (LED strip)
```bash
curl -X POST http://localhost:8080/api/v1/nodes/4/command \
  -H "Content-Type: application/json" \
  -d '{"action": "led_solid", "colour": [255, 0, 0]}'
```

Response (202 Accepted):
```json
{ "success": true, "data": { "commandId": "a1b2c3d4-..." } }
```

Track acknowledgement via SSE or poll:
```bash
curl http://localhost:8080/api/v1/nodes/4/command/a1b2c3d4-...
```

---

## Zones

### List zones
```bash
curl http://localhost:8080/api/v1/zones
```

### Create a zone
```bash
curl -X POST http://localhost:8080/api/v1/zones \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "stage"}'
```

### Send command to all output nodes in a zone
```bash
curl -X POST http://localhost:8080/api/v1/zones/stage-id/command \
  -H "Content-Type: application/json" \
  -d '{"action": "led_off"}'
```

---

## Enrollment

### List pending enrollments
```bash
curl http://localhost:8080/api/v1/enrollments/pending \
  -H "Authorization: Bearer $API_KEY"
```

Response:
```json
{
  "success": true,
  "data": [{
    "mac": "aa:bb:cc:dd:ee:ff",
    "publicKey": "0102030405...",
    "status": 0,
    "receivedAt": 1751289600,
    "approvedAt": 0
  }]
}
```

### Approve enrollment
```bash
curl -X POST http://localhost:8080/api/v1/enrollments/aa:bb:cc:dd:ee:ff/approve \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "entrance-left", "zone": "lobby", "type": "pir", "nodeId": 3}'
```

### Reject enrollment
```bash
curl -X POST http://localhost:8080/api/v1/enrollments/aa:bb:cc:dd:ee:ff/reject \
  -H "Authorization: Bearer $ADMIN_KEY"
```

---

## Real-time Events (SSE)

```javascript
const es = new EventSource("http://localhost:8080/api/v1/events");

es.addEventListener("motion", (e) => {
  const data = JSON.parse(e.data);
  console.log(`Motion: node ${data.nodeId} (${data.name}) in ${data.zone}`);
});

es.addEventListener("command_ack", (e) => {
  const data = JSON.parse(e.data);
  console.log(`Command ${data.commandId} acknowledged by node ${data.nodeId}`);
});
```

Event types: `motion`, `health`, `node_online`, `node_offline`, `enrolled`, `command_ack`
