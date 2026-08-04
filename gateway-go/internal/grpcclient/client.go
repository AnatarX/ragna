package grpcclient

import (
	"context"
	"fmt"

	pb "gateway-go/internal/api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Stream — псевдоним для сгенерированного gRPC стрим-клиента
type Stream = pb.RagService_StreamQueryClient

type Client struct {
	grpcClient pb.RagServiceClient
	conn       *grpc.ClientConn
}

func New(target string) (*Client, error) {
	conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ML gRPC server at %s: %w", target, err)
	}

	return &Client{
		grpcClient: pb.NewRagServiceClient(conn),
		conn:       conn,
	}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) StreamQuery(ctx context.Context, query string) (Stream, error) {
	req := &pb.QueryRequest{
		Query: query,
	}

	stream, err := c.grpcClient.StreamQuery(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start StreamQuery gRPC: %w", err)
	}

	return stream, nil
}

func (c *Client) GetEmbedding(ctx context.Context, query string) ([]float32, error) {
	req := &pb.EmbeddingRequest{
		Text: query,
	}

	resp, err := c.grpcClient.GetEmbedding(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	return resp.GetVector(), nil
}

func (c *Client) IngestDocument(ctx context.Context, docID, title, content string, metadata map[string]string) (*pb.IngestResponse, error) {
	req := &pb.IngestRequest{
		DocumentId: docID,
		Title:      title,
		Content:    content,
		Metadata:   metadata,
	}

	resp, err := c.grpcClient.IngestDocument(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to ingest document: %w", err)
	}

	return resp, nil
}
