import { useFetcher } from "react-router";
import type { Route } from "../+types/root";
import { apiService } from "../services/apiService";
import { useState } from "react";

interface ServerStatus {
  serial: { primary: string; secondary: string };
  nodes: { total: number; online: number; offline: number };
  mesh: { masterOnline: boolean };
}

export async function loader(): Promise<ServerStatus> {
  return apiService.getStatus() as Promise<ServerStatus>;
}

export default function Server({ loaderData }: { loaderData?: ServerStatus }) {
  const fetcher = useFetcher<ServerStatus>();
  const [serverData] = useState<ServerStatus>(
    loaderData ?? {
      serial: { primary: "disconnected", secondary: "not_configured" },
      nodes: { total: 0, online: 0, offline: 0 },
      mesh: { masterOnline: false },
    }
  );

  const isSubmitting = fetcher.state === "submitting";

  return (
    <div className="max-w-md mx-auto mt-10 p-8 bg-emerald-700 rounded-lg shadow-md text-center">
      <h1 className="text-3xl font-bold mb-4">Server Status</h1>
      <p className="text-lg mb-6">Master Online: <span>{String(serverData.mesh.masterOnline)}</span></p>
      <p className="text-lg mb-6">Serial Primary: <span>{serverData.serial.primary}</span></p>
      <p className="text-lg mb-6">Total Nodes: <span>{serverData.nodes.total}</span></p>
      <p className="text-lg mb-6">Online Nodes: <span>{serverData.nodes.online}</span></p>
      <p className="text-lg mb-6">Offline Nodes: <span>{serverData.nodes.offline}</span></p>
      <fetcher.Form method="post" className="flex gap-4 justify-center mb-4">
        <button type="submit" name="action" value="start" disabled={isSubmitting}
          className="px-4 py-2 rounded bg-blue-600 text-white font-medium transition hover:bg-blue-700 disabled:opacity-60 disabled:cursor-not-allowed">
          Start Server
        </button>
        <button type="submit" name="action" value="stop" disabled={isSubmitting}
          className="px-4 py-2 rounded bg-red-600 text-white font-medium transition hover:bg-red-700 disabled:opacity-60 disabled:cursor-not-allowed">
          Stop Server
        </button>
      </fetcher.Form>
      {isSubmitting && <p className="mt-2 animate-pulse">Processing...</p>}
    </div>
  );
}

export async function action({ request }: Route.ActionArgs) {
  const formData = await request.formData();
  const actionType = formData.get("action");

  const baseUrl = import.meta.env.VITE_API_URL ?? "http://localhost:8080";
  const headers: HeadersInit = {
    Authorization: `Bearer ${import.meta.env.VITE_API_KEY ?? ""}`,
  };

  if (actionType === "start") {
    await fetch(`${baseUrl}/server/start`, { method: "POST", headers });
  } else if (actionType === "stop") {
    await fetch(`${baseUrl}/server/stop`, { method: "POST", headers });
  }

  return apiService.getStatus() as Promise<ServerStatus>;
}
