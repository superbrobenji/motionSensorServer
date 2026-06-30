package mesh

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func (api *APIServer) v1GetNodes(w http.ResponseWriter, r *http.Request) {
	timeout := api.meshServer.GetHealthTimeout()
	nodes := api.meshServer.GetNodeRegistry().GetAllNodes()
	result := make([]NodeV1, 0, len(nodes))
	for _, n := range nodes {
		if n.NodeID > 0 && n.Status != "replaced" { // only include active nodes with assigned IDs
			result = append(result, nodeToV1(n, timeout))
		}
	}
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: result})
}

func (api *APIServer) v1GetNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseNodeID(mux.Vars(r)["id"])
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid node id")
		return
	}
	node, ok := api.meshServer.GetNodeRegistry().GetNodeByID(id)
	if !ok {
		api.writeError(w, http.StatusNotFound, "node not found")
		return
	}
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: nodeToV1(node, api.meshServer.GetHealthTimeout())})
}

func (api *APIServer) v1UpdateNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseNodeID(mux.Vars(r)["id"])
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid node id")
		return
	}
	node, ok := api.meshServer.GetNodeRegistry().GetNodeByID(id)
	if !ok {
		api.writeError(w, http.StatusNotFound, "node not found")
		return
	}

	var body struct {
		Name *string `json:"name"`
		Zone *string `json:"zone"`
		Type *string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 1. Validate type FIRST (before any mutations)
	var adapterType int32
	if body.Type != nil {
		var ok bool
		adapterType, ok = adapterTypeFromString(*body.Type)
		if !ok {
			api.writeError(w, http.StatusBadRequest, "unknown type: "+*body.Type)
			return
		}
	}

	// 2. Then update name/zone
	name := node.Name
	zone := node.Zone
	if body.Name != nil {
		name = *body.Name
	}
	if body.Zone != nil {
		zone = *body.Zone
	}
	api.meshServer.GetNodeRegistry().AssignNode(node.MAC, node.NodeID, name, zone)

	// 3. Then configure type if provided
	if body.Type != nil {
		if err := api.meshServer.ConfigureNode(node.MAC, adapterType); err != nil {
			api.writeError(w, http.StatusInternalServerError, "failed to configure node")
			return
		}
	}

	updated, _ := api.meshServer.GetNodeRegistry().GetNodeByID(id)
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: nodeToV1(updated, api.meshServer.GetHealthTimeout())})
}

func (api *APIServer) v1DeleteNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseNodeID(mux.Vars(r)["id"])
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid node id")
		return
	}
	node, ok := api.meshServer.GetNodeRegistry().GetNodeByID(id)
	if !ok {
		api.writeError(w, http.StatusNotFound, "node not found")
		return
	}
	api.meshServer.GetNodeRegistry().RemoveNode(node.MAC)
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Message: "node removed"})
}

func (api *APIServer) v1NodeCommand(w http.ResponseWriter, r *http.Request) {
	id, err := parseNodeID(mux.Vars(r)["id"])
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid node id")
		return
	}
	if _, ok := api.meshServer.GetNodeRegistry().GetNodeByID(id); !ok {
		api.writeError(w, http.StatusNotFound, "node not found")
		return
	}
	api.writeError(w, http.StatusNotImplemented, "node commands not yet implemented — pending shared protocol repo (Phase 3)")
}

// parseNodeID converts a URL path segment to a uint8 node ID (1-255).
func parseNodeID(s string) (uint8, error) {
	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid node id")
	}
	return uint8(n), nil
}
