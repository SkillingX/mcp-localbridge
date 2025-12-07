package transports

import (
	"context"
	"log/slog"

	mcpServer "github.com/SkillingX/mcp-localbridge/server"
)

// InProcessTransport implements in-process transport
// This is a simple implementation for testing or direct programmatic access
type InProcessTransport struct {
	mcpServer *mcpServer.MCPServer
	logger    *slog.Logger
	healthy   bool
}

// NewInProcessTransport creates a new in-process transport
func NewInProcessTransport(mcpSrv *mcpServer.MCPServer, logger *slog.Logger) *InProcessTransport {
	return &InProcessTransport{
		mcpServer: mcpSrv,
		logger:    logger,
		healthy:   false,
	}
}

// Start starts the in-process transport
func (t *InProcessTransport) Start(ctx context.Context) error {
	t.logger.Info("Starting InProcess transport")
	t.healthy = true

	// InProcess transport just keeps the server available
	// Client code can directly call the MCP server methods

	// Wait for context cancellation
	<-ctx.Done()

	return nil
}

// Stop stops the in-process transport
func (t *InProcessTransport) Stop(ctx context.Context) error {
	t.logger.Info("Stopping InProcess transport")
	t.healthy = false
	return nil
}

// Name returns the transport name
func (t *InProcessTransport) Name() string {
	return "inprocess"
}

// IsHealthy checks if the transport is healthy
func (t *InProcessTransport) IsHealthy() bool {
	return t.healthy
}

// GetServer returns the MCP server for direct in-process calls
func (t *InProcessTransport) GetServer() *mcpServer.MCPServer {
	return t.mcpServer
}
