package mesh

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (api *APIServer) v1Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if behind proxy

	ch := api.meshServer.GetEventBroker().Subscribe()
	defer api.meshServer.GetEventBroker().Unsubscribe(ch)

	for {
		select {
		case event, open := <-ch:
			if !open {
				return
			}
			data, err := json.Marshal(event.Data)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
