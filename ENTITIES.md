# Codex Go — 实体定义

本文档定义所有数据结构、数据库表、API 端点、WebSocket 协议和前端组件类型。

> 术语含义见 [GLOSSARY.md](./GLOSSARY.md)，约束规则见 [SPEC.md](./SPEC.md)。

---

## 1. 数据模型

### 1.1 Provider

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID v4 | PK, 不可变 | 唯一标识 |
| name | string(255) | UNIQUE NOT NULL | 显示名称 |
| icon | string(64) | default '' | 图标标识符 |
| icon_color | string(7) | default '' | HEX 主题色 |
| category | enum | default 'third_party' | official / third_party / partner |
| notes | text | default '' | 备注 |
| in_failover_queue | bool | default true | 故障转移参与标记 |
| is_current | bool | default false | 当前激活（全局唯一为 1） |
| api_format | enum | default '' | openai_chat / openai_responses / anthropic |
| cost_multiplier | decimal(5,2) | default '1.00' | 成本倍率 |
| limit_daily_usd | decimal(10,4) | default '' | 日消费限额 |
| limit_monthly_usd | decimal(10,4) | default '' | 月消费限额 |
| is_full_url | bool | default false | 使用完整 URL |
| endpoint_auto_select | bool | default false | 自动选择最佳端点 |
| prompt_cache_key | string(255) | default '' | Prompt Cache 注入键 |
| max_output_tokens | int | default 0 | 最大输出 token，0=不限制 |
| custom_user_agent | string(255) | default '' | 自定义 UA |
| created_at | unix ts | NOT NULL | 创建时间 |
| updated_at | unix ts | NOT NULL | 更新时间 |

### 1.2 Backend

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | auto int | PK | 自增 ID |
| provider_id | UUID | FK → providers(id) ON DELETE CASCADE | 归属 Provider |
| label | string(255) | NOT NULL, (provider_id, label) UNIQUE | 端点标识 |
| api_key | text | NOT NULL, AES-256 加密存储 | API Key |
| base_url | text | NOT NULL | API 基础 URL |
| weight | int | >= 1, default 1 | 负载权重 |
| headers | text | JSON object, default {} | 自定义请求头 |
| health_status | enum | default 'unknown' | healthy / degraded / unhealthy / unknown |
| last_probe_at | unix ts | default 0 | 上次探测时间 |
| fail_count | int | >= 0, default 0 | 连续失败计数 |
| created_at | unix ts | NOT NULL | 创建时间 |

### 1.3 BackendModel

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | auto int | PK | 自增 ID |
| backend_id | int | FK → backends(id) ON DELETE CASCADE | 归属 Backend |
| name | string(255) | NOT NULL, (backend_id, name) UNIQUE | 模型名 |
| type | string(64) | default '' | 模型类型标记 |
| context_length | int | default 0 | 上下文窗口大小 |

### 1.4 ProviderPreset

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | auto int | PK | 自增 ID |
| name | string(255) | UNIQUE NOT NULL | 预设名称 |
| category | string(64) | NOT NULL | official / third_party / partner |
| icon | string(64) | default '' | 图标标识符 |
| icon_color | string(7) | default '' | HEX 主题色 |
| website_url | string(512) | default '' | 供应商官网 |
| api_key_url | string(512) | default '' | API Key 获取地址 |
| sort_order | int | default 0 | 排序权重 |

### 1.5 Agent

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| name | text | PK, 即目录名 | Agent 唯一标识 |
| display_name | text | NOT NULL | 前端展示名 |
| provider | text | NOT NULL | 引用 Provider name |
| model | text | NOT NULL | 模型名 |
| system_prompt | text | default '' | 基础系统提示词 |
| max_turns | int | >= 1, default 50 | 最大对话轮数 |
| reasoning_effort | text | default 'medium' | low / medium / high |
| tools_mode | text | default 'all' | all / custom |
| tools_list | text | JSON array, default [] | custom 模式下的工具列表 |
| mcp_mode | text | default 'all' | all / custom / none |
| mcp_list | text | JSON array, default [] | custom 模式下的 MCP 列表 |
| skills_mode | text | default 'all' | all / custom / none |
| skills_list | text | JSON array, default [] | custom 模式下的技能列表 |
| created_at | unix ts | NOT NULL | 创建时间 |
| updated_at | unix ts | NOT NULL | 更新时间 |

