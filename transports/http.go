package transports

import (
	"context"
	"log/slog"

	"github.com/SkillingX/mcp-localbridge/config"
	mcpServer "github.com/SkillingX/mcp-localbridge/server"
)

// HTTPTransport is a placeholder for future HTTP JSON-RPC transport
//
// NOTE: The mcp-go library (v0.11.0) does not provide a traditional HTTP REST API server.
// For HTTP-based MCP communication, use SSE (Server-Sent Events) transport instead,
// which is the standard HTTP-based streaming protocol for MCP.
//
// SSE transport (transports/sse.go) provides:
//   - HTTP-based communication
//   - Real-time streaming
//   - Multiple client sessions
//   - Standard HTTP ports and endpoints
//
// This HTTP transport is reserved for future implementation of:
//   - HTTP JSON-RPC polling
//   - Webhook-based communication
//   - Custom HTTP endpoints
//
// Current recommendation: Use SSE transport for all HTTP-based MCP needs.
type HTTPTransport struct {
	mcpServer *mcpServer.MCPServer
	config    config.HTTPConfig
	logger    *slog.Logger
	healthy   bool
}

// NewHTTPTransport creates a new HTTP transport placeholder
func NewHTTPTransport(mcpSrv *mcpServer.MCPServer, cfg config.HTTPConfig, logger *slog.Logger) *HTTPTransport {
	return &HTTPTransport{
		mcpServer: mcpSrv,
		config:    cfg,
		logger:    logger,
		healthy:   false,
	}
}

// Start starts the HTTP transport
func (t *HTTPTransport) Start(ctx context.Context) error {
	t.logger.Info("HTTP transport is not available in current mcp-go version")
	t.logger.Info("For HTTP-based MCP communication, use SSE transport instead")
	t.logger.Info("HTTP transport would start on", "address", t.config.Address())

	// Mark as "healthy" but inactive - SSE transport provides HTTP functionality
	t.healthy = true
	<-ctx.Done()
	return nil
}

// Stop stops the HTTP transport
func (t *HTTPTransport) Stop(ctx context.Context) error {
	t.logger.Info("Stopping HTTP transport placeholder")
	t.healthy = false
	return nil
}

// Name returns the transport name
func (t *HTTPTransport) Name() string {
	return "http"
}

// IsHealthy checks if the transport is healthy
func (t *HTTPTransport) IsHealthy() bool {
	return t.healthy
}
