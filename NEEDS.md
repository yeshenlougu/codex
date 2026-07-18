# Codex Go — 项目需求文档

## 项目定位

Codex Go 是基于 OpenAI Codex CLI 用 Golang 重写的 AI 编程智能体。保持 Codex 原生编码能力的同时，强化多供应商代理、多 Agent 协作、以及 spec-driven 开发工作流。

**核心理念**：一个本地运行的、可自托管的、多模型多供应商的 AI 编程助手，支持 CLI 和 Web 双模式。

---

## 数据持久化架构

### SQLite 作为核心存储

所有结构化数据（配置、状态、历史、元数据）统一使用 SQLite 持久化，存储在 `~/.codex/codex.db`。

**为何 SQLite：**
- 单文件，零配置，无需额外服务进程
- 支持事务和并发读，适合本地桌面场景
- 结构化查询比 JSON/YAML 文件遍历更高效
- 天然支持关联查询（Provider → Backend、Agent → Session、Task → Log）

**仍保留的文件：**
- `agent.yaml` / `soul.md` / `rules/*.md` / `skills/*.md` — Agent 人格和规则，保持纯文本便于编辑和版本管理
- `SKILL.md` — 技能定义，兼容 Claude Code / Hermes 格式
- `config.yaml` — 启动级核心配置（端口、日志等），保持 YAML 便于手动编辑
- `*.spec.md` / `*.plan.md` — 项目规格和计划文档

### 数据库表设计

以下为核心表结构需求，按模块分组：

**Provider 模块：**
```sql
-- 供应商
CREATE TABLE providers (
    id          TEXT PRIMARY KEY,       -- UUID
    name        TEXT NOT NULL UNIQUE,
    icon        TEXT DEFAULT '',
    icon_color  TEXT DEFAULT '',
    category    TEXT DEFAULT 'third_party',  -- official/third_party/partner
    notes       TEXT DEFAULT '',
    in_failover_queue INTEGER DEFAULT 1,
    is_current  INTEGER DEFAULT 0,      -- 当前激活的供应商（全局唯一 =1）
    -- Meta（JSON 列，扁平化为列或存为 JSON blob，二选一）
    api_format            TEXT DEFAULT '',
    cost_multiplier       TEXT DEFAULT '',
    limit_daily_usd       TEXT DEFAULT '',
    limit_monthly_usd     TEXT DEFAULT '',
    is_full_url           INTEGER DEFAULT 0,
    endpoint_auto_select  INTEGER DEFAULT 0,
    prompt_cache_key      TEXT DEFAULT '',
    max_output_tokens     INTEGER DEFAULT 0,
    custom_user_agent     TEXT DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- 后端端点（归属 provider）
CREATE TABLE backends (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,
    api_key     TEXT NOT NULL,
    base_url    TEXT NOT NULL,
    weight      INTEGER DEFAULT 1,
    headers     TEXT DEFAULT '{}',       -- JSON object
    health_status TEXT DEFAULT 'unknown', -- healthy/degraded/unhealthy
    last_probe_at INTEGER DEFAULT 0,
    fail_count  INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL,
    UNIQUE(provider_id, label)
);

-- 后端支持的模型
CREATE TABLE backend_models (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    backend_id  INTEGER NOT NULL REFERENCES backends(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    type        TEXT DEFAULT '',
    context_length INTEGER DEFAULT 0,
    UNIQUE(backend_id, name)
);

-- 供应商预设模板
CREATE TABLE provider_presets (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    category    TEXT NOT NULL,
    icon        TEXT DEFAULT '',
    icon_color  TEXT DEFAULT '',
    website_url TEXT DEFAULT '',
    api_key_url TEXT DEFAULT '',
    sort_order  INTEGER DEFAULT 0
);
```

**Agent 模块：**
```sql
-- Agent 核心配置（agent.yaml 的数据库映射，运行时从 DB 加载，yaml 文件作为备份/导入）
CREATE TABLE agents (
    name            TEXT PRIMARY KEY,     -- 目录名 / 唯一标识
    display_name    TEXT NOT NULL,
    provider        TEXT NOT NULL DEFAULT '',
    model           TEXT NOT NULL DEFAULT '',
    system_prompt   TEXT DEFAULT '',
    max_turns       INTEGER DEFAULT 50,
    reasoning_effort TEXT DEFAULT 'medium',
    tools_mode      TEXT DEFAULT 'all',   -- all / custom
    tools_list      TEXT DEFAULT '[]',    -- JSON array
    mcp_mode        TEXT DEFAULT 'all',   -- all / custom / none
    mcp_list        TEXT DEFAULT '[]',    -- JSON array
    skills_mode     TEXT DEFAULT 'all',   -- all / custom / none
    skills_list     TEXT DEFAULT '[]',    -- JSON array
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

-- Agent 长期记忆（键值对）
CREATE TABLE agent_memory (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_name  TEXT NOT NULL REFERENCES agents(name) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    UNIQUE(agent_name, key)
);
```

