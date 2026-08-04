package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	pb "github.com/AnatarX/ragna/gateway-go/api/v1"
	mlclient "github.com/AnatarX/ragna/gateway-go/internal/clients/ml_client"
)

type Handler struct {
	mlClient *mlclient.Client
}

func NewHandler(mlClient *mlclient.Client) *Handler {
	return &Handler{mlClient: mlClient}
}

type IngestPayload struct {
	DocumentID string            `json:"document_id"`
	Title      string            `json:"title"`
	Content    string            `json:"content"`
	Metadata   map[string]string `json:"metadata"`
}

func (h *Handler) HandleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req IngestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	res, err := h.mlClient.IngestDocument(r.Context(), req.DocumentID, req.Title, req.Content, req.Metadata)
	if err != nil {
		log.Printf("[ERROR] Ingest document failed: %v", err)
		http.Error(w, "Failed to ingest document", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) HandleStreamQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	stream, err := h.mlClient.StreamQuery(r.Context(), query, 5, true)
	if err != nil {
		log.Printf("[ERROR] StreamQuery failed: %v", err)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", "Failed to communicate with ML Engine")
		flusher.Flush()
		return
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintf(w, "event: done\ndata: [DONE]\n\n")
			flusher.Flush()
			break
		}
		if err != nil {
			log.Printf("[ERROR] Error reading gRPC stream: %v", err)
			break
		}

		switch payload := resp.Payload.(type) {
		case *pb.StreamQueryResponse_Source:
			data, _ := json.Marshal(payload.Source)
			fmt.Fprintf(w, "event: source\ndata: %s\n\n", string(data))
		case *pb.StreamQueryResponse_Delta:
			data, _ := json.Marshal(map[string]string{"text": payload.Delta})
			fmt.Fprintf(w, "event: delta\ndata: %s\n\n", string(data))
		}

		flusher.Flush()
	}
}
