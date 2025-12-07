package server

import (
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/SkillingX/mcp-localbridge/cache"
	"github.com/SkillingX/mcp-localbridge/config"
	"github.com/SkillingX/mcp-localbridge/db"
	"github.com/SkillingX/mcp-localbridge/insights"
	"github.com/SkillingX/mcp-localbridge/tools"
)

// MCPServer wraps the mcp-go server with our custom configuration
type MCPServer struct {
	server       *server.MCPServer
	config       *config.Config
	repositories map[string]db.Repository
	redisClients map[string]*cache.RedisClient
	logger       *slog.Logger
}

// NewMCPServer creates and initializes a new MCP server
func NewMCPServer(cfg *config.Config, logger *slog.Logger) (*MCPServer, error) {
	// Initialize repositories (databases)
	repositories, err := initializeRepositories(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repositories: %w", err)
	}

	// Initialize Redis clients
	redisClients, err := initializeRedisClients(cfg, logger)
	if err != nil {
		logger.Warn("Failed to initialize Redis clients", "error", err)
		// Redis is optional, continue without it
	}

	// Create MCP server instance
	var serverOpts []server.ServerOption
	// Note: WithRecovery might not be available in all versions of mcp-go
	// if cfg.Server.EnableRecovery {
	// 	serverOpts = append(serverOpts, server.WithRecovery())
	// }
	serverOpts = append(serverOpts, server.WithToolCapabilities(true))

	mcpServer := server.NewMCPServer(
		cfg.Server.Name,
		cfg.Server.Version,
		serverOpts...,
	)

	mcpSrv := &MCPServer{
		server:       mcpServer,
		config:       cfg,
		repositories: repositories,
		redisClients: redisClients,
		logger:       logger,
	}

	// Register all tools
	if err := mcpSrv.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return mcpSrv, nil
}

// registerTools registers all MCP tools
func (s *MCPServer) registerTools() error {
	s.logger.Info("Registering MCP tools")

	// Initialize tool handlers
	dbToolsHandler := tools.NewDBToolsHandler(s.repositories, s.config.Tools.DB, s.logger)
	redisToolsHandler := tools.NewRedisToolsHandler(s.redisClients, s.config.Tools.Redis, s.logger)

	// Insights handlers
	introspectionHandler := insights.NewIntrospectionHandler(s.repositories, s.redisClients, s.config.Tools.Insights.Introspection, s.logger)
	semanticSummaryHandler := insights.NewSemanticSummaryHandler(s.repositories, s.config.Tools.Insights.SemanticSummary, s.logger)
	relationshipHandler := insights.NewRelationshipHandler(s.repositories, s.redisClients, s.config.Tools.Insights.Relationship, s.logger)
	analyticsHandler := insights.NewAnalyticsHandler(s.repositories, s.config.Tools.Insights.Analytics, s.logger)
	metadataHandler := insights.NewMetadataHandler(s.repositories, s.logger)

	// Register database tools
	s.registerDBQueryTool(dbToolsHandler)
	s.registerDBTableListTool(dbToolsHandler)
	s.registerDBTablePreviewTool(dbToolsHandler)

	// Register Redis tools
	s.registerRedisGetTool(redisToolsHandler)
	s.registerRedisSetTool(redisToolsHandler)
	s.registerRedisScanTool(redisToolsHandler)

	// Register insights tools
	s.registerIntrospectionTool(introspectionHandler)
	s.registerSemanticSummaryTool(semanticSummaryHandler)
	s.registerRelationshipTool(relationshipHandler)
	s.registerAnalyticsTool(analyticsHandler)
	s.registerMetadataTool(metadataHandler)

	s.logger.Info("All MCP tools registered successfully")
	return nil
}

// Database Tools Registration

func (s *MCPServer) registerDBQueryTool(handler *tools.DBToolsHandler) {
	tool := mcp.NewTool("db_query",
		mcp.WithDescription("Execute a parameterized database query with conditions, limit, offset, and order_by. ALWAYS uses safe parameterized queries to prevent SQL injection. Supports dry-run mode to preview SQL without execution."),
		mcp.WithString("database",
			mcp.Required(),
			mcp.Description("Name of the database instance to query (e.g., 'mysql_main', 'postgres_main')")),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Name of the table to query")),
		mcp.WithString("conditions",
			mcp.Description("JSON object of WHERE conditions (e.g., '{\"status\":\"active\",\"age\":25}'). Supports equality and LIKE patterns.")),
		mcp.WithString("limit",
			mcp.Description(fmt.Sprintf("Maximum number of rows to return (max: %d)", s.config.Tools.DB.MaxRows))),
		mcp.WithString("offset",
			mcp.Description("Number of rows to skip")),
		mcp.WithString("order_by",
			mcp.Description("Column(s) to sort by (e.g., 'created_at DESC, id ASC')")),
		mcp.WithString("dry_run",
			mcp.Description(fmt.Sprintf("If 'true', return SQL preview without execution. Default: %v", s.config.Tools.DB.DefaultDryRun))),
	)
	s.server.AddTool(tool, handler.HandleDBQuery)
}

