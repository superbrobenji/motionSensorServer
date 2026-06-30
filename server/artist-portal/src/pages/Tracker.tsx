import { useEffect, useState } from "react";
import { api, type Node } from "../services/api";
import { connectSSE, type SSEEvent } from "../services/sse";
import { NodeGrid } from "../components/NodeGrid";
import { EventFeed } from "../components/EventFeed";
import { ServerBanner } from "../components/ServerBanner";

export function Tracker() {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [events, setEvents] = useState<SSEEvent[]>([]);
  const [serverOnline, setServerOnline] = useState<boolean | null>(null);

  // Initial node fetch
  useEffect(() => {
    api.getNodes()
      .then((ns) => { setNodes(ns); setServerOnline(true); })
      .catch(() => setServerOnline(false));
  }, []);

  // SSE — update node grid on health events
  useEffect(() => {
    const disconnect = connectSSE(
      (event) => {
        setServerOnline(true);
        setEvents((prev) => [event, ...prev].slice(0, 200));
        // Refresh node list on health reports (keeps grid current)
        if (event.type === "health" || event.type === "node_online" || event.type === "node_offline") {
          api.getNodes().then(setNodes).catch(() => {});
        }
      },
      () => setServerOnline(false),
    );
    return disconnect;
  }, []);

  return (
    <div className="tracker">
      <ServerBanner online={serverOnline} />
      <section>
        <h2>Node State</h2>
        <NodeGrid nodes={nodes} />
      </section>
      <section>
        <h2>Event Feed</h2>
        <EventFeed events={events} />
      </section>
    </div>
  );
}
