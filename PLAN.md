# Codex Go — 实施计划

> 基于 SPEC.md 约束，从当前 JSON/YAML 架构迁移到 SQLite 统一持久化架构。

---

## 背景

当前代码仓库是「JSON 文件 + YAML 配置」架构，SPEC 定义的是「SQLite 统一持久化」架构。两个架构在数据层完全不兼容。本计划从零引入 SQLite，逐步替换所有 JSON/YAML 存储，同时补全 SPEC 要求的缺失功能。

**关键原则：**
- 每个 Phase 结束时 `make build + go vet` 通过
- 旧格式兼容：已有 JSON/YAML 数据可迁移到 SQLite
- 新代码不引入循环依赖

---

## Phase 0: SQLite 数据层

> 目标：引入 SQLite，创建所有表，实现 Store 接口，做好旧数据迁移。本 Phase 完成后系统可以用 SQLite 启动并读写。

### 0.1 添加依赖 + 数据库初始化

**文件：** `internal/store/sqlite.go`（新建）

- 引入 `mattn/go-sqlite3`
- `func InitDB(path string) (*sql.DB, error)` — 打开/创建 DB，设置 WAL + 外键 + 单连接
- `func runMigrations(db *sql.DB) error` — 从 `schema_version` 表读版本，按序执行 migration
- `func seedDefaults(db *sql.DB) error` — 插入 12 个系统工具 + 50+ 预设 + default agent（仅当表为空）

**文件：** `go.mod`（修改）— 添加 `github.com/mattn/go-sqlite3`

**文件：** `internal/store/migrations/001_initial.sql`（新建）— 17 张表的完整 DDL

**校验：** 启动后 `~/.codex/codex.db` 自动创建，`sqlite3 codex.db ".tables"` 显示所有表

### 0.2 Store 接口 + Provider 实现

**文件：** `internal/store/store.go`（新建）

- 定义 `Store` 结构体，包装 `*sql.DB`
- Provider CRUD 方法（全部用 SQL，替换现有 JSON 读写）：
  - `ListProviders()` — JOIN backends 聚合 backend_count/healthy_count
  - `GetProvider(id)` — 单行查询
  - `CreateProvider(p)` — 事务：INSERT providers + INSERT backends + INSERT backend_models
  - `UpdateProvider(p)` — UPDATE providers
  - `DeleteProvider(id)` — DELETE providers（CASCADE）
  - `SwitchProvider(id)` — 事务：UPDATE SET is_current=0; UPDATE SET is_current=1 WHERE id=?
  - `ListPresets()` / `CreateFromPreset(name, apiKey)`

**文件：** `internal/store/provider_store.go`（重写）— 改为调用 Store 的 SQL 方法，删除 JSON 读写

**校验：** `POST /api/providers` → SQLite 中可见新行；`POST /switch` 原子切换 is_current

### 0.3 Backend 实现

**文件：** `internal/store/store.go`（追加方法）

- `ListBackends(providerID)` — JOIN backend_models 聚合模型列表
- `CreateBackend(b)` — INSERT backends + INSERT backend_models
- `UpdateBackend(b)` / `DeleteBackend(id)` / `ProbeBackends(providerID)` — 探测并更新 health_status

**文件：** `internal/api/handler_backends.go`（修改）— 调用 Store 方法替换现有逻辑

**校验：** 创建 Backend 后在 SQLite 中可见关联的 backend_models 行

### 0.4 Session 迁移

**文件：** `internal/store/store.go`（追加方法）

- `CreateSession(s)` / `GetSession(id)` / `ListSessions(agentName, limit)` / `DeleteSession(id)`
- `AddMessage(m)` — INSERT + UPDATE sessions.msg_count
- `GetMessages(sessionID)` — SELECT 按 id ASC

**文件：** `internal/session/store.go`（重写）— 改为调用 Store，删除 JSON 文件读写。Session 结构体增加 `AgentName` 字段。