**会话模块：**
```sql
-- 对话会话
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,         -- 会话 ID（时间戳格式）
    title       TEXT DEFAULT '',
    agent_name  TEXT NOT NULL REFERENCES agents(name) ON DELETE CASCADE,
    provider    TEXT DEFAULT '',
    model       TEXT DEFAULT '',
    msg_count   INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- 会话消息
CREATE TABLE messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role        TEXT NOT NULL,            -- user / assistant / system / tool
    content     TEXT NOT NULL,
    tool_calls  TEXT DEFAULT '[]',        -- JSON array of tool call objects
    token_count INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL
);
CREATE INDEX idx_messages_session ON messages(session_id, id);
```

**工具与 MCP 模块：**
```sql
-- 全局工具注册表
CREATE TABLE tools (
    name        TEXT PRIMARY KEY,
    description TEXT DEFAULT '',
    category    TEXT DEFAULT 'optional',  -- system / optional
    risk        TEXT DEFAULT 'low',       -- low / medium / high
    approval_required INTEGER DEFAULT 0,
    enabled     INTEGER DEFAULT 1,
    sort_order  INTEGER DEFAULT 0
);

-- 全局 MCP 服务器注册表
CREATE TABLE mcp_servers (
    name        TEXT PRIMARY KEY,
    description TEXT DEFAULT '',
    command     TEXT NOT NULL,
    args        TEXT DEFAULT '[]',        -- JSON array
    env         TEXT DEFAULT '{}',        -- JSON object
    enabled     INTEGER DEFAULT 1,
    sort_order  INTEGER DEFAULT 0
);
```

**任务与调度模块：**
```sql
-- Spec/Plan 任务
CREATE TABLE tasks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    spec_file   TEXT DEFAULT '',          -- 关联的 SPEC.md
    plan_file   TEXT DEFAULT '',          -- 关联的 PLAN.md
    title       TEXT NOT NULL,
    description TEXT DEFAULT '',
    status      TEXT DEFAULT 'pending',   -- pending/in_progress/completed/cancelled
    depends_on  TEXT DEFAULT '[]',        -- JSON array of task IDs
    agent_name  TEXT DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- 定时调度任务
CREATE TABLE scheduled_jobs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    cron_expr   TEXT NOT NULL,
    prompt      TEXT NOT NULL,
    agent_name  TEXT NOT NULL,
    enabled     INTEGER DEFAULT 1,
    last_run    INTEGER DEFAULT 0,
    next_run    INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- 定时任务执行日志
CREATE TABLE job_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id      INTEGER NOT NULL REFERENCES scheduled_jobs(id) ON DELETE CASCADE,
    status      TEXT NOT NULL,            -- success / failed / timeout
    output      TEXT DEFAULT '',
    error       TEXT DEFAULT '',
    duration_ms INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL
);
```

**用量统计模块：**
```sql
-- API 调用用量记录
CREATE TABLE usage_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    backend_id  INTEGER DEFAULT 0,
    model       TEXT DEFAULT '',
    input_tokens  INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_est      REAL DEFAULT 0.0,
    created_at INTEGER NOT NULL
);

-- 每日用量汇总（物化视图或定时聚合）
CREATE TABLE usage_daily (
    date        TEXT NOT NULL,            -- YYYY-MM-DD
    provider_id TEXT NOT NULL,
    model       TEXT NOT NULL,
    input_tokens  INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    request_count INTEGER DEFAULT 0,
    cost_est      REAL DEFAULT 0.0,
    PRIMARY KEY (date, provider_id, model)
);
```

**技能索引模块：**
```sql
-- 技能元数据索引（源文件仍为 SKILL.md）
CREATE TABLE skills (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    tags        TEXT DEFAULT '[]',        -- JSON array
    file_path   TEXT NOT NULL,            -- SKILL.md 文件路径
    source      TEXT DEFAULT '',          -- 来源（local / github）
    enabled     INTEGER DEFAULT 1,
    created_at  INTEGER NOT NULL
);
```

**插件模块：**
```sql
CREATE TABLE plugins (
    id          TEXT PRIMARY KEY,         -- 插件名
    version     TEXT DEFAULT '',
    description TEXT DEFAULT '',
    source_url  TEXT DEFAULT '',
    status      TEXT DEFAULT 'stopped',   -- running / stopped / error
    pid         INTEGER DEFAULT 0,
    installed_at INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);
```

