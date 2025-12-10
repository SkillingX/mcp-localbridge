# MCP LocalBridge

[English](README.md) | 中文

一个功能完整、生产就绪的 MCP (Model Context Protocol) 服务器实现，提供数据库查询、缓存访问和智能分析能力。

## 项目简介

MCP LocalBridge 是基于 [mcp-go](https://github.com/mark3labs/mcp-go) 构建的高性能 MCP 服务器，为 LLM 应用（如 Vibe Coding、Claude Desktop 等）提供安全、高效的数据访问能力。

### 核心特性

- 🔐 **安全第一**：所有数据库查询使用参数化查询，防止 SQL 注入
- 🚀 **多传输协议**：支持 Stdio、SSE（基于 HTTP 的流式传输）、InProcess 三种传输方式
- 💾 **多数据库支持**：MySQL、PostgreSQL，易于扩展
- ⚡ **Redis 缓存**：高性能缓存支持，提升查询效率
- 🔍 **智能洞察**：提供数据库结构分析、关系图谱、语义摘要等高级功能
- 🐳 **容器化部署**：完整的 Docker 支持，一键部署
- 🧪 **dry-run 模式**：预览 SQL 查询而不执行，安全可靠

## 快速开始

### 前置条件

- Go 1.24+
- MySQL 或 PostgreSQL（在宿主机运行）
- Redis（可选，用于缓存）

**重要**：本项目不会自动启动数据库容器。您需要在宿主机上预先运行 MySQL/PostgreSQL/Redis。

### 本地运行

1. **克隆项目**

```bash
git clone https://github.com/SkillingX/mcp-localbridge.git
cd mcp-localbridge
```

2. **下载依赖**

```bash
go mod download
```

3. **配置数据库连接**

编辑 `config/config.yaml`，配置您的数据库连接信息：

```yaml
databases:
  mysql:
    - name: "mysql_main"
      enabled: true
      host: "localhost"  # 或 host.docker.internal（Docker 中）
      port: 3306
      user: "your_username"
      password: "your_password"
      database: "your_database"
```

4. **构建并运行**

```bash
# 方式 1：使用 Makefile
make build
make run

# 方式 2：直接运行
go run cmd/server/main.go -config config/config.yaml

# 方式 3：使用脚本
./scripts/start.sh
```

### Docker 部署（推荐）

**快速开始 - 3 步搞定：**

```bash
# 1. 启动容器
make docker-run

# 2. 验证运行状态
docker ps | grep mcp-localbridge

# 3. 查看日志
docker compose logs -f
```

**常用命令：**

```bash
make docker-run         # 构建并启动容器
make docker-stop        # 停止容器
make docker-update      # 重新构建并重启（配置更改后使用）
```

**可选 - 环境变量配置：**

创建 `.env` 文件覆盖配置：

```bash
DB_MYSQL_HOST=host.docker.internal
DB_MYSQL_USER=root
DB_MYSQL_PASSWORD=your_password
# ... 其他变量
```

**Linux 用户注意**：`docker-compose.yml` 已配置 `extra_hosts` 以支持 `host.docker.internal`。

### 验证运行状态

```bash
# 检查容器状态
docker ps | grep mcp-localbridge

# 检查 SSE 端点（返回 404 是正常的）
curl http://localhost:28028/api/mcp/sse

# 查看服务日志
docker compose logs --tail=50
```

**预期结果：**
- MySQL、PostgreSQL、Redis 初始化成功
- 传输协议已启动：`["stdio","sse(0.0.0.0:28028)"]`
- 日志中无错误信息

## 配置说明

### 传输协议配置

在 `config/config.yaml` 中可以启用任意组合的传输协议：

```yaml
transports:
  # Stdio 传输 - 用于本地进程通信
  stdio:
    enabled: true  # 标准输入输出（用于 Claude Desktop、Cursor、VS Code）

  # SSE 传输 - 基于 HTTP 的流式传输（推荐用于 HTTP 客户端）
  # 这是 MCP 协议的主要 HTTP 传输方式
  sse:
    enabled: true
    host: "0.0.0.0"
    port: 28028
    base_path: "/api/mcp"
    # 端点：GET  /api/mcp/sse（流式连接）
    #      POST /api/mcp/message（消息发送）

  # HTTP 传输 - 未来 JSON-RPC over HTTP 的占位符
  # 注意：请使用 SSE 传输进行所有基于 HTTP 的 MCP 通信
  http:
    enabled: false  # mcp-go v0.11.0 中未实现
    port: 28027

  # InProcess 传输 - 用于测试和嵌入式场景
  inprocess:
    enabled: false
```

**重要说明**：**SSE（服务端发送事件）传输是 MCP 的标准 HTTP 协议**。它通过 HTTP 提供实时流式传输，是以下场景的推荐选择：
- Docker 部署
- 基于 Web 的客户端
- IDE 集成（Cursor、VS Code）
- 远程服务器访问

### 数据库连接配置

支持多个数据库实例：

```yaml
databases:
  mysql:
    - name: "mysql_main"
      enabled: true
      host: "${DB_MYSQL_HOST:-localhost}"
      # ... 其他配置

  postgres:
    - name: "postgres_main"
      enabled: false  # 可以禁用不需要的数据库
      # ...
```

### 安全配置

```yaml
tools:
  db:
    default_dry_run: true  # 默认启用 dry-run，生产环境推荐
    max_rows: 1000         # 最大返回行数
    query_timeout: 30      # 查询超时（秒）
```

### 环境变量优先级

配置优先级：**环境变量 > config.yaml**

常用环境变量：

- `DB_MYSQL_HOST`、`DB_MYSQL_PORT`、`DB_MYSQL_USER`、`DB_MYSQL_PASSWORD`
- `REDIS_HOST`、`REDIS_PORT`、`REDIS_PASSWORD`
- `LOG_LEVEL`：日志级别（debug/info/warn/error）
- `TOOLS_DB_DRY_RUN`：是否默认启用 dry-run

## MCP 工具说明

### 数据库工具

#### `db_query`
执行参数化数据库查询，支持条件、分页、排序。

**参数**：
- `database`（必需）：数据库实例名称
- `table`（必需）：表名
- `conditions`（可选）：JSON 格式的 WHERE 条件，如 `{"status":"active","age":25}`
- `limit`、`offset`、`order_by`（可选）
- `dry_run`（可选）：`true` 时只返回 SQL 预览，不执行

**示例**：
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
列出数据库中所有表。

#### `db_table_preview`
预览表数据（默认前 10 行）。

### Redis 工具

#### `redis_get`、`redis_set`、`redis_scan`
Redis 键值操作和扫描。

### Insights 工具

#### `introspection`
数据库结构内省，获取所有表、列、索引、外键信息。支持缓存。

#### `semantic_summary`
生成表数据的语义摘要，返回 LLM 提示词模板供 MCP 客户端使用。

#### `relationship`
分析表之间的外键关系，生成关系图谱和 LLM 分析提示词。

#### `analytics`
执行聚合查询（COUNT/SUM/AVG/MIN/MAX），支持分组和筛选。

#### `metadata`
检索表和列的元数据（注释、描述等）。

## 开发指南

### 项目结构

```
mcp-localbridge/
├── cmd/
│   ├── server/          # MCP 服务器入口
│   └── client/          # MCP 客户端入口
├── config/              # 配置管理
├── server/              # MCP 服务器核心
├── transports/          # 传输层实现
├── db/                  # 数据库访问层
├── cache/               # Redis 缓存层
├── tools/               # MCP 工具实现
├── insights/            # 智能分析工具
├── tests/               # 单元测试
└── scripts/             # 辅助脚本
```

### 运行测试

```bash
# 运行所有测试
make test

# 运行测试并生成覆盖率报告
make test-coverage

# 查看覆盖率
open coverage.html
```

### 代码检查

```bash
# 格式化代码
make fmt

# 运行 go vet
make vet

# 运行 golangci-lint（需要安装）
make lint
```

### 连接宿主机数据库

#### macOS/Windows（Docker Desktop）

直接使用 `host.docker.internal`：

```yaml
databases:
  mysql:
    - host: "host.docker.internal"
      port: 3306
```

#### Linux

使用 `host.docker.internal`（docker-compose.yml 已配置 `extra_hosts`）或宿主机 IP：

```yaml
databases:
  mysql:
    - host: "host.docker.internal"  # 或 "172.17.0.1" 等
      port: 3306
```

确保数据库监听 `0.0.0.0` 而非 `127.0.0.1`。

### 禁用 dry_run

在生产环境中，如果需要实际执行查询：

1. **配置文件方式**：编辑 `config/config.yaml`
   ```yaml
   tools:
     db:
       default_dry_run: false
   ```

2. **环境变量方式**：
   ```bash
   export TOOLS_DB_DRY_RUN=false
   ```

3. **工具调用时指定**：
   ```json
   {"dry_run": "false"}
   ```

## IDE 集成配置

### Claude Desktop / Claude App 配置

Claude Desktop 可以使用 Stdio 传输方式连接 MCP 服务器。

1. **获取服务器路径**
   ```bash
   which mcp-server
   # 或如果使用 docker
   pwd  # 获取项目目录
   ```

2. **编辑 Claude Desktop 配置文件**
   - **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
   - **Linux**: `~/.config/Claude/claude_desktop_config.json`

3. **添加 MCP LocalBridge 配置**
   ```json
   {
     "mcpServers": {
       "mcp-localbridge": {
         "command": "/path/to/mcp-server",
         "args": ["-config", "/path/to/config/config.yaml"],
         "env": {
           "LOG_LEVEL": "info",
           "DB_MYSQL_HOST": "localhost",
           "DB_MYSQL_USER": "root",
           "DB_MYSQL_PASSWORD": "your_password"
         }
       }
     }
   }
   ```

4. **在 Claude Desktop 中验证**
   - 重启 Claude Desktop
   - MCP 服务器会在您与其交互时自动启动
   - 检查服务器日志：`docker compose logs -f`（如果使用 Docker）

### Cursor IDE 配置

Cursor 支持通过 SSE（基于 HTTP）和 Stdio 两种方式连接 MCP 服务器。

**快速配置：**

在 Cursor 的 MCP 设置中添加以下配置：

**SSE 传输方式（推荐）：**
```json
{
  "mcpServers": {
    "mcp-localbridge": {
      "type": "sse",
      "url": "http://localhost:28028/api/mcp/sse",
      "timeout": 30000
    }
  }
}
```

**Stdio 传输方式：**
```json
{
  "mcpServers": {
    "mcp-localbridge": {
      "command": "/path/to/mcp-server",
      "args": ["-config", "/path/to/config/config.yaml"]
    }
  }
}
```

#### 方式一：使用 SSE 传输（推荐，适用于 Docker 部署）

1. **启动 MCP 服务器**（确保 SSE 传输已启用）
   ```bash
   # 使用 Docker（推荐）
   make docker-run

   # 或本地运行
   make run-server
   ```

   确保在 `config/config.yaml` 中启用了 SSE：
   ```yaml
   transports:
     sse:
       enabled: true
       host: "0.0.0.0"
       port: 28028
       base_path: "/api/mcp"
   ```

2. **打开 Cursor 设置** → **MCP Servers** → **Add Server**

3. **添加 SSE 配置**（见上方快速配置）

4. **验证连接**
   - 服务器应在 Cursor 的 MCP 面板中显示为"已连接"
   - 可使用 `db_table_list`、`db_table_preview` 等工具测试

#### 方式二：使用 Stdio 传输（本地开发）

适用于本地开发，需要本地二进制文件。添加 stdio 配置（见上方快速配置）。

**注意**：Stdio 需要本地二进制文件。Docker 部署请使用 SSE 传输。

### Claude Code (VS Code 扩展) 配置

Claude Code 可以通过 Stdio 或 SSE 传输方式使用 MCP 服务器。

#### 方式一：使用 Stdio 传输（本地开发）

1. **在 VS Code 中安装 Claude Code 扩展**

2. **在项目根目录创建或编辑 `.claude/mcp-config.json`**
   ```json
   {
     "servers": [
       {
         "name": "mcp-localbridge",
         "command": "/path/to/bin/mcp-server",
         "args": ["-config", "/path/to/config/config.yaml"],
         "env": {
           "LOG_LEVEL": "info",
           "DB_MYSQL_HOST": "localhost"
         },
         "transport": "stdio"
       }
     ]
   }
   ```

#### 方式二：使用 SSE 传输（Docker 部署）

对于基于 Docker 的部署，使用 SSE 传输：

```json
{
  "servers": [
    {
      "name": "mcp-localbridge",
      "type": "sse",
      "url": "http://localhost:28028/api/mcp/sse",
      "timeout": 30000
    }
  ]
}
```

**在 Claude Code 中验证**：
- 在 Claude Code 中打开 MCP 面板
- 服务器应被列出并已连接
- 测试可用工具：`db_query`、`redis_get`、`introspection`

### 快速 Docker 设置（IDE 集成）

快速使用 Docker 设置：

```bash
# 构建并运行服务器
make docker-run

# 配置更改后更新服务器
make docker-update

# 查看日志
docker compose logs -f

# 停止服务器
make docker-stop
```

然后配置您的 IDE 连接到：
- **SSE（推荐）**：`http://localhost:28028/api/mcp/sse`
  - MCP 的主要基于 HTTP 的传输方式
  - 支持实时流式传输
  - 适用于 Docker 部署

- **Stdio（仅本地）**：直接进程通信
  - 命令：`/path/to/bin/mcp-server -config /path/to/config/config.yaml`
  - 最适合本地开发
  - 需要本地二进制文件

### IDE 连接故障排除

1. **连接被拒绝**
   - 验证服务器是否运行：`docker ps` 或 `curl http://localhost:28027/health`
   - 检查防火墙规则
   - 确保端口号正确

2. **工具未显示**
   - 检查服务器日志：`docker compose logs mcp-server`
   - 验证数据库连接：`docker compose logs | grep -i "error"`
   - 确保 config.yaml 是有效的 YAML

3. **IDE 中的数据库连接错误**
   - 验证数据库是否在宿主机上运行
   - 检查 `config/config.yaml` 中的配置凭据
   - 对于 Docker：确保使用 `host.docker.internal`（macOS/Windows）或正确的宿主机 IP（Linux）

4. **日志和调试**
   ```bash
   # 查看实时日志
   docker compose logs -f

   # 增加日志级别以进行调试
   LOG_LEVEL=debug make docker-run

   # 检查服务器健康状态
   curl -v http://localhost:28027/health
   ```

## 安全最佳实践

1. ✅ **所有 SQL 查询都使用参数化查询**，防止 SQL 注入
2. ✅ **默认启用 dry-run 模式**，需要明确禁用才能执行
3. ✅ **查询超时控制**，防止长时间运行的查询
4. ✅ **结果行数限制**，防止返回过大数据集
5. ✅ **输入验证**，包括表名、列名、聚合函数等
6. ⚠️ **生产环境审计**：在生产环境中，建议对所有数据库操作进行审计和日志记录

## 常见问题

### Q: 为什么连接数据库失败？

A: 请检查：
1. 数据库是否在宿主机上运行
2. 防火墙是否允许连接
3. 数据库是否监听 `0.0.0.0`（而非仅 `127.0.0.1`）
4. Linux 用户确认 `extra_hosts` 配置正确

### Q: 如何查看详细日志？

A: 设置日志级别为 debug：
```yaml
logging:
  level: "debug"
```
或使用环境变量：`LOG_LEVEL=debug`

### Q: 支持哪些数据库？

A: 当前支持 MySQL 和 PostgreSQL。其他数据库可以通过实现 `db.Repository` 接口轻松添加。

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License

## 致谢

- [mcp-go](https://github.com/mark3labs/mcp-go) - Model Context Protocol Go 实现
- [sqlx](https://github.com/jmoiron/sqlx) - Go 数据库扩展库
- [go-redis](https://github.com/go-redis/redis) - Redis Go 客户端

---

**注意**：本项目为完整的工程实践示例，包含完善的错误处理、日志记录、测试和文档。适合用于学习 MCP 协议实现和 Go 项目工程化。
