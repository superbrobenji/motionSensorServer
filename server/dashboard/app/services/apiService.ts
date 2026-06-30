import type { INode, IZone, IEnrollment } from "../interfaces/INodes";

const BASE_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      Authorization: `Bearer ${import.meta.env.VITE_API_KEY ?? ""}`,
      ...(options?.headers as Record<string, string> ?? {}),
    },
  });
  const body = await res.json();
  if (!body.success) throw new Error(body.error ?? "request failed");
  return body.data as T;
}

export const apiService = {
  getNodes: () => apiFetch<INode[]>("/api/v1/nodes"),
  getNode: (id: number) => apiFetch<INode>(`/api/v1/nodes/${id}`),
  updateNode: (id: number, patch: Partial<{ name: string; zone: string; type: string }>) =>
    apiFetch<INode>(`/api/v1/nodes/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(patch),
    }),
  deleteNode: (id: number) => apiFetch(`/api/v1/nodes/${id}`, { method: "DELETE" }),
  getZones: () => apiFetch<IZone[]>("/api/v1/zones"),
  createZone: (name: string) =>
    apiFetch<IZone>("/api/v1/zones", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    }),
  deleteZone: (id: string) => apiFetch(`/api/v1/zones/${id}`, { method: "DELETE" }),
  getPendingEnrollments: () => apiFetch<IEnrollment[]>("/api/v1/enrollments/pending"),
  approveEnrollment: (mac: string, params: { name?: string; zone?: string; type?: string; nodeId?: number }) =>
    apiFetch(`/api/v1/enrollments/${mac}/approve`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(params),
    }),
  rejectEnrollment: (mac: string) =>
    apiFetch(`/api/v1/enrollments/${mac}/reject`, { method: "POST" }),
  getStatus: () => apiFetch("/api/v1/status"),
};
