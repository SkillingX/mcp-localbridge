package transports

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/SkillingX/mcp-localbridge/config"
	"github.com/SkillingX/mcp-localbridge/server"
)

// Manager manages all enabled transports
type Manager struct {
	config      *config.Config
	mcpServer   *server.MCPServer
	logger      *slog.Logger
	transports  []Transport
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	healthCheck *HealthChecker
}

// Transport defines the interface for all transport implementations
type Transport interface {
	// Start starts the transport
	Start(ctx context.Context) error
	// Stop stops the transport gracefully
	Stop(ctx context.Context) error
	// Name returns the transport name
	Name() string
	// IsHealthy checks if the transport is healthy
	IsHealthy() bool
}

// NewManager creates a new transport manager
func NewManager(cfg *config.Config, mcpSrv *server.MCPServer, logger *slog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:      cfg,
		mcpServer:   mcpSrv,
		logger:      logger,
		transports:  []Transport{},
		ctx:         ctx,
		cancel:      cancel,
		healthCheck: NewHealthChecker(logger),
	}
}

// Initialize initializes all configured transports
func (m *Manager) Initialize() error {
	m.logger.Info("Initializing transports")

	// Initialize Stdio transport
	if m.config.Transports.Stdio.Enabled {
		stdioTransport := NewStdioTransport(m.mcpServer, m.logger)
		m.transports = append(m.transports, stdioTransport)
		m.healthCheck.RegisterTransport(stdioTransport)
		m.logger.Info("Stdio transport initialized")
	}

	// Initialize HTTP transport
	if m.config.Transports.HTTP.Enabled {
		httpTransport := NewHTTPTransport(m.mcpServer, m.config.Transports.HTTP, m.logger)
		m.transports = append(m.transports, httpTransport)
		m.healthCheck.RegisterTransport(httpTransport)
		m.logger.Info("HTTP transport initialized", "address", m.config.Transports.HTTP.Address())
	}

	// Initialize SSE transport
	if m.config.Transports.SSE.Enabled {
		sseTransport := NewSSETransport(m.mcpServer, m.config.Transports.SSE, m.logger)
		m.transports = append(m.transports, sseTransport)
		m.healthCheck.RegisterTransport(sseTransport)
		m.logger.Info("SSE transport initialized", "address", m.config.Transports.SSE.Address())
	}

	// Initialize InProcess transport (if enabled)
	if m.config.Transports.InProcess.Enabled {
		inProcessTransport := NewInProcessTransport(m.mcpServer, m.logger)
		m.transports = append(m.transports, inProcessTransport)
		m.healthCheck.RegisterTransport(inProcessTransport)
		m.logger.Info("InProcess transport initialized")
	}

	if len(m.transports) == 0 {
		return fmt.Errorf("no transports enabled")
	}

	m.logger.Info("All transports initialized", "count", len(m.transports))
	return nil
}

// StartAll starts all transports with panic recovery
func (m *Manager) StartAll() error {
	m.logger.Info("Starting all transports", "count", len(m.transports))

	for _, transport := range m.transports {
		// Start each transport in its own goroutine with panic recovery
		m.wg.Add(1)
		go func(t Transport) {
			defer m.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					m.logger.Error("Transport panic recovered", "transport", t.Name(), "panic", r)
				}
			}()

			m.logger.Info("Starting transport", "name", t.Name())
			if err := t.Start(m.ctx); err != nil {
				m.logger.Error("Transport failed", "name", t.Name(), "error", err)
			}
		}(transport)
	}

	// Start health checker
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.healthCheck.Run(m.ctx)
	}()

	m.logger.Info("All transports started")
	return nil
}

// StopAll stops all transports gracefully
func (m *Manager) StopAll() error {
	m.logger.Info("Stopping all transports")

	// Cancel context to signal all transports to stop
	m.cancel()

	// Stop each transport with a timeout
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()

	for _, transport := range m.transports {
		m.logger.Info("Stopping transport", "name", transport.Name())
		if err := transport.Stop(stopCtx); err != nil {
			m.logger.Error("Failed to stop transport", "name", transport.Name(), "error", err)
		}
	}

	// Wait for all goroutines to finish
	m.wg.Wait()

	m.logger.Info("All transports stopped")
	return nil
}

// Wait waits for all transports to finish
func (m *Manager) Wait() {
	m.wg.Wait()
}

// GetHealthStatus returns the health status of all transports
func (m *Manager) GetHealthStatus() map[string]bool {
	return m.healthCheck.GetStatus()
}

// HealthChecker periodically checks transport health
type HealthChecker struct {
	transports []Transport
	logger     *slog.Logger
	interval   time.Duration
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger *slog.Logger) *HealthChecker {
	return &HealthChecker{
		transports: []Transport{},
		logger:     logger,
		interval:   30 * time.Second,
	}
}

// RegisterTransport registers a transport for health checking
func (h *HealthChecker) RegisterTransport(t Transport) {
	h.transports = append(h.transports, t)
}

// Run runs the health check loop
func (h *HealthChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.checkHealth()
		}
	}
}

// checkHealth checks the health of all registered transports
func (h *HealthChecker) checkHealth() {
	for _, t := range h.transports {
		healthy := t.IsHealthy()
		if !healthy {
			h.logger.Warn("Transport unhealthy", "name", t.Name())
		}
	}
}

// GetStatus returns the health status of all transports
func (h *HealthChecker) GetStatus() map[string]bool {
	status := make(map[string]bool)
	for _, t := range h.transports {
		status[t.Name()] = t.IsHealthy()
	}
	return status
}