---

## 1. 多供应商代理池（cc-switch 架构对齐）

对标 cc-switch 的 Provider 代理体系，实现供应商级别的管理和智能路由。

### 1.1 Provider 数据模型

每个 Provider 代表一个 AI 供应商（如 OpenAI、Anthropic、第三方代理），存储在 `providers` 表。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 唯一标识 |
| name | string | 显示名称 |
| icon | string | 图标标识符 |
| icon_color | string | 图标主题色 |
| category | enum | official / third_party / partner |
| notes | string | 备注说明 |
| in_failover_queue | bool | 是否参与故障转移队列 |
| is_current | bool | 是否为当前激活供应商（全局唯一） |
| created_at | int64 | 创建时间戳 |

### 1.2 ProviderMeta 元数据

扩展配置字段，对标 cc-switch 的 30+ 字段子集，扁平化存入 `providers` 表：

- `api_format`：API 协议格式（anthropic / openai_chat / openai_responses）
- `cost_multiplier`：成本倍率
- `limit_daily_usd` / `limit_monthly_usd`：日/月消费限额
- `is_full_url`：是否使用完整 URL（不拼接 /v1/chat/completions）
- `endpoint_auto_select`：是否自动选择最佳端点
- `prompt_cache_key`：Prompt Cache 注入配置
- `max_output_tokens`：最大输出 token 数
- `custom_user_agent`：自定义 User-Agent

### 1.3 Backend 端点管理

存储在 `backends` 表，通过 `provider_id` 外键关联 Provider：

- `label`：端点标识名（同一 Provider 内唯一）
- `api_key`：API Key（存储时加密/脱敏）
- `base_url`：API 基础 URL
- `weight`：负载权重（影响选择优先级）
- `headers`：自定义 HTTP 请求头（JSON）
- `health_status`：健康状态（healthy / degraded / unhealthy）
- `fail_count`：连续失败计数
- 支持的模型列表存储在 `backend_models` 表

### 1.4 负载均衡

支持多种池策略：

- `fill_first`：始终使用最高优先级端点，仅失败时切换
- `round_robin`：轮询分配
- `random`：随机选择

端点在以下情况自动降级：
- 连续失败达到阈值
- 健康探测返回非 200
- 超时无响应

### 1.5 健康探测

- 定期向每个 Backend 的 `/models` 端点发送探测请求
- 根据响应状态更新 `backends.health_status`
- 不健康的端点自动从池中移除，恢复后自动加回
- 探测时间记录在 `backends.last_probe_at`
- 前端实时展示健康状态气泡

### 1.6 熔断器（CircuitBreaker）

每个 Backend 需具备三态熔断器：

- **关闭（Closed）**：正常通行，记录失败计数到 `backends.fail_count`
- **半开（Half-Open）**：尝试放行少量请求探测恢复
- **全开（Open）**：拒绝所有请求，等待冷却时间后进入半开

配置参数：
- 失败阈值（默认 5 次连续失败）
- 冷却时间（默认 30 秒）
- 半开探测请求数（默认 1 次）

### 1.7 故障转移队列

- 多个 Provider 按优先级排列成故障转移链
- 当前 Provider 全部 Backend 不可用时，自动切换到下一个 Provider
- 支持显式标记 `in_failover_queue` 控制是否参与

### 1.8 协议适配层

代理层需支持以下协议转换：

- OpenAI Chat Completions ↔ OpenAI Responses
- OpenAI → Anthropic Messages（流式 + 转换）
- 流式 SSE 统一处理
- 模型名映射（别名 → 真实模型名）

### 1.9 媒体过滤（MediaSanitizer）

- 检测请求中的图片/文件内容
- 自动过滤不支持多模态的模型
- 图片格式验证和大小限制

### 1.10 cc-switch 互操作

- 支持从 cc-switch 配置文件导入 Provider 和 Backend（写入 SQLite）
- 支持从 SQLite 导出为 cc-switch 兼容格式
- CLI 命令：`codex-go provider import/export/status`

### 1.11 Provider 预设系统

- 预设模板存储在 `provider_presets` 表
- 内置 50+ 供应商预设（名称、分类、图标、官网、API Key 获取地址）
- 用户可一键从预设创建 Provider（INSERT INTO providers ...）
- 预设按分类筛选：official / third_party / partner
- 可扩展预设数据源（SQLite 本地 + 远程更新）

### 1.12 Provider CRUD API

