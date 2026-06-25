import { useFetcher } from "react-router";
import type { Route } from "../+types/root";
import ApiService, { dev_ApiService, getTxPower, setTxPower } from "../services/apiService";
import type { IApiResponse, ITxPowerStatus } from "~/interfaces/IApiService";
import { useState, useEffect } from "react";
import { formatTime } from "~/services/formatDateTime";

export async function loader({ request }: Route.LoaderArgs) {
  // Get server status from API
     const status = await ApiService<{ status: string }>("getStatus");
  //const response = (await dev_ApiService("getStatus")) as IApiResponse;
  console.log("SERVER STATUS: ", status);

  return status;
}

const TX_POWER_OPTIONS = [
  { value: 0, label: "Short Range (2dBm) — same room" },
  { value: 1, label: "Indoor (14dBm) — through walls" },
  { value: 2, label: "Outdoor (20dBm) — maximum range" },
];

function TxPowerSelector() {
  const [current, setCurrent] = useState<ITxPowerStatus | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    getTxPower()
      .then(setCurrent)
      .catch(() => {/* server may not be reachable on load */});
  }, []);

  const handleChange = async (preset: number) => {
    setLoading(true);
    try {
      await setTxPower(preset);
      setCurrent((prev) =>
        prev
          ? { ...prev, preset, name: TX_POWER_OPTIONS[preset]?.label ?? "" }
          : null
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mt-6 p-4 border border-emerald-500 rounded">
      <h2 className="text-lg font-bold mb-2">TX Power Preset</h2>
      <p className="text-sm text-emerald-200 mb-3">
        Sets transmit power on all nodes. Applies immediately — no reboot needed.
      </p>
      {TX_POWER_OPTIONS.map((opt) => (
        <label
          key={opt.value}
          className="flex items-center gap-2 mb-2 cursor-pointer">
          <input
            type="radio"
            name="txPower"
            value={opt.value}
            checked={current?.preset === opt.value}
            onChange={() => handleChange(opt.value)}
            disabled={loading}
          />
          {opt.label}
        </label>
      ))}
      {loading && (
        <p className="text-sm text-blue-300 animate-pulse">
          Applying to all nodes...
        </p>
      )}
    </div>
  );
}

export default function Server({ loaderData }: { loaderData?: IApiResponse }) {
  const fetcher = useFetcher<{ status: string }>();
  const [serverData, setServerData] = useState(
    loaderData?.data ?? { running: false }
  );

  // Show loading state if fetcher is submitting
  const isSubmitting = fetcher.state === "submitting";

  return (
    <div className="max-w-md mx-auto mt-10 p-8 bg-emerald-700 rounded-lg shadow-md text-center">
      <h1 className="text-3xl font-bold mb-4">Server Statuses</h1>
      <p className="text-lg mb-6">
        Running: <span>{serverData.running.toString()}</span>
      </p>
      <p className="text-lg mb-6">
        Total Nodes: <span>{serverData.totalNodes}</span>
      </p>
      <p className="text-lg mb-6">
        Online Nodes: <span>{serverData.onlineNodes}</span>
      </p>
      <p className="text-lg mb-6">
        Last Checked: <span>{formatTime(serverData.timestamp)}</span>
      </p>
      <fetcher.Form method="post" className="flex gap-4 justify-center mb-4">
        <button
          type="submit"
          name="action"
          value="start"
          disabled={isSubmitting}
          className={`px-4 py-2 rounded bg-blue-600 text-white font-medium transition hover:bg-blue-700 disabled:opacity-60 disabled:cursor-not-allowed`}>
          Start Server
        </button>
        <button
          type="submit"
          name="action"
          value="stop"
          disabled={isSubmitting}
          className={`px-4 py-2 rounded bg-red-600 text-white font-medium transition hover:bg-red-700 disabled:opacity-60 disabled:cursor-not-allowed`}>
          Stop Server
        </button>
      </fetcher.Form>
      {isSubmitting && <p className="mt-2 animate-pulse">Processing...</p>}
      <TxPowerSelector />
    </div>
  );
}

// Add an action to handle start/stop requests
export async function action({ request }: Route.ActionArgs) {
  const formData = await request.formData();
  const actionType = formData.get("action");

  if (actionType === "start") {
    await ApiService("startServer", { method: "POST" });
  } else if (actionType === "stop") {
    await ApiService("stopServer", { method: "POST" });
  }

  // Return updated status after action
  const status = await ApiService<{ status: string }>("getStatus");
  return { status };
}