### 1.6 AgentMemory

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | auto int | PK | 自增 ID |
| agent_name | text | FK → agents(name) ON DELETE CASCADE | 归属 Agent |
| key | text | NOT NULL, (agent_name, key) UNIQUE | 记忆键 |
| value | text | NOT NULL | 记忆值 |
| created_at | unix ts | NOT NULL | 创建时间 |
| updated_at | unix ts | NOT NULL | 更新时间 |

### 1.7 Session

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | text | PK, 格式 YYYYMMDD-HHmmss | 会话 ID |
| title | text | default '', 首条用户消息前100字符 | 标题 |
| agent_name | text | FK → agents(name) ON DELETE CASCADE | 归属 Agent |
| provider | text | default '' | 快照：Provider |
| model | text | default '' | 快照：Model |
| msg_count | int | default 0 | 消息计数 |
| created_at | unix ts | NOT NULL | 创建时间 |
| updated_at | unix ts | NOT NULL | 更新时间 |

### 1.8 Message

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | auto int | PK | 自增 ID |
| session_id | text | FK → sessions(id) ON DELETE CASCADE, INDEXED | 归属 Session |
| role | text | NOT NULL | user / assistant / system / tool |
| content | text | NOT NULL | 消息正文（Markdown） |
| tool_calls | text | JSON array, default [] | 工具调用信息 |
| token_count | int | default 0 | Token 估算数 |
| created_at | unix ts | NOT NULL | 创建时间 |

### 1.9 Tool

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| name | text | PK | 工具名 |
| description | text | default '' | 功能描述 |
| category | text | default 'optional' | system / optional |
| risk | text | default 'low' | low / medium / high |
| approval_required | int | default 0 | 是否需要审批 |
| enabled | int | default 1 | 是否启用 |
| sort_order | int | default 0 | 排序权重 |

### 1.10 MCPServer

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| name | text | PK | MCP 服务器名 |
| description | text | default '' | 功能描述 |
| command | text | NOT NULL | 可执行命令 |
| args | text | JSON array, default [] | 命令行参数 |
| env | text | JSON object, default {} | 环境变量 |
| enabled | int | default 1 | 是否启用 |
| sort_order | int | default 0 | 排序权重 |

### 1.11 Task

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | auto int | PK | 自增 ID |
| spec_file | text | default '' | 关联 SPEC.md 路径 |
| plan_file | text | default '' | 关联 PLAN.md 路径 |
| title | text | NOT NULL | 任务标题 |
| description | text | default '' | 任务描述 |
| status | text | default 'pending' | pending / in_progress / completed / cancelled |
| depends_on | text | JSON array, default [] | 前置任务 ID 列表 |
| agent_name | text | default '' | 分配 Agent |
| created_at | unix ts | NOT NULL | 创建时间 |
| updated_at | unix ts | NOT NULL | 更新时间 |

### 1.12 ScheduledJob

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | auto int | PK | 自增 ID |
| name | text | NOT NULL | 任务名称 |
| cron_expr | text | NOT NULL | 5 字段 cron 表达式 |
| prompt | text | NOT NULL | 发送给 Agent 的提示词 |
| agent_name | text | NOT NULL | 执行 Agent |
| enabled | int | default 1 | 启用标记 |
| last_run | unix ts | default 0 | 上次执行时间 |
| next_run | unix ts | default 0 | 下次执行时间 |
| created_at | unix ts | NOT NULL | 创建时间 |
| updated_at | unix ts | NOT NULL | 更新时间 |