```
GET    /api/providers            — 列表（JOIN backends 聚合健康状态）
POST   /api/providers            — 创建（INSERT providers + backends）
PUT    /api/providers/:id        — 更新
DELETE /api/providers/:id        — 删除（CASCADE 删除关联 backends/usage_logs）
POST   /api/providers/:id/switch — 切换为当前（UPDATE is_current=1，其余=0）
POST   /api/providers/:id/probe  — 手动触发健康探测
```

### 1.13 Backend CRUD API

```
GET    /api/providers/:id/backends        — 后端列表（JOIN backend_models）
POST   /api/providers/:id/backends        — 添加后端
PUT    /api/providers/:id/backends/:label — 更新后端
DELETE /api/providers/:id/backends/:label — 删除后端
POST   /api/providers/:id/backends/probe  — 全量探测
```

### 1.14 独立代理进程

- 可选启动为独立 HTTP Proxy Server（类似 cc-switch 的 15721 端口服务）
- 动态端口绑定
- Live Config 接管：切换 Provider 时更新 `providers.is_current`，无需重启

### 1.15 用量统计

- 每次 API 调用写入 `usage_logs` 表
- Token 用量计算（input + output）
- 按 Provider / Backend / Model 维度查询
- 每日聚合到 `usage_daily` 表（定时任务触发或写入时 upsert）
- 前端用量面板可视化（SELECT + GROUP BY + 图表）

---

## 2. 多 Agent 协作系统

### 2.1 Agent 文件 + 数据库混合存储

每个 Agent 的**人格和规则**以文件形式存储（便于编辑和版本管理），**配置和运行时数据**以 SQLite 存储（便于查询和管理）。

**Agent 目录结构**（`~/.codex/agents/<name>/`）：

```
<agent-name>/
├── soul.md             # 人格/角色定义，Markdown 格式
├── rules/              # 自定义规则文件
│   └── custom-rules.md
└── skills/             # Agent 专属技能 SKILL.md
    └── ...
```

**Agent 配置存储在 `agents` 表**：

| 字段 | 类型 | 说明 |
|------|------|------|
| name | TEXT PK | Agent 名称（唯一标识，即目录名） |
| display_name | TEXT | 前端展示名称 |
| provider | TEXT | 使用的供应商名称 |
| model | TEXT | 使用的模型 |
| system_prompt | TEXT | 基础系统提示词（运行时与 soul.md 合并） |
| max_turns | INTEGER | 最大对话轮数 |
| reasoning_effort | TEXT | 推理深度 |
| tools_mode | TEXT | 工具启用模式：`all` / `custom` |
| tools_list | TEXT | 自定义工具列表（JSON array） |
| mcp_mode | TEXT | MCP 启用模式：`all` / `custom` / `none` |
| mcp_list | TEXT | 自定义 MCP 服务器列表（JSON array） |
| skills_mode | TEXT | 技能启用模式：`all` / `custom` / `none` |
| skills_list | TEXT | 自定义技能列表（JSON array） |

**soul.md**：
- Agent 的人格/角色定义文件
- Markdown 格式，自由书写
- 运行时与 `system_prompt` 字段合并注入到对话上下文
- 独立于数据库，方便版本管理和分享

**Agent 长期记忆存储在 `agent_memory` 表**：
- 键值对模型，可存任意结构化数据
- 跨会话持久化
- 按 agent_name + key 唯一索引

**隔离原则**：
- 每个 Agent 的 session（`sessions` 表按 `agent_name` 过滤）互不可见
- 每个 Agent 的 memory（`agent_memory` 表按 `agent_name` 过滤）互不共享
- 删除 Agent 时：CASCADE 删除 sessions/messages/agent_memory，同时清理文件目录

---

### 2.2 全局 Tool / MCP / Skills 配置

Tools、MCP、Skills 在系统级别统一存储在 SQLite，所有 Agent 共享同一份资源池。

**全局 Tool 注册表**（`tools` 表）：

系统默认工具（`category = 'system'`）是所有 Agent 必备的，不可移除：

| name | category | risk | approval_required |
|------|----------|------|-------------------|
| read_file | system | low | 0 |
| write_file | system | low | 0 |
| edit_file | system | medium | 0 |
| grep | system | low | 0 |
| ls | system | low | 0 |
| shell | optional | high | 1 |
| git | optional | medium | 0 |
| web_fetch | optional | medium | 0 |
| git_worktree | optional | medium | 0 |
| image_gen | optional | low | 0 |
| browser | optional | high | 1 |
| code_review | optional | low | 0 |

**全局 MCP 注册表**（`mcp_servers` 表）：

```sql
INSERT INTO mcp_servers (name, command, args, description) VALUES
('filesystem', 'npx', '["-y","@anthropic/mcp-filesystem"]', '文件系统操作'),
('github',     'npx', '["-y","@anthropic/mcp-github"]',      'GitHub API 集成'),
('database',   'python', '["-m","mcp_server_db"]',           '数据库查询');
```

