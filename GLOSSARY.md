# Codex Go — 术语表

本文档定义 Codex Go 中的核心概念和术语，所有文档引用这些术语时含义一致。

---

## 核心概念

### Provider（供应商）

AI 服务供应商，如 OpenAI、Anthropic 或第三方代理。每个 Provider 包含一套身份信息（名称、图标、分类）和一个或多个后端端点（Backend）。系统中可配置多个 Provider，通过切换激活不同的供应商。

### Backend（后端端点）

Provider 下的具体 API 接入点。一个 Provider 可拥有多个 Backend（如不同区域、不同 key、不同代理），每个 Backend 有独立的 API Key、Base URL、权重和健康状态。Backend 是负载均衡和故障转移的基本单位。

### Agent（智能体）

一个独立的 AI 角色实例，拥有：
- 独立的配置文件（agents 表记录）
- 独立的人格定义（soul.md）
- 独立的对话历史（sessions 表，按 agent_name 隔离）
- 独立的长期记忆（agent_memory 表，按 agent_name 隔离）
- 从全局资源池中选择启用的工具、MCP 和技能

### Session（会话）

一次连续的对话，归属于特定 Agent。包含多条消息，有唯一的会话 ID 和可选标题。会话结束时持久化存储，可恢复继续。

### Message（消息）

会话中的一条对话记录。角色可以是 user（用户）、assistant（AI）、system（系统指令）或 tool（工具调用结果）。每条消息可附带 token 计数和工具调用信息。

### Tool（工具）

Agent 可调用的函数或能力，如执行 Shell 命令、读写文件、Git 操作。分为系统工具（所有 Agent 必备）和可选工具（Agent 选择性启用）。高风险工具需要人工审批。

### MCP Server（MCP 服务器）

实现 Model Context Protocol 的外部工具服务器。Agent 通过 MCP 客户端连接这些服务器，将其提供的工具注册到 Tool Registry。MCP 工具与内置工具在调用层面统一。

### Skill（技能）

以 SKILL.md 格式定义的可复用知识模块，包含特定任务的步骤说明、命令示例和注意事项。兼容 Claude Code 和 Hermes 的 SKILL.md 格式。

### Plugin（插件）

独立进程运行的扩展模块，可通过 Git 仓库或本地路径安装。每个插件有独立的生命周期（启动/停止/重启）、状态追踪和日志收集。

---

## 代理层概念

### Load Balancing（负载均衡）

在多个 Backend 之间分配请求的策略。

- **fill_first**：始终使用优先级最高的 Backend，仅失败时切换
- **round_robin**：按顺序轮询分配
- **random**：按权重加权随机选择

### Circuit Breaker（熔断器）

保护系统免受持续故障 Backend 影响的机制。三态模型：
- **Closed（闭合）**：正常状态，请求通过，累计失败计数
- **Open（断开）**：拒绝所有请求，等待冷却时间
- **Half-Open（半开）**：允许少量探测请求，成功则恢复 Closed，失败则回到 Open

### Failover（故障转移）

当前 Provider 的所有 Backend 均不可用时，自动切换到下一个可用的 Provider。切换依据 `in_failover_queue` 标记和配置的优先级顺序。

### Failover Queue（故障转移队列）

按优先级排列的 Provider 列表。`in_failover_queue = true` 的 Provider 参与队列。当当前 Provider 不可用时，按队列顺序尝试下一个。

### Health Probe（健康探测）

定期向 Backend 的 `/models` 端点发送请求以验证可用性。根据响应将 Backend 标记为 healthy（健康）、degraded（降级）或 unhealthy（不可用）。

### Protocol Adapter（协议适配器）

在不同 AI API 协议之间转换请求和响应的中间层。支持 OpenAI Chat Completions ↔ OpenAI Responses 互转，以及流式 SSE 事件格式的统一。

### Model Mapper（模型映射）

将别名映射为真实模型名的机制。例如用户配置 `gpt-5.5` 但后端实际模型名为 `gpt-5.5-proxy`。

### Media Sanitizer（媒体过滤器）

检测请求中的图片/文件内容，过滤不支持多模态的模型，验证图片格式和大小。

---

## 工作流概念

### SPEC（需求规格）

从自然语言描述生成的结构化需求文档，包含功能清单、差距分析、架构设计、分阶段路线图。

### PLAN（实施计划）

从 SPEC 派生的可执行计划文档，包含任务列表、依赖关系和验收条件。

### Task（任务）

Plan 中的可执行单元。有独立的状态流转（pending → in_progress → completed / cancelled），支持依赖关系和 Agent 分配。

### Steer Mode（引导模式）

前端 ChatPage 中的可视化开发管线，将 spec → plan → tasks → execute 四个阶段以卡片形式展示，支持分步和全自动执行。

### Approval（审批）

高风险工具执行前的人工确认机制。工具标记 `approval_required = 1` 时，必须推送审批请求到前端，用户确认后方可执行。超时自动拒绝。

---

## 持久化概念

### SQLite Database（SQLite 数据库）

系统核心数据存储，位于 `~/.codex/codex.db`。采用 WAL 模式，外键约束开启。所有结构化数据（Provider、Agent、Session、Message、Tool、Task、Job 等）均存于此。

### WAL Mode（WAL 模式）

SQLite 的 Write-Ahead Logging 模式，允许多个读操作与一个写操作并发执行，提升并发性能并增强崩溃恢复能力。

### Migration（迁移）

数据库版本升级机制。通过 `schema_version` 表追踪当前版本，按版本号递增执行迁移脚本（纯 SQL 或 Go 函数）。

### Seed Data（种子数据）

系统首次启动时自动填充的默认数据，包括系统工具定义、供应商预设模板和默认 Agent 配置。

### Legacy Migration（旧格式迁移）

将旧版本（JSON/YAML 文件存储）的数据导入 SQLite 的过程。迁移源：config.yaml → providers/backends 表，sessions/*.json → sessions/messages 表。

### Agent Memory（Agent 记忆）

Agent 的长期记忆，以键值对形式存储在 `agent_memory` 表。跨会话持久化，与 Agent 生命周期绑定。Agent 删除时级联删除。

---

## 数据目录

### ~/.codex/

Codex Go 的数据根目录。

| 路径 | 内容 |
|------|------|
| `~/.codex/codex.db` | SQLite 核心数据库 |
| `~/.codex/config.yaml` | 启动级配置（端口、日志级别等） |
| `~/.codex/agents/<name>/` | Agent 文件目录（soul.md / rules / skills） |
| `~/.codex/skills/` | 全局技能 SKILL.md 文件 |
| `~/.codex/specs/` | SPEC.md / PLAN.md 项目文档 |
| `~/.codex/logs/` | 运行日志 |
