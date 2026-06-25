import type { INodeCardProps } from "~/interfaces/INodes";
import { formatTime } from "~/services/formatDateTime";

export default function NodeCard({ nodeData }: INodeCardProps) {
  return (
    <div className="node-card h-max rounded-2xl bg-emerald-700 p-3">
      <p className="text-center text-2xl">{nodeData.macString}</p>
      <p className="text-xs text-right">mac: {nodeData.mac}</p>
      <p>Type: {nodeData.adapterType}</p>
      <p>Uptime: {nodeData.uptime}s</p>
      <p>Hops: {nodeData.hopCount}</p>
      <p>Last Seen: {formatTime(nodeData.lastSeen)}</p>
    </div>
  );
}