**Agent 对全局资源的选择模式**：

| 模式 | 行为 |
|------|------|
| `all` | 启用全局所有资源（系统工具 + 全部可选工具 / 全部 MCP / 全部 Skills） |
| `custom` | 仅启用 `*_list` 中明确列出的资源 |
| `none` | 不启用该类资源（仅适用于 MCP 和 Skills；Tools 至少包含 system 工具） |

---

### 2.3 Agent Registry

- Agent 目录 `~/.codex/agents/` 下的每个子目录即为一个 Agent
- 启动时扫描目录 → 检查 `agents` 表是否存在记录，不存在则从 `agent.yaml` 导入或创建默认配置
- 支持热加载：检测目录/文件变更后自动更新 `agents` 表
- 加载失败时跳过该 Agent 并在日志告警

### 2.4 Agent Manager

- 运行时从 `agents` 表加载所有 Agent 配置
- 根据 `tools_mode` / `mcp_mode` / `skills_mode` 从全局表装配 Agent 能力
- 按名称获取/切换 Agent 实例
- 每个 Agent 维护独立的对话历史（`sessions` 表过滤）和长期记忆（`agent_memory` 表过滤）

### 2.5 Agent Copy（快速复制）

支持一键复制已有 Agent，完整克隆其配置和人格。

**复制范围**：

| 内容 | 是否复制 | 说明 |
|------|---------|------|
| `agents` 表记录 | ✅ 复制 | name 重命名，其余字段完整克隆 |
| soul.md | ✅ 复制 | 人格文件完整克隆 |
| rules/ | ✅ 复制 | 自定义规则完整克隆 |
| skills/ | ✅ 复制 | Agent 专属技能完整克隆 |
| `agent_memory` 表 | ❌ 不复制 | 新 Agent 从空白记忆开始 |
| `sessions` / `messages` 表 | ❌ 不复制 | 不复制对话历史 |

**CLI 命令**：
```bash
codex-go agent copy <source-name> <target-name>
```

**API**：
```
POST /api/agents/copy
Body: { "source": "code-reviewer", "target": "code-reviewer-v2" }
```

---

### 2.6 多 Agent 聊天室

- 用户可在对话中 @mention 不同的 Agent
- Agent 之间可互相回复和协作
- 消息按 Agent 着色区分
- 支持 Agent 对话路由：根据消息内容自动分派给合适的 Agent

### 2.7 Agent 委派

- 一个 Agent 可将子任务委派给另一个 Agent
- 委派时传递上下文和指令
- 子 Agent 完成后返回结果给主 Agent
- 支持委派链（深度可配置）

### 2.8 Agent 并行执行

- 多个 Agent 同时处理不同子任务
- 结果汇总到主 Agent 或直接返回用户
- 并行数可配置（默认 3）

### 2.9 WebSocket 流式通信

- 实时推送 Agent 思考过程
- 工具调用进度通知
- 审批请求推送
- 多 Agent 消息分流

### 2.10 Agent CRUD API

```
GET    /api/agents              — 所有 Agent 列表（SELECT * FROM agents）
GET    /api/agents/:name        — Agent 详情（JOIN agent_memory 聚合记忆条数）
POST   /api/agents              — 创建 Agent（INSERT agents + 创建目录 + soul.md）
PUT    /api/agents/:name        — 更新 Agent 配置（UPDATE agents）
DELETE /api/agents/:name        — 删除 Agent（CASCADE 删除 sessions/messages/memory + 清理目录）
POST   /api/agents/copy         — 复制 Agent（INSERT + 文件拷贝）
GET    /api/agents/:name/soul   — 读取 soul.md 内容（从文件系统）
PUT    /api/agents/:name/soul   — 更新 soul.md
GET    /api/agents/:name/memory — 读取 Agent 长期记忆列表
PUT    /api/agents/:name/memory — 写入/更新记忆键值对
```

---

## 3. Spec-Driven 开发工作流

### 3.1 管线

完整的从需求到代码的结构化管线：

```
/spec <描述>  →  生成 SPEC.md（需求规格）
/plan [spec]  →  生成 PLAN.md（实施计划）
/tasks        →  查看任务列表（可执行单元，从 tasks 表读取）
/implement n  →  实现第 n 个任务
```

### 3.2 SPEC 生成

- 输入：自然语言功能描述
- 输出：结构化的 SPEC.md，包含功能清单、差距分析、架构设计、分阶段路线图
- AI 驱动生成，产出需可直接进入 plan 阶段的质量

### 3.3 PLAN 生成

