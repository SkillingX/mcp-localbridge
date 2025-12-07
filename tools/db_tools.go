package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/SkillingX/mcp-localbridge/config"
	"github.com/SkillingX/mcp-localbridge/db"
)

// DBToolsHandler provides database-related MCP tools
type DBToolsHandler struct {
	repositories map[string]db.Repository
	config       config.DBToolsConfig
	logger       *slog.Logger
}

// NewDBToolsHandler creates a new database tools handler
func NewDBToolsHandler(repos map[string]db.Repository, cfg config.DBToolsConfig, logger *slog.Logger) *DBToolsHandler {
	return &DBToolsHandler{
		repositories: repos,
		config:       cfg,
		logger:       logger,
	}
}

// HandleDBQuery executes a database query with safe parameter binding
// CRITICAL: Uses parameterized queries to prevent SQL injection
func (h *DBToolsHandler) HandleDBQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling db_query tool request")

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

	// Get repository
	repo, ok := h.repositories[dbName]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("database '%s' not found or not enabled", dbName)), nil
	}

	// Parse conditions (WHERE clause as JSON object)
	var conditions map[string]any
	if condStr, ok := args["conditions"].(string); ok && condStr != "" {
		if err := json.Unmarshal([]byte(condStr), &conditions); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid conditions JSON: %v", err)), nil
		}
	}

	// Parse limit and offset
	limit := h.config.MaxRows
	if limitStr, ok := args["limit"].(string); ok {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= h.config.MaxRows {
			limit = l
		}
	}

	offset := 0
	if offsetStr, ok := args["offset"].(string); ok {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Parse order_by
	orderBy := ""
	if ob, ok := args["order_by"].(string); ok {
		orderBy = ob
	}

	// Check dry-run mode (returns SQL preview without execution)
	dryRun := h.config.DefaultDryRun
	if dryRunStr, ok := args["dry_run"].(string); ok {
		dryRun = strings.ToLower(dryRunStr) == "true"
	}

	// Build query using QueryBuilder (always parameterized)
	qb := db.NewQueryBuilder(repo.GetDriver())
	query, params := qb.BuildSelect(tableName, conditions, limit, offset, orderBy)

	// If dry-run, return the query preview without executing
	if dryRun {
		preview := map[string]any{
			"dry_run":     true,
			"query":       query,
			"params":      params,
			"description": "Preview of the SQL query. Set dry_run=false to execute.",
		}
		previewJSON, _ := json.MarshalIndent(preview, "", "  ")
		return mcp.NewToolResultText(string(previewJSON)), nil
	}

	// Execute query with timeout
	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(h.config.QueryTimeout)*time.Second)
	defer cancel()

	// CRITICAL: Execute parameterized query to prevent SQL injection
	rows, err := repo.Query(queryCtx, query, params...)
	if err != nil {
		h.logger.ErrorContext(ctx, "Query execution failed", "error", err, "query", query)
		return mcp.NewToolResultError(fmt.Sprintf("query execution failed: %v", err)), nil
	}
	defer rows.Close()

	// Parse results
	result, err := h.parseQueryResult(rows)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse query results: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// HandleDBTableList returns a list of all tables in the database
func (h *DBToolsHandler) HandleDBTableList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling db_table_list tool request")

	// Extract database name
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

	// Get table list based on repository type
	var tables []string
	var err error
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

	result := map[string]any{
		"database": dbName,
		"tables":   tables,
		"count":    len(tables),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// HandleDBTablePreview returns a preview of table data
func (h *DBToolsHandler) HandleDBTablePreview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling db_table_preview tool request")

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

	// Get repository
	repo, ok := h.repositories[dbName]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("database '%s' not found or not enabled", dbName)), nil
	}

	// Build preview query (limit to configured preview limit)
	qb := db.NewQueryBuilder(repo.GetDriver())
	query, params := qb.BuildSelect(tableName, nil, h.config.PreviewLimit, 0, "")

	// Execute query
	// CRITICAL: Uses parameterized query
	rows, err := repo.Query(ctx, query, params...)
	if err != nil {
		h.logger.ErrorContext(ctx, "Preview query failed", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("preview query failed: %v", err)), nil
	}
	defer rows.Close()

	// Parse results
	result, err := h.parseQueryResult(rows)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse preview results: %v", err)), nil
	}

	// Add metadata
	response := map[string]any{
		"database":      dbName,
		"table":         tableName,
		"preview_limit": h.config.PreviewLimit,
		"data":          result,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// parseQueryResult parses SQL rows into a QueryResult structure
func (h *DBToolsHandler) parseQueryResult(rows *sql.Rows) (*db.QueryResult, error) {
	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Prepare result storage
	var resultRows []map[string]any

	// Iterate through rows
	for rows.Next() {
		// Create a slice of any to hold each column value
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// Scan row
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert to map
		rowMap := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for better JSON serialization
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}
		resultRows = append(resultRows, rowMap)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return &db.QueryResult{
		Columns:  columns,
		Rows:     resultRows,
		RowCount: len(resultRows),
	}, nil
}
