# Codex Go — 规格文档

> 本文档定义 Codex Go 的架构契约、行为规则和校验标准。
> 术语定义见 [GLOSSARY.md](./GLOSSARY.md)，实体定义见 [ENTITIES.md](./ENTITIES.md)。

---

## 1. 系统概述

Codex Go 是基于 OpenAI Codex CLI 用 Golang 重写的 AI 编程智能体，支持 CLI 和 Web 双模式，采用 SQLite 统一持久化。

---

## 2. 架构契约

### 2.1 分层

```
Frontend (React + Antd)     ← HTTP/WS 通信
─────────── REST + WebSocket ───────────
API Layer (net/http)         ← 路由 + Handler
Agent Engine                 ← Agent Manager + Tool Registry
Provider Proxy               ← Pool + CircuitBreaker + ProtocolAdapter
SQLite Store                 ← 所有持久化
```

### 2.2 数据流

1. 用户输入 → `POST /api/chat` → AgentManager 选择 Agent
2. Agent 从 Store 加载配置（agents 表 + soul.md）
3. Agent 从 Store 装配工具（tools 表 × tools_mode）
4. Agent 从 Store 获取当前 Provider
5. ProviderPool 选择 Backend（策略 + 健康 + 熔断）
6. API 调用 → 流式响应 → WS push → 前端渲染
7. 写入 usage_logs + messages

### 2.3 模块依赖

- `internal/store` — 零内部依赖，仅依赖 database/sql + go-sqlite3
- `internal/config` — 依赖 store
- `internal/agent` — 依赖 store + provider + tool + mcp
- `internal/provider` — 依赖 store
- `internal/api` — 依赖 agent + store + session + schedule + skill + plugin
- 禁止循环依赖

### 2.4 数据库配置

