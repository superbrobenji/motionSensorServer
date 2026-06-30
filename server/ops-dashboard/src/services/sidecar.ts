// adminKey is set by the operator at login — stored in sessionStorage
function getAdminKey(): string {
  return sessionStorage.getItem("adminKey") ?? "";
}

function sidecarFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const url = (import.meta.env.VITE_SIDECAR_URL ?? "http://localhost:9000") + path;
  return fetch(url, {
    ...options,
    headers: {
      Authorization: `Bearer ${getAdminKey()}`,
      "Content-Type": "application/json",
      ...options?.headers,
    },
  }).then((r) => r.json());
}

export const sidecar = {
  getContainers: () => sidecarFetch<{ containers: ContainerInfo[] }>("/sidecar/containers"),
  restartContainer: (name: string) =>
    sidecarFetch(`/sidecar/containers/${name}/restart`, { method: "POST" }),
  getLogs: (name: string, tail = 100) =>
    fetch(
      (import.meta.env.VITE_SIDECAR_URL ?? "http://localhost:9000") +
        `/sidecar/containers/${name}/logs?tail=${tail}`,
      { headers: { Authorization: `Bearer ${getAdminKey()}` } }
    ).then((r) => r.text()),
  getKafkaStatus: () => sidecarFetch("/sidecar/kafka/status"),
  getRecentEvents: (n = 50) => sidecarFetch(`/sidecar/kafka/events/recent?n=${n}`),
};

export interface ContainerInfo {
  id: string;
  names: string[];
  image: string;
  status: string;
  state: string;
}
