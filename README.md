# MCP LocalBridge

English | [中文](README_CN.md)

A production-ready MCP (Model Context Protocol) server implementation providing database querying, caching, and intelligent analytics capabilities.

## Overview

MCP LocalBridge is a high-performance MCP server built on [mcp-go](https://github.com/mark3labs/mcp-go), offering secure and efficient data access for LLM applications (such as Vibe Coding, Claude Desktop, etc.).

### Core Features

- 🔐 **Security First**: All database queries use parameterized queries to prevent SQL injection
- 🚀 **Multiple Transports**: Supports Stdio, HTTP, SSE, and InProcess transports
- 💾 **Multi-Database**: MySQL, PostgreSQL support with easy extensibility
- ⚡ **Redis Caching**: High-performance caching for improved query efficiency
- 🔍 **Intelligent Insights**: Database schema analysis, relationship graphs, semantic summaries
- 🐳 **Containerized**: Full Docker support with one-command deployment
- 🧪 **Dry-run Mode**: Preview SQL queries without execution for safety

## Quick Start

### Prerequisites

- Go 1.24+
- MySQL or PostgreSQL (running on host machine)
- Redis (optional, for caching)

**Important**: This project does NOT start database containers automatically. You must have MySQL/PostgreSQL/Redis running on your host machine.

### Local Development

1. **Clone the repository**

```bash
git clone https://github.com/SkillingX/mcp-localbridge.git
cd mcp-localbridge
```

2. **Install dependencies**

```bash
go mod download
```

3. **Configure database connection**

Edit `config/config.yaml` with your database credentials:

```yaml
databases:
  mysql:
    - name: "mysql_main"
      enabled: true
      host: "localhost"  # or host.docker.internal in Docker
      port: 3306
      user: "your_username"
      password: "your_password"
      database: "your_database"
```

4. **Build and run**

```bash
# Option 1: Using Makefile
make build
make run

# Option 2: Direct execution
go run cmd/server/main.go -config config/config.yaml

# Option 3: Using script
./scripts/start.sh
```

### Docker Deployment

1. **Configure environment variables**

Create `.env` file (optional):

```bash
DB_MYSQL_HOST=host.docker.internal
DB_MYSQL_PORT=3306
DB_MYSQL_USER=root
DB_MYSQL_PASSWORD=your_password
DB_MYSQL_DATABASE=your_database
```

2. **Start services**

```bash
# Build and start
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

**Linux Users**: The `docker-compose.yml` includes `extra_hosts` configuration to support `host.docker.internal`.

### Verify Installation

```bash
# Check HTTP transport health
curl http://localhost:8080/health

# View logs
docker-compose logs mcp-server
```

## Configuration

### Transport Configuration

Enable any combination of transports in `config/config.yaml`:

```yaml
transports:
  stdio:
    enabled: true  # Standard I/O (for Claude Desktop, etc.)

  http:
    enabled: true
    host: "0.0.0.0"
    port: 8080

  sse:
    enabled: true
    host: "0.0.0.0"
    port: 8081

  inprocess:
    enabled: false  # In-process calls (for testing)
```

### Database Configuration

Support multiple database instances:

```yaml
databases:
  mysql:
    - name: "mysql_main"
      enabled: true
      host: "${DB_MYSQL_HOST:-localhost}"
      # ... other configs

  postgres:
    - name: "postgres_main"
      enabled: false  # Disable unused databases
      # ...
```

### Security Configuration

```yaml
tools:
  db:
    default_dry_run: true  # Enable dry-run by default (recommended for production)
    max_rows: 1000         # Maximum rows to return
    query_timeout: 30      # Query timeout (seconds)
```

### Environment Variable Priority

Configuration priority: **Environment Variables > config.yaml**

Common environment variables:

- `DB_MYSQL_HOST`, `DB_MYSQL_PORT`, `DB_MYSQL_USER`, `DB_MYSQL_PASSWORD`
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`
- `LOG_LEVEL`: Log level (debug/info/warn/error)
- `TOOLS_DB_DRY_RUN`: Default dry-run mode

## MCP Tools

### Database Tools

#### `db_query`
Execute parameterized database queries with conditions, pagination, and sorting.

**Parameters**:
- `database` (required): Database instance name
- `table` (required): Table name
- `conditions` (optional): JSON WHERE conditions, e.g., `{"status":"active","age":25}`
- `limit`, `offset`, `order_by` (optional)
- `dry_run` (optional): Returns SQL preview without execution when `true`

**Example**:
```json
{
  "database": "mysql_main",
  "table": "users",
  "conditions": "{\"status\":\"active\"}",
  "limit": "10",
  "dry_run": "true"
}
```

#### `db_table_list`
List all tables in a database.

#### `db_table_preview`
Preview table data (default: first 10 rows).

### Redis Tools

#### `redis_get`, `redis_set`, `redis_scan`
Redis key-value operations and scanning.

### Insights Tools

#### `introspection`
Database schema introspection: tables, columns, indexes, foreign keys. Cached for performance.

#### `semantic_summary`
Generate semantic summaries of table data, returns LLM prompt template for MCP clients to use.

#### `relationship`
Analyze foreign key relationships between tables, generates relationship graph and LLM analysis prompt.

#### `analytics`
Execute aggregation queries (COUNT/SUM/AVG/MIN/MAX) with grouping and filtering.

#### `metadata`
Retrieve table and column metadata (comments, descriptions, etc.).

## Development

### Project Structure

```
mcp-localbridge/
├── cmd/
│   ├── server/          # MCP server entry point
│   └── client/          # MCP client entry point
├── config/              # Configuration management
├── server/              # MCP server core
├── transports/          # Transport layer implementations
├── db/                  # Database access layer
├── cache/               # Redis cache layer
├── tools/               # MCP tool implementations
├── insights/            # Intelligent analytics tools
├── tests/               # Unit tests
└── scripts/             # Helper scripts
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage

# View coverage
open coverage.html
```

### Code Quality

```bash
# Format code
make fmt

# Run go vet
make vet

# Run golangci-lint (requires installation)
make lint
```

### Connecting to Host Databases

#### macOS/Windows (Docker Desktop)

Use `host.docker.internal` directly:

```yaml
databases:
  mysql:
    - host: "host.docker.internal"
      port: 3306
```

#### Linux

Use `host.docker.internal` (configured in docker-compose.yml with `extra_hosts`) or host IP:

```yaml
databases:
  mysql:
    - host: "host.docker.internal"  # or "172.17.0.1", etc.
      port: 3306
```

Ensure your database listens on `0.0.0.0` instead of just `127.0.0.1`.

### Disabling Dry-run

To execute queries in production:

1. **Config file**: Edit `config/config.yaml`
   ```yaml
   tools:
     db:
       default_dry_run: false
   ```

2. **Environment variable**:
   ```bash
   export TOOLS_DB_DRY_RUN=false
   ```

3. **Per-tool invocation**:
   ```json
   {"dry_run": "false"}
   ```

## Security Best Practices

1. ✅ **All SQL queries use parameterized queries** to prevent SQL injection
2. ✅ **Dry-run mode enabled by default**, must be explicitly disabled
3. ✅ **Query timeout control** to prevent long-running queries
4. ✅ **Result row limits** to prevent excessive data returns
5. ✅ **Input validation** for table names, column names, aggregate functions, etc.
6. ⚠️ **Production auditing**: In production, audit and log all database operations

## FAQ

### Q: Why can't I connect to the database?

A: Please check:
1. Database is running on host machine
2. Firewall allows connections
3. Database listens on `0.0.0.0` (not just `127.0.0.1`)
4. Linux users: verify `extra_hosts` configuration

### Q: How do I see detailed logs?

A: Set log level to debug:
```yaml
logging:
  level: "debug"
```
Or use environment variable: `LOG_LEVEL=debug`

### Q: Which databases are supported?

A: Currently MySQL and PostgreSQL. Other databases can be easily added by implementing the `db.Repository` interface.

## Contributing

Issues and Pull Requests are welcome!

## License

MIT License

## Acknowledgments

- [mcp-go](https://github.com/mark3labs/mcp-go) - Model Context Protocol Go implementation
- [sqlx](https://github.com/jmoiron/sqlx) - Go database extension library
- [go-redis](https://github.com/go-redis/redis) - Redis Go client

---

**Note**: This project is a complete engineering practice example with comprehensive error handling, logging, testing, and documentation. Suitable for learning MCP protocol implementation and Go project engineering practices.
