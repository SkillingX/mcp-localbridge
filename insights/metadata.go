package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/SkillingX/mcp-localbridge/db"
)

// MetadataHandler retrieves database metadata (table/column comments, etc.)
type MetadataHandler struct {
	repositories map[string]db.Repository
	logger       *slog.Logger
}

// NewMetadataHandler creates a new metadata handler
func NewMetadataHandler(
	repos map[string]db.Repository,
	logger *slog.Logger,
) *MetadataHandler {
	return &MetadataHandler{
		repositories: repos,
		logger:       logger,
	}
}

// HandleMetadata retrieves metadata for tables and columns
func (h *MetadataHandler) HandleMetadata(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling metadata tool request")

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
		return mcp.NewToolResultError(fmt.Sprintf("database '%s' not found or not enabled", dbName)), nil
	}

	// Get table metadata based on database type
	var metadata map[string]any

	switch r := repo.(type) {
	case *db.MySQLRepository:
		metadata, err = h.getMySQLMetadata(ctx, r, tableName)
	case *db.PostgresRepository:
		metadata, err = h.getPostgresMetadata(ctx, r, tableName)
	default:
		return mcp.NewToolResultError("unsupported repository type"), nil
	}

	if err != nil {
		h.logger.WarnContext(ctx, "Failed to retrieve metadata", "error", err)
		// Return empty metadata instead of error
		metadata = map[string]any{
			"database":      dbName,
			"table":         tableName,
			"warning":       "Metadata retrieval failed or not supported",
			"error":         err.Error(),
			"columns":       []string{},
			"table_comment": "",
		}
	}

	resultJSON, _ := json.MarshalIndent(metadata, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// getMySQLMetadata retrieves metadata from MySQL information_schema
func (h *MetadataHandler) getMySQLMetadata(ctx context.Context, repo *db.MySQLRepository, tableName string) (map[string]any, error) {
	// Query for table comment
	tableCommentQuery := `
		SELECT table_comment
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?`

	var tableComment string
	row := repo.QueryRow(ctx, tableCommentQuery, tableName)
	if err := row.Scan(&tableComment); err != nil {
		h.logger.WarnContext(ctx, "Failed to get table comment", "error", err)
		tableComment = ""
	}

	// Query for column comments
	columnCommentQuery := `
		SELECT column_name, column_comment, column_type, is_nullable, column_key
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ?
		ORDER BY ordinal_position`

	rows, err := repo.Query(ctx, columnCommentQuery, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get column metadata: %w", err)
	}
	defer rows.Close()

	var columns []map[string]any
	for rows.Next() {
		var colName, colComment, colType, isNullable, colKey string
		if err := rows.Scan(&colName, &colComment, &colType, &isNullable, &colKey); err != nil {
			continue
		}

		columns = append(columns, map[string]any{
			"name":     colName,
			"type":     colType,
			"nullable": isNullable == "YES",
			"key":      colKey,
			"comment":  colComment,
		})
	}

	return map[string]any{
		"database":      repo.GetName(),
		"table":         tableName,
		"table_comment": tableComment,
		"columns":       columns,
		"column_count":  len(columns),
	}, nil
}

// getPostgresMetadata retrieves metadata from PostgreSQL information_schema
func (h *MetadataHandler) getPostgresMetadata(ctx context.Context, repo *db.PostgresRepository, tableName string) (map[string]any, error) {
	// PostgreSQL table comments require accessing pg_catalog
	tableCommentQuery := `
		SELECT obj_description($1::regclass, 'pg_class')`

	var tableComment sql.NullString
	row := repo.QueryRow(ctx, tableCommentQuery, tableName)
	if err := row.Scan(&tableComment); err != nil {
		h.logger.WarnContext(ctx, "Failed to get table comment", "error", err)
	}

	// Query for column comments
	columnCommentQuery := `
		SELECT
			c.column_name,
			c.data_type,
			c.is_nullable,
			c.column_default,
			pgd.description as column_comment
		FROM information_schema.columns c
		LEFT JOIN pg_catalog.pg_statio_all_tables st
			ON c.table_schema = st.schemaname AND c.table_name = st.relname
		LEFT JOIN pg_catalog.pg_description pgd
			ON pgd.objoid = st.relid AND pgd.objsubid = c.ordinal_position
		WHERE c.table_schema = 'public' AND c.table_name = $1
		ORDER BY c.ordinal_position`

	rows, err := repo.Query(ctx, columnCommentQuery, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get column metadata: %w", err)
	}
	defer rows.Close()

	var columns []map[string]any
	for rows.Next() {
		var colName, dataType, isNullable string
		var colDefault, colComment sql.NullString

		if err := rows.Scan(&colName, &dataType, &isNullable, &colDefault, &colComment); err != nil {
			continue
		}

		columns = append(columns, map[string]any{
			"name":     colName,
			"type":     dataType,
			"nullable": isNullable == "YES",
			"default":  colDefault.String,
			"comment":  colComment.String,
		})
	}

	result := map[string]any{
		"database":     repo.GetName(),
		"table":        tableName,
		"columns":      columns,
		"column_count": len(columns),
	}

	if tableComment.Valid {
		result["table_comment"] = tableComment.String
	} else {
		result["table_comment"] = ""
	}

	return result, nil
}
