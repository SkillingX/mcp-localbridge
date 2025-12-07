package transports

import (
	"context"
	"log/slog"

	"github.com/SkillingX/mcp-localbridge/config"
	mcpServer "github.com/SkillingX/mcp-localbridge/server"
)

// HTTPTransport implements Streamable HTTP transport
// TODO: HTTP transport requires mcp-go library APIs that are not yet available
// The NewStreamableHTTPServer and related options need to be implemented
type HTTPTransport struct {
	mcpServer *mcpServer.MCPServer
	config    config.HTTPConfig
	logger    *slog.Logger
	healthy   bool
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(mcpSrv *mcpServer.MCPServer, cfg config.HTTPConfig, logger *slog.Logger) *HTTPTransport {
	// TODO: Implement when mcp-go library supports Streamable HTTP server
	// httpServer := server.NewStreamableHTTPServer(
	// 	mcpSrv.GetServer(),
	// 	server.WithEndpointPath(cfg.EndpointPath),
	// 	server.WithHeartbeatInterval(time.Duration(cfg.HeartbeatInterval)*time.Second),
	// 	server.WithStateLess(cfg.Stateless),
	// )

	return &HTTPTransport{
		mcpServer: mcpSrv,
		config:    cfg,
		logger:    logger,
		healthy:   false,
	}
}

// Start starts the HTTP transport
func (t *HTTPTransport) Start(ctx context.Context) error {
	t.logger.Warn("HTTP transport is not yet implemented - waiting for mcp-go library support")
	t.logger.Info("HTTP transport would start on", "address", t.config.Address())

	// TODO: Implement when library is ready
	// if err := t.httpServer.Start(t.config.Address()); err != nil {
	// 	t.healthy = false
	// 	t.logger.Error("HTTP transport error", "error", err)
	// 	return fmt.Errorf("HTTP transport failed: %w", err)
	// }

	t.healthy = true
	<-ctx.Done()
	return nil
}

// Stop stops the HTTP transport
func (t *HTTPTransport) Stop(ctx context.Context) error {
	t.logger.Info("Stopping HTTP transport")
	t.healthy = false
	// TODO: Implement graceful shutdown when HTTP server is available
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
