package mesh

import (
	"net/http"
	"time"
)

func (api *APIServer) v1Status(w http.ResponseWriter, r *http.Request) {
	timeout := api.meshServer.GetHealthTimeout()
	allNodes := api.meshServer.GetNodeRegistry().GetAllNodes()
	total := 0
	online := 0
	for _, n := range allNodes {
		if n.NodeID > 0 {
			total++
			if isOnline(n, timeout) {
				online++
			}
		}
	}

	primaryConnected, secondaryConnected, secondaryConfigured := api.meshServer.SerialStatus()

	primaryStatus := "disconnected"
	if primaryConnected {
		primaryStatus = "connected"
	}

	secondaryStatus := "not_configured"
	if secondaryConfigured {
		if secondaryConnected {
			secondaryStatus = "connected"
		} else {
			secondaryStatus = "disconnected"
		}
	}

	api.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"serial": map[string]string{
				"primary":   primaryStatus,
				"secondary": secondaryStatus,
			},
			"nodes": map[string]int{
				"total":   total,
				"online":  online,
				"offline": total - online,
			},
			"mesh": map[string]bool{
				"masterOnline": api.meshServer.IsRunning(),
			},
		},
	})
}

func isOnline(n *NodeInfo, timeout time.Duration) bool {
	return time.Since(n.LastSeen) <= timeout
}
