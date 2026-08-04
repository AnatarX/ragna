package http_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pb "gateway-go/internal/api/v1"
	deliveryHTTP "gateway-go/internal/delivery/http"
	"gateway-go/internal/grpcclient"

	"google.golang.org/grpc/metadata"
)

type mockStream struct {
	responses []*pb.StreamQueryResponse
	index     int
}

func (m *mockStream) Recv() (*pb.StreamQueryResponse, error) {
	if m.index >= len(m.responses) {
		return nil, io.EOF
	}
	resp := m.responses[m.index]
	m.index++
	return resp, nil
}

func (m *mockStream) Header() (metadata.MD, error) { return nil, nil }
func (m *mockStream) Trailer() metadata.MD         { return nil }
func (m *mockStream) CloseSend() error             { return nil }
func (m *mockStream) Context() context.Context     { return context.Background() }
func (m *mockStream) SendMsg(m_ interface{}) error { return nil }
func (m *mockStream) RecvMsg(m_ interface{}) error { return nil }

type mockMLClient struct {
	getEmbeddingFunc func(ctx context.Context, query string) ([]float32, error)
	streamQueryFunc  func(ctx context.Context, query string) (grpcclient.Stream, error)
}

func (m *mockMLClient) GetEmbedding(ctx context.Context, query string) ([]float32, error) {
	if m.getEmbeddingFunc != nil {
		return m.getEmbeddingFunc(ctx, query)
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockMLClient) StreamQuery(ctx context.Context, query string) (grpcclient.Stream, error) {
	if m.streamQueryFunc != nil {
		return m.streamQueryFunc(ctx, query)
	}
	return &mockStream{}, nil
}

func TestStreamQuery_BadRequestOnMissingQuery(t *testing.T) {
	mockClient := &mockMLClient{}
	handler := deliveryHTTP.NewHandler(mockClient, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/query/stream", nil)
	rec := httptest.NewRecorder()

	handler.StreamQuery(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, "Query parameter 'q' is required") {
		t.Errorf("unexpected response body: %s", body)
	}
}

func TestStreamQuery_HeadersAndSSEFormat(t *testing.T) {
	mockClient := &mockMLClient{
		getEmbeddingFunc: func(ctx context.Context, query string) ([]float32, error) {
			return nil, nil
		},
		streamQueryFunc: func(ctx context.Context, query string) (grpcclient.Stream, error) {
			return &mockStream{
				responses: []*pb.StreamQueryResponse{
					{
						Payload: &pb.StreamQueryResponse_Delta{
							Delta: "Hello ",
						},
					},
					{
						Payload: &pb.StreamQueryResponse_Delta{
							Delta: "World!",
						},
					},
				},
			}, nil
		},
	}

	handler := deliveryHTTP.NewHandler(mockClient, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/query/stream?q=test", nil)
	rec := httptest.NewRecorder()

	handler.StreamQuery(rec, req)

	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Cache-Status") != "MISS" {
		t.Errorf("expected X-Cache-Status MISS, got %s", rec.Header().Get("X-Cache-Status"))
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: delta") || !strings.Contains(body, "Hello ") {
		t.Errorf("expected SSE body to contain delta event, got: %s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Errorf("expected SSE body to contain done event, got: %s", body)
	}
}
