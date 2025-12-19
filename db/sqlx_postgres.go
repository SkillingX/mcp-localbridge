package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/SkillingX/mcp-localbridge/config"
)

// PostgresRepository implements Repository for PostgreSQL databases
type PostgresRepository struct {
	db     *sqlx.DB
	name   string
	config config.PostgresConfig
}

// NewPostgresRepository creates a new PostgreSQL repository
// CRITICAL: Uses parameterized queries throughout to prevent SQL injection
func NewPostgresRepository(cfg config.PostgresConfig) (*PostgresRepository, error) {
	// Connect to PostgreSQL database
	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL %s: %w", cfg.Name, err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL %s: %w", cfg.Name, err)
	}

	return &PostgresRepository{
		db:     db,
		name:   cfg.Name,
		config: cfg,
	}, nil
}

// Query executes a parameterized SELECT query
// CRITICAL: Always use parameterized queries. Never concatenate user input into SQL!
func (r *PostgresRepository) Query(ctx context.Context, query string, params ...any) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, query, params...)
}

// QueryRow executes a parameterized query that returns at most one row
func (r *PostgresRepository) QueryRow(ctx context.Context, query string, params ...any) *sql.Row {
	return r.db.QueryRowContext(ctx, query, params...)
}

// Exec executes a parameterized statement (INSERT, UPDATE, DELETE)
// CRITICAL: Always use parameterized queries. Never concatenate user input!
func (r *PostgresRepository) Exec(ctx context.Context, query string, params ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, params...)
}

// Close closes the database connection
func (r *PostgresRepository) Close() error {
	return r.db.Close()
}

// GetName returns the repository name
func (r *PostgresRepository) GetName() string {
	return r.name
}

// GetDriver returns the database driver name
func (r *PostgresRepository) GetDriver() string {
	return "postgres"
}

// Ping checks if the database connection is alive
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// GetTableList returns a list of all tables in the database
func (r *PostgresRepository) GetTableList(ctx context.Context) ([]string, error) {
	qb := NewQueryBuilder("postgres")
	query, params := qb.BuildTableList("")

	rows, err := r.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to query table list: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating table list: %w", err)
	}

	return tables, nil
}

// GetTableInfo returns detailed information about a table
func (r *PostgresRepository) GetTableInfo(ctx context.Context, tableName string) (*TableInfo, error) {
	qb := NewQueryBuilder("postgres")
	query, params, err := qb.BuildTableSchema(tableName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to build table schema query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to query table schema: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var isNullable string
		var defaultVal sql.NullString

		if err := rows.Scan(&col.Name, &col.DataType, &isNullable, &defaultVal, &col.IsPrimaryKey); err != nil {
			return nil, fmt.Errorf("failed to scan column info: %w", err)
		}

		col.IsNullable = (isNullable == "YES")
		if defaultVal.Valid {
			col.DefaultValue = &defaultVal.String
		}

		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating columns: %w", err)
	}

	// Get row count (approximate from pg_class)
	var rowCount int64
	countQuery := fmt.Sprintf("SELECT reltuples::bigint FROM pg_class WHERE relname = $1")
	if err := r.db.QueryRowContext(ctx, countQuery, tableName).Scan(&rowCount); err != nil {
		// Row count is optional, don't fail if we can't get it
		rowCount = 0
	}

	return &TableInfo{
		TableName: tableName,
		Schema:    "public",
		Columns:   columns,
		RowCount:  &rowCount,
	}, nil
}

// GetForeignKeys returns foreign key information for a table
func (r *PostgresRepository) GetForeignKeys(ctx context.Context, tableName string) ([]ForeignKeyInfo, error) {
	query := `
		SELECT
			tc.constraint_name,
			tc.table_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_name = $1
		ORDER BY tc.constraint_name`

	rows, err := r.db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer rows.Close()

	var foreignKeys []ForeignKeyInfo
	for rows.Next() {
		var fk ForeignKeyInfo
		if err := rows.Scan(&fk.Name, &fk.SourceTable, &fk.SourceColumn, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}
		foreignKeys = append(foreignKeys, fk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating foreign keys: %w", err)
	}

	return foreignKeys, nil
}
