package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/SkillingX/mcp-localbridge/config"
	"github.com/SkillingX/mcp-localbridge/db"
)

// SemanticSummaryHandler generates semantic summaries of table data
type SemanticSummaryHandler struct {
	repositories map[string]db.Repository
	config       config.SemanticSummaryConfig
	logger       *slog.Logger
}

// NewSemanticSummaryHandler creates a new semantic summary handler
func NewSemanticSummaryHandler(
	repos map[string]db.Repository,
	cfg config.SemanticSummaryConfig,
	logger *slog.Logger,
) *SemanticSummaryHandler {
	return &SemanticSummaryHandler{
		repositories: repos,
		config:       cfg,
		logger:       logger,
	}
}

// HandleSemanticSummary generates a semantic summary of table data
// This handler provides an LLM prompt template that MCP clients can use to generate summaries
func (h *SemanticSummaryHandler) HandleSemanticSummary(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.InfoContext(ctx, "Handling semantic_summary tool request")

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
		return mcp.NewToolResultError(formatDatabaseNotFoundError(dbName, h.repositories)), nil
	}

	// Get table schema
	var tableInfo *db.TableInfo
	switch r := repo.(type) {
	case *db.MySQLRepository:
		tableInfo, err = r.GetTableInfo(ctx, tableName)
	case *db.PostgresRepository:
		tableInfo, err = r.GetTableInfo(ctx, tableName)
	default:
		return mcp.NewToolResultError("unsupported repository type"), nil
	}

	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get table schema", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to get table schema: %v", err)), nil
	}

	// Sample data from the table
	qb := db.NewQueryBuilder(repo.GetDriver())
	query, params := qb.BuildSelect(tableName, nil, h.config.SampleSize, 0, "")

	rows, err := repo.Query(ctx, query, params...)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to sample table data", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to sample table data: %v", err)), nil
	}
	defer rows.Close()

	// Parse sample data
	sampleData := []map[string]any{}
	columns, _ := rows.Columns()

	for rows.Next() && len(sampleData) < h.config.SampleSize {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			h.logger.WarnContext(ctx, "Failed to scan sample data row", "table", tableName, "error", err)
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
		sampleData = append(sampleData, rowMap)
	}

	// Build LLM prompt template for MCP clients
	llmPromptTemplate := buildSemanticSummaryPrompt(tableName, tableInfo, sampleData)

	result := map[string]any{
		"database":     dbName,
		"table":        tableName,
		"schema":       tableInfo,
		"sample_count": len(sampleData),
		"sample_data":  sampleData,
		"llm_prompt":   llmPromptTemplate,
		"description":  "This result includes an LLM prompt template for generating semantic summaries. MCP clients (like Vibe Coding) can use this prompt to call their LLM and get business-meaningful insights.",
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to marshal semantic summary response", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// buildSemanticSummaryPrompt creates an LLM prompt template for semantic summarization
// This template is provided to MCP clients for them to invoke their own LLM
func buildSemanticSummaryPrompt(tableName string, tableInfo *db.TableInfo, sampleData []map[string]any) string {
	sampleJSON, _ := json.MarshalIndent(sampleData, "", "  ")

	return fmt.Sprintf(`# Task: Generate a Semantic Summary of Database Table

You are analyzing a database table named "%s" to provide business-meaningful insights.

## Table Schema:
- Table Name: %s
- Column Count: %d
- Columns:
%s

## Sample Data (%d rows):
%s

## Your Task:
Please analyze the table schema and sample data, then provide:

1. **Business Purpose**: What is this table likely used for in the business context?
2. **Data Patterns**: What patterns or trends do you observe in the sample data?
3. **Key Insights**: What are the most important characteristics of this data?
4. **Data Quality**: Are there any potential data quality issues visible in the sample?
5. **Recommendations**: Any suggestions for data usage or further analysis?

Please provide your response in a clear, structured format.`,
		tableName,
		tableInfo.TableName,
		len(tableInfo.Columns),
		formatColumnsForPrompt(tableInfo.Columns),
		len(sampleData),
		sampleJSON,
	)
}

// formatColumnsForPrompt formats column information for the LLM prompt
func formatColumnsForPrompt(columns []db.ColumnInfo) string {
	result := ""
	for _, col := range columns {
		nullable := "NOT NULL"
		if col.IsNullable {
			nullable = "NULL"
		}
		primaryKey := ""
		if col.IsPrimaryKey {
			primaryKey = " [PRIMARY KEY]"
		}
		result += fmt.Sprintf("  - %s (%s, %s)%s\n", col.Name, col.DataType, nullable, primaryKey)
	}
	return result
}