### 1.13 JobLog

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | auto int | PK | 自增 ID |
| job_id | int | FK → scheduled_jobs(id) ON DELETE CASCADE | 归属 Job |
| status | text | NOT NULL | success / failed / timeout |
| output | text | default '' | 执行输出 |
| error | text | default '' | 错误信息 |
| duration_ms | int | default 0 | 执行耗时 |
| created_at | unix ts | NOT NULL | 创建时间 |

### 1.14 UsageLog

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | auto int | PK | 自增 ID |
| provider_id | UUID | FK → providers(id) ON DELETE CASCADE | 调用 Provider |
| backend_id | int | default 0 | 调用 Backend |
| model | text | default '' | 使用的模型 |
| input_tokens | int | default 0 | 输入 token 数 |
| output_tokens | int | default 0 | 输出 token 数 |
| cost_est | real | default 0.0 | 估算费用 |
| created_at | unix ts | NOT NULL | 创建时间 |

### 1.15 UsageDaily

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| date | text | PK, YYYY-MM-DD | 日期 |
| provider_id | UUID | PK | Provider |
| model | text | PK | 模型 |
| input_tokens | int | default 0 | 输入 token 汇总 |
| output_tokens | int | default 0 | 输出 token 汇总 |
| request_count | int | default 0 | 请求次数 |
| cost_est | real | default 0.0 | 费用汇总 |

### 1.16 Skill

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | auto int | PK | 自增 ID |
| name | text | UNIQUE NOT NULL | 技能名 |
| description | text | default '' | 描述 |
| tags | text | JSON array, default [] | 标签列表 |
| file_path | text | NOT NULL | SKILL.md 文件路径 |
| source | text | default '' | 来源（local / github） |
| enabled | int | default 1 | 启用标记 |
| created_at | unix ts | NOT NULL | 创建时间 |

### 1.17 Plugin

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | text | PK | 插件名 |
| version | text | default '' | 版本号 |
| description | text | default '' | 描述 |
| source_url | text | default '' | 安装来源 |
| status | text | default 'stopped' | running / stopped / error |
| pid | int | default 0 | 进程 PID |
| installed_at | unix ts | NOT NULL | 安装时间 |
| updated_at | unix ts | NOT NULL | 更新时间 |

### 1.18 SchemaVersion

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| version | int | PK | 当前数据库版本号 |
| applied_at | unix ts | NOT NULL | 应用时间 |

---

## 2. 数据库 DDL