- 输入：SPEC.md 文件
- 输出：PLAN.md + `tasks` 表记录（自动解析任务条目并写入）
- 每个任务需有明确的完成标准和验收条件

### 3.4 任务管理

- 任务存储在 `tasks` 表
- 支持状态流转：pending → in_progress → completed → cancelled
- 任务可标记依赖关系（`depends_on` JSON 数组）
- 前端 Kanban 或列表视图
- 任务关联 SPEC/PLAN 文件路径（`spec_file` / `plan_file`）

### 3.5 Steer 模式（前端）

- ChatPage 中可视化 spec → plan → tasks → execute 管线
- 每个阶段有独立卡片，显示进度和状态
- 支持分步执行和全自动执行
- 阶段颜色编码：spec(indigo) → plan(purple) → tasks(green) → implement(amber) → execute(red)

### 3.6 审批机制

- 高风险操作需人工确认
- 审批请求通过 WebSocket 实时推送
- 前端弹窗展示操作详情和风险等级
- 支持批量审批和自动审批白名单

---

## 4. 工具系统

### 4.1 核心工具集

Agent 可调用的工具，存储在 `tools` 表：

| 工具 | 功能 | 风险等级 | 需审批 |
|------|------|---------|--------|
| shell | 执行 Shell 命令 | 高 | 是 |
| read_file | 读取文件内容 | 低 | 否 |
| edit_file | 编辑文件（查找替换） | 中 | 否 |
| write_file | 创建/覆盖文件 | 中 | 否 |
| grep | 内容搜索 | 低 | 否 |
| ls | 目录浏览 | 低 | 否 |
| git | Git 操作 | 中 | 否 |
| web_fetch | HTTP 请求 | 中 | 否 |

### 4.2 MCP 集成

- 实现 Model Context Protocol 客户端
- MCP 服务器配置存储在 `mcp_servers` 表
- 支持 stdio 和 HTTP 两种传输
- 动态注册外部工具服务器提供的工具
- MCP 工具与内置工具统一注册到 Tool Registry

### 4.3 沙箱执行

- 命令在安全隔离环境中执行
- 可配置允许/禁止的命令列表
- 文件系统访问范围限制
- 网络访问控制

### 4.4 工具审批

- 高风险工具（`approval_required = 1`）调用需人工确认
- 审批请求包含：工具名、参数、风险等级、描述
- 超时自动拒绝
- 支持会话级别的审批记忆

---

## 5. 插件系统

### 5.1 插件管理

- 插件配置存储在 `plugins` 表
- 安装：从本地路径或 Git 仓库安装
- 卸载：清理插件文件 + DELETE FROM plugins + 停止进程
- 启用/禁用：运行时控制插件状态

### 5.2 插件进程

- 每个插件作为独立进程运行
- 生命周期管理：启动/停止/重启
- 进程健康监控（更新 `plugins.pid` 和 `plugins.status`）
- 标准输出/错误日志收集

### 5.3 插件 API

```
GET    /api/plugins          — SELECT * FROM plugins
POST   /api/plugins/install  — 安装插件
POST   /api/plugins/:id/start   — 启动
POST   /api/plugins/:id/stop    — 停止
DELETE /api/plugins/:id         — 卸载
```

---

## 6. 定时任务调度

### 6.1 Cron 引擎

- 支持标准 cron 表达式（5 字段：分 时 日 月 周）
- 任务存储在 `scheduled_jobs` 表
- 执行日志存储在 `job_logs` 表

### 6.2 任务配置

每个定时任务（`scheduled_jobs` 表）包含：
- `name`：任务名称
- `cron_expr`：cron 表达式
- `prompt`：发送给 Agent 的提示词
- `agent_name`：执行的 Agent 名称
- `enabled`：启用状态
- `last_run` / `next_run`：时间戳

### 6.3 任务链

- 通过 `depends_on`（JSON 数组关联 `tasks` 表或 `scheduled_jobs` 表）
- 前置任务完成后自动触发后续任务
- 失败时的处理策略（跳过/重试/中止）

### 6.4 前端管理

- ScheduledPage：SELECT * FROM scheduled_jobs 列表
- 创建/编辑/删除/启停
- 执行历史查看（SELECT * FROM job_logs WHERE job_id = ? ORDER BY created_at DESC）
- 手动触发执行

---

## 7. Skills 技能系统

### 7.1 技能格式

- 兼容 SKILL.md 格式（与 Claude Code、Hermes 一致）
- YAML frontmatter：name、description、tags、triggers
- Markdown body：步骤说明、命令示例、注意事项

### 7.2 技能加载与索引

