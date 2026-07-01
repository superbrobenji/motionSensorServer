<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Lattice Dashboard

React Router v7 web application for monitoring and controlling the Lattice
motion sensor mesh network. Communicates with the orchestrator REST API.

## Features

- Node list with live status (online/offline, adapter type, uptime)
- Enrollment management — approve or reject pending nodes
- Server start/stop control
- Automatic polling of the orchestrator API

## Environment Variables

| Variable | Description |
|----------|-------------|
| `VITE_API_URL` | Orchestrator base URL (e.g. `http://localhost:8080`) |
| `VITE_API_KEY` | Must match the orchestrator's `API_KEY` |

Copy `server/env.example` to `server/.env` and set both values before starting.

## Development

Prerequisites: Node.js LTS, npm.

```bash
cd server/dashboard
npm install
npm run dev
```

The dev server starts at `http://localhost:5173` with hot module replacement.
Set `VITE_API_URL` and `VITE_API_KEY` in `server/.env` to connect to the
orchestrator.

### Type checking

```bash
npm run typecheck
```

### Linting

```bash
npm run lint
```

## Production (Docker)

The dashboard is served as a containerised Node.js app. Run it via Docker
Compose from `server/`:

```bash
docker compose up -d dashboard
```

The service listens on port `3000` and connects to the orchestrator container
on the internal `kafka-net` Docker network using the `VITE_API_URL` environment
variable set in `server/.env`.

### Manual Docker build

```bash
docker build -t lattice-dashboard server/dashboard/
docker run -p 3000:3000 \
  -e VITE_API_URL=http://localhost:8080 \
  -e VITE_API_KEY=your-key \
  lattice-dashboard
```

## Project Structure

```
app/
├── components/
│   ├── Navigation/    # Top navigation bar
│   └── NodeCard/      # Per-node status card
├── interfaces/        # TypeScript interfaces (IApiService, INodes)
├── routes/            # File-based routes (nodes, enrollments, server, welcome)
└── services/
    ├── apiService.ts  # Orchestrator API client
    └── formatDateTime.ts
```

## License

Copyright (C) 2026 Lattice Contributors.
GNU General Public License v3.0 — see root [LICENSE](../../LICENSE).
