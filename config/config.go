package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Logging    LoggingConfig    `yaml:"logging"`
	Transports TransportsConfig `yaml:"transports"`
	Databases  DatabasesConfig  `yaml:"databases"`
	Redis      RedisConfig      `yaml:"redis"`
	Tools      ToolsConfig      `yaml:"tools"`
}

// ServerConfig defines the core server settings
type ServerConfig struct {
	Name           string `yaml:"name"`
	Version        string `yaml:"version"`
	RequestTimeout int    `yaml:"request_timeout"` // seconds
	EnableRecovery bool   `yaml:"enable_recovery"`
}

// LoggingConfig defines logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
	Output string `yaml:"output"` // stdout, stderr, or file path
}

// TransportsConfig defines all transport protocol settings
type TransportsConfig struct {
	Stdio     StdioConfig     `yaml:"stdio"`
	HTTP      HTTPConfig      `yaml:"http"`
	SSE       SSEConfig       `yaml:"sse"`
	InProcess InProcessConfig `yaml:"inprocess"`
}

// StdioConfig for standard input/output transport
type StdioConfig struct {
	Enabled bool `yaml:"enabled"`
}

// HTTPConfig for HTTP/Streamable transport
type HTTPConfig struct {
	Enabled           bool   `yaml:"enabled"`
	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	EndpointPath      string `yaml:"endpoint_path"`
	HeartbeatInterval int    `yaml:"heartbeat_interval"` // seconds
	Stateless         bool   `yaml:"stateless"`
}

// Address returns the full address string (host:port)
func (h HTTPConfig) Address() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

// SSEConfig for Server-Sent Events transport
type SSEConfig struct {
	Enabled           bool   `yaml:"enabled"`
	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	BasePath          string `yaml:"base_path"`
	SSEEndpoint       string `yaml:"sse_endpoint"`
	MessageEndpoint   string `yaml:"message_endpoint"`
	KeepaliveInterval int    `yaml:"keepalive_interval"` // seconds
}

// Address returns the full address string (host:port)
func (s SSEConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// InProcessConfig for in-process transport
type InProcessConfig struct {
	Enabled bool `yaml:"enabled"`
}

// DatabasesConfig defines all database connections
type DatabasesConfig struct {
	MySQL    []MySQLConfig    `yaml:"mysql"`
	Postgres []PostgresConfig `yaml:"postgres"`
}

// MySQLConfig for MySQL database connection
type MySQLConfig struct {
	Name            string `yaml:"name"`
	Enabled         bool   `yaml:"enabled"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Database        string `yaml:"database"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"` // seconds
}

// DSN returns MySQL connection string
func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		m.User, m.Password, m.Host, m.Port, m.Database)
}

// PostgresConfig for PostgreSQL database connection
type PostgresConfig struct {
	Name            string `yaml:"name"`
	Enabled         bool   `yaml:"enabled"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Database        string `yaml:"database"`
	SSLMode         string `yaml:"sslmode"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"` // seconds
}

// DSN returns PostgreSQL connection string
func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.Database, p.SSLMode)
}

// RedisConfig defines Redis connections
type RedisConfig struct {
	Instances []RedisInstanceConfig `yaml:"instances"`
}

// RedisInstanceConfig for a single Redis instance
type RedisInstanceConfig struct {
	Name         string `yaml:"name"`
	Enabled      bool   `yaml:"enabled"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	PoolSize     int    `yaml:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns"`
	DialTimeout  int    `yaml:"dial_timeout"`  // seconds
	ReadTimeout  int    `yaml:"read_timeout"`  // seconds
	WriteTimeout int    `yaml:"write_timeout"` // seconds
}

// Address returns the full address string (host:port)
func (r RedisInstanceConfig) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// ToolsConfig defines MCP tools settings
type ToolsConfig struct {
	DB       DBToolsConfig       `yaml:"db"`
	Redis    RedisToolsConfig    `yaml:"redis"`
	Insights InsightsToolsConfig `yaml:"insights"`
}

// DBToolsConfig for database query tools
type DBToolsConfig struct {
	DefaultDryRun bool `yaml:"default_dry_run"`
	MaxRows       int  `yaml:"max_rows"`
	QueryTimeout  int  `yaml:"query_timeout"` // seconds
	EnablePreview bool `yaml:"enable_preview"`
	PreviewLimit  int  `yaml:"preview_limit"`
}

