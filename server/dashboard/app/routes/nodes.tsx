import ApiService from "~/services/apiService";
import type { IApiResponse } from "~/interfaces/IApiService";
import type { Route } from "../+types/root";
import type { INode, INodes } from "~/interfaces/INodes";
import NodeCard from "~/components/NodeCard/nodeCard";

export async function loader({}: Route.LoaderArgs) {
  const response = (await ApiService("getNodes")) as IApiResponse;

  if (!response.success) {
    throw new Response(response.error ?? "Unknown error", { status: 500 });
  }
  if (!response.data) {
    throw new Response("No nodes found", { status: 404 });
  }

  return (response.data as INodes) ?? [];
}

export default function Nodes({ loaderData }: Route.ComponentProps) {
  const nodes = loaderData as INodes | undefined;
  return (
    <div className="p-6 justify-center">
      <h1 className="text-center">Nodes</h1>
      <br />
      <div className="nodes-container w-[80%] grid grid-cols-3 gap-4 justify-center m-auto">
        {nodes?.map((node: INode, index: number) => (
          <NodeCard key={index} nodeData={node} />
        ))}
      </div>
    </div>
  );
}
