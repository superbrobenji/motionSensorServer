package mesh

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// APIServer provides HTTP API for mesh network control
type APIServer struct {
	meshServer *MeshServer
	router     *mux.Router
}

// NewAPIServer creates a new API server
func NewAPIServer(meshServer *MeshServer) *APIServer {
	api := &APIServer{
		meshServer: meshServer,
		router:     mux.NewRouter(),
	}

	api.setupRoutes()
	return api
}

// setupRoutes configures the HTTP routes
func (api *APIServer) setupRoutes() {
	// Node management
	api.router.HandleFunc("/nodes", api.getNodes).Methods("GET")
	api.router.HandleFunc("/nodes/{mac}", api.getNode).Methods("GET")
	api.router.HandleFunc("/nodes/{mac}/configure", api.configureNode).Methods("POST")
	api.router.HandleFunc("/nodes/configure-all", api.configureAllNodes).Methods("POST")

	// Health and monitoring
	api.router.HandleFunc("/health/request", api.requestHealth).Methods("POST")
	api.router.HandleFunc("/status", api.getStatus).Methods("GET")

	// Data broadcasting
	api.router.HandleFunc("/broadcast", api.broadcastData).Methods("POST")

	// Server control
	api.router.HandleFunc("/server/start", api.startServer).Methods("POST")
	api.router.HandleFunc("/server/stop", api.stopServer).Methods("POST")

	// Enrollment management
	api.router.HandleFunc("/api/enrollments/pending", api.getPendingEnrollments).Methods("GET")
	api.router.HandleFunc("/api/enrollments", api.getAllEnrollments).Methods("GET")
	api.router.HandleFunc("/api/enrollments/{mac}/approve", api.approveEnrollment).Methods("POST")
	api.router.HandleFunc("/api/enrollments/{mac}/reject", api.rejectEnrollment).Methods("POST")

	// TX power
	api.router.HandleFunc("/api/tx-power", api.handleGetTxPower).Methods("GET")
	api.router.HandleFunc("/api/tx-power", api.handleSetTxPower).Methods("POST")
}

// ServeHTTP implements the http.Handler interface
func (api *APIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.router.ServeHTTP(w, r)
}

// enrollmentResponse is the JSON shape for enrollment list entries.
type enrollmentResponse struct {
	MAC        string `json:"mac"`
	PublicKey  string `json:"publicKey"`
	Status     int    `json:"status"`
	ReceivedAt int64  `json:"receivedAt"`
	ApprovedAt int64  `json:"approvedAt"`
}

// Response structures
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type ConfigureRequest struct {
	AdapterType int32 `json:"adapterType"`
}

type BroadcastRequest struct {
	DataType int32  `json:"dataType"`
	Data     []byte `json:"data"`
}

// writeJSON writes a JSON response
func (api *APIServer) writeJSON(w http.ResponseWriter, status int, response APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// writeError writes an error response
func (api *APIServer) writeError(w http.ResponseWriter, status int, message string) {
	api.writeJSON(w, status, APIResponse{
		Success: false,
		Error:   message,
	})
}

// getNodes returns all known nodes
func (api *APIServer) getNodes(w http.ResponseWriter, r *http.Request) {
	nodes := api.meshServer.GetNodeRegistry().GetAllNodes()
	
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    nodes,
	})
}

// getNode returns information about a specific node
func (api *APIServer) getNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	macStr := vars["mac"]
	
	mac, err := StringToMAC(macStr)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid MAC address: %v", err))
		return
	}
	
	node, exists := api.meshServer.GetNodeRegistry().GetNode(mac)
	if !exists {
		api.writeError(w, http.StatusNotFound, "Node not found")
		return
	}
	
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    node,
	})
}

// configureNode configures a specific node's adapter type
func (api *APIServer) configureNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	macStr := vars["mac"]
	
	mac, err := StringToMAC(macStr)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid MAC address: %v", err))
		return
	}
	
	var req ConfigureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	
	if err := api.meshServer.ConfigureNode(mac, req.AdapterType); err != nil {
		api.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to configure node: %v", err))
		return
	}
	
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Node %s configured to adapter type %s", macStr, GetAdapterTypeName(req.AdapterType)),
	})
}

