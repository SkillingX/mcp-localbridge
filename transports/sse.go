package transports

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/SkillingX/mcp-localbridge/config"
	mcpServer "github.com/SkillingX/mcp-localbridge/server"
)

// SSETransport implements Server-Sent Events (SSE) transport for MCP
//
// SSE is the recommended HTTP-based transport for MCP protocol. It provides:
//   - Real-time streaming communication over HTTP
//   - Automatic reconnection support
//   - Multiple concurrent client sessions
//   - Standard HTTP/HTTPS ports and protocols
//
// The SSE transport exposes HTTP endpoints:
//   - GET  {BasePath}/sse      - SSE connection endpoint (streaming)
//   - POST {BasePath}/message  - Message posting endpoint
//
// Example configuration:
//   - BasePath: "/api/mcp"
//   - SSE endpoint: http://host:port/api/mcp/sse
//   - Message endpoint: http://host:port/api/mcp/message
//
// This is the preferred transport for:
//   - Web-based clients
//   - IDE integrations that support HTTP
//   - Remote server deployments
//   - Docker containerized services
type SSETransport struct {
	mcpServer *mcpServer.MCPServer
	sseServer *server.SSEServer
	config    config.SSEConfig
	logger    *slog.Logger
	healthy   bool
}

// NewSSETransport creates a new SSE transport
func NewSSETransport(mcpSrv *mcpServer.MCPServer, cfg config.SSEConfig, logger *slog.Logger) *SSETransport {
	// Create SSE server with full configuration options
	sseServer := server.NewSSEServer(
		mcpSrv.GetServer(),
		server.WithStaticBasePath(cfg.BasePath),
		server.WithSSEEndpoint(cfg.SSEEndpoint),
		server.WithMessageEndpoint(cfg.MessageEndpoint),
		server.WithKeepAlive(cfg.KeepaliveInterval > 0),
		server.WithKeepAliveInterval(time.Duration(cfg.KeepaliveInterval)*time.Second),
	)
	// Note: mcp-go v0.43.2+ supports full configuration options

	return &SSETransport{
		mcpServer: mcpSrv,
		sseServer: sseServer,
		config:    cfg,
		logger:    logger,
		healthy:   false,
	}
}

// Start starts the SSE transport
func (t *SSETransport) Start(ctx context.Context) error {
	t.logger.Info("Starting SSE transport", "address", t.config.Address())
	t.healthy = true

	// Start is a blocking call
	if err := t.sseServer.Start(t.config.Address()); err != nil {
		t.healthy = false
		t.logger.Error("SSE transport error", "error", err)
		return fmt.Errorf("SSE transport failed: %w", err)
	}

	return nil
}

// Stop stops the SSE transport
func (t *SSETransport) Stop(ctx context.Context) error {
	t.logger.Info("Stopping SSE transport")
	t.healthy = false

	// Gracefully shutdown SSE server
	if err := t.sseServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown SSE server: %w", err)
	}

	return nil
}

// Name returns the transport name
func (t *SSETransport) Name() string {
	return "sse"
}

// IsHealthy checks if the transport is healthy
func (t *SSETransport) IsHealthy() bool {
	return t.healthy
}
