import { useEffect, useState } from "react";
import { sidecar } from "../services/sidecar";

interface KafkaEvent {
  offset: number;
  timestamp: number;
  value: string;
}

export function KafkaEvents() {
  const [events, setEvents] = useState<KafkaEvent[]>([]);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const data: any = await sidecar.getRecentEvents(100);
      setEvents(data.events ?? []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  return (
    <div className="kafka-events">
      <h2>Recent Kafka Events (motion-trigger)</h2>
      <button onClick={load} disabled={loading}>Refresh</button>
      <table>
        <thead><tr><th>Offset</th><th>Time</th><th>Payload</th></tr></thead>
        <tbody>
          {events.map((e) => (
            <tr key={e.offset}>
              <td>{e.offset}</td>
              <td>{new Date(e.timestamp * 1000).toLocaleString()}</td>
              <td><pre>{e.value}</pre></td>
            </tr>
          ))}
          {!events.length && <tr><td colSpan={3}>No events</td></tr>}
        </tbody>
      </table>
    </div>
  );
}
