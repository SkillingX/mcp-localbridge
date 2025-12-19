package insights

import (
	"github.com/SkillingX/mcp-localbridge/db"
)

// formatDatabaseNotFoundError is a package-level wrapper for db.FormatDatabaseNotFoundError
// This provides a convenient local alias while using the shared implementation
func formatDatabaseNotFoundError(dbName string, repositories map[string]db.Repository) string {
	return db.FormatDatabaseNotFoundError(dbName, repositories)
}
