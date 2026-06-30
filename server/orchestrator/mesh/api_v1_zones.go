package mesh

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
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
	api.writeError(w, http.StatusNotImplemented, "zone commands not yet implemented — pending shared protocol repo (Phase 3)")
}