- 默认路径：`~/.codex/codex.db`
- CLI 覆盖：`--db <path>`
- 必须开启 WAL 模式（`_journal_mode=WAL`）
- 必须开启外键（`_foreign_keys=on`）
- 最大连接数：1
- 完整 Schema 见 [ENTITIES.md §2](./ENTITIES.md#2-数据库-ddl)

---

## 3. 行为规则

### 3.1 Provider 管理

- Provider 的 `is_current` 字段全局唯一（最多一行 = 1）
- 切换 Provider（`POST /api/providers/:id/switch`）必须是原子操作：先 `UPDATE SET is_current=0`，再 `UPDATE SET is_current=1 WHERE id=?`
- 删除 Provider 必须级联删除其 backends 和 usage_logs
- Provider API 完整端点见 [ENTITIES.md §3.1](./ENTITIES.md#31-provider)

### 3.2 负载均衡

三种策略，必须在 ProviderPool 中实现：

- **fill_first**：按 weight 降序排列，始终选择第一个 healthy backend，不可用时依次尝试下一个
- **round_robin**：跨请求维护轮转计数器，按序选择
- **random**：按 weight 加权随机

### 3.3 健康探测与降级

- 定期探测各 Backend 的 `/models` 端点
- 连续失败 >= 5 次 → `health_status = unhealthy`
- unhealthy backend 不得被选中参与请求
- 探测成功 → `health_status = healthy`，自动恢复参与
- 探测时间记录于 `last_probe_at`

### 3.4 CircuitBreaker

每个 Backend 必须具备三态熔断器：

| 状态 | 行为 | 进入条件 |
|------|------|---------|
| Closed | 正常放行 | 初始状态；HalfOpen 探测成功 |
| Open | 拒绝所有请求 | 连续失败 >= 阈值（5 次） |
| HalfOpen | 放行 1 次探测 | 冷却时间到达（30 秒） |

HalfOpen 探测成功 → Closed；失败 → Open（重新计时）。

### 3.5 故障转移

- 当前 Provider 所有 backend 均为 unhealthy 时触发
- 按 `in_failover_queue = true` 的 Provider 列表顺序，依次尝试
- 任意 Provider 有可用 Backend 即停止
- 切换需记录日志
- 原 Provider 恢复后不自动切回

### 3.6 协议适配

代理层必须支持：
- OpenAI Chat Completions ↔ OpenAI Responses 双向转换
- SSE 事件格式统一归一到内部格式
- 模型别名映射（如 `gpt-5.5` ↔ `gpt-5.5-proxy`）

---

## 4. Agent 规则

### 4.1 文件 + 数据库混合存储

| 内容 | 存储位置 |
|------|---------|
| 配置 | agents 表 |
| 人格 | `~/.codex/agents/<name>/soul.md` |
| 规则 | `~/.codex/agents/<name>/rules/*.md` |
| 专属技能 | `~/.codex/agents/<name>/skills/*.md` |
| 长期记忆 | agent_memory 表（按 agent_name 隔离） |
| 对话历史 | sessions + messages 表（按 agent_name 隔离） |

### 4.2 Agent 隔离

- sessions 表通过 `agent_name` 过滤，每个 Agent 仅可见自己的会话
- agent_memory 表通过 `agent_name` 过滤，每个 Agent 仅可访问自己的记忆
- 删除 Agent 时，必须级联删除：sessions + messages + agent_memory + 文件目录

### 4.3 工具/MCP/Skills 装配

全局资源存储在 `tools`、`mcp_servers`、`skills` 表，Agent 通过 mode 字段选择：

| mode | 行为 |
|------|------|
| `all` | 加载全部 enabled 资源 |
| `custom` | 仅加载 `*_list` 中指定的 |
| `none` | 不加载（仅 MCP 和 Skills 可用，Tools 至少含 system 工具） |

`category = system` 的工具必须始终加载，不受 tools_mode 和 tools_list 影响。

### 4.4 soul.md 注入

运行时必须将 soul.md 内容拼接到 system_prompt 头部，格式：

```
[Agent Soul]
<soul.md 内容>

[System]
<system_prompt>
```

### 4.5 Agent Copy

- 必须复制：agents 表记录（name 重命名）、soul.md、rules/、skills/
- 禁止复制：agent_memory 记录、sessions/messages 记录

### 4.6 Agent API

完整端点见 [ENTITIES.md §3.2](./ENTITIES.md#32-agent)。

---

## 5. 会话与审批规则

### 5.1 WebSocket 协议

协议字段必须与 [ENTITIES.md §4](./ENTITIES.md#4-websocket-协议) 严格一致。

### 5.2 审批

- `tools.approval_required = 1` 的工具调用前，必须通过 WS 推送 `approval_request`
- 审批超时 60 秒自动拒绝
- 同一会话中相同工具 + 相同参数的审批可以缓存（白名单机制）

---

## 6. 校验标准

所有实现必须通过以下验证方可合入。

### 6.1 构建

- `make build` 必须零错误
- `go vet ./...` 必须零 warning

### 6.2 数据库

- 必须开启 SQLite WAL 模式（PRAGMA journal_mode = wal）
- 必须开启外键约束（PRAGMA foreign_keys = ON）
- 所有表必须使用 `CREATE TABLE IF NOT EXISTS`
- 所有外键必须使用 `ON DELETE CASCADE`
- `backends.api_key` 必须以 AES-256 加密存储，禁止明文落盘
- `schema_version` 表必须正确追踪当前版本号
- 种子数据（tools / provider_presets / default agent）必须在首次启动时正确填充

### 6.3 API

- 所有端点必须返回 `Content-Type: application/json`
- 错误响应格式必须统一为 `{ "error": "..." }` + 对应 HTTP 4xx/5xx
- `GET /api/providers` 响应必须包含每个 provider 的 `backend_count` 和 `healthy_count`
- `POST /api/providers/:id/switch` 必须是原子操作
- `DELETE /api/providers/:id` 必须级联删除关联 backends 和 usage_logs
- `POST /api/agents/copy` 禁止复制 agent_memory 和 sessions
- `DELETE /api/agents/:name` 必须级联删除 sessions/messages/agent_memory 及文件目录
- WebSocket 消息格式必须与 [ENTITIES.md §4](./ENTITIES.md#4-websocket-协议) 一致

### 6.4 Agent 装配

- `tools_mode = all` 时，必须加载 `tools` 表中所有 `enabled = 1` 的工具
- `tools_mode = custom` 时，必须仅加载 `tools_list` 中指定的工具
- `category = system` 的工具必须始终加载
- `mcp_mode = none` 时，禁止初始化任何 MCP 客户端
- soul.md 内容必须按 §4.4 格式拼接到 system_prompt

### 6.5 Provider 代理

- fill_first / round_robin / random 策略必须行为正确
- 连续失败 5 次后，backend 必须标记为 unhealthy
- unhealthy backend 不得被选中
- 健康探测成功后，health_status 必须恢复为 healthy
- CircuitBreaker 三态转换必须正确
- 当前 Provider 全 unhealthy 时必须触发故障转移

### 6.6 前端

- ProviderCard 必须展示图标、名称、分类标签、Backend 健康摘要
- Agent 列表必须展示 session_count
- Agent Copy 必须正确创建独立副本
- 明/暗主题切换必须即时生效
- WebSocket 流式消息必须实时渲染
- 审批弹窗必须正确展示 tool/args/risk/description 并正确响应
- Steer 管线必须正确可视化

### 6.7 CLI

- `--db <path>` 参数必须覆盖默认数据库路径
- `codex-go agent copy <source> <target>` 必须成功
- `codex-go provider import/export` 必须正确
- 交互模式下所有内置命令必须可用
- 管道模式必须正确读取 stdin

### 6.8 安全

- API Key 在日志和 API 响应中必须脱敏（前 3 位 + 后 4 位）
- `codex.db` 文件权限必须为 0600
- `approval_required = 1` 的工具必须经审批方可执行
- 审批超时 60 秒必须自动拒绝

---

## 7. 技术选型

| 层级 | 选型 |
|------|------|
| 语言 | Go >= 1.21 |
| 数据库驱动 | mattn/go-sqlite3 |
| HTTP 路由 | net/http (stdlib) |
| WebSocket | gorilla/websocket |
| YAML 解析 | gopkg.in/yaml.v3 |
| 前端框架 | React 18 |
| UI 库 | Ant Design 5 |
| 构建工具 | Vite 5 |
