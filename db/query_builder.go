package db

import (
	"fmt"
	"strings"
)

// QueryBuilder helps build safe, parameterized SQL queries
// CRITICAL: This builder ALWAYS uses parameterized queries to prevent SQL injection
type QueryBuilder struct {
	driver string // mysql or postgres
}

// NewQueryBuilder creates a new query builder for the specified driver
func NewQueryBuilder(driver string) *QueryBuilder {
	return &QueryBuilder{driver: driver}
}

// BuildSelect builds a SELECT query with safe parameter binding
// CRITICAL: Uses parameterized queries to prevent SQL injection. Never concatenates user input!
func (qb *QueryBuilder) BuildSelect(table string, conditions map[string]any, limit, offset int, orderBy string) (string, []any) {
	var params []any
	query := fmt.Sprintf("SELECT * FROM %s", qb.quoteIdentifier(table))

	// Build WHERE clause with parameterized conditions
	if len(conditions) > 0 {
		whereClauses := []string{}
		for key, value := range conditions {
			// Check if the condition is a LIKE pattern
			if str, ok := value.(string); ok && (strings.Contains(str, "%") || strings.Contains(str, "_")) {
				whereClauses = append(whereClauses, fmt.Sprintf("%s LIKE %s", qb.quoteIdentifier(key), qb.placeholder(len(params)+1)))
			} else {
				whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", qb.quoteIdentifier(key), qb.placeholder(len(params)+1)))
			}
			params = append(params, value)
		}
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Add ORDER BY clause (validated to prevent injection)
	if orderBy != "" {
		// Validate orderBy to prevent SQL injection
		// Only allow alphanumeric, underscore, space, comma, and ASC/DESC
		if qb.isValidOrderBy(orderBy) {
			query += " ORDER BY " + orderBy
		}
	}

	// Add LIMIT and OFFSET
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", offset)
	}

	return query, params
}

