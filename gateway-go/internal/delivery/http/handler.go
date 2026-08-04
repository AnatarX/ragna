package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gateway-go/internal/cache"
	"gateway-go/internal/grpcclient"
)

type Handler struct {
	mlClient MLClient
	cache    *cache.SemanticCache
}

type MLClient interface {
	GetEmbedding(ctx context.Context, query string) ([]float32, error)
	StreamQuery(ctx context.Context, query string) (grpcclient.Stream, error)
}

func NewHandler(mlClient MLClient, semanticCache *cache.SemanticCache) *Handler {
	return &Handler{
		mlClient: mlClient,
		cache:    semanticCache,
	}
}

func (h *Handler) StreamQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 1. Получаем вектор запроса
	vec, err := h.mlClient.GetEmbedding(ctx, query)
	if err == nil && len(vec) > 0 {
		// 2. Проверяем Семантический Кэш в Redis
		cached, hit, cacheErr := h.cache.Get(ctx, vec)
		if cacheErr == nil && hit {
			w.Header().Set("X-Cache-Status", "HIT")

			for _, doc := range cached.Sources {
				sendSSE(w, flusher, "source", doc)
			}
			sendSSE(w, flusher, "delta", map[string]string{"text": cached.Answer})
			sendSSE(w, flusher, "done", "[DONE]")
			return
		}
	}

	// 3. Cache Miss -> Идем в ML Сервис
	w.Header().Set("X-Cache-Status", "MISS")

	stream, err := h.mlClient.StreamQuery(ctx, query)
	if err != nil {
		http.Error(w, fmt.Sprintf("ML service error: %v", err), http.StatusInternalServerError)
		return
	}

	var fullAnswer strings.Builder
	var sources []cache.SourceDoc

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		if resp.GetSource() != nil {
			src := resp.GetSource()
			doc := cache.SourceDoc{
				ID:       src.GetId(),
				Content:  src.GetContent(),
				Score:    src.GetScore(),
				Metadata: src.GetMetadata(),
			}
			sources = append(sources, doc)
			sendSSE(w, flusher, "source", doc)
		}

		if resp.GetDelta() != "" {
			delta := resp.GetDelta()
			fullAnswer.WriteString(delta)
			sendSSE(w, flusher, "delta", map[string]string{"text": delta})
		}
	}

	sendSSE(w, flusher, "done", "[DONE]")

	// 4. Сохраняем в Семантический Кэш для последующих похожих запросов
	if len(vec) > 0 && fullAnswer.Len() > 0 {
		cachePayload := &cache.CachedResponse{
			Answer:  fullAnswer.String(),
			Sources: sources,
		}
		_ = h.cache.Set(ctx, query, vec, cachePayload)
	}
}

func sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	var payload []byte
	switch v := data.(type) {
	case string:
		payload = []byte(v)
	default:
		payload, _ = json.Marshal(v)
	}

	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	flusher.Flush()
}
