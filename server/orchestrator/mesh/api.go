package mesh

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

const maxRequestBodyBytes = 64 * 1024 // 64KB

func isValidAdapterType(t int32) bool {
	switch t {
	case AdapterTypeUnknown, AdapterTypeSerial, AdapterTypePIR, AdapterTypeLED, AdapterTypeRelay, AdapterTypeWIFI:
		return true
	}
	return false
}

// APIServer provides HTTP API for mesh network control
type APIServer struct {
	meshServer *MeshServer
	router     *mux.Router
	apiKey     string
}

// NewAPIServer creates a new API server
func NewAPIServer(meshServer *MeshServer, apiKey string, allowedOrigins []string) *APIServer {
	api := &APIServer{
		meshServer: meshServer,
		router:     mux.NewRouter(),
		apiKey:     apiKey,
	}
	if len(allowedOrigins) > 0 {
		api.router.Use(CORSMiddleware(allowedOrigins))
	}
	api.setupRoutes()
	return api
}

// setupRoutes configures the HTTP routes
func (api *APIServer) setupRoutes() {
	// Public endpoints — no auth required
	// /metrics: Prometheus scrapers don't send Bearer tokens
	api.router.Handle("/metrics", MetricsHandler())
	// /health: used by Docker Compose and Dockerfile HEALTHCHECK; must not require auth
	// so the container is marked healthy before any dependent services start.
	api.router.HandleFunc("/health", api.getHealth).Methods("GET")

	// All other routes — wrapped with auth when an API key is configured
	sub := api.router.PathPrefix("").Subrouter()
	if api.apiKey != "" {
		sub.Use(AuthMiddleware(api.apiKey))
	}

	// Node management
	sub.Handle("/nodes", InstrumentHandler("/nodes", http.HandlerFunc(api.getNodes))).Methods("GET")
	sub.Handle("/nodes/{mac}", InstrumentHandler("/nodes/{mac}", http.HandlerFunc(api.getNode))).Methods("GET")
	sub.Handle("/nodes/{mac}/configure", InstrumentHandler("/nodes/{mac}/configure", http.HandlerFunc(api.configureNode))).Methods("POST")
	sub.Handle("/nodes/configure-all", InstrumentHandler("/nodes/configure-all", http.HandlerFunc(api.configureAllNodes))).Methods("POST")

	// Health and monitoring
	sub.Handle("/health/request", InstrumentHandler("/health/request", http.HandlerFunc(api.requestHealth))).Methods("POST")
	sub.Handle("/status", InstrumentHandler("/status", http.HandlerFunc(api.getStatus))).Methods("GET")

	// Data broadcasting
	sub.Handle("/broadcast", InstrumentHandler("/broadcast", http.HandlerFunc(api.broadcastData))).Methods("POST")

	// Server control
	sub.Handle("/server/start", InstrumentHandler("/server/start", http.HandlerFunc(api.startServer))).Methods("POST")
	sub.Handle("/server/stop", InstrumentHandler("/server/stop", http.HandlerFunc(api.stopServer))).Methods("POST")

	// Enrollment management
	sub.Handle("/api/enrollments/pending", InstrumentHandler("/api/enrollments/pending", http.HandlerFunc(api.getPendingEnrollments))).Methods("GET")
	sub.Handle("/api/enrollments", InstrumentHandler("/api/enrollments", http.HandlerFunc(api.getAllEnrollments))).Methods("GET")
	sub.Handle("/api/enrollments/{mac}/approve", InstrumentHandler("/api/enrollments/{mac}/approve", http.HandlerFunc(api.approveEnrollment))).Methods("POST")
	sub.Handle("/api/enrollments/{mac}/reject", InstrumentHandler("/api/enrollments/{mac}/reject", http.HandlerFunc(api.rejectEnrollment))).Methods("POST")

	// TX power
	sub.Handle("/api/tx-power", InstrumentHandler("/api/tx-power", http.HandlerFunc(api.handleGetTxPower))).Methods("GET")
	sub.Handle("/api/tx-power", InstrumentHandler("/api/tx-power", http.HandlerFunc(api.handleSetTxPower))).Methods("POST")

	// /api/v1/zones
	sub.Handle("/api/v1/zones", InstrumentHandler("/api/v1/zones", http.HandlerFunc(api.v1GetZones))).Methods("GET")
	sub.Handle("/api/v1/zones", InstrumentHandler("/api/v1/zones", http.HandlerFunc(api.v1CreateZone))).Methods("POST")
	sub.Handle("/api/v1/zones/{id}", InstrumentHandler("/api/v1/zones/{id}", http.HandlerFunc(api.v1UpdateZone))).Methods("PATCH")
	sub.Handle("/api/v1/zones/{id}", InstrumentHandler("/api/v1/zones/{id}", http.HandlerFunc(api.v1DeleteZone))).Methods("DELETE")
	sub.Handle("/api/v1/zones/{id}/command", InstrumentHandler("/api/v1/zones/{id}/command", http.HandlerFunc(api.v1ZoneCommand))).Methods("POST")

	// /api/v1/nodes
	sub.Handle("/api/v1/nodes", InstrumentHandler("/api/v1/nodes", http.HandlerFunc(api.v1GetNodes))).Methods("GET")
	sub.Handle("/api/v1/nodes/{id}", InstrumentHandler("/api/v1/nodes/{id}", http.HandlerFunc(api.v1GetNode))).Methods("GET")
	sub.Handle("/api/v1/nodes/{id}", InstrumentHandler("/api/v1/nodes/{id}", http.HandlerFunc(api.v1UpdateNode))).Methods("PATCH")
	sub.Handle("/api/v1/nodes/{id}", InstrumentHandler("/api/v1/nodes/{id}", http.HandlerFunc(api.v1DeleteNode))).Methods("DELETE")
	sub.Handle("/api/v1/nodes/{id}/command", InstrumentHandler("/api/v1/nodes/{id}/command", http.HandlerFunc(api.v1NodeCommand))).Methods("POST")

	// /api/v1/events (SSE)
	sub.Handle("/api/v1/events", InstrumentHandler("/api/v1/events", http.HandlerFunc(api.v1Events))).Methods("GET")

	// /api/v1/status
	sub.Handle("/api/v1/status", InstrumentHandler("/api/v1/status", http.HandlerFunc(api.v1Status))).Methods("GET")

	// /api/v1/enrollments
	sub.Handle("/api/v1/enrollments/pending", InstrumentHandler("/api/v1/enrollments/pending", http.HandlerFunc(api.v1GetPendingEnrollments))).Methods("GET")
	sub.Handle("/api/v1/enrollments", InstrumentHandler("/api/v1/enrollments", http.HandlerFunc(api.v1GetAllEnrollments))).Methods("GET")
	sub.Handle("/api/v1/enrollments/{mac}/approve", InstrumentHandler("/api/v1/enrollments/{mac}/approve", http.HandlerFunc(api.v1ApproveEnrollment))).Methods("POST")
	sub.Handle("/api/v1/enrollments/{mac}/reject", InstrumentHandler("/api/v1/enrollments/{mac}/reject", http.HandlerFunc(api.v1RejectEnrollment))).Methods("POST")
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode JSON response", "err", err)
	}
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	var req ConfigureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if !isValidAdapterType(req.AdapterType) {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid adapterType: %d", req.AdapterType))
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
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	var req ConfigureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if !isValidAdapterType(req.AdapterType) {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid adapterType: %d", req.AdapterType))
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
	onlineNodes := registry.GetOnlineNodes(75 * time.Second) // 2.5× the 30s health interval — single missed report no longer marks offline
	
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

// getHealth is an unauthenticated liveness probe used by Docker healthchecks.
// It returns 200 + {"ok":true} as long as the HTTP server is reachable.
// Intentionally kept trivial — it must not depend on mesh state so it never
// blocks the container from being marked healthy.
func (api *APIServer) getHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}` + "\n"))
}

// broadcastData broadcasts data to all nodes
func (api *APIServer) broadcastData(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
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

// ApprovalRequest is the optional JSON body for the approve-enrollment endpoint.
type ApprovalRequest struct {
	NodeID         uint8  `json:"nodeId"`
	Name           string `json:"name"`
	Zone           string `json:"zone"`
	AdapterTypeStr string `json:"adapterTypeStr"`
}

// approveEnrollment approves a pending node enrollment
func (api *APIServer) approveEnrollment(w http.ResponseWriter, r *http.Request) {
	mac := mux.Vars(r)["mac"]
	var req ApprovalRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req) // body is optional; ignore decode errors
	}
	params := ApprovalParams(req)
	if err := api.meshServer.ApproveEnrollment(mac, params); err != nil {
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
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
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

// StartAPIServer starts the HTTP API server and returns a shutdown function.
func StartAPIServer(meshServer *MeshServer, port int, apiKey string, corsOrigins []string) (shutdown func(context.Context) error, err error) {
	api := NewAPIServer(meshServer, apiKey, corsOrigins)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      api,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("API server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Give the server a moment to bind; surface immediate errors (e.g. port in use).
	select {
	case err := <-errCh:
		return nil, err
	case <-time.After(100 * time.Millisecond):
	}

	return srv.Shutdown, nil
}
