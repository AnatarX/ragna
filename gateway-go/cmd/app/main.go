package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mlclient "github.com/AnatarX/ragna/gateway-go/internal/clients/ml_client"
	httphandler "github.com/AnatarX/ragna/gateway-go/internal/delivery/http"
)

func main() {
	mlTarget := os.Getenv("ML_GRPC_TARGET")
	if mlTarget == "" {
		mlTarget = "localhost:50051"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Connecting to ML Service at %s...", mlTarget)
	client, err := mlclient.NewClient(mlTarget)
	if err != nil {
		log.Fatalf("Failed to create ML client: %v", err)
	}
	defer client.Close()

	handler := httphandler.NewHandler(client)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ingest", handler.HandleIngest)
	mux.HandleFunc("/api/v1/query/stream", handler.HandleStreamQuery)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
	}

	go func() {
		log.Printf("API Gateway listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down Gateway...")
}
