package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/SkillingX/mcp-localbridge/config"
)

// MySQLRepository implements Repository for MySQL databases
type MySQLRepository struct {
	db     *sqlx.DB
	name   string
	config config.MySQLConfig
}

// NewMySQLRepository creates a new MySQL repository
// CRITICAL: Uses parameterized queries throughout to prevent SQL injection
func NewMySQLRepository(cfg config.MySQLConfig) (*MySQLRepository, error) {
	// Connect to MySQL database
	db, err := sqlx.Connect("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL %s: %w", cfg.Name, err)
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
		return nil, fmt.Errorf("failed to ping MySQL %s: %w", cfg.Name, err)
	}

	return &MySQLRepository{
		db:     db,
		name:   cfg.Name,
		config: cfg,
	}, nil
}

// Query executes a parameterized SELECT query
// CRITICAL: Always use parameterized queries. Never concatenate user input into SQL!
func (r *MySQLRepository) Query(ctx context.Context, query string, params ...any) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, query, params...)
}

// QueryRow executes a parameterized query that returns at most one row
func (r *MySQLRepository) QueryRow(ctx context.Context, query string, params ...any) *sql.Row {
	return r.db.QueryRowContext(ctx, query, params...)
}

// Exec executes a parameterized statement (INSERT, UPDATE, DELETE)
// CRITICAL: Always use parameterized queries. Never concatenate user input!
func (r *MySQLRepository) Exec(ctx context.Context, query string, params ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, params...)
}

// Close closes the database connection
func (r *MySQLRepository) Close() error {
	return r.db.Close()
}

// GetName returns the repository name
func (r *MySQLRepository) GetName() string {
	return r.name
}

// GetDriver returns the database driver name
func (r *MySQLRepository) GetDriver() string {
	return "mysql"
}

// Ping checks if the database connection is alive
func (r *MySQLRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// GetTableList returns a list of all tables in the database
func (r *MySQLRepository) GetTableList(ctx context.Context) ([]string, error) {
	qb := NewQueryBuilder("mysql")
	query := qb.BuildTableList("")

	rows, err := r.db.QueryContext(ctx, query)
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
func (r *MySQLRepository) GetTableInfo(ctx context.Context, tableName string) (*TableInfo, error) {
	qb := NewQueryBuilder("mysql")
	query := qb.BuildTableSchema(tableName, "")

	rows, err := r.db.QueryContext(ctx, query)
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

	// Get row count (approximate)
	var rowCount int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&rowCount); err != nil {
		// Row count is optional, don't fail if we can't get it
		rowCount = 0
	}

	return &TableInfo{
		TableName: tableName,
		Columns:   columns,
		RowCount:  &rowCount,
	}, nil
}

// GetForeignKeys returns foreign key information for a table
func (r *MySQLRepository) GetForeignKeys(ctx context.Context, tableName string) ([]ForeignKeyInfo, error) {
	query := `
		SELECT
			constraint_name,
			table_name,
			column_name,
			referenced_table_name,
			referenced_column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = DATABASE()
			AND table_name = ?
			AND referenced_table_name IS NOT NULL
		ORDER BY constraint_name, ordinal_position`

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
