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

// RelationshipHandler analyzes relationships between database tables
type RelationshipHandler struct {
	repositories map[string]db.Repository
	redisClients map[string]*cache.RedisClient
	config       config.RelationshipConfig
	logger       *slog.Logger
}

// NewRelationshipHandler creates a new relationship handler
func NewRelationshipHandler(
	repos map[string]db.Repository,
	redisClients map[string]*cache.RedisClient,
	cfg config.RelationshipConfig,
	logger *slog.Logger,
) *RelationshipHandler {
	return &RelationshipHandler{
		repositories: repos,
		redisClients: redisClients,
		config:       cfg,
		logger:       logger,
	}
}

// HandleRelationship analyzes table relationships (foreign keys)
func (h *RelationshipHandler) HandleRelationship(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling relationship tool request")

	// Extract parameters
	args := request.Params.Arguments
	dbName, ok := args["database"].(string)
	if !ok || dbName == "" {
		return mcp.NewToolResultError("missing required parameter 'database'"), nil
	}

	// Get repository
	repo, ok := h.repositories[dbName]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("database '%s' not found or not enabled", dbName)), nil
	}

	// Optional: specific table to analyze
	tableName := ""
	if tn, ok := args["table"].(string); ok {
		tableName = tn
	}

	// Check cache
	cacheKey := fmt.Sprintf("relationships:%s", dbName)
	if tableName != "" {
		cacheKey = fmt.Sprintf("relationships:%s:%s", dbName, tableName)
	}

	if h.config.CacheEnabled && len(h.redisClients) > 0 {
		for _, redisClient := range h.redisClients {
			cached, err := redisClient.Get(ctx, cacheKey)
			if err == nil && cached != "" {
				h.logger.InfoContext(ctx, "Returning relationships from cache", "database", dbName)
				return mcp.NewToolResultText(cached), nil
			}
			break
		}
	}

	// Get table list
	var tables []string
	var err error
	if tableName != "" {
		tables = []string{tableName}
	} else {
		switch r := repo.(type) {
		case *db.MySQLRepository:
			tables, err = r.GetTableList(ctx)
		case *db.PostgresRepository:
			tables, err = r.GetTableList(ctx)
		default:
			return mcp.NewToolResultError("unsupported repository type"), nil
		}

		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get table list: %v", err)), nil
		}
	}

	// Build relationship graph
	relationshipGraph := make(map[string][]db.ForeignKeyInfo)

	for _, table := range tables {
		var fks []db.ForeignKeyInfo
		var fkErr error
		switch r := repo.(type) {
		case *db.MySQLRepository:
			fks, fkErr = r.GetForeignKeys(ctx, table)
		case *db.PostgresRepository:
			fks, fkErr = r.GetForeignKeys(ctx, table)
		}

		if fkErr != nil {
			h.logger.WarnContext(ctx, "Failed to get foreign keys", "table", table, "error", fkErr)
			continue
		}

		if len(fks) > 0 {
			relationshipGraph[table] = fks
		}
	}

	// Build result
	result := map[string]any{
		"database":           dbName,
		"table_filter":       tableName,
		"relationships":      relationshipGraph,
		"relationship_count": countRelationships(relationshipGraph),
		"cached_at":          time.Now().UTC().Format(time.RFC3339),
		"llm_prompt":         buildRelationshipPrompt(dbName, relationshipGraph),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	// Cache the result
	if h.config.CacheEnabled && len(h.redisClients) > 0 {
		for _, redisClient := range h.redisClients {
			ttl := time.Duration(h.config.CacheTTL) * time.Second
			if err := redisClient.Set(ctx, cacheKey, string(resultJSON), ttl); err != nil {
				h.logger.WarnContext(ctx, "Failed to cache relationships", "error", err)
			}
			break
		}
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// countRelationships counts total number of foreign key relationships
func countRelationships(graph map[string][]db.ForeignKeyInfo) int {
	count := 0
	for _, fks := range graph {
		count += len(fks)
	}
	return count
}

// buildRelationshipPrompt creates an LLM prompt for relationship analysis
func buildRelationshipPrompt(dbName string, graph map[string][]db.ForeignKeyInfo) string {
	graphJSON, _ := json.MarshalIndent(graph, "", "  ")

	return fmt.Sprintf(`# Task: Analyze Database Relationships

You are analyzing the relationships (foreign keys) in the "%s" database to understand the data model.

## Relationship Graph:
%s

## Your Task:
Please analyze the foreign key relationships and provide:

1. **Entity Relationship Overview**: Describe the main entities and how they relate to each other
2. **Central Tables**: Identify which tables are most connected (hub tables)
3. **Data Flow**: Describe typical data flow patterns based on relationships
4. **Potential Issues**: Identify any missing relationships or potential design issues
5. **Query Recommendations**: Suggest useful JOIN queries based on these relationships

Please provide your response in a clear, structured format.`,
		dbName,
		graphJSON,
	)
}
