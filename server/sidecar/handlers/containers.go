package handlers

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
)

type ContainerHandler struct {
	docker *client.Client
}

func NewContainerHandler() (*ContainerHandler, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &ContainerHandler{docker: cli}, nil
}

func (h *ContainerHandler) ListContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := h.docker.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		http.Error(w, `{"error":"docker unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	type containerInfo struct {
		ID     string   `json:"id"`
		Names  []string `json:"names"`
		Image  string   `json:"image"`
		Status string   `json:"status"`
		State  string   `json:"state"`
	}
	out := make([]containerInfo, 0, len(containers))
	for _, c := range containers {
		out = append(out, containerInfo{
			ID:     func() string { if len(c.ID) > 12 { return c.ID[:12] }; return c.ID }(),
			Names:  c.Names,
			Image:  c.Image,
			Status: c.Status,
			State:  c.State,
		})
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"containers": out})
}

func (h *ContainerHandler) RestartContainer(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	timeout := 10
	if err := h.docker.ContainerRestart(context.Background(), name, container.StopOptions{Timeout: &timeout}); err != nil {
		http.Error(w, `{"error":"restart failed"}`, http.StatusInternalServerError)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "restarting", "container": name})
}

func (h *ContainerHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	if n, err := strconv.Atoi(tail); err != nil || n < 1 || n > 1000 {
		tail = "100"
	}
	logs, err := h.docker.ContainerLogs(context.Background(), name, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	})
	if err != nil {
		http.Error(w, `{"error":"logs unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	defer logs.Close()
	w.Header().Set("Content-Type", "text/plain")
	io.Copy(w, logs)
}
