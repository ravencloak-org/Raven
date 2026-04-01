// Package grpc provides a gRPC client for communicating with the Python AI worker.
package grpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/ravencloak-org/Raven/internal/grpc/pb"
)

// Client wraps a gRPC connection and exposes the AIWorker service stub.
type Client struct {
	conn   *grpc.ClientConn
	worker pb.AIWorkerClient
}

// NewClient dials the AI worker at addr and returns a ready-to-use Client.
func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, worker: pb.NewAIWorkerClient(conn)}, nil
}

// Worker returns the AIWorkerClient stub for making RPC calls.
func (c *Client) Worker() pb.AIWorkerClient { return c.worker }

// Close releases the underlying gRPC connection.
func (c *Client) Close() error { return c.conn.Close() }
