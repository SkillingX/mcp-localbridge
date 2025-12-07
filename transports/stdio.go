package transports

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/server"

	mcpServer "github.com/SkillingX/mcp-localbridge/server"
)

// StdioTransport implements stdio transport
type StdioTransport struct {
	mcpServer *mcpServer.MCPServer
	logger    *slog.Logger
	healthy   bool
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(mcpSrv *mcpServer.MCPServer, logger *slog.Logger) *StdioTransport {
	return &StdioTransport{
		mcpServer: mcpSrv,
		logger:    logger,
		healthy:   false,
	}
}

// Start starts the stdio transport
func (t *StdioTransport) Start(ctx context.Context) error {
	t.logger.Info("Starting Stdio transport")
	t.healthy = true

	// ServeStdio is a blocking call
	if err := server.ServeStdio(t.mcpServer.GetServer()); err != nil {
		t.healthy = false
		t.logger.Error("Stdio transport error", "error", err)
		return err
	}

	return nil
}

// Stop stops the stdio transport
func (t *StdioTransport) Stop(ctx context.Context) error {
	t.logger.Info("Stopping Stdio transport")
	t.healthy = false
	// Stdio transport doesn't need explicit shutdown
	return nil
}

// Name returns the transport name
func (t *StdioTransport) Name() string {
	return "stdio"
}

// IsHealthy checks if the transport is healthy
func (t *StdioTransport) IsHealthy() bool {
	return t.healthy
}