**文件：** `internal/store/migrations/002_legacy_sessions.go`（新建）— 扫描 `~/.codex/sessions/*.json`，逐文件导入 sessions + messages 表

**校验：** 旧 JSON 会话可正常读取；新会话写入 SQLite

### 0.5 Agent 表实现

**文件：** `internal/store/store.go`（追加方法）

- `ListAgents()` / `GetAgent(name)` / `CreateAgent(a)` / `UpdateAgent(a)` / `DeleteAgent(name)`
- `CopyAgent(source, target)` — 事务：INSERT agents SELECT + 返回成功
- `GetMemory(agent, key)` / `SetMemory(agent, key, value)` / `ListMemory(agent)` — agent_memory 表 CRUD

**文件：** `internal/agent/profile.go`（修改）— 增加从 Store 加载/保存 Agent 配置的方法。保留 YAML 读取作为导入路径。

**校验：** Agent 配置写入 agents 表；Copy 创建新行，不影响源行；memory 读写隔离

### 0.6 Tools / MCP 表实现 + Agent 装配

**文件：** `internal/store/store.go`（追加方法）

- `ListTools(category)` — SELECT from tools 表
- `UpdateTool(name, enabled, approval)` 
- `ListMCPServers()` / `CreateMCPServer()` / `UpdateMCPServer()` / `DeleteMCPServer()`

**文件：** `internal/agent/agent_store.go`（新建）

- `func AssembleTools(store, agent) ([]Tool, error)` — 根据 tools_mode 从 tools 表加载
- `func AssembleMCP(store, agent) ([]MCPServer, error)` — 根据 mcp_mode
- `func AssembleSkills(store, agent) ([]Skill, error)` — 根据 skills_mode
- `func LoadSoul(agentName) (string, error)` — 读取 `~/.codex/agents/<name>/soul.md`

**校验：** Agent 的 tools_mode=custom 时只加载 tools_list 中的工具；system 工具始终加载

### 0.7 旧数据迁移 + --db 参数

**文件：** `cmd/codex/main.go`（修改）

- 新增 `--db` flag，默认 `~/.codex/codex.db`
- 启动时调用 `store.InitDB(*dbPath)`
- 自动检测旧格式（`~/.codex/config.yaml` 有 provider.backends 但 providers 表为空）→ 运行迁移

**文件：** `internal/store/migrations/002_legacy.go`（新建）— 从 config.yaml 迁移 Provider/Backend 数据到 SQLite

**校验：** `--db /tmp/test.db` 使用指定路径；旧 config.yaml 数据自动出现在 SQLite

### 0.8 集成 + 全量回归

- `internal/api/server.go` — 注入 Store 替代原有的 ProviderStore/Config
- `internal/provider/pool.go` — 从 Store 读取 Backend 列表和健康状态
- 删除不再需要的 JSON 读写代码（`provider_store.go` 中的 JSON 部分、`session/store.go` 中的 JSON 部分）
- `make build + go vet` 零错误
- 完整启动流程可用（`--serve` 模式 + CLI 交互模式）

---

## Phase 1: Agent 系统重构

> 目标：Agent 文件隔离 + soul.md + Copy（SPEC 版）+ 记忆系统。

### 1.1 Agent 文件系统

**文件：** `internal/agent/profile.go`（修改）

- `CreateAgentDir(name)` — 创建 `~/.codex/agents/<name>/` + soul.md（空模板）+ rules/ + skills/
- 首次创建 Agent 时自动生成目录
- 从旧 YAML Agent 迁移：YAML → agents 表 + 文件目录

**校验：** 新建 Agent 后检查 `~/.codex/agents/<name>/` 下存在 soul.md、rules/、skills/

### 1.2 soul.md 注入

**文件：** `internal/agent/agent.go`（修改）

- `buildSystemPrompt()` 方法 — 读取 soul.md → 拼接格式：
  ```
  [Agent Soul]
  <soul.md 内容>
  
  [System]
  <system_prompt>
  ```

**校验：** 对话中 Agent 行为反映 soul.md 定义的人格

