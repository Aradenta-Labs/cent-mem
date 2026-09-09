package daemon

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a gRPC client connected to centmemd.
type Client struct {
	conn         *grpc.ClientConn
	memoryClient centmemv1.MemoryServiceClient
	syncClient   centmemv1.SyncServiceClient
}

// ProbeDaemon checks if a daemon is actively listening on socketPath within timeout.
func ProbeDaemon(socketPath string, timeout time.Duration) bool {
	if socketPath == "" {
		return false
	}
	if timeout <= 0 {
		timeout = 10 * time.Millisecond
	}

	network := "unix"
	addr := socketPath
	if strings.Contains(socketPath, ":") && !strings.HasPrefix(socketPath, "/") {
		network = "tcp"
	}

	conn, err := net.DialTimeout(network, addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// NewClient connects to centmemd over Unix domain socket or TCP.
func NewClient(target string) (*Client, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	network := "unix"
	addr := target
	if strings.Contains(target, ":") && !strings.HasPrefix(target, "/") {
		network = "tcp"
	}

	opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}))

	conn, err := grpc.NewClient("passthrough:///"+addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", target, err)
	}

	return &Client{
		conn:         conn,
		memoryClient: centmemv1.NewMemoryServiceClient(conn),
		syncClient:   centmemv1.NewSyncServiceClient(conn),
	}, nil
}

// Close terminates the client connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// MemoryClient returns the underlying MemoryServiceClient.
func (c *Client) MemoryClient() centmemv1.MemoryServiceClient {
	return c.memoryClient
}

// SyncClient returns the underlying SyncServiceClient.
func (c *Client) SyncClient() centmemv1.SyncServiceClient {
	return c.syncClient
}

// Recall delegates search to centmemd.
func (c *Client) Recall(ctx context.Context, req *centmemv1.RecallRequest) (*centmemv1.RecallResponse, error) {
	return c.memoryClient.Recall(ctx, req)
}

// Put delegates memory insertion to centmemd.
func (c *Client) Put(ctx context.Context, req *centmemv1.PutRequest) (*centmemv1.PutResponse, error) {
	return c.memoryClient.Put(ctx, req)
}

// Set delegates fact setting to centmemd.
func (c *Client) Set(ctx context.Context, req *centmemv1.SetRequest) (*centmemv1.SetResponse, error) {
	return c.memoryClient.Set(ctx, req)
}

// Get delegates fact retrieval to centmemd.
func (c *Client) Get(ctx context.Context, req *centmemv1.GetRequest) (*centmemv1.GetResponse, error) {
	return c.memoryClient.Get(ctx, req)
}

// Timeline delegates timeline retrieval to centmemd.
func (c *Client) Timeline(ctx context.Context, req *centmemv1.TimelineRequest) (*centmemv1.TimelineResponse, error) {
	return c.memoryClient.Timeline(ctx, req)
}

// List delegates memory listing to centmemd.
func (c *Client) List(ctx context.Context, req *centmemv1.ListRequest) (*centmemv1.ListResponse, error) {
	return c.memoryClient.List(ctx, req)
}

// Forget delegates memory deletion to centmemd.
func (c *Client) Forget(ctx context.Context, req *centmemv1.ForgetRequest) (*centmemv1.ForgetResponse, error) {
	return c.memoryClient.Forget(ctx, req)
}

// Stats delegates statistics aggregation to centmemd.
func (c *Client) Stats(ctx context.Context, req *centmemv1.StatsRequest) (*centmemv1.StatsResponse, error) {
	return c.memoryClient.Stats(ctx, req)
}

// Compact delegates retention compaction to centmemd.
func (c *Client) Compact(ctx context.Context, req *centmemv1.CompactRequest) (*centmemv1.CompactResponse, error) {
	return c.memoryClient.Compact(ctx, req)
}

// Link delegates link creation or status change to centmemd.
func (c *Client) Link(ctx context.Context, req *centmemv1.LinkRequest) (*centmemv1.LinkResponse, error) {
	return c.memoryClient.Link(ctx, req)
}

// Unlink delegates link removal to centmemd.
func (c *Client) Unlink(ctx context.Context, req *centmemv1.UnlinkRequest) (*centmemv1.UnlinkResponse, error) {
	return c.memoryClient.Unlink(ctx, req)
}

// Links delegates relationship query to centmemd.
func (c *Client) Links(ctx context.Context, req *centmemv1.LinksRequest) (*centmemv1.LinksResponse, error) {
	return c.memoryClient.Links(ctx, req)
}
