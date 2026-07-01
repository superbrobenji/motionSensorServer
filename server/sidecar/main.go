package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/superbrobenji/lattice-hub/sidecar/handlers"
)

func main() {
	adminKey := os.Getenv("ADMIN_KEY")
	if adminKey == "" {
		log.Fatal("ADMIN_KEY is required")
	}
	kafkaBroker := envOrDefault("KAFKA_BROKER", "kafka:9092")

	containerHandler, err := handlers.NewContainerHandler()
	if err != nil {
		log.Fatalf("Docker client init failed: %v", err)
	}
	kafkaHandler := handlers.NewKafkaHandler(kafkaBroker)

	r := mux.NewRouter()
	r.Use(handlers.AuthMiddleware(adminKey))

	r.HandleFunc("/sidecar/containers", containerHandler.ListContainers).Methods("GET")
	r.HandleFunc("/sidecar/containers/{name}/restart", containerHandler.RestartContainer).Methods("POST")
	r.HandleFunc("/sidecar/containers/{name}/logs", containerHandler.GetLogs).Methods("GET")
	r.HandleFunc("/sidecar/kafka/status", kafkaHandler.Status).Methods("GET")
	r.HandleFunc("/sidecar/kafka/events/recent", kafkaHandler.RecentEvents).Methods("GET")

	log.Printf("Sidecar listening on :9000")
	if err := http.ListenAndServe(":9000", r); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
