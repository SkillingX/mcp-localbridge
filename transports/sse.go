package transports

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/server"

	"github.com/SkillingX/mcp-localbridge/config"
	mcpServer "github.com/SkillingX/mcp-localbridge/server"
)

// SSETransport implements Server-Sent Events transport
// TODO: SSE transport requires mcp-go library APIs that need adjustment
// The NewSSEServer options need to match the actual library API
type SSETransport struct {
	mcpServer *mcpServer.MCPServer
	sseServer *server.SSEServer
	config    config.SSEConfig
	logger    *slog.Logger
	healthy   bool
}

// NewSSETransport creates a new SSE transport
func NewSSETransport(mcpSrv *mcpServer.MCPServer, cfg config.SSEConfig, logger *slog.Logger) *SSETransport {
	// Create SSE server (simplified - using basic constructor)
	sseServer := server.NewSSEServer(
		mcpSrv.GetServer(),
		cfg.BasePath,
	)
	// TODO: Add configuration options when library supports them:
	// - WithSSEEndpoint(cfg.SSEEndpoint)
	// - WithMessageEndpoint(cfg.MessageEndpoint)
	// - WithKeepAliveInterval(time.Duration(cfg.KeepaliveInterval)*time.Second)

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