- 技能源文件存放在 `~/.codex/skills/`（兼容 `~/.claude/skills/`、`~/.agents/skills/`）
- 启动时扫描所有 SKILL.md → 解析 frontmatter → 写入/更新 `skills` 表
- `skills.file_path` 记录源文件路径，运行时从文件系统读取完整内容
- 按名称或标签检索（`skills.tags` JSON 数组 + JSON 查询）

### 7.3 技能商店

- 从 GitHub 仓库浏览和安装技能
- 安装时下载文件到技能目录 + INSERT INTO skills
- 技能元数据展示：名称、描述、标签、作者

---

## 8. 会话管理

### 8.1 会话存储

- 会话存储在 `sessions` 表
- 消息存储在 `messages` 表（按 session_id + id 索引）
- 每次对话轮次自动 INSERT 新消息 + UPDATE sessions.msg_count

### 8.2 会话操作

- 创建新会话：INSERT INTO sessions（自动生成 ID）
- 恢复历史会话：SELECT messages WHERE session_id = ? ORDER BY id
- 列出所有会话：SELECT * FROM sessions WHERE agent_name = ? ORDER BY updated_at DESC
- 删除会话：DELETE FROM sessions WHERE id = ?（CASCADE 删除 messages）
- 清空当前会话：DELETE FROM messages WHERE session_id = ?（保留 sessions 记录）
- 会话摘要查询：id + title + msg_count + updated_at

---

## 9. Web 前端

### 9.1 技术栈

- React 18 + TypeScript
- Ant Design 5（组件库）
- Vite（构建工具）
- WebSocket（实时通信）

### 9.2 设计规范

- 主题：明/暗双模式切换
- 主色调：#5e6ad2（Linear Indigo）
- 字体：Inter, system-ui, -apple-system, sans-serif
- 圆角：6px
- 回复列表使用卡片/气泡布局，不使用表格

### 9.3 ChatPage（主对话页）

- 消息列表：从 `messages` 表加载，用户/AI 消息气泡，支持 Markdown 渲染
- 多 Agent 选择器：下拉切换活跃 Agent
- 输入区域：文本输入 + 文件附件 + 发送/停止按钮
- Steer 模式：spec → plan → tasks → execute 可视化管线
- 审批弹窗：操作详情 + 风险等级 + 通过/拒绝按钮
- 进度指示：思考中动画 + 工具调用状态
- 快捷键：Ctrl+Enter 发送，Escape 停止

### 9.4 SettingsPage（设置页）

- **Provider 列表**：卡片网格布局，展示图标、名称、分类标签、健康状态气泡、操作按钮
- **Provider 表单**：分类标签筛选、从预设添加、详情编辑（含 Meta 高级配置）
- **Agent Profile 管理**：列表 + 创建/编辑表单
- **Backends 管理**：表格式列表，添加/编辑/删除端点，健康状态列
- **Import/Export**：cc-switch 配置互导

### 9.5 ScheduledPage（定时任务页）

- 任务列表表格（`scheduled_jobs` + LEFT JOIN `job_logs` 最近一次执行状态）
- 创建/编辑表单（cron 表达式 + prompt + Agent）
- 启停开关
- 执行历史面板

### 9.6 PluginsPage（插件页）

- 已安装插件列表（`plugins` 表）
- 插件详情（名称、版本、状态、描述）
- 安装对话框（GitHub URL 输入）
- 启停/卸载操作

### 9.7 RightPanel（右侧面板）

多个 Tab 页：
- **Review**：代码审查 diff 视图
- **Terminal**：Web 终端
- **Browser**：内嵌浏览器
- **Files**：工作区文件树
- **SideTasks**：子任务列表（`tasks` 表前端过滤）

### 9.8 布局

```
┌──────────────────────────────────────────────────┐
│ TitleBar (toggle left | title | toggle right)    │
├────────┬───────────────────────────┬─────────────┤
│        │                           │             │
│ Left   │    Chat / Page Content    │   Right     │
│ Sidebar│                           │   Panel     │
│        │                           │             │
└────────┴───────────────────────────┴─────────────┘
```

- 左侧边栏：会话列表（`sessions` 表） + 导航
- 中央：当前页面内容
- 右侧面板：可折叠的辅助面板
- 非 Chat 页面自动隐藏左右侧栏，使用页面内导航
- 键盘快捷键：Ctrl+Alt+B（切换右侧面板）、Ctrl+Alt+S（打开 SideTasks）

---

## 10. CLI 模式

### 10.1 启动参数