### 1.3 Agent Copy（SPEC 版）

**文件：** `internal/api/handler_agents.go`（修改）

- `POST /api/agents/copy` — 调用 Store.CopyAgent + 复制文件目录（soul.md, rules/, skills/）
- 不复制 memory 和 sessions（由 Store 层保证）

**文件：** `cmd/codex/main.go`（修改）

- 新增 `agent copy <source> <target>` 子命令

**校验：** Copy 后新 Agent 有独立的 soul.md/rules/skills，无旧 memory/sessions

### 1.4 Agent 删除级联

**文件：** `internal/api/handler_agents.go`（修改）

- `DELETE /api/agents/:name` — Store.DeleteAgent（CASCADE sessions/messages/agent_memory）+ os.RemoveAll 目录

**校验：** 删除后 `~/.codex/agents/<name>/` 不存在，DB 中关联行被级联删除

### 1.5 前端 AgentsPage

**文件：** `web/src/pages/AgentsPage.tsx`（新建）

- Agent 卡片网格：展示 display_name / provider / model / session_count
- Agent 详情面板：配置表单 + soul.md 编辑器（TextArea）+ memory 列表
- Agent Copy 按钮 → 弹窗输入名称 → 成功刷新列表
- Agent 删除（确认弹窗）

**文件：** `web/src/components/AgentCard.tsx`（新建）

**文件：** `web/src/App.tsx`（修改）— 添加 `/agents` 路由

**文件：** `web/src/components/LeftSidebar.tsx`（修改）— 导航增加 Agents

---

## Phase 2: Provider 代理增强

> 目标：CircuitBreaker + 故障转移 + 协议适配 + 用量统计。

### 2.1 CircuitBreaker

**文件：** `internal/provider/circuit_breaker.go`（新建）

- `CircuitBreaker` 结构体：state（Closed/Open/HalfOpen）+ failCount + lastFailTime + cooldown
- `Allow() bool` — Closed: true; Open: 检查冷却时间→HalfOpen; HalfOpen: true（仅一次）
- `RecordSuccess()` / `RecordFailure()` — 状态转换
- 每个 Backend 绑定一个 CircuitBreaker

**文件：** `internal/provider/pool.go`（修改）— 在选择 Backend 前检查 CircuitBreaker；调用后 RecordSuccess/RecordFailure

**校验：** 模拟连续 5 次失败 → Backend 进入 Open → 30 秒后 HalfOpen → 探测成功恢复

### 2.2 ProviderRouter + 故障转移

**文件：** `internal/provider/router.go`（新建）

- `ProviderRouter` — 持有 Store 引用，管理多 Provider 切换
- `SelectProvider() (*Provider, error)` — 从 is_current 开始，全 unhealthy 时遍历 in_failover_queue 列表
- 切换时记录日志

**文件：** `internal/provider/pool.go`（修改）— SelectBackend 失败时通过 Router 尝试下一个 Provider

**校验：** 当前 Provider 全部 backend unhealthy → 自动切到下一个 Provider

### 2.3 协议适配

**文件：** `internal/provider/protocol_adapter.go`（新建）

- `ConvertChatToResponses(req)` — Chat Completions 请求体 → Responses 请求体
- `ConvertResponsesToChat(resp)` — Responses 响应 → Chat Completions 格式
- `NormalizeSSE(event, fromProtocol)` — 归一化 SSE 事件
- 模型名映射表（alias → real）

**文件：** `internal/provider/openai.go`（修改）— 根据 Backend 的 api_format 决定是否需要协议转换

**校验：** 配置 `api_format=openai_responses` 的 Backend 走 Responses API 但对外呈现 Chat Completions 行为

### 2.4 用量统计

**文件：** `internal/store/store.go`（追加方法）

- `LogUsage(u)` — INSERT INTO usage_logs + upsert usage_daily
- `UsageDaily(providerID, from, to)` — 聚合查询

**文件：** `internal/provider/openai.go`（修改）— 每次 API 调用完成后调用 Store.LogUsage

