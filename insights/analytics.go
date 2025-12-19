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

	// Extract required parameters using mcp-go v0.43.2 best practices
	dbName, err := request.RequireString("database")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tableName, err := request.RequireString("table")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	column, err := request.RequireString("column")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	aggFunction, err := request.RequireString("function")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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
		return mcp.NewToolResultError(formatDatabaseNotFoundError(dbName, h.repositories)), nil
	}

	// Optional: conditions and group_by
	var conditions map[string]any
	condStr := request.GetString("conditions", "")
	if condStr != "" {
		if err := json.Unmarshal([]byte(condStr), &conditions); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid conditions JSON: %v", err)), nil
		}
	}

	groupBy := request.GetString("group_by", "")

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
			h.logger.WarnContext(ctx, "Failed to scan analytics row", "error", err)
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

	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to marshal analytics response", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}