| 参数 | 说明 |
|------|------|
| `--serve` | 启动 HTTP/WebSocket API 服务 |
| `--addr` | API 服务监听地址（默认 :1977） |
| `--db` | SQLite 数据库路径（默认 `~/.codex/codex.db`） |
| `--model` | 覆盖模型名 |
| `--provider` | 覆盖供应商 |
| `--api-key` | 覆盖 API Key |
| `--base-url` | 覆盖 Base URL |
| `--prompt` | 单次 prompt 模式（非交互） |
| `--resume` | 恢复指定会话 |
| `--list` | 列出历史会话 |
| `--delete` | 删除指定会话 |
| `--version` | 版本信息 |
| `--system-prompt` | 覆盖系统提示词 |
| `--skills-dir` | 额外技能目录 |

### 10.2 交互模式

```
Codex Go · provider/model · N tools · M skills
Session: 20260718-030000
Type /exit, /history, /clear, /save, /sessions
      /spec <desc>, /plan [spec], /tasks, /implement <n>
>
```

内置命令：
- `/exit`, `/quit` — 退出
- `/history` — 查看对话历史（SELECT messages）
- `/clear` — 清空当前对话
- `/save` — 保存会话
- `/sessions` — 列出历史会话（SELECT sessions）
- `/spec <描述>` — 生成 SPEC.md
- `/plan [spec文件]` — 生成 PLAN.md
- `/tasks` — 列出任务（SELECT tasks）
- `/implement <编号>` — 实现任务

### 10.3 管道模式

支持从 stdin 读取 prompt：
```bash
echo "解释这段代码" | codex-go
cat bug_report.txt | codex-go
```

### 10.4 Provider 子命令

```bash
codex-go provider import <cc-switch-config.yaml>   # 导入（→ SQLite）
codex-go provider export [output.yaml]             # 导出（SQLite → YAML）
codex-go provider status                           # 查看状态（查询 SQLite）
```

---

## 11. 部署与构建

### 11.1 构建

- `make build`：构建 CLI 二进制 + 前端
- `make build-all`：全平台交叉编译
- `make web`：仅构建前端
- `make desktop`：构建 Electron 桌面应用

### 11.2 运行

```bash
# CLI 交互模式
./dist/cli/codex-go

# API 服务模式
./dist/cli/codex-go --serve --addr :1977

# 指定数据库路径
./dist/cli/codex-go --db /path/to/custom.db --serve

# 单次 prompt
./dist/cli/codex-go --prompt "重构这个函数"
```

### 11.3 数据目录

```
~/.codex/
├── codex.db           # SQLite 数据库（核心数据）
├── codex.db-wal       # SQLite WAL 日志（自动管理）
├── codex.db-shm       # SQLite 共享内存（自动管理）
├── config.yaml        # 启动级配置（端口、日志级别等）
├── agents/            # Agent 目录（soul.md / rules / skills 文件）
│   ├── default/
│   └── code-reviewer/
├── skills/            # 全局技能 SKILL.md 文件
├── specs/             # SPEC.md / PLAN.md 项目文档
└── logs/              # 运行日志
```

### 11.4 数据库迁移

- 首次启动时自动创建 `codex.db` 和所有表（`CREATE TABLE IF NOT EXISTS`）
- 版本升级时运行迁移脚本（`schema_version` 表追踪当前版本）
- 支持从旧格式（JSON/YAML 文件）导入到 SQLite

```sql
CREATE TABLE schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  INTEGER NOT NULL
);
```

---

## 12. 非功能需求

### 12.1 性能

- API 服务启动时间 < 2 秒
- SQLite 查询（单表主键）< 1ms
- 会话消息列表（1000 条）加载 < 50ms（利用 `idx_messages_session` 索引）
- 流式响应首字延迟 < 1 秒
- 健康探测间隔 5-30 秒可配置
- 前端首屏加载 < 3 秒

### 12.2 安全

- API Key 在 `backends.api_key` 存储时加密（AES-256）
- API Key 在日志和界面中脱敏显示（前 3 位 + 后 4 位）
- SQLite 文件权限 0600（仅 owner 可读写）
- 沙箱执行限制文件系统和网络访问
- 高风险工具需人工审批

### 12.3 可靠性

- SQLite WAL 模式：支持并发读不阻塞写，崩溃恢复
- 单 Backend 故障不影响整体服务
- 自动故障转移，无人工干预
- 会话消息自动保存（每轮 INSERT）
- 日志轮转，防止磁盘占满
- 数据库定期 VACUUM（回收删除空间）

### 12.4 兼容性

- 兼容 OpenAI Chat Completions / Responses API
- 兼容 Anthropic Messages API
- 兼容 cc-switch 配置格式（导入/导出）
- 兼容 Claude Code / Hermes SKILL.md 格式
- 跨平台：Linux / macOS / Windows
- SQLite 3.35+（内嵌，无需系统安装）