**文件：** `internal/api/handler_usage.go`（新建）— `GET /api/usage` + `GET /api/usage/summary`

**校验：** 几次对话后 usage_daily 有聚合数据，API 返回正确

### 2.5 前端 ProvidersPage + UsagePage

**文件：** `web/src/pages/ProvidersPage.tsx`（新建）

- ProviderCard 网格：图标 + 名称 + 分类标签 + 健康摘要
- 预设选择器（分类筛选 + 一键添加）
- Provider 详情：Meta 编辑 + Backend 管理表

**文件：** `web/src/pages/UsagePage.tsx`（新建）

- UsageDashboard：日期范围选择 + Provider 筛选 + 柱状图/折线图

**文件：** `web/src/components/ProviderCard.tsx`（新建）
**文件：** `web/src/components/HealthBadge.tsx`（新建）

**文件：** `web/src/App.tsx`（修改）— 添加 `/providers`、`/usage` 路由
**文件：** `web/src/components/LeftSidebar.tsx`（修改）— 导航增加 Providers、Usage

---

## Phase 3: 高级功能

### 3.1 Skills 表索引

**文件：** `internal/store/store.go`（追加方法）

- `IndexSkills()` — 扫描 `~/.codex/skills/`、`~/.claude/skills/`、`~/.agents/skills/` → 解析 SKILL.md frontmatter → upsert skills 表
- `SearchSkills(query)` — 全文检索

**文件：** `internal/skill/registry.go`（修改）— 从 Store 读取技能索引，源文件仍从磁盘读取

### 3.2 前端 ToolsPage

**文件：** `web/src/pages/ToolsPage.tsx`（新建）

- 系统工具列表（只读展示）+ 可选工具开关
- MCP 服务器管理（添加/删除/启停）

**文件：** `web/src/App.tsx`（修改）— 添加 `/tools` 路由
**文件：** `web/src/components/LeftSidebar.tsx`（修改）— 导航增加 Tools

### 3.3 独立代理进程

**文件：** `cmd/codex/main.go`（修改）

- `--proxy` flag：启动独立 HTTP Proxy Server
- 从 Store 加载 Provider，对外暴露统一 API 端点
- Live Config：Provider 切换时通过 API 实时生效

---

## 附录: 文件变更清单

### 新建文件

```
internal/store/sqlite.go
internal/store/store.go
internal/store/migrations/001_initial.sql
internal/store/migrations/002_legacy.go
internal/agent/agent_store.go
internal/provider/circuit_breaker.go
internal/provider/router.go
internal/provider/protocol_adapter.go
internal/api/handler_tools.go
internal/api/handler_usage.go
internal/api/handler_system.go
web/src/pages/AgentsPage.tsx
web/src/pages/ProvidersPage.tsx
web/src/pages/ToolsPage.tsx
web/src/pages/UsagePage.tsx
web/src/components/AgentCard.tsx
web/src/components/ProviderCard.tsx
web/src/components/HealthBadge.tsx
```

### 修改文件

```
go.mod                                # + go-sqlite3
cmd/codex/main.go                     # + --db, + --proxy, + agent copy 子命令
internal/store/provider_store.go      # JSON → SQLite
internal/session/store.go             # JSON → SQLite
internal/agent/profile.go             # + Store 加载, + soul.md
internal/agent/agent.go               # + soul.md 注入
internal/agent/manager.go             # + Store 注入
internal/api/server.go                # + Store 注入, + 新路由
internal/api/handler_providers.go     # 调用 Store
internal/api/handler_backends.go      # 调用 Store
internal/api/handler_agents.go        # + copy API, + delete 级联
internal/provider/pool.go             # + CircuitBreaker, + Router
internal/provider/openai.go           # + ProtocolAdapter, + usage 记录
internal/skill/registry.go            # + Store 索引
web/src/App.tsx                       # + 新路由
web/src/components/LeftSidebar.tsx    # + 新导航项
```
