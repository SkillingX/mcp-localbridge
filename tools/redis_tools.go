package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/SkillingX/mcp-localbridge/cache"
	"github.com/SkillingX/mcp-localbridge/config"
)

// RedisToolsHandler provides Redis-related MCP tools
type RedisToolsHandler struct {
	clients map[string]*cache.RedisClient
	config  config.RedisToolsConfig
	logger  *slog.Logger
}

// NewRedisToolsHandler creates a new Redis tools handler
func NewRedisToolsHandler(clients map[string]*cache.RedisClient, cfg config.RedisToolsConfig, logger *slog.Logger) *RedisToolsHandler {
	return &RedisToolsHandler{
		clients: clients,
		config:  cfg,
		logger:  logger,
	}
}

// HandleRedisGet retrieves a value from Redis by key
func (h *RedisToolsHandler) HandleRedisGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling redis_get tool request")

	// Extract required parameters using mcp-go v0.43.2 best practices
	redisName, err := request.RequireString("redis")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	key, err := request.RequireString("key")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get Redis client
	client, ok := h.clients[redisName]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("Redis '%s' not found or not enabled", redisName)), nil
	}

	// Get value
	value, err := client.Get(ctx, key)
	if err != nil {
		h.logger.ErrorContext(ctx, "Redis GET failed", "error", err, "key", key)
		return mcp.NewToolResultError(fmt.Sprintf("Redis GET failed: %v", err)), nil
	}

	result := map[string]any{
		"redis": redisName,
		"key":   key,
		"value": value,
		"found": value != "",
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// HandleRedisSet sets a key-value pair in Redis
func (h *RedisToolsHandler) HandleRedisSet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling redis_set tool request")

	// Extract required parameters using mcp-go v0.43.2 best practices
	redisName, err := request.RequireString("redis")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	key, err := request.RequireString("key")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	value, err := request.RequireString("value")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get Redis client
	client, ok := h.clients[redisName]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("Redis '%s' not found or not enabled", redisName)), nil
	}

	// Parse optional TTL with GetInt (more type-safe)
	ttl := request.GetInt("ttl", 0)
	var expiration time.Duration
	if ttl > 0 {
		expiration = time.Duration(ttl) * time.Second
	}

	// Set value
	if err := client.Set(ctx, key, value, expiration); err != nil {
		h.logger.ErrorContext(ctx, "Redis SET failed", "error", err, "key", key)
		return mcp.NewToolResultError(fmt.Sprintf("Redis SET failed: %v", err)), nil
	}

	result := map[string]any{
		"redis":   redisName,
		"key":     key,
		"success": true,
		"ttl":     expiration.Seconds(),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// HandleRedisScan scans Redis keys matching a pattern
func (h *RedisToolsHandler) HandleRedisScan(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling redis_scan tool request")

	// Extract required parameter using mcp-go v0.43.2 best practices
	redisName, err := request.RequireString("redis")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get pattern (default to "*")
	pattern := request.GetString("pattern", "*")

	// Get Redis client
	client, ok := h.clients[redisName]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("Redis '%s' not found or not enabled", redisName)), nil
	}

	// Scan keys (use multiple iterations to get more keys, up to max)
	var allKeys []string
	cursor := uint64(0)
	maxKeys := h.config.MaxScanKeys

	for len(allKeys) < maxKeys {
		keys, newCursor, err := client.Scan(ctx, cursor, pattern, int64(h.config.ScanCount))
		if err != nil {
			h.logger.ErrorContext(ctx, "Redis SCAN failed", "error", err, "pattern", pattern)
			return mcp.NewToolResultError(fmt.Sprintf("Redis SCAN failed: %v", err)), nil
		}

		allKeys = append(allKeys, keys...)
		cursor = newCursor

		// If cursor is 0, we've completed the full iteration
		if cursor == 0 {
			break
		}

		// Stop if we've reached max keys
		if len(allKeys) >= maxKeys {
			allKeys = allKeys[:maxKeys]
			break
		}
	}

	result := map[string]any{
		"redis":   redisName,
		"pattern": pattern,
		"keys":    allKeys,
		"count":   len(allKeys),
		"limited": len(allKeys) >= maxKeys,
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