```sql
CREATE TABLE schema_version (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL);

CREATE TABLE providers (
    id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, icon TEXT DEFAULT '', icon_color TEXT DEFAULT '',
    category TEXT DEFAULT 'third_party', notes TEXT DEFAULT '', in_failover_queue INTEGER DEFAULT 1,
    is_current INTEGER DEFAULT 0,
    api_format TEXT DEFAULT '', cost_multiplier TEXT DEFAULT '', limit_daily_usd TEXT DEFAULT '',
    limit_monthly_usd TEXT DEFAULT '', is_full_url INTEGER DEFAULT 0, endpoint_auto_select INTEGER DEFAULT 0,
    prompt_cache_key TEXT DEFAULT '', max_output_tokens INTEGER DEFAULT 0, custom_user_agent TEXT DEFAULT '',
    created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);

CREATE TABLE backends (
    id INTEGER PRIMARY KEY AUTOINCREMENT, provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    label TEXT NOT NULL, api_key TEXT NOT NULL, base_url TEXT NOT NULL, weight INTEGER DEFAULT 1,
    headers TEXT DEFAULT '{}', health_status TEXT DEFAULT 'unknown', last_probe_at INTEGER DEFAULT 0,
    fail_count INTEGER DEFAULT 0, created_at INTEGER NOT NULL,
    UNIQUE(provider_id, label)
);

CREATE TABLE backend_models (
    id INTEGER PRIMARY KEY AUTOINCREMENT, backend_id INTEGER NOT NULL REFERENCES backends(id) ON DELETE CASCADE,
    name TEXT NOT NULL, type TEXT DEFAULT '', context_length INTEGER DEFAULT 0,
    UNIQUE(backend_id, name)
);

CREATE TABLE provider_presets (
    id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, category TEXT NOT NULL,
    icon TEXT DEFAULT '', icon_color TEXT DEFAULT '', website_url TEXT DEFAULT '',
    api_key_url TEXT DEFAULT '', sort_order INTEGER DEFAULT 0
);

CREATE TABLE agents (
    name TEXT PRIMARY KEY, display_name TEXT NOT NULL, provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '', system_prompt TEXT DEFAULT '', max_turns INTEGER DEFAULT 50,
    reasoning_effort TEXT DEFAULT 'medium', tools_mode TEXT DEFAULT 'all', tools_list TEXT DEFAULT '[]',
    mcp_mode TEXT DEFAULT 'all', mcp_list TEXT DEFAULT '[]', skills_mode TEXT DEFAULT 'all',
    skills_list TEXT DEFAULT '[]', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);

CREATE TABLE agent_memory (
    id INTEGER PRIMARY KEY AUTOINCREMENT, agent_name TEXT NOT NULL REFERENCES agents(name) ON DELETE CASCADE,
    key TEXT NOT NULL, value TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
    UNIQUE(agent_name, key)
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY, title TEXT DEFAULT '', agent_name TEXT NOT NULL REFERENCES agents(name) ON DELETE CASCADE,
    provider TEXT DEFAULT '', model TEXT DEFAULT '', msg_count INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);

CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL, content TEXT NOT NULL, tool_calls TEXT DEFAULT '[]',
    token_count INTEGER DEFAULT 0, created_at INTEGER NOT NULL
);
CREATE INDEX idx_messages_session ON messages(session_id, id);

CREATE TABLE tools (
    name TEXT PRIMARY KEY, description TEXT DEFAULT '', category TEXT DEFAULT 'optional',
    risk TEXT DEFAULT 'low', approval_required INTEGER DEFAULT 0, enabled INTEGER DEFAULT 1,
    sort_order INTEGER DEFAULT 0
);

CREATE TABLE mcp_servers (
    name TEXT PRIMARY KEY, description TEXT DEFAULT '', command TEXT NOT NULL,
    args TEXT DEFAULT '[]', env TEXT DEFAULT '{}', enabled INTEGER DEFAULT 1, sort_order INTEGER DEFAULT 0
);

CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT, spec_file TEXT DEFAULT '', plan_file TEXT DEFAULT '',
    title TEXT NOT NULL, description TEXT DEFAULT '', status TEXT DEFAULT 'pending',
    depends_on TEXT DEFAULT '[]', agent_name TEXT DEFAULT '',
    created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);

CREATE TABLE scheduled_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, cron_expr TEXT NOT NULL,
    prompt TEXT NOT NULL, agent_name TEXT NOT NULL, enabled INTEGER DEFAULT 1,
    last_run INTEGER DEFAULT 0, next_run INTEGER DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);

CREATE TABLE job_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT, job_id INTEGER NOT NULL REFERENCES scheduled_jobs(id) ON DELETE CASCADE,
    status TEXT NOT NULL, output TEXT DEFAULT '', error TEXT DEFAULT '',
    duration_ms INTEGER DEFAULT 0, created_at INTEGER NOT NULL
);

CREATE TABLE usage_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT, provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    backend_id INTEGER DEFAULT 0, model TEXT DEFAULT '',
    input_tokens INTEGER DEFAULT 0, output_tokens INTEGER DEFAULT 0, cost_est REAL DEFAULT 0.0,
    created_at INTEGER NOT NULL
);

CREATE TABLE usage_daily (
    date TEXT NOT NULL, provider_id TEXT NOT NULL, model TEXT NOT NULL,
    input_tokens INTEGER DEFAULT 0, output_tokens INTEGER DEFAULT 0,
    request_count INTEGER DEFAULT 0, cost_est REAL DEFAULT 0.0,
    PRIMARY KEY (date, provider_id, model)
);

CREATE TABLE skills (
    id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, description TEXT DEFAULT '',
    tags TEXT DEFAULT '[]', file_path TEXT NOT NULL, source TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1, created_at INTEGER NOT NULL
);

CREATE TABLE plugins (
    id TEXT PRIMARY KEY, version TEXT DEFAULT '', description TEXT DEFAULT '',
    source_url TEXT DEFAULT '', status TEXT DEFAULT 'stopped', pid INTEGER DEFAULT 0,
    installed_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
```

