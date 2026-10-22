package observability

import (
	"context"
	"net"
	"sync"
	"testing"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// fakeCollectorは、実Collectorを起動せずにOTLP/gRPCの受信内容を確認するin-process stub。
type fakeCollector struct {
	collectortrace.UnimplementedTraceServiceServer

	addr string
	// exportErrが設定されている場合、受信内容を保存せずにそのerrorを返す。
	exportErr error

	mu       sync.Mutex
	requests []*collectortrace.ExportTraceServiceRequest
	metadata []metadata.MD
}

func startFakeCollector(t *testing.T) *fakeCollector {
	t.Helper()
	return startFakeCollectorReturning(t, nil)
}

func startFakeCollectorReturning(t *testing.T, exportErr error) *fakeCollector {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake collector: %v", err)
	}
	collector := &fakeCollector{addr: listener.Addr().String(), exportErr: exportErr}
	server := grpc.NewServer()
	collectortrace.RegisterTraceServiceServer(server, collector)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return collector
}

func (c *fakeCollector) Export(
	ctx context.Context,
	request *collectortrace.ExportTraceServiceRequest,
) (*collectortrace.ExportTraceServiceResponse, error) {
	if c.exportErr != nil {
		return nil, c.exportErr
	}
	md, _ := metadata.FromIncomingContext(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, request)
	c.metadata = append(c.metadata, md)
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

func (c *fakeCollector) received() ([]*collectortrace.ExportTraceServiceRequest, []metadata.MD) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*collectortrace.ExportTraceServiceRequest(nil), c.requests...),
		append([]metadata.MD(nil), c.metadata...)
}
