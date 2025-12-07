package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/SkillingX/mcp-localbridge/config"
	"github.com/SkillingX/mcp-localbridge/db"
)

// AnalyticsHandler provides analytical queries on database tables
type AnalyticsHandler struct {
	repositories map[string]db.Repository
	config       config.AnalyticsConfig
	logger       *slog.Logger
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(
	repos map[string]db.Repository,
	cfg config.AnalyticsConfig,
	logger *slog.Logger,
) *AnalyticsHandler {
	return &AnalyticsHandler{
		repositories: repos,
		config:       cfg,
		logger:       logger,
	}
}

// HandleAnalytics performs analytical aggregations on table data
// CRITICAL: Uses parameterized queries to prevent SQL injection
func (h *AnalyticsHandler) HandleAnalytics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling analytics tool request")

	// Extract parameters
	args := request.Params.Arguments
	dbName, ok := args["database"].(string)
	if !ok || dbName == "" {
		return mcp.NewToolResultError("missing required parameter 'database'"), nil
	}

	tableName, ok := args["table"].(string)
	if !ok || tableName == "" {
		return mcp.NewToolResultError("missing required parameter 'table'"), nil
	}

	column, ok := args["column"].(string)
	if !ok || column == "" {
		return mcp.NewToolResultError("missing required parameter 'column'"), nil
	}

	aggFunction, ok := args["function"].(string)
	if !ok || aggFunction == "" {
		return mcp.NewToolResultError("missing required parameter 'function'"), nil
	}

	// Validate aggregate function
	aggFunction = strings.ToUpper(aggFunction)
	validFuncs := map[string]bool{
		"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true,
	}
	if !validFuncs[aggFunction] {
		return mcp.NewToolResultError(fmt.Sprintf("invalid aggregate function: %s. Must be one of: COUNT, SUM, AVG, MIN, MAX", aggFunction)), nil
	}

	// Get repository
	repo, ok := h.repositories[dbName]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("database '%s' not found or not enabled", dbName)), nil
	}

	// Optional: conditions and group_by
	var conditions map[string]any
	if condStr, ok := args["conditions"].(string); ok && condStr != "" {
		if err := json.Unmarshal([]byte(condStr), &conditions); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid conditions JSON: %v", err)), nil
		}
	}

	groupBy := ""
	if gb, ok := args["group_by"].(string); ok {
		groupBy = gb
	}

	// Build aggregation query using QueryBuilder (always parameterized)
	qb := db.NewQueryBuilder(repo.GetDriver())
	query, params, err := qb.BuildAggregation(tableName, column, aggFunction, conditions, groupBy)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to build query: %v", err)), nil
	}

	// Execute query with timeout
	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(h.config.ExecutionTimeout)*time.Second)
	defer cancel()

	// CRITICAL: Execute parameterized query to prevent SQL injection
	rows, err := repo.Query(queryCtx, query, params...)
	if err != nil {
		h.logger.ErrorContext(ctx, "Analytics query failed", "error", err, "query", query)
		return mcp.NewToolResultError(fmt.Sprintf("query execution failed: %v", err)), nil
	}
	defer rows.Close()

	// Parse results
	var results []map[string]any
	columns, _ := rows.Columns()

	for rows.Next() && len(results) < h.config.MaxResultRows {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		rowMap := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}
		results = append(results, rowMap)
	}

	// Build response
	response := map[string]any{
		"database":     dbName,
		"table":        tableName,
		"column":       column,
		"function":     aggFunction,
		"group_by":     groupBy,
		"result_count": len(results),
		"results":      results,
		"query":        query,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