// configureAllNodes configures all nodes' adapter type
func (api *APIServer) configureAllNodes(w http.ResponseWriter, r *http.Request) {
	var req ConfigureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	
	if err := api.meshServer.ConfigureAllNodes(req.AdapterType); err != nil {
		api.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to configure all nodes: %v", err))
		return
	}
	
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("All nodes configured to adapter type %s", GetAdapterTypeName(req.AdapterType)),
	})
}

// requestHealth requests health reports from all nodes
func (api *APIServer) requestHealth(w http.ResponseWriter, r *http.Request) {
	if err := api.meshServer.RequestHealthReports(); err != nil {
		api.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to request health reports: %v", err))
		return
	}
	
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Health reports requested",
	})
}

// getStatus returns the server status and statistics
func (api *APIServer) getStatus(w http.ResponseWriter, r *http.Request) {
	registry := api.meshServer.GetNodeRegistry()
	allNodes := registry.GetAllNodes()
	onlineNodes := registry.GetOnlineNodes(30 * time.Second) // 30 second timeout
	
	status := map[string]interface{}{
		"running":     api.meshServer.IsRunning(),
		"totalNodes":  len(allNodes),
		"onlineNodes": len(onlineNodes),
		"timestamp":   time.Now().Unix(),
	}
	
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    status,
	})
}

// broadcastData broadcasts data to all nodes
func (api *APIServer) broadcastData(w http.ResponseWriter, r *http.Request) {
	var req BroadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	
	if err := api.meshServer.BroadcastData(req.DataType, req.Data); err != nil {
		api.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to broadcast data: %v", err))
		return
	}
	
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Data broadcasted to all nodes (type: %s, length: %d)", 
			GetAdapterTypeName(req.DataType), len(req.Data)),
	})
}

// startServer starts the mesh server
func (api *APIServer) startServer(w http.ResponseWriter, r *http.Request) {
	if api.meshServer.IsRunning() {
		api.writeError(w, http.StatusConflict, "Server is already running")
		return
	}
	
	if err := api.meshServer.Start(); err != nil {
		api.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start server: %v", err))
		return
	}
	
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Mesh server started",
	})
}

// stopServer stops the mesh server
func (api *APIServer) stopServer(w http.ResponseWriter, r *http.Request) {
	if !api.meshServer.IsRunning() {
		api.writeError(w, http.StatusConflict, "Server is not running")
		return
	}
	
	if err := api.meshServer.Stop(); err != nil {
		api.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop server: %v", err))
		return
	}
	
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Mesh server stopped",
	})
}

// getPendingEnrollments returns all nodes awaiting enrollment approval
func (api *APIServer) getPendingEnrollments(w http.ResponseWriter, r *http.Request) {
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
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    out,
	})
}

// getAllEnrollments returns all enrollment records (pending, approved, rejected)
func (api *APIServer) getAllEnrollments(w http.ResponseWriter, r *http.Request) {
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
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    out,
	})
}

// approveEnrollment approves a pending node enrollment
func (api *APIServer) approveEnrollment(w http.ResponseWriter, r *http.Request) {
	mac := mux.Vars(r)["mac"]
	if err := api.meshServer.ApproveEnrollment(mac); err != nil {
		api.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Enrollment approved for %s", mac),
	})
}

// rejectEnrollment rejects a pending node enrollment
func (api *APIServer) rejectEnrollment(w http.ResponseWriter, r *http.Request) {
	mac := mux.Vars(r)["mac"]
	if err := api.meshServer.RejectEnrollment(mac); err != nil {
		api.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Enrollment rejected for %s", mac),
	})
}

// handleGetTxPower returns the current TX power preset and available options
func (api *APIServer) handleGetTxPower(w http.ResponseWriter, r *http.Request) {
	preset, name := api.meshServer.GetTxPowerPreset()
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"preset": preset,
			"name":   name,
		},
	})
}

// handleSetTxPower sets the TX power preset on all nodes
func (api *APIServer) handleSetTxPower(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Preset uint8 `json:"preset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if err := api.meshServer.SetTxPowerPreset(body.Preset); err != nil {
		api.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, name := api.meshServer.GetTxPowerPreset()
	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"status": "ok",
			"preset": body.Preset,
			"name":   name,
		},
	})
}

// StartAPIServer starts the HTTP API server
func StartAPIServer(meshServer *MeshServer, port int) error {
	api := NewAPIServer(meshServer)

	log.Printf("Starting API server on port %d", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), api)
}
