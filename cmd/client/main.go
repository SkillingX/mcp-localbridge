package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPClient provides a simple client for testing and debugging MCP server
type MCPClient struct {
	mcpClient client.MCPClient
	cancel    context.CancelFunc // For canceling long-lived SSE connection
}

func main() {
	// Parse command-line flags
	serverCmd := flag.String("server", "./bin/mcp-server", "Path to MCP server executable (for stdio)")
	sseURL := flag.String("sse", "", "SSE endpoint URL (e.g., 'http://localhost:28028/api/mcp/sse')")
	tool := flag.String("tool", "", "Tool name to call (e.g., 'db_table_list')")
	args := flag.String("args", "{}", "JSON string of tool arguments")
	list := flag.Bool("list", false, "List available tools")
	flag.Parse()

	// Determine transport type
	var mcpClient *MCPClient
	var err error

	if *sseURL != "" {
		// Use SSE transport
		log.Printf("Connecting to SSE server at %s...\n", *sseURL)
		mcpClient, err = newSSEMCPClient(*sseURL)
	} else {
		// Use stdio transport (default)
		log.Printf("Starting stdio server: %s\n", *serverCmd)
		mcpClient, err = newStdioMCPClient(*serverCmd)
	}

	if err != nil {
		log.Fatalf("Failed to create MCP client: %v", err)
	}
	defer mcpClient.Close()

	ctx := context.Background()

	// List tools if requested
	if *list {
		if err := mcpClient.ListTools(ctx); err != nil {
			log.Fatalf("Failed to list tools: %v", err)
		}
		return
	}

	// Call tool if specified
	if *tool != "" {
		if err := mcpClient.CallTool(ctx, *tool, *args); err != nil {
			log.Fatalf("Failed to call tool: %v", err)
		}
		return
	}

	// If no action specified, show usage
	flag.Usage()
}

// newStdioMCPClient creates a new MCP client connected to the server via stdio
func newStdioMCPClient(serverCmd string) (*MCPClient, error) {
	// Create stdio transport client
	stdioClient, err := client.NewStdioMCPClient(serverCmd, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdio client: %w", err)
	}

	// Initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "mcp-localbridge-client",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	}

	_, err = stdioClient.Initialize(ctx, initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	return &MCPClient{mcpClient: stdioClient}, nil
}

// newSSEMCPClient creates a new MCP client connected to the server via SSE
func newSSEMCPClient(baseURL string) (*MCPClient, error) {
	// Create SSE transport client
	sseClient, err := client.NewSSEMCPClient(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSE client: %w", err)
	}

	// Start the SSE client with a long-lived context
	// The SSE stream needs to stay alive for the entire client lifetime
	// Note: Start() creates a child context internally for the stream
	ctx, cancel := context.WithCancel(context.Background())

	if err := sseClient.Start(ctx); err != nil {
		cancel() // Clean up the long-lived context
		return nil, fmt.Errorf("failed to start SSE client: %w", err)
	}

	// Initialize MCP connection
	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initCancel()

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "mcp-localbridge-client",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	}

	_, err = sseClient.Initialize(initCtx, initReq)
	if err != nil {
		cancel() // Clean up the long-lived context
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	log.Println("SSE connection initialized successfully")
	return &MCPClient{mcpClient: sseClient, cancel: cancel}, nil
}

// ListTools lists all available tools from the MCP server
func (c *MCPClient) ListTools(ctx context.Context) error {
	req := mcp.ListToolsRequest{}
	resp, err := c.mcpClient.ListTools(ctx, req)
	if err != nil {
		return fmt.Errorf("list tools failed: %w", err)
	}

	fmt.Println("Available Tools:")
	fmt.Println("================")
	for _, tool := range resp.Tools {
		fmt.Printf("\nTool: %s\n", tool.Name)
		fmt.Printf("Description: %s\n", tool.Description)

		if tool.InputSchema.Properties != nil {
			fmt.Println("Parameters:")
			for key, prop := range tool.InputSchema.Properties {
				propJSON, _ := json.MarshalIndent(prop, "  ", "  ")
				fmt.Printf("  %s: %s\n", key, string(propJSON))
			}
		}
	}

	return nil
}

// CallTool calls a specific MCP tool with the given arguments
func (c *MCPClient) CallTool(ctx context.Context, toolName, argsJSON string) error {
	// Parse arguments
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Call tool
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	}

	resp, err := c.mcpClient.CallTool(ctx, req)
	if err != nil {
		return fmt.Errorf("call tool failed: %w", err)
	}

	// Print response
	fmt.Printf("Tool: %s\n", toolName)
	fmt.Println("Response:")
	fmt.Println("=========")

	// Print response content
	contentJSON, _ := json.MarshalIndent(resp.Content, "", "  ")
	fmt.Println(string(contentJSON))

	return nil
}

// Close closes the client connection
func (c *MCPClient) Close() error {
	// Cancel the SSE context first
	if c.cancel != nil {
		c.cancel()
	}

	// Then close the client
	if c.mcpClient != nil {
		return c.mcpClient.Close()
	}
	return nil
}
