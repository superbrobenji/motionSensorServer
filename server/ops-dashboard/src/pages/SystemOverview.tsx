import { useEffect, useState } from "react";
import { sidecar, type ContainerInfo } from "../services/sidecar";

export function SystemOverview() {
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [kafkaStatus, setKafkaStatus] = useState<{ reachable: boolean; partitions?: number } | null>(null);
  const [error, setError] = useState("");

  const refresh = async () => {
    try {
      const [c, k] = await Promise.all([
        sidecar.getContainers(),
        sidecar.getKafkaStatus(),
      ]);
      setContainers(c.containers);
      setKafkaStatus(k as any);
      setError("");
    } catch {
      setError("Failed to reach sidecar — check ADMIN_KEY and sidecar status");
    }
  };

  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, 15_000);
    return () => clearInterval(interval);
  }, []);

  const restart = async (name: string) => {
    if (!confirm(`Restart ${name}?`)) return;
    await sidecar.restartContainer(name);
    setTimeout(refresh, 2000);
  };

  return (
    <div className="system-overview">
      {error && <div className="banner error">{error}</div>}
      <h2>Containers</h2>
      <table>
        <thead>
          <tr><th>Name</th><th>Image</th><th>State</th><th>Status</th><th>Actions</th></tr>
        </thead>
        <tbody>
          {containers.map((c) => (
            <tr key={c.id} className={`state-${c.state}`}>
              <td>{c.names[0]?.replace("/", "") ?? c.id}</td>
              <td>{c.image}</td>
              <td>{c.state}</td>
              <td>{c.status}</td>
              <td>
                <button onClick={() => restart(c.names[0] ?? c.id)}>Restart</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <h2>Kafka</h2>
      {kafkaStatus ? (
        <div className={`kafka-status ${kafkaStatus.reachable ? "ok" : "error"}`}>
          {kafkaStatus.reachable
            ? `Reachable — ${kafkaStatus.partitions ?? 0} partitions`
            : "Unreachable"}
        </div>
      ) : (
        <p>Loading…</p>
      )}

      <button onClick={refresh}>Refresh</button>
    </div>
  );
}
