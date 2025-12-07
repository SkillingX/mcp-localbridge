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
	client *client.StdioMCPClient
}

func main() {
	// Parse command-line flags
	serverCmd := flag.String("server", "./bin/mcp-server", "Path to MCP server executable")
	tool := flag.String("tool", "", "Tool name to call (e.g., 'db_table_list')")
	args := flag.String("args", "{}", "JSON string of tool arguments")
	list := flag.Bool("list", false, "List available tools")
	flag.Parse()

	// Create client
	mcpClient, err := newMCPClient(*serverCmd)
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

// newMCPClient creates a new MCP client connected to the server via stdio
func newMCPClient(serverCmd string) (*MCPClient, error) {
	// Create stdio transport client
	stdioClient, err := client.NewStdioMCPClient(serverCmd, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdio client: %w", err)
	}

	// Initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{
		Params: struct {
			ProtocolVersion string                 `json:"protocolVersion"`
			Capabilities    mcp.ClientCapabilities `json:"capabilities"`
			ClientInfo      mcp.Implementation     `json:"clientInfo"`
		}{
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

	return &MCPClient{client: stdioClient}, nil
}

// ListTools lists all available tools from the MCP server
func (c *MCPClient) ListTools(ctx context.Context) error {
	req := mcp.ListToolsRequest{}
	resp, err := c.client.ListTools(ctx, req)
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
		Params: struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name:      toolName,
			Arguments: args,
		},
	}

	resp, err := c.client.CallTool(ctx, req)
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
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
