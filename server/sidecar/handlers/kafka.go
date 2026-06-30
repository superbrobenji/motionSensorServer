package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaHandler struct {
	broker string
}

func NewKafkaHandler(broker string) *KafkaHandler {
	return &KafkaHandler{broker: broker}
}

func (h *KafkaHandler) Status(w http.ResponseWriter, r *http.Request) {
	conn, err := kafka.DialContext(context.Background(), "tcp", h.broker)
	if err != nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"reachable": false,
			"error":     err.Error(),
		})
		return
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions("motion-trigger")
	if err != nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"reachable": true,
			"topics":    map[string]interface{}{"motion-trigger": "error reading partitions"},
		})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"reachable":  true,
		"broker":     h.broker,
		"partitions": len(partitions),
		"checkedAt":  time.Now().Unix(),
	})
}

func (h *KafkaHandler) RecentEvents(w http.ResponseWriter, r *http.Request) {
	n := 50
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 && parsed <= 500 {
			n = parsed
		}
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{h.broker},
		Topic:     "motion-trigger",
		Partition: 0,
		MaxBytes:  1024 * 1024,
	})
	defer reader.Close()

	if err := reader.SetOffset(kafka.LastOffset); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to seek"})
		return
	}

	// Seek to end then read last N
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var events []map[string]interface{}
	for i := 0; i < n; i++ {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			break
		}
		events = append(events, map[string]interface{}{
			"offset":    msg.Offset,
			"timestamp": msg.Time.Unix(),
			"value":     string(msg.Value),
		})
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"topic":  "motion-trigger",
		"events": events,
		"count":  len(events),
	})
}
