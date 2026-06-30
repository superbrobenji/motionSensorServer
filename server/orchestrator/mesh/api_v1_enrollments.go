package mesh

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/gorilla/mux"
)

func (api *APIServer) v1GetPendingEnrollments(w http.ResponseWriter, r *http.Request) {
	pending := api.meshServer.GetPendingEnrollments()
	out := make([]enrollmentResponse, 0, len(pending))
	for _, n := range pending {
		out = append(out, enrollmentResponse{
			MAC:        n.MACString,
			PublicKey:  fmt.Sprintf("%x", n.PublicKey),
			Status:     int(n.Status),
			ReceivedAt: n.ReceivedAt.Unix(),
			ApprovedAt: n.ApprovedAt.Unix(),
		})
	}
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: out})
}

func (api *APIServer) v1GetAllEnrollments(w http.ResponseWriter, r *http.Request) {
	all := api.meshServer.GetAuthRegistry().GetAll()
	out := make([]enrollmentResponse, 0, len(all))
	for _, n := range all {
		out = append(out, enrollmentResponse{
			MAC:        n.MACString,
			PublicKey:  fmt.Sprintf("%x", n.PublicKey),
			Status:     int(n.Status),
			ReceivedAt: n.ReceivedAt.Unix(),
			ApprovedAt: n.ApprovedAt.Unix(),
		})
	}
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: out})
}

func (api *APIServer) v1ApproveEnrollment(w http.ResponseWriter, r *http.Request) {
	mac := mux.Vars(r)["mac"]
	var body struct {
		Name   string `json:"name"`
		Zone   string `json:"zone"`
		Type   string `json:"type"`
		NodeID uint8  `json:"nodeId"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body) // body optional
	}
	params := ApprovalParams{
		NodeID:         body.NodeID,
		Name:           body.Name,
		Zone:           body.Zone,
		AdapterTypeStr: body.Type,
	}
	if err := api.meshServer.ApproveEnrollment(mac, params); err != nil {
		api.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Type != "" {
		if adapterType, ok := adapterTypeFromString(body.Type); ok {
			hwAddr, err := net.ParseMAC(mac)
			if err == nil {
				if configErr := api.meshServer.ConfigureNode(hwAddr, adapterType); configErr != nil {
					slog.Warn("ConfigureNode failed after enrollment", "mac", mac, "error", configErr)
				}
			}
		}
	}
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Message: "enrollment approved"})
}

func (api *APIServer) v1RejectEnrollment(w http.ResponseWriter, r *http.Request) {
	mac := mux.Vars(r)["mac"]
	if err := api.meshServer.RejectEnrollment(mac); err != nil {
		api.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	api.writeJSON(w, http.StatusOK, APIResponse{Success: true, Message: "enrollment rejected"})
}
