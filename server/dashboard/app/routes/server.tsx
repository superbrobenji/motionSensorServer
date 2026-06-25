import { useFetcher } from "react-router";
import type { Route } from "../+types/root";
import ApiService from "../services/apiService";
import type { IApiResponse } from "~/interfaces/IApiService";
import { useState } from "react";
import { formatTime } from "~/services/formatDateTime";

interface ServerStatus {
  running: boolean;
  totalNodes: number;
  onlineNodes: number;
  timestamp: number;
}

export async function loader({ request: _request }: Route.LoaderArgs) {
  const response = (await ApiService("getStatus")) as IApiResponse<ServerStatus>;
  if (!response.success || !response.data) {
    throw new Response("Failed to get server status", { status: 500 });
  }
  return response.data;
}

export default function Server({ loaderData }: { loaderData?: ServerStatus }) {
  const fetcher = useFetcher<ServerStatus>();
  const [serverData] = useState<ServerStatus>(
    loaderData ?? { running: false, totalNodes: 0, onlineNodes: 0, timestamp: 0 }
  );

  const isSubmitting = fetcher.state === "submitting";

  return (
    <div className="max-w-md mx-auto mt-10 p-8 bg-emerald-700 rounded-lg shadow-md text-center">
      <h1 className="text-3xl font-bold mb-4">Server Status</h1>
      <p className="text-lg mb-6">Running: <span>{String(serverData.running)}</span></p>
      <p className="text-lg mb-6">Total Nodes: <span>{serverData.totalNodes}</span></p>
      <p className="text-lg mb-6">Online Nodes: <span>{serverData.onlineNodes}</span></p>
      <p className="text-lg mb-6">Last Checked: <span>{formatTime(serverData.timestamp)}</span></p>
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

  if (actionType === "start") {
    await ApiService("startServer", { method: "POST" });
  } else if (actionType === "stop") {
    await ApiService("stopServer", { method: "POST" });
  }

  const response = (await ApiService("getStatus")) as IApiResponse<ServerStatus>;
  return response.data ?? { running: false, totalNodes: 0, onlineNodes: 0, timestamp: 0 };
}
