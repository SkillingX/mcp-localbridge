package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
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

// getAvailableDatabases returns a sorted list of available database names
func (h *DBToolsHandler) getAvailableDatabases() []string {
	databases := make([]string, 0, len(h.repositories))
	for name := range h.repositories {
		databases = append(databases, name)
	}
	sort.Strings(databases)
	return databases
}

// formatDatabaseNotFoundError creates a helpful error message with available databases
// Uses shared implementation from db package
func (h *DBToolsHandler) formatDatabaseNotFoundError(dbName string) string {
	return db.FormatDatabaseNotFoundError(dbName, h.repositories)
}

// HandleDBQuery executes a database query with safe parameter binding
// CRITICAL: Uses parameterized queries to prevent SQL injection
func (h *DBToolsHandler) HandleDBQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling db_query tool request")

	// Extract required parameters using mcp-go v0.43.2 best practices
	dbName, err := request.RequireString("database")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tableName, err := request.RequireString("table")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get repository
	repo, ok := h.repositories[dbName]
	if !ok {
		return mcp.NewToolResultError(h.formatDatabaseNotFoundError(dbName)), nil
	}

	// Parse conditions (WHERE clause as JSON object)
	var conditions map[string]any
	condStr := request.GetString("conditions", "")
	if condStr != "" {
		if err := json.Unmarshal([]byte(condStr), &conditions); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid conditions JSON: %v", err)), nil
		}
	}

	// Parse limit and offset with GetInt (more type-safe)
	limit := request.GetInt("limit", h.config.MaxRows)
	if limit <= 0 || limit > h.config.MaxRows {
		limit = h.config.MaxRows
	}

	offset := request.GetInt("offset", 0)
	if offset < 0 {
		offset = 0
	}

	// Parse order_by
	orderBy := request.GetString("order_by", "")

	// Check dry-run mode with GetBool
	dryRun := request.GetBool("dry_run", h.config.DefaultDryRun)

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
		previewJSON, err := json.MarshalIndent(preview, "", "  ")
		if err != nil {
			h.logger.ErrorContext(ctx, "Failed to marshal dry-run preview", "error", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal preview: %v", err)), nil
		}
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

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to marshal query result", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// HandleDBTableList returns a list of all tables in the database
func (h *DBToolsHandler) HandleDBTableList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling db_table_list tool request")

	// Extract required parameter using mcp-go v0.43.2 best practices
	dbName, err := request.RequireString("database")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get repository
	repo, ok := h.repositories[dbName]
	if !ok {
		return mcp.NewToolResultError(h.formatDatabaseNotFoundError(dbName)), nil
	}

	// Get table list based on repository type
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

	result := map[string]any{
		"database": dbName,
		"tables":   tables,
		"count":    len(tables),
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to marshal table list", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal table list: %v", err)), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// HandleDBTablePreview returns a preview of table data
func (h *DBToolsHandler) HandleDBTablePreview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling db_table_preview tool request")

	// Extract required parameters using mcp-go v0.43.2 best practices
	dbName, err := request.RequireString("database")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tableName, err := request.RequireString("table")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get repository
	repo, ok := h.repositories[dbName]
	if !ok {
		return mcp.NewToolResultError(h.formatDatabaseNotFoundError(dbName)), nil
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

	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to marshal table preview", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal preview: %v", err)), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// HandleDBListDatabases returns a list of all available database instances
func (h *DBToolsHandler) HandleDBListDatabases(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling db_list_databases tool request")

	// Get all available databases with their types
	databases := make([]map[string]any, 0, len(h.repositories))
	for name, repo := range h.repositories {
		dbInfo := map[string]any{
			"name":   name,
			"driver": repo.GetDriver(),
		}
		databases = append(databases, dbInfo)
	}

	// Sort by name for consistent output
	sort.Slice(databases, func(i, j int) bool {
		return databases[i]["name"].(string) < databases[j]["name"].(string)
	})

	result := map[string]any{
		"databases": databases,
		"count":     len(databases),
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to marshal database list", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal database list: %v", err)), nil
	}
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
