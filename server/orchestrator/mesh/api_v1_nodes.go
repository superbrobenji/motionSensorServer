package mesh

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/superbrobenji/lattice-protocol/opcodes"
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
	node, ok := api.meshServer.GetNodeRegistry().GetNodeByID(id)
	if !ok {
		api.writeError(w, http.StatusNotFound, "node not found")
		return
	}

	var body struct {
		Action string `json:"action"`
		Colour []byte `json:"colour"` // [r, g, b] for led_solid
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Action == "" {
		api.writeError(w, http.StatusBadRequest, "action is required")
		return
	}

	// Validate: only output adapters accept commands
	if !adapterIsOutput(node.AdapterType) {
		api.writeError(w, http.StatusBadRequest, "node adapter type does not accept commands")
		return
	}

	commandID := uuid.New()

	payload := make([]byte, MaxDataLength)
	switch body.Action {
	case "led_solid":
		if len(body.Colour) != 3 {
			api.writeError(w, http.StatusBadRequest, "led_solid requires colour [r,g,b]")
			return
		}
		payload[0] = opcodes.OpLEDSolid
		payload[1] = body.Colour[0]
		payload[2] = body.Colour[1]
		payload[3] = body.Colour[2]
	case "led_off":
		payload[0] = opcodes.OpLEDOff
	case "relay_on":
		payload[0] = opcodes.OpRelaySet
		payload[1] = 0x01
	case "relay_off":
		payload[0] = opcodes.OpRelaySet
		payload[1] = 0x00
	default:
		api.writeError(w, http.StatusBadRequest, "unknown action: "+body.Action)
		return
	}

	// Embed correlation token (low 2 bytes of UUID) at end of payload
	// so node can echo it back in OP_COMMAND_ACK
	idBytes := commandID[14:16]
	payload[MaxDataLength-2] = idBytes[0]
	payload[MaxDataLength-1] = idBytes[1]

	api.meshServer.GetCommandStore().Add(&PendingCommand{
		ID:     commandID.String(),
		NodeID: id,
		Action: body.Action,
		SentAt: time.Now(),
		Status: CommandStatusPending,
	})

	if err := api.meshServer.SendNodeData(node.AdapterType, payload); err != nil {
		api.writeError(w, http.StatusInternalServerError, "failed to send command")
		return
	}
	api.writeJSON(w, http.StatusAccepted, APIResponse{
		Success: true,
		Data:    map[string]string{"commandId": commandID.String()},
	})
}

func (api *APIServer) v1GetCommandStatus(w http.ResponseWriter, r *http.Request) {
	commandID := mux.Vars(r)["commandId"]
	cmd, ok := api.meshServer.GetCommandStore().Get(commandID)
	if !ok {
		api.writeError(w, http.StatusNotFound, "command not found")
		return
	}
	data := map[string]interface{}{
		"commandId": cmd.ID,
		"nodeId":    cmd.NodeID,
		"action":    cmd.Action,
		"status":    string(cmd.Status),
		"sentAt":    cmd.SentAt.Unix(),
	}
	if cmd.AckedAt != nil {
		data["ackedAt"] = cmd.AckedAt.Unix()
	}
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: data})
}

// adapterIsOutput returns true for adapter types that receive commands from the server.
func adapterIsOutput(t int32) bool {
	return t == AdapterTypeLED || t == AdapterTypeRelay
}

// parseNodeID converts a URL path segment to a uint8 node ID (1-255).
func parseNodeID(s string) (uint8, error) {
	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid node id")
	}
	return uint8(n), nil
}
