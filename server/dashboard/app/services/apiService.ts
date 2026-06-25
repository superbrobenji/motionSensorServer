import type { IEnrollment, ITxPowerStatus } from "../interfaces/IApiService";

const HOST_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";
const API_KEY = import.meta.env.VITE_API_KEY ?? "";

function authHeaders(): HeadersInit {
  return API_KEY ? { Authorization: `Bearer ${API_KEY}` } : {};
}

type ServiceName =
  | "getNodes"
  | "getNode"
  | "configureNode"
  | "configureAllNodes"
  | "requestHealth"
  | "getStatus"
  | "broadcastData"
  | "startServer"
  | "stopServer";

const endpoints: Record<ServiceName, string | ((mac: string) => string)> = {
  // node management
  getNodes: "/nodes",
  getNode: (mac: string) => `/nodes/${mac}`,
  configureNode: (mac: string) => `/nodes/${mac}/configure`,
  configureAllNodes: "/nodes/configure-all",
  // health and monitoring
  requestHealth: "/health/request",
  getStatus: "/status",
  // Data broadcasting
  broadcastData: "/broadcast",
  // server control
  startServer: "/server/start",
  stopServer: "/server/stop",
};

export function buildUrl(service: ServiceName, mac?: string): string {
  const endpoint = endpoints[service];
  if (typeof endpoint === "function") {
    if (!mac) throw new Error(`Service ${service} requires a mac parameter`);
    return `${HOST_URL}${endpoint(mac)}`;
  }
  return `${HOST_URL}${endpoint}`;
}

export default async function ApiService<ApiResponse>(
  service: ServiceName,
  options?: RequestInit,
  mac?: string
): Promise<ApiResponse> {
  const url = buildUrl(service, mac);
  const response: Response = await fetch(url, {
    ...options,
    headers: {
      ...authHeaders(),
      ...(options?.headers ?? {}),
    },
  });
  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }
  return response.json();
}

// Use this one during deving with dummy data

export async function dev_ApiService<ApiResponse>(
  service: ServiceName,
  _options?: RequestInit
): Promise<ApiResponse> {
  const dev_endpoints = {
    // node management
    getNodes: {
      success: true,
      data: [
        {
          mac: "AA:BB:CC:DD:EE:FF",
          name: "Node 1",
          online: true,
          lastSeen: 1693400000,
        },
        {
          mac: "11:22:33:44:55:66",
          name: "Node 2",
          online: false,
          lastSeen: 1693399000,
        },
        {
          mac: "AA:BB:CC:44:55:66",
          name: "Node 3",
          online: true,
          lastSeen: 1693400000,
        },
        {
          mac: "DD:EE:FF:11:22:33",
          name: "Node 4",
          online: false,
          lastSeen: 1693399000,
        },
      ],
    },
    getNode: {
      success: true,
      data: {
        mac: "AA:BB:CC:DD:EE:FF",
        name: "Node 1",
        online: true,
        lastSeen: 1693400000,
        adapterType: 1,
      },
    },
    configureNode: {
      success: true,
      message: "Node AA:BB:CC:DD:EE:FF configured to adapter type WiFi",
    },
    configureAllNodes: {
      success: true,
      message: "All nodes configured to adapter type Bluetooth",
    },
    // health and monitoring
    requestHealth: {
      success: true,
      message: "Health reports requested",
    },
    getStatus: {
      success: true,
      data: {
        running: true,
        totalNodes: 5,
        onlineNodes: 3,
        timestamp: 1693400100,
      },
    },
    // Data broadcasting
    broadcastData: {
      success: true,
      message: "Data broadcasted to all nodes (type: WiFi, length: 128)",
    },
    // server control
    startServer: {
      success: true,
      message: "Mesh server started",
    },
    stopServer: {
      success: true,
      message: "Mesh server stopped",
    },
    errorResponse: {
      success: false,
      error: "This is an error message. Ohh no..",
    },
  };

  return dev_endpoints[service] as ApiResponse;
}

// Enrollment API functions

export async function getPendingEnrollments(): Promise<IEnrollment[]> {
  const res = await fetch(`${HOST_URL}/api/enrollments/pending`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error("Failed to fetch pending enrollments");
  const body = await res.json();
  return body.data ?? [];
}

export async function getAllEnrollments(): Promise<IEnrollment[]> {
  const res = await fetch(`${HOST_URL}/api/enrollments`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error("Failed to fetch enrollments");
  const body = await res.json();
  return body.data ?? [];
}

export async function approveEnrollment(mac: string): Promise<void> {
  const res = await fetch(`${HOST_URL}/api/enrollments/${mac}/approve`, {
    method: "POST",
    headers: authHeaders(),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    const msg =
      body?.error ??
      body?.message ??
      `Failed to approve enrollment for ${mac}`;
    throw new Error(msg);
  }
}

export async function rejectEnrollment(mac: string): Promise<void> {
  const res = await fetch(`${HOST_URL}/api/enrollments/${mac}/reject`, {
    method: "POST",
    headers: authHeaders(),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    const msg =
      body?.error ??
      body?.message ??
      `Failed to reject enrollment for ${mac}`;
    throw new Error(msg);
  }
}

// TX power API functions

export async function getTxPower(): Promise<ITxPowerStatus> {
  const res = await fetch(`${HOST_URL}/api/tx-power`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error("Failed to fetch TX power preset");
  const body = await res.json();
  return body.data as ITxPowerStatus;
}

export async function setTxPower(preset: number): Promise<void> {
  const res = await fetch(`${HOST_URL}/api/tx-power`, {
    method: "POST",
    headers: { ...authHeaders(), "Content-Type": "application/json" },
    body: JSON.stringify({ preset }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    const msg = body?.error ?? `Failed to set TX power preset to ${preset}`;
    throw new Error(msg);
  }
}

// Usage examples:
// callService('service_one');
// callService('service_two', { method: 'POST', body: JSON.stringify(data) });