func (s *MCPServer) registerDBTableListTool(handler *tools.DBToolsHandler) {
	tool := mcp.NewTool("db_table_list",
		mcp.WithDescription("List all tables in a database"),
		mcp.WithString("database",
			mcp.Required(),
			mcp.Description("Name of the database instance")),
	)
	s.server.AddTool(tool, handler.HandleDBTableList)
}

func (s *MCPServer) registerDBTablePreviewTool(handler *tools.DBToolsHandler) {
	tool := mcp.NewTool("db_table_preview",
		mcp.WithDescription(fmt.Sprintf("Preview first %d rows of a table", s.config.Tools.DB.PreviewLimit)),
		mcp.WithString("database",
			mcp.Required(),
			mcp.Description("Name of the database instance")),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Name of the table to preview")),
	)
	s.server.AddTool(tool, handler.HandleDBTablePreview)
}

// Redis Tools Registration

func (s *MCPServer) registerRedisGetTool(handler *tools.RedisToolsHandler) {
	tool := mcp.NewTool("redis_get",
		mcp.WithDescription("Get a value from Redis by key"),
		mcp.WithString("redis",
			mcp.Required(),
			mcp.Description("Name of the Redis instance (e.g., 'redis_main')")),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("Redis key to retrieve")),
	)
	s.server.AddTool(tool, handler.HandleRedisGet)
}

func (s *MCPServer) registerRedisSetTool(handler *tools.RedisToolsHandler) {
	tool := mcp.NewTool("redis_set",
		mcp.WithDescription("Set a key-value pair in Redis with optional TTL"),
		mcp.WithString("redis",
			mcp.Required(),
			mcp.Description("Name of the Redis instance")),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("Redis key to set")),
		mcp.WithString("value",
			mcp.Required(),
			mcp.Description("Value to store")),
		mcp.WithString("ttl",
			mcp.Description("Time-to-live in seconds (optional)")),
	)
	s.server.AddTool(tool, handler.HandleRedisSet)
}

func (s *MCPServer) registerRedisScanTool(handler *tools.RedisToolsHandler) {
	tool := mcp.NewTool("redis_scan",
		mcp.WithDescription(fmt.Sprintf("Scan Redis keys matching a pattern (max %d keys)", s.config.Tools.Redis.MaxScanKeys)),
		mcp.WithString("redis",
			mcp.Required(),
			mcp.Description("Name of the Redis instance")),
		mcp.WithString("pattern",
			mcp.Description("Key pattern to match (e.g., 'user:*'). Default: '*'")),
	)
	s.server.AddTool(tool, handler.HandleRedisScan)
}

// Insights Tools Registration

func (s *MCPServer) registerIntrospectionTool(handler *insights.IntrospectionHandler) {
	tool := mcp.NewTool("introspection",
		mcp.WithDescription("Introspect database schema to get detailed information about all tables, columns, indexes, and relationships. Results are cached for better performance."),
		mcp.WithString("database",
			mcp.Required(),
			mcp.Description("Name of the database instance")),
		mcp.WithString("refresh",
			mcp.Description("Set to 'true' to refresh cache. Default: false")),
	)
	s.server.AddTool(tool, handler.HandleIntrospection)
}

func (s *MCPServer) registerSemanticSummaryTool(handler *insights.SemanticSummaryHandler) {
	tool := mcp.NewTool("semantic_summary",
		mcp.WithDescription("Generate a semantic summary of table data. Returns schema, sample data, and an LLM prompt template that MCP clients can use to generate business-meaningful insights."),
		mcp.WithString("database",
			mcp.Required(),
			mcp.Description("Name of the database instance")),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Name of the table to summarize")),
	)
	s.server.AddTool(tool, handler.HandleSemanticSummary)
}

