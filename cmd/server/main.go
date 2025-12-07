package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/SkillingX/mcp-localbridge/config"
	"github.com/SkillingX/mcp-localbridge/server"
	"github.com/SkillingX/mcp-localbridge/transports"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config/config.yaml", "Path to configuration file")
	flag.Parse()

	// Initialize logger
	logger := setupLogger()
	logger.Info("Starting MCP LocalBridge service", "config", *configPath)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Update logger level based on configuration
	logger = setupLoggerWithConfig(cfg)

	logger.Info("Configuration loaded successfully",
		"server", cfg.Server.Name,
		"version", cfg.Server.Version)

	// Create MCP server
	mcpServer, err := server.NewMCPServer(cfg, logger)
	if err != nil {
		logger.Error("Failed to create MCP server", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := mcpServer.Close(); err != nil {
			logger.Error("Error closing MCP server", "error", err)
		}
	}()

	logger.Info("MCP server created successfully")

	// Create transport manager
	transportMgr := transports.NewManager(cfg, mcpServer, logger)

	// Initialize transports
	if err := transportMgr.Initialize(); err != nil {
		logger.Error("Failed to initialize transports", "error", err)
		os.Exit(1)
	}

	logger.Info("Transports initialized successfully")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start signal handler goroutine
	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", "signal", sig)

		// Trigger graceful shutdown
		if err := transportMgr.StopAll(); err != nil {
			logger.Error("Error during graceful shutdown", "error", err)
		}
	}()

	// Start all transports
	if err := transportMgr.StartAll(); err != nil {
		logger.Error("Failed to start transports", "error", err)
		os.Exit(1)
	}

	logger.Info("MCP LocalBridge service started successfully",
		"transports", getEnabledTransports(cfg))

	// Wait for shutdown signal
	transportMgr.Wait()

	logger.Info("MCP LocalBridge service stopped")
}

// setupLogger creates a basic logger
func setupLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// setupLoggerWithConfig creates a logger based on configuration
func setupLoggerWithConfig(cfg *config.Config) *slog.Logger {
	// Determine log level
	level := cfg.GetLogLevel()

	// Determine output
	var output *os.File
	switch cfg.Logging.Output {
	case "stderr":
		output = os.Stderr
	case "stdout":
		output = os.Stdout
	default:
		// If it's a file path, open it
		if cfg.Logging.Output != "" {
			file, err := os.OpenFile(cfg.Logging.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open log file: %v, using stdout\n", err)
				output = os.Stdout
			} else {
				output = file
			}
		} else {
			output = os.Stdout
		}
	}

	// Create handler based on format
	var handler slog.Handler
	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = slog.NewTextHandler(output, &slog.HandlerOptions{
			Level: level,
		})
	}

	return slog.New(handler)
}

// getEnabledTransports returns a list of enabled transport names
func getEnabledTransports(cfg *config.Config) []string {
	var enabled []string

	if cfg.Transports.Stdio.Enabled {
		enabled = append(enabled, "stdio")
	}
	if cfg.Transports.HTTP.Enabled {
		enabled = append(enabled, fmt.Sprintf("http(%s)", cfg.Transports.HTTP.Address()))
	}
	if cfg.Transports.SSE.Enabled {
		enabled = append(enabled, fmt.Sprintf("sse(%s)", cfg.Transports.SSE.Address()))
	}
	if cfg.Transports.InProcess.Enabled {
		enabled = append(enabled, "inprocess")
	}

	return enabled
}
