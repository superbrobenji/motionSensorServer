export interface INode {
  mac: string;
  macString: string;
  adapterType: number;
  uptime: number;
  lastSeen: string; // ISO string from Go's time.Time JSON serialization
  hopCount: number;
  online?: boolean;  // derived client-side, not from API
  name?: string;     // not in API, kept optional for dev fixture compatibility
}

export type INodes = INode[];

export interface INodeCardProps {
  nodeData: INode;
}