// RedisToolsConfig for Redis tools
type RedisToolsConfig struct {
	MaxScanKeys int `yaml:"max_scan_keys"`
	ScanCount   int `yaml:"scan_count"`
}

// InsightsToolsConfig for insights tools
type InsightsToolsConfig struct {
	Introspection   IntrospectionConfig   `yaml:"introspection"`
	SemanticSummary SemanticSummaryConfig `yaml:"semantic_summary"`
	Analytics       AnalyticsConfig       `yaml:"analytics"`
	Relationship    RelationshipConfig    `yaml:"relationship"`
}

// IntrospectionConfig for introspection tool
type IntrospectionConfig struct {
	CacheTTL      int  `yaml:"cache_ttl"` // seconds
	UseRedisCache bool `yaml:"use_redis_cache"`
}

// SemanticSummaryConfig for semantic summary tool
type SemanticSummaryConfig struct {
	SampleSize int `yaml:"sample_size"`
	MaxColumns int `yaml:"max_columns"`
}

// AnalyticsConfig for analytics tool
type AnalyticsConfig struct {
	MaxResultRows    int `yaml:"max_result_rows"`
	ExecutionTimeout int `yaml:"execution_timeout"` // seconds
}

// RelationshipConfig for relationship analysis tool
type RelationshipConfig struct {
	MaxDepth     int  `yaml:"max_depth"`
	CacheEnabled bool `yaml:"cache_enabled"`
	CacheTTL     int  `yaml:"cache_ttl"` // seconds
}

// Load reads and parses the configuration file
// Environment variables take precedence over config file values
func Load(configPath string) (*Config, error) {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the YAML content
	expanded := os.ExpandEnv(string(data))

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply additional environment variable overrides
	applyEnvOverrides(&cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration
func applyEnvOverrides(cfg *Config) {
	// Server overrides
	if v := os.Getenv("SERVER_NAME"); v != "" {
		cfg.Server.Name = v
	}
	if v := os.Getenv("SERVER_VERSION"); v != "" {
		cfg.Server.Version = v
	}
	if v := os.Getenv("REQUEST_TIMEOUT"); v != "" {
		if timeout, err := strconv.Atoi(v); err == nil {
			cfg.Server.RequestTimeout = timeout
		}
	}

	// Logging overrides
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}

	// Transport overrides
	if v := os.Getenv("TRANSPORT_STDIO_ENABLED"); v != "" {
		cfg.Transports.Stdio.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("TRANSPORT_HTTP_ENABLED"); v != "" {
		cfg.Transports.HTTP.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("TRANSPORT_HTTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Transports.HTTP.Port = port
		}
	}
	if v := os.Getenv("TRANSPORT_SSE_ENABLED"); v != "" {
		cfg.Transports.SSE.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("TRANSPORT_SSE_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Transports.SSE.Port = port
		}
	}

	// Tools overrides
	if v := os.Getenv("TOOLS_DB_DRY_RUN"); v != "" {
		cfg.Tools.DB.DefaultDryRun = strings.ToLower(v) == "true"
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate at least one transport is enabled
	if !c.Transports.Stdio.Enabled && !c.Transports.HTTP.Enabled &&
		!c.Transports.SSE.Enabled && !c.Transports.InProcess.Enabled {
		return fmt.Errorf("at least one transport must be enabled")
	}

	// Validate server settings
	if c.Server.RequestTimeout <= 0 {
		return fmt.Errorf("request_timeout must be positive")
	}

	// Validate logging level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(c.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	// Validate HTTP transport settings
	if c.Transports.HTTP.Enabled {
		if c.Transports.HTTP.Port <= 0 || c.Transports.HTTP.Port > 65535 {
			return fmt.Errorf("invalid HTTP port: %d", c.Transports.HTTP.Port)
		}
	}

	// Validate SSE transport settings
	if c.Transports.SSE.Enabled {
		if c.Transports.SSE.Port <= 0 || c.Transports.SSE.Port > 65535 {
			return fmt.Errorf("invalid SSE port: %d", c.Transports.SSE.Port)
		}
	}

	return nil
}

// GetRequestTimeout returns the request timeout as a duration
func (c *Config) GetRequestTimeout() time.Duration {
	return time.Duration(c.Server.RequestTimeout) * time.Second
}

// GetLogLevel returns the slog.Level for the configured log level
func (c *Config) GetLogLevel() slog.Level {
	switch strings.ToLower(c.Logging.Level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