func (s *MCPServer) registerRelationshipTool(handler *insights.RelationshipHandler) {
	tool := mcp.NewTool("relationship",
		mcp.WithDescription("Analyze foreign key relationships between tables. Returns a relationship graph and LLM prompt template for understanding the data model."),
		mcp.WithString("database",
			mcp.Required(),
			mcp.Description("Name of the database instance")),
		mcp.WithString("table",
			mcp.Description("Optional: specific table to analyze. If omitted, analyzes all tables.")),
	)
	s.server.AddTool(tool, handler.HandleRelationship)
}

func (s *MCPServer) registerAnalyticsTool(handler *insights.AnalyticsHandler) {
	tool := mcp.NewTool("analytics",
		mcp.WithDescription("Perform analytical aggregations (COUNT, SUM, AVG, MIN, MAX) on table data with optional grouping and filtering. Uses parameterized queries for security."),
		mcp.WithString("database",
			mcp.Required(),
			mcp.Description("Name of the database instance")),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Name of the table to analyze")),
		mcp.WithString("column",
			mcp.Required(),
			mcp.Description("Column to aggregate")),
		mcp.WithString("function",
			mcp.Required(),
			mcp.Description("Aggregate function: COUNT, SUM, AVG, MIN, or MAX")),
		mcp.WithString("conditions",
			mcp.Description("JSON object of WHERE conditions (optional)")),
		mcp.WithString("group_by",
			mcp.Description("Column to group by (optional)")),
	)
	s.server.AddTool(tool, handler.HandleAnalytics)
}

func (s *MCPServer) registerMetadataTool(handler *insights.MetadataHandler) {
	tool := mcp.NewTool("metadata",
		mcp.WithDescription("Retrieve database metadata including table and column comments/descriptions (if supported by the database)."),
		mcp.WithString("database",
			mcp.Required(),
			mcp.Description("Name of the database instance")),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Name of the table")),
	)
	s.server.AddTool(tool, handler.HandleMetadata)
}

// GetServer returns the underlying mcp-go server
func (s *MCPServer) GetServer() *server.MCPServer {
	return s.server
}

// Close closes all database and Redis connections
func (s *MCPServer) Close() error {
	s.logger.Info("Closing MCP server resources")

	// Close all repositories
	for name, repo := range s.repositories {
		if err := repo.Close(); err != nil {
			s.logger.Error("Failed to close repository", "name", name, "error", err)
		}
	}

	// Close all Redis clients
	for name, client := range s.redisClients {
		if err := client.Close(); err != nil {
			s.logger.Error("Failed to close Redis client", "name", name, "error", err)
		}
	}

	return nil
}

// initializeRepositories initializes all configured database repositories
func initializeRepositories(cfg *config.Config, logger *slog.Logger) (map[string]db.Repository, error) {
	repositories := make(map[string]db.Repository)

	// Initialize MySQL repositories
	for _, mysqlCfg := range cfg.Databases.MySQL {
		if !mysqlCfg.Enabled {
			continue
		}

		logger.Info("Initializing MySQL repository", "name", mysqlCfg.Name)
		repo, err := db.NewMySQLRepository(mysqlCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create MySQL repository %s: %w", mysqlCfg.Name, err)
		}

		repositories[mysqlCfg.Name] = repo
		logger.Info("MySQL repository initialized successfully", "name", mysqlCfg.Name)
	}

	// Initialize PostgreSQL repositories
	for _, pgCfg := range cfg.Databases.Postgres {
		if !pgCfg.Enabled {
			continue
		}

		logger.Info("Initializing PostgreSQL repository", "name", pgCfg.Name)
		repo, err := db.NewPostgresRepository(pgCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create PostgreSQL repository %s: %w", pgCfg.Name, err)
		}

		repositories[pgCfg.Name] = repo
		logger.Info("PostgreSQL repository initialized successfully", "name", pgCfg.Name)
	}

	if len(repositories) == 0 {
		return nil, fmt.Errorf("no databases configured or enabled")
	}

	return repositories, nil
}

// initializeRedisClients initializes all configured Redis clients
func initializeRedisClients(cfg *config.Config, logger *slog.Logger) (map[string]*cache.RedisClient, error) {
	clients := make(map[string]*cache.RedisClient)

	for _, redisCfg := range cfg.Redis.Instances {
		if !redisCfg.Enabled {
			continue
		}

		logger.Info("Initializing Redis client", "name", redisCfg.Name)
		client, err := cache.NewRedisClient(redisCfg)
		if err != nil {
			logger.Warn("Failed to create Redis client", "name", redisCfg.Name, "error", err)
			continue // Redis is optional, continue without it
		}

		clients[redisCfg.Name] = client
		logger.Info("Redis client initialized successfully", "name", redisCfg.Name)
	}

	return clients, nil
}