---

## 3. API 端点

### 3.1 Provider

```
GET    /api/providers
GET    /api/providers/presets
POST   /api/providers
POST   /api/providers/from-preset
GET    /api/providers/:id
PUT    /api/providers/:id
DELETE /api/providers/:id
POST   /api/providers/:id/switch
POST   /api/providers/:id/probe
GET    /api/providers/:id/backends
POST   /api/providers/:id/backends
PUT    /api/providers/:id/backends/:label
DELETE /api/providers/:id/backends/:label
POST   /api/providers/:id/backends/probe
```

### 3.2 Agent

```
GET    /api/agents
POST   /api/agents
GET    /api/agents/:name
PUT    /api/agents/:name
DELETE /api/agents/:name
POST   /api/agents/copy
GET    /api/agents/:name/soul
PUT    /api/agents/:name/soul
GET    /api/agents/:name/memory
PUT    /api/agents/:name/memory
GET    /api/agents/:name/sessions
```

### 3.3 Chat / Session

```
POST   /api/chat
GET    /api/sessions
GET    /api/sessions/:id/messages
DELETE /api/sessions/:id
```

### 3.4 Tasks

```
GET    /api/tasks
POST   /api/tasks
PUT    /api/tasks/:id
DELETE /api/tasks/:id
POST   /api/tasks/:id/execute
```

### 3.5 Tools / MCP

```
GET    /api/tools
PUT    /api/tools/:name
GET    /api/mcp
POST   /api/mcp
PUT    /api/mcp/:name
DELETE /api/mcp/:name
```

### 3.6 Jobs

```
GET    /api/jobs
POST   /api/jobs
PUT    /api/jobs/:id
DELETE /api/jobs/:id
POST   /api/jobs/:id/run
GET    /api/jobs/:id/logs
```

### 3.7 Usage

```
GET    /api/usage
GET    /api/usage/summary
```

### 3.8 Plugins

```
GET    /api/plugins
POST   /api/plugins/install
POST   /api/plugins/:id/start
POST   /api/plugins/:id/stop
DELETE /api/plugins/:id
```

### 3.9 System

```
GET    /api/health
GET    /api/db/stats
GET    /api/config
PUT    /api/config
```

### 3.10 WebSocket

```
WS     /ws
```

---

## 4. WebSocket 协议

### 4.1 Client → Server

```
{ "type": "chat",     "session_id": "...", "agent_name": "...", "message": "..." }
{ "type": "stop",     "session_id": "..." }
{ "type": "approve",  "check_id": N, "approved": true|false }
```

### 4.2 Server → Client

```
{ "type": "stream",           "session_id": "...", "chunk": "..." }
{ "type": "tool_call",        "session_id": "...", "tool": "...", "args": {...} }
{ "type": "tool_result",      "session_id": "...", "tool": "...", "result": "..." }
{ "type": "approval_request", "check_id": N, "tool": "...", "args": {...}, "risk": "...", "description": "..." }
{ "type": "done",             "session_id": "..." }
{ "type": "error",            "session_id": "...", "message": "..." }
```

