package mesh

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/superbrobenji/lattice-protocol/opcodes"
)

func (api *APIServer) v1GetZones(w http.ResponseWriter, r *http.Request) {
	zones := api.meshServer.GetZoneRegistry().List()
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: zones})
}

func (api *APIServer) v1CreateZone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		api.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	zone, err := api.meshServer.GetZoneRegistry().Add(body.Name)
	if err != nil {
		api.writeError(w, http.StatusConflict, err.Error())
		return
	}
	api.writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: zone})
}

func (api *APIServer) v1UpdateZone(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		api.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	zone, ok := api.meshServer.GetZoneRegistry().Update(id, body.Name)
	if !ok {
		api.writeError(w, http.StatusNotFound, "zone not found")
		return
	}
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: zone})
}

func (api *APIServer) v1DeleteZone(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if !api.meshServer.GetZoneRegistry().Delete(id) {
		api.writeError(w, http.StatusNotFound, "zone not found")
		return
	}
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Message: "zone deleted"})
}

func (api *APIServer) v1ZoneCommand(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, ok := api.meshServer.GetZoneRegistry().Get(id); !ok {
		api.writeError(w, http.StatusNotFound, "zone not found")
		return
	}
	var body struct {
		Action string `json:"action"`
		Colour []byte `json:"colour"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Action == "" {
		api.writeError(w, http.StatusBadRequest, "action is required")
		return
	}

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

	nodes := api.meshServer.GetNodeRegistry().GetNodesByZone(id)
	sent := 0
	for _, node := range nodes {
		if adapterIsOutput(node.AdapterType) {
			if err := api.meshServer.SendNodeData(node.AdapterType, payload); err == nil {
				sent++
			}
		}
	}
	api.writeJSON(w, http.StatusAccepted, APIResponse{
		Success: true,
		Data:    map[string]int{"sent": sent},
	})
}