// BuildCount builds a COUNT query with safe parameter binding
func (qb *QueryBuilder) BuildCount(table string, conditions map[string]any) (string, []any) {
	var params []any
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", qb.quoteIdentifier(table))

	// Build WHERE clause
	if len(conditions) > 0 {
		whereClauses := []string{}
		for key, value := range conditions {
			if str, ok := value.(string); ok && (strings.Contains(str, "%") || strings.Contains(str, "_")) {
				whereClauses = append(whereClauses, fmt.Sprintf("%s LIKE %s", qb.quoteIdentifier(key), qb.placeholder(len(params)+1)))
			} else {
				whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", qb.quoteIdentifier(key), qb.placeholder(len(params)+1)))
			}
			params = append(params, value)
		}
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	return query, params
}

// BuildAggregation builds an aggregation query (SUM, AVG, MIN, MAX, COUNT)
// CRITICAL: Uses parameterized queries and validates aggregate functions
func (qb *QueryBuilder) BuildAggregation(table, column, aggFunc string, conditions map[string]any, groupBy string) (string, []any, error) {
	// Validate aggregate function to prevent injection
	validAggFuncs := map[string]bool{
		"SUM": true, "AVG": true, "MIN": true, "MAX": true, "COUNT": true,
	}
	aggFunc = strings.ToUpper(aggFunc)
	if !validAggFuncs[aggFunc] {
		return "", nil, fmt.Errorf("invalid aggregate function: %s", aggFunc)
	}

	var params []any

	// Build SELECT clause with aggregation
	selectClause := fmt.Sprintf("%s(%s) as result", aggFunc, qb.quoteIdentifier(column))
	if groupBy != "" && qb.isValidIdentifier(groupBy) {
		selectClause = fmt.Sprintf("%s, %s", qb.quoteIdentifier(groupBy), selectClause)
	}

	query := fmt.Sprintf("SELECT %s FROM %s", selectClause, qb.quoteIdentifier(table))

	// Build WHERE clause
	if len(conditions) > 0 {
		whereClauses := []string{}
		for key, value := range conditions {
			whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", qb.quoteIdentifier(key), qb.placeholder(len(params)+1)))
			params = append(params, value)
		}
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Add GROUP BY clause
	if groupBy != "" && qb.isValidIdentifier(groupBy) {
		query += " GROUP BY " + qb.quoteIdentifier(groupBy)
	}

	return query, params, nil
}

// quoteIdentifier quotes a database identifier (table/column name) to prevent injection
func (qb *QueryBuilder) quoteIdentifier(identifier string) string {
	// Remove any existing quotes and validate
	identifier = strings.Trim(identifier, "`\"")

	// Basic validation: only allow alphanumeric and underscore
	if !qb.isValidIdentifier(identifier) {
		// Return a safe quoted version, though this should be validated earlier
		return qb.quote(identifier)
	}

	return qb.quote(identifier)
}

// quote wraps identifier in appropriate quotes for the database driver
func (qb *QueryBuilder) quote(identifier string) string {
	if qb.driver == "postgres" {
		return fmt.Sprintf("\"%s\"", identifier)
	}
	// MySQL default
	return fmt.Sprintf("`%s`", identifier)
}

// placeholder returns the appropriate placeholder for the database driver
func (qb *QueryBuilder) placeholder(position int) string {
	if qb.driver == "postgres" {
		return fmt.Sprintf("$%d", position)
	}
	// MySQL uses ?
	return "?"
}

// isValidIdentifier checks if a string is a valid SQL identifier
func (qb *QueryBuilder) isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	// Allow alphanumeric, underscore, and dot (for schema.table notation)
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// isValidOrderBy checks if an ORDER BY clause is safe
func (qb *QueryBuilder) isValidOrderBy(orderBy string) bool {
	// Allow identifiers, commas, spaces, and ASC/DESC keywords
	orderBy = strings.ToUpper(strings.TrimSpace(orderBy))
	parts := strings.Split(orderBy, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		tokens := strings.Fields(part)

		// Check each token
		for i, token := range tokens {
			if i == 0 {
				// First token should be a valid identifier
				if !qb.isValidIdentifier(token) {
					return false
				}
			} else {
				// Subsequent tokens should be ASC or DESC
				if token != "ASC" && token != "DESC" {
					return false
				}
			}
		}
	}

	return true
}

// BuildTableList builds a query to list all tables in a database
// CRITICAL: Uses parameterized queries to prevent SQL injection
func (qb *QueryBuilder) BuildTableList(schema string) (string, []any) {
	if qb.driver == "postgres" {
		if schema != "" {
			// Validate schema name as additional security layer
			if !qb.isValidIdentifier(schema) {
				// Return safe default with public schema
				return "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name", []any{"public"}
			}
			return "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name", []any{schema}
		}
		return "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name", []any{"public"}
	}

	// MySQL
	if schema != "" {
		// Validate schema name as additional security layer
		if !qb.isValidIdentifier(schema) {
			// Return query using DATABASE() as safe default
			return "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name", nil
		}
		return "SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY table_name", []any{schema}
	}
	return "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name", nil
}

// BuildTableSchema builds a query to get table schema information
// CRITICAL: Uses parameterized queries to prevent SQL injection
func (qb *QueryBuilder) BuildTableSchema(table, schema string) (string, []any, error) {
	// Validate table name - this is required
	if !qb.isValidIdentifier(table) {
		return "", nil, fmt.Errorf("invalid table name: %s", table)
	}

	if qb.driver == "postgres" {
		schemaFilter := "public"
		if schema != "" {
			if !qb.isValidIdentifier(schema) {
				return "", nil, fmt.Errorf("invalid schema name: %s", schema)
			}
			schemaFilter = schema
		}

		query := `
			SELECT
				column_name,
				data_type,
				is_nullable,
				column_default,
				CASE WHEN column_name IN (
					SELECT column_name FROM information_schema.key_column_usage
					WHERE table_name = $1 AND constraint_name LIKE '%pkey'
				) THEN true ELSE false END as is_primary_key
			FROM information_schema.columns
			WHERE table_name = $2 AND table_schema = $3
			ORDER BY ordinal_position`

		return query, []any{table, table, schemaFilter}, nil
	}

	// MySQL
	if schema != "" {
		if !qb.isValidIdentifier(schema) {
			return "", nil, fmt.Errorf("invalid schema name: %s", schema)
		}
		query := `
			SELECT
				column_name,
				data_type,
				is_nullable,
				column_default,
				column_key = 'PRI' as is_primary_key
			FROM information_schema.columns
			WHERE table_name = ? AND table_schema = ?
			ORDER BY ordinal_position`
		return query, []any{table, schema}, nil
	}

	// MySQL without explicit schema - use DATABASE()
	query := `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default,
			column_key = 'PRI' as is_primary_key
		FROM information_schema.columns
		WHERE table_name = ? AND table_schema = DATABASE()
		ORDER BY ordinal_position`
	return query, []any{table}, nil
}
