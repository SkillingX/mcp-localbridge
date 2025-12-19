package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/SkillingX/mcp-localbridge/cache"
	"github.com/SkillingX/mcp-localbridge/config"
	"github.com/SkillingX/mcp-localbridge/db"
)

// IntrospectionHandler provides database schema introspection capabilities
type IntrospectionHandler struct {
	repositories map[string]db.Repository
	redisClients map[string]*cache.RedisClient
	config       config.IntrospectionConfig
	logger       *slog.Logger
}

// NewIntrospectionHandler creates a new introspection handler
func NewIntrospectionHandler(
	repos map[string]db.Repository,
	redisClients map[string]*cache.RedisClient,
	cfg config.IntrospectionConfig,
	logger *slog.Logger,
) *IntrospectionHandler {
	return &IntrospectionHandler{
		repositories: repos,
		redisClients: redisClients,
		config:       cfg,
		logger:       logger,
	}
}

// HandleIntrospection performs database schema introspection
func (h *IntrospectionHandler) HandleIntrospection(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling introspection tool request")

	// Extract required parameter using mcp-go v0.43.2 best practices
	dbName, err := request.RequireString("database")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get repository
	repo, ok := h.repositories[dbName]
	if !ok {
		return mcp.NewToolResultError(formatDatabaseNotFoundError(dbName, h.repositories)), nil
	}

	// Check if refresh is requested
	refresh := request.GetBool("refresh", false)

	// Try to get from cache first (if not refresh and Redis cache is enabled)
	cacheKey := fmt.Sprintf("introspection:%s", dbName)
	if !refresh && h.config.UseRedisCache && len(h.redisClients) > 0 {
		// Get first available Redis client
		for _, redisClient := range h.redisClients {
			cached, err := redisClient.Get(ctx, cacheKey)
			if err == nil && cached != "" {
				h.logger.InfoContext(ctx, "Returning introspection from cache", "database", dbName)
				return mcp.NewToolResultText(cached), nil
			}
			break
		}
	}

	// Get table list
	var tables []string
	switch r := repo.(type) {
	case *db.MySQLRepository:
		tables, err = r.GetTableList(ctx)
	case *db.PostgresRepository:
		tables, err = r.GetTableList(ctx)
	default:
		return mcp.NewToolResultError("unsupported repository type"), nil
	}

	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get table list", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to get table list: %v", err)), nil
	}

	// Get detailed info for each table
	var tableInfos []db.TableInfo
	for _, tableName := range tables {
		var info *db.TableInfo
		switch r := repo.(type) {
		case *db.MySQLRepository:
			info, err = r.GetTableInfo(ctx, tableName)
		case *db.PostgresRepository:
			info, err = r.GetTableInfo(ctx, tableName)
		}

		if err != nil {
			h.logger.WarnContext(ctx, "Failed to get table info", "table", tableName, "error", err)
			continue
		}

		// Get foreign keys
		var fks []db.ForeignKeyInfo
		switch r := repo.(type) {
		case *db.MySQLRepository:
			fks, _ = r.GetForeignKeys(ctx, tableName)
		case *db.PostgresRepository:
			fks, _ = r.GetForeignKeys(ctx, tableName)
		}

		// Add relationship info to table metadata
		if len(fks) > 0 {
			info.Description = fmt.Sprintf("Has %d foreign key(s)", len(fks))
		}

		tableInfos = append(tableInfos, *info)
	}

	// Build result
	result := map[string]any{
		"database":    dbName,
		"table_count": len(tableInfos),
		"tables":      tableInfos,
		"cached_at":   time.Now().UTC().Format(time.RFC3339),
		"cache_ttl":   h.config.CacheTTL,
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to marshal introspection response", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	// Cache the result if Redis cache is enabled
	if h.config.UseRedisCache && len(h.redisClients) > 0 {
		for _, redisClient := range h.redisClients {
			ttl := time.Duration(h.config.CacheTTL) * time.Second
			if err := redisClient.Set(ctx, cacheKey, string(resultJSON), ttl); err != nil {
				h.logger.WarnContext(ctx, "Failed to cache introspection result", "error", err)
			}
			break
		}
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}
