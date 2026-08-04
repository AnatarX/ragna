package mlclient

import (
	"context"
	"fmt"
	"time"

	pb "github.com/AnatarX/ragna/gateway-go/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client pb.RagServiceClient
}

func NewClient(target string) (*Client, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ML gRPC server: %w", err)
	}

	return &Client{
		conn:   conn,
		client: pb.NewRagServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) IngestDocument(ctx context.Context, docID, title, content string, metadata map[string]string) (*pb.IngestResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.IngestDocument(ctx, &pb.IngestRequest{
		DocumentId: docID,
		Title:      title,
		Content:    content,
		Metadata:   metadata,
	})
}

func (c *Client) StreamQuery(ctx context.Context, query string, topK int32, useReranker bool) (pb.RagService_StreamQueryClient, error) {
	return c.client.StreamQuery(ctx, &pb.QueryRequest{
		Query:       query,
		TopK:        topK,
		UseReranker: useReranker,
	})
}