---

## 5. 系统工具清单

| name | category | risk | approval_required | 说明 |
|------|----------|------|-------------------|------|
| read_file | system | low | 0 | 读取文件内容 |
| write_file | system | low | 0 | 创建/覆盖文件 |
| edit_file | system | medium | 0 | 查找替换编辑 |
| grep | system | low | 0 | 内容搜索 |
| ls | system | low | 0 | 目录浏览 |
| shell | optional | high | 1 | 执行 Shell 命令 |
| git | optional | medium | 0 | Git 操作 |
| web_fetch | optional | medium | 0 | HTTP 请求 |
| git_worktree | optional | medium | 0 | Git 并行工作树 |
| image_gen | optional | low | 0 | AI 图片生成 |
| browser | optional | high | 1 | Playwright 浏览器自动化 |
| code_review | optional | low | 0 | 代码审查 |

---

## 6. 前端组件

### 6.1 页面

| 路由 | 组件 | 功能 |
|------|------|------|
| `/chat` | ChatPage | 对话 + Steer 管线 + 审批 |
| `/agents` | AgentsPage | Agent 列表 + 详情 + Copy |
| `/providers` | ProvidersPage | Provider 卡片网格 + 预设 + Backend 管理 |
| `/tools` | ToolsPage | 全局 Tool 开关 + MCP 服务器管理 |
| `/scheduled` | ScheduledPage | 定时任务 CRUD + 执行历史 |
| `/plugins` | PluginsPage | 插件列表 + 安装/启停 |
| `/usage` | UsagePage | 用量图表 Dashboard |
| `/settings` | SettingsPage | 通用设置 + Import/Export + DB 状态 |

### 6.2 组件树

```
App
├── TitleBar
├── LeftSidebar
│   ├── NavMenu
│   └── SessionList
├── PageContent
│   ├── ChatPage
│   │   ├── MessageList
│   │   ├── InputArea
│   │   ├── AgentSelector
│   │   ├── SteerPipeline
│   │   └── ApprovalModal
│   ├── AgentsPage
│   │   ├── AgentCard
│   │   ├── AgentDetail
│   │   └── AgentCopyDialog
│   ├── ProvidersPage
│   │   ├── ProviderCard
│   │   ├── PresetSelector
│   │   ├── ProviderDetail
│   │   └── BackendManager
│   ├── ToolsPage
│   │   ├── ToolList
│   │   └── MCPServerList
│   ├── ScheduledPage
│   │   ├── JobTable
│   │   └── JobLogPanel
│   ├── PluginsPage
│   │   ├── PluginList
│   │   └── InstallDialog
│   ├── UsagePage
│   │   └── UsageDashboard
│   └── SettingsPage
│       ├── GeneralSettings
│       ├── ImportExport
│       └── DBStatus
└── RightPanel
    ├── ReviewTab
    ├── TerminalTab
    ├── BrowserTab
    ├── FilesTab
    └── SideTasksTab
```

### 6.3 设计约束

- 主色调：`#5e6ad2`
- 字体：`Inter, system-ui, -apple-system, sans-serif`
- 圆角：`6px`
- ProviderCard 必须展示：图标、名称、分类标签、Backend 健康摘要、操作按钮
- HealthBadge 颜色：healthy=green, degraded=orange, unhealthy=red, unknown=grey
- 桌面 Provider 网格 >= 3 列

---

## 7. 配置目录

```
~/.codex/
├── codex.db           # SQLite 核心数据库
├── codex.db-wal       # WAL 日志
├── codex.db-shm       # 共享内存
├── config.yaml        # 启动级配置
├── agents/            # Agent 文件
│   └── <name>/
│       ├── soul.md
│       ├── rules/
│       └── skills/
├── skills/            # 全局技能
├── specs/             # SPEC/PLAN 文档
└── logs/              # 运行日志
```
