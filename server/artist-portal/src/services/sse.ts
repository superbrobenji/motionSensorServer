const BASE_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export type SSEEvent =
  | { type: "motion"; nodeId: number; name: string; zone: string; timestamp: string }
  | { type: "health"; nodeId: number; name: string; online: boolean; uptime: number }
  | { type: "node_online"; nodeId: number; name: string }
  | { type: "node_offline"; nodeId: number; name: string }
  | { type: "enrolled"; nodeId: number; name: string; adapterType: string }
  | { type: "command_ack"; commandId: string; nodeId: number; status: string };

type SSEHandler = (event: SSEEvent) => void;

export function connectSSE(onEvent: SSEHandler, onError: (err: Event) => void): () => void {
  const es = new EventSource(`${BASE_URL}/api/v1/events`);
  const events: SSEEvent["type"][] = ["motion", "health", "node_online", "node_offline", "enrolled", "command_ack"];
  events.forEach((name) => {
    es.addEventListener(name, (e: MessageEvent) => {
      try {
        onEvent({ type: name, ...JSON.parse(e.data) } as SSEEvent);
      } catch {}
    });
  });
  es.onerror = onError;
  return () => es.close();
}
