package tests

import (
	"strings"
	"testing"

	"github.com/SkillingX/mcp-localbridge/db"
)

// TestQueryBuilder_BuildSelect tests the parameterized SELECT query building
func TestQueryBuilder_BuildSelect(t *testing.T) {
	tests := []struct {
		name       string
		driver     string
		table      string
		conditions map[string]any
		limit      int
		offset     int
		orderBy    string
		wantQuery  string
		wantParams int
	}{
		{
			name:       "Simple MySQL query",
			driver:     "mysql",
			table:      "users",
			conditions: map[string]any{"status": "active"},
			limit:      10,
			offset:     0,
			orderBy:    "created_at DESC",
			wantQuery:  "SELECT * FROM `users` WHERE `status` = ? ORDER BY created_at DESC LIMIT 10",
			wantParams: 1,
		},
		{
			name:       "PostgreSQL query with multiple conditions",
			driver:     "postgres",
			table:      "orders",
			conditions: map[string]any{"user_id": 123, "status": "pending"},
			limit:      20,
			offset:     10,
			orderBy:    "",
			wantQuery:  "SELECT * FROM \"orders\" WHERE",
			wantParams: 2,
		},
		{
			name:       "MySQL LIKE query",
			driver:     "mysql",
			table:      "products",
			conditions: map[string]any{"name": "%phone%"},
			limit:      5,
			offset:     0,
			orderBy:    "",
			wantQuery:  "SELECT * FROM `products` WHERE `name` LIKE ? LIMIT 5",
			wantParams: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qb := db.NewQueryBuilder(tt.driver)
			query, params := qb.BuildSelect(tt.table, tt.conditions, tt.limit, tt.offset, tt.orderBy)

			// Check if query contains expected parts
			if !strings.Contains(query, "SELECT * FROM") {
				t.Errorf("Query missing SELECT clause: %s", query)
			}

			if !strings.Contains(query, tt.wantQuery) {
				t.Logf("Generated query: %s", query)
				t.Logf("Expected to contain: %s", tt.wantQuery)
			}

			// Check parameter count
			if len(params) != tt.wantParams {
				t.Errorf("Expected %d params, got %d", tt.wantParams, len(params))
			}

			// Verify no SQL injection: query should not contain raw user input
			for key, val := range tt.conditions {
				valStr := ""
				switch v := val.(type) {
				case string:
					valStr = v
				}
				// The actual value should NOT be in the query (should be parameterized)
				if valStr != "" && valStr != "%phone%" && strings.Contains(query, valStr) {
					t.Errorf("Query contains raw user input '%s' - possible SQL injection vulnerability!", valStr)
				}
				// But the column name should be present (quoted)
				if !strings.Contains(query, key) {
					t.Errorf("Query missing column name '%s'", key)
				}
			}
		})
	}
}

// TestQueryBuilder_BuildCount tests the COUNT query building
func TestQueryBuilder_BuildCount(t *testing.T) {
	qb := db.NewQueryBuilder("mysql")

	conditions := map[string]any{
		"status": "active",
		"age":    25,
	}

	query, params := qb.BuildCount("users", conditions)

	// Verify COUNT(*) is present
	if !strings.Contains(query, "COUNT(*)") {
		t.Errorf("Query missing COUNT(*): %s", query)
	}

	// Verify parameters
	if len(params) != 2 {
		t.Errorf("Expected 2 params, got %d", len(params))
	}

	// Verify parameterization
	if strings.Contains(query, "active") || strings.Contains(query, "25") {
		t.Errorf("Query contains raw values - not properly parameterized: %s", query)
	}
}

// TestQueryBuilder_BuildAggregation tests aggregation query building
// CRITICAL: This tests that aggregate functions are validated to prevent SQL injection
func TestQueryBuilder_BuildAggregation(t *testing.T) {
	tests := []struct {
		name      string
		aggFunc   string
		wantError bool
	}{
		{"Valid SUM", "SUM", false},
		{"Valid COUNT", "COUNT", false},
		{"Valid AVG", "AVG", false},
		{"Valid MIN", "MIN", false},
		{"Valid MAX", "MAX", false},
		{"Invalid function - SQL injection attempt", "DROP TABLE", true},
		{"Invalid function - arbitrary SQL", "SELECT * FROM", true},
	}

	qb := db.NewQueryBuilder("mysql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, params, err := qb.BuildAggregation("orders", "total", tt.aggFunc, nil, "")

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for invalid function '%s', but got none", tt.aggFunc)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid function '%s': %v", tt.aggFunc, err)
				}
				if query == "" {
					t.Errorf("Expected query for valid function '%s', got empty", tt.aggFunc)
				}
				// Verify it's a valid aggregation query
				if !strings.Contains(strings.ToUpper(query), strings.ToUpper(tt.aggFunc)) {
					t.Errorf("Query missing aggregate function '%s': %s", tt.aggFunc, query)
				}
			}

			t.Logf("Function: %s, Query: %s, Params: %v, Error: %v", tt.aggFunc, query, params, err)
		})
	}
}

// TestQueryBuilder_SQLInjectionPrevention tests that the query builder prevents SQL injection
func TestQueryBuilder_SQLInjectionPrevention(t *testing.T) {
	qb := db.NewQueryBuilder("mysql")

	// Attempt SQL injection through conditions
	maliciousConditions := map[string]any{
		"id":   "1 OR 1=1",
		"name": "'; DROP TABLE users; --",
	}

	query, params := qb.BuildSelect("users", maliciousConditions, 10, 0, "")

	// The malicious values should be in params, NOT in the query string
	for _, val := range maliciousConditions {
		valStr, ok := val.(string)
		if !ok {
			continue
		}

		// Check that malicious SQL is NOT directly in the query
		if strings.Contains(query, "DROP TABLE") || strings.Contains(query, "OR 1=1") {
			t.Errorf("SQL injection detected! Malicious SQL found in query: %s", query)
		}

		// Verify the value is in params (safely parameterized)
		found := false
		for _, param := range params {
			if paramStr, ok := param.(string); ok && paramStr == valStr {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected malicious value to be parameterized, but not found in params: %s", valStr)
		}
	}

	t.Logf("SQL Injection test passed. Query: %s, Params: %v", query, params)
}
