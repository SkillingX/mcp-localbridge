package db

import (
	"context"
	"database/sql"
)

// Repository defines the interface for database operations
// This design allows easy testing and supports multiple database types
type Repository interface {
	// Query executes a parameterized query and returns rows
	// CRITICAL: params must be used to prevent SQL injection
	Query(ctx context.Context, query string, params ...any) (*sql.Rows, error)

	// QueryRow executes a parameterized query that returns at most one row
	QueryRow(ctx context.Context, query string, params ...any) *sql.Row

	// Exec executes a parameterized statement (INSERT, UPDATE, DELETE)
	// CRITICAL: params must be used to prevent SQL injection
	Exec(ctx context.Context, query string, params ...any) (sql.Result, error)

	// Close closes the database connection
	Close() error

	// GetName returns the repository name/identifier
	GetName() string

	// GetDriver returns the database driver name (mysql, postgres, etc.)
	GetDriver() string

	// Ping checks if the database connection is alive
	Ping(ctx context.Context) error
}

// QueryResult represents a generic query result
type QueryResult struct {
	Columns  []string         `json:"columns"`
	Rows     []map[string]any `json:"rows"`
	RowCount int              `json:"row_count"`
}

// TableInfo represents database table metadata
type TableInfo struct {
	TableName   string       `json:"table_name"`
	Schema      string       `json:"schema,omitempty"`
	Columns     []ColumnInfo `json:"columns"`
	Indexes     []IndexInfo  `json:"indexes,omitempty"`
	RowCount    *int64       `json:"row_count,omitempty"`
	Description string       `json:"description,omitempty"`
}

// ColumnInfo represents database column metadata
type ColumnInfo struct {
	Name         string  `json:"name"`
	DataType     string  `json:"data_type"`
	IsNullable   bool    `json:"is_nullable"`
	IsPrimaryKey bool    `json:"is_primary_key"`
	DefaultValue *string `json:"default_value,omitempty"`
	Description  string  `json:"description,omitempty"`
}

// IndexInfo represents database index metadata
type IndexInfo struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	IsUnique  bool     `json:"is_unique"`
	IsPrimary bool     `json:"is_primary"`
}

// ForeignKeyInfo represents foreign key relationship
type ForeignKeyInfo struct {
	Name             string `json:"name"`
	SourceTable      string `json:"source_table"`
	SourceColumn     string `json:"source_column"`
	ReferencedTable  string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
	OnDelete         string `json:"on_delete,omitempty"`
	OnUpdate         string `json:"on_update,omitempty"`
}
