# Codex Go — Full-Stack AI Coding Agent

## 项目目标

基于 `openai/codex` 的完整功能，用 **Go + Node/React** 重写，并增加 **Pet（桌面宠物）** 系统。

## 技术栈

| 层 | 技术 | 说明 |
|----|------|------|
| **Agent 引擎** | Go 1.22+ | 核心循环、工具执行、provider 通信 |
| **API 服务** | Go net/http + gorilla/websocket | REST + WebSocket，供前端和 CLI 共用 |
| **前端** | React 18 + TypeScript + Vite | Web 聊天界面 |
| **UI 框架** | Tailwind CSS + Radix UI | 样式和组件 |
| **Pet 引擎** | Canvas/WebGL + 帧动画 | 桌面宠物渲染 |
| **CLI** | Go（同后端代码） | `codex-go` 命令行入口 |
| **数据** | SQLite + 文件系统 | 会话、配置、技能、记忆 |

## 架构

```
┌─────────────────────────────────────────────────────┐
│                  React Frontend                      │
│  http://localhost:1977                               │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐  │
│  │ 💬 Chat │ │ 📁 Files │ │ ⚡ Shell │ │ 🐱 Pet │  │
│  │  Panel  │ │  Browser │ │ Terminal │ │ Panel  │  │
│  └────┬────┘ └────┬─────┘ └────┬─────┘ └───┬────┘  │
│       └───────────┴────────────┴───────────┘        │
│               WebSocket (streaming)                  │
│               REST API (commands)                    │
└───────────────────────┬─────────────────────────────┘
                        │
┌───────────────────────┴─────────────────────────────┐
│                  Go Backend                           │
│  :1977 (API)  /  :1978 (WS)                          │
│  ┌──────────────────────────────────────────────┐   │
│  │  api/          HTTP + WebSocket handlers      │   │
│  │  agent/        Core loop + message routing    │   │
│  │  provider/     LLM clients (OpenAI,Anthr,..)  │   │
│  │  tool/         Tool registry + implementations │   │
│  │  session/      Conversation persistence       │   │
│  │  skill/        Skill loader + registry        │   │
│  │  mcp/          MCP client/server              │   │
│  │  sandbox/      Execution isolation            │   │
│  │  pet/          Pet state + animation frames   │   │
│  │  config/       Configuration management       │   │
│  └──────────────────────────────────────────────┘   │
│  CLI: codex-go (same binary)                         │
└─────────────────────────────────────────────────────┘

```

## 目录结构

```
/app/codex/                          # Go 后端 + CLI
├── cmd/codex/main.go               # CLI 入口
├── internal/
│   ├── agent/agent.go              # Agent 核心循环
│   ├── api/                        # HTTP/WebSocket API
│   │   ├── server.go               # 服务器启动
│   │   ├── handler_chat.go         # 聊天端点
│   │   ├── handler_session.go      # 会话 CRUD
│   │   ├── handler_config.go       # 配置端点
│   │   ├── handler_pet.go          # 宠物状态端点
│   │   └── ws.go                   # WebSocket 管理
│   ├── provider/                   # LLM 客户端
│   │   ├── openai.go               # OpenAI 兼容
│   │   ├── anthropic.go            # Anthropic Messages
│   │   ├── ollama.go               # Ollama 本地
│   │   └── pool.go                 # 多 key 池
│   ├── tool/                       # 工具系统
│   │   ├── tool.go                 # 接口 + Registry
│   │   ├── shell.go                # Shell 执行
│   │   ├── file.go                 # 文件读写/搜索
│   │   ├── git.go                  # Git 操作
│   │   ├── web.go                  # Web 抓取/搜索
│   │   └── pet.go                  # 宠物控制工具
│   ├── session/                    # 会话持久化
│   │   └── store.go                # SQLite + JSON
│   ├── skill/                      # 技能系统
│   │   ├── loader.go               # SKILL.md 解析
│   │   └── registry.go             # 技能注册表
│   ├── mcp/                        # MCP 协议
│   │   ├── client.go               # MCP 客户端
│   │   └── server.go               # MCP 服务端
│   ├── sandbox/                    # 沙箱执行
│   │   └── bwrap.go                # bubblewrap 封装
│   ├── pet/                        # 宠物系统
│   │   ├── state.go                # 宠物状态机
│   │   └── assets.go               # 动画素材管理
│   └── config/config.go            # 配置管理

/app/codex-frontend/                # React 前端
├── src/
│   ├── App.tsx                     # 主入口
│   ├── components/
│   │   ├── Chat/                   # 聊天面板
│   │   │   ├── ChatPanel.tsx       # 消息列表
│   │   │   ├── MessageBubble.tsx   # 消息气泡
│   │   │   ├── StreamingText.tsx   # 流式文本
│   │   │   ├── ToolCallCard.tsx    # 工具调用卡片
│   │   │   └── InputBar.tsx        # 输入栏
│   │   ├── Files/                  # 文件浏览器
│   │   │   ├── FileTree.tsx        # 文件树
│   │   │   └── FileViewer.tsx      # 文件查看/编辑
│   │   ├── Shell/                  # 终端面板
│   │   │   └── Terminal.tsx        # Xterm.js 终端
│   │   ├── Pet/                    # 宠物系统
│   │   │   ├── PetCanvas.tsx       # Canvas 渲染
│   │   │   ├── PetSprite.tsx       # 精灵动画
│   │   │   ├── PetMenu.tsx         # 宠物菜单
│   │   │   └── usePetState.ts      # 状态 hook
│   │   ├── Settings/               # 设置面板
│   │   │   ├── SettingsPanel.tsx
│   │   │   ├── ProviderConfig.tsx
│   │   │   └── PetConfig.tsx
│   │   └── Layout/                 # 布局
│   │       ├── AppShell.tsx        # 主布局
│   │       └── Sidebar.tsx         # 侧边栏
│   ├── hooks/
│   │   ├── useWebSocket.ts         # WS 连接
│   │   ├── useAgent.ts             # Agent 交互
│   │   └── useSessions.ts          # 会话管理
│   ├── lib/
│   │   ├── api.ts                  # REST 客户端
│   │   └── types.ts                # TypeScript 类型
│   └── assets/
│       └── pets/                   # 宠物素材
│           ├── cat/                # 猫咪
│           ├── dog/                # 狗狗
│           └── fox/                # 狐狸
├── package.json
├── vite.config.ts
└── tailwind.config.ts
```

---

## 功能对照表：Codex 原版 → Go Codex

### A. 核心 Agent — 已部分实现，需扩展

| 功能 | 状态 | 说明 |
|------|------|------|
| think→act→observe 循环 | ✅ Go | 已实现 |
| 流式响应 | ✅ Go | SSE streaming |
| 工具调用 + 结果注入 | ✅ Go | OpenAI tool calling |
| 上下文压缩 | ❌ | Token 超限时自动压缩历史 |
| 多模态输入 | ❌ | 图片/音频输入 |
| 并行工具执行 | ❌ | 多工具同时调用 |

### B. LLM Provider — 需扩展为多 Provider

| Provider | 状态 | 说明 |
|----------|------|------|
| OpenAI 兼容 | ✅ Go | 已实现，支持任意兼容 API |
| Anthropic Messages | ❌ | Claude 系列 |
| Ollama 本地 | ❌ | 本地开源模型 |
| LM Studio | ❌ | 本地 GUI 模型管理 |
| 多 key 池 | ❌ | 轮换 + 故障转移 |

### C. 工具系统 — 需大量扩展

| 工具 | 状态 | 说明 |
|------|------|------|
| `shell` | ✅ Go | 执行 bash 命令 |
| `read_file` | ✅ Go | 读文件 + 行号 |
| `edit_file` | ✅ Go | 查找替换编辑 |
| `write_file` | ❌ | 创建/覆盖文件 |
| `grep` | ❌ | 正则内容搜索 |
| `ls` | ❌ | 目录列表 |
| `git` | ❌ | Git 操作 |
| `web_fetch` | ❌ | HTTP 抓取 |
| `web_search` | ❌ | 网络搜索（集成搜索引擎） |
| `pet_action` | ❌ | 宠物控制（新增） |
| 文件监听 | ❌ | fsnotify 实时变化 |
| 动态工具注册 | ❌ | 插件热加载 |

### D. 会话管理 — 全部待做

| 功能 | 状态 | 说明 |
|------|------|------|
| 会话 CRUD | ❌ | 新建/保存/加载/删除 |
| 会话列表 | ❌ | 历史浏览 |
| 会话搜索 | ❌ | 全文搜索 |
| 会话导出 | ❌ | Markdown/JSON 导出 |
| 自动保存 | ❌ | 每轮对话自动存盘 |

### E. API 服务器 — 全部待做

| 功能 | 状态 | 说明 |
|------|------|------|
| HTTP REST API | ❌ | 会话 CRUD、配置、模型列表 |
| WebSocket | ❌ | 流式消息推送 |
| 多客户端 | ❌ | 同时连接多个前端 |
| 守护进程模式 | ❌ | 后台运行 |
| 自动更新 | ❌ | 版本检查 + 升级 |

### F. 技能系统 — 全部待做

| 功能 | 状态 | 说明 |
|------|------|------|
| SKILL.md 解析 | ❌ | 读取 YAML frontmatter + Markdown |
| 技能注册表 | ❌ | 加载 ~/.codex/skills/ 下所有技能 |
| 用户调用 | ❌ | /skill-name 触发 |
| 模型调用 | ❌ | LLM 自动选择匹配技能 |
| 技能市场 | ❌ | 安装/更新/卸载 |

### G. MCP 协议 — 待做

| 功能 | 状态 | 说明 |
|------|------|------|
| MCP 客户端 | ❌ | 连接外部 MCP 服务器 |
| MCP 服务端 | ❌ | 暴露 Codex 为 MCP 服务 |
| 工具自动发现 | ❌ | 列出远程工具 |
| OAuth 认证 | ❌ | MCP 授权流程 |

### H. 沙箱执行 — 待做

| 功能 | 状态 | 说明 |
|------|------|------|
| bubblewrap 隔离 | ❌ | Linux 容器化 |
| 文件系统限制 | ❌ | 只读/读写分区 |
| 网络限制 | ❌ | 允许/禁止外网 |
| 审批确认 | ❌ | 敏感操作弹窗 |

### I. 前端界面 — 全部待做

| 面板 | 状态 | 说明 |
|------|------|------|
| 聊天面板 | ❌ | 消息流 + 流式渲染 |
| 文件浏览器 | ❌ | 文件树 + 内容查看 |
| 终端面板 | ❌ | Xterm.js 嵌入式终端 |
| 设置面板 | ❌ | 模型/Provider/Key 配置 |
| Diff 查看器 | ❌ | 代码变更对比 |
| 宠物面板 | ❌ | 动画 + 交互 |

### J. 宠物系统 — 全新功能

| 功能 | 状态 | 说明 |
|------|------|------|
| 宠物渲染 | ❌ | Canvas 帧动画 |
| 宠物选择 | ❌ | 多种宠物切换 |
| 宠物互动 | ❌ | 点击/拖拽/喂食 |
| 宠物状态 | ❌ | 心情/饥饿/动作 |
| 宠物提示 | ❌ | Agent 状态提示（思考中/执行中/完成） |
| 宠物市场 | ❌ | 下载/分享宠物 |

### K. 登录认证

| 功能 | 状态 | 说明 |
|------|------|------|
| API Key | ✅ | 环境变量/配置文件 |
| OAuth (ChatGPT) | ❌ | OpenAI 账号登录 |
| 多账号 | ❌ | 切换 profile |

---

## 分阶段路线图

### Phase 1 — 后端夯实（当前）
**目标：Go 后端成为功能完整的 Agent，CLI 可用，API 就绪**

- 会话持久化（SQLite）
- 扩展工具：write_file、grep、ls、git、web_fetch
- 多 key 池 + 故障转移
- HTTP API（会话 CRUD、chat 端点）
- WebSocket 流式推送
- 技能加载器（读取 ~/.codex/skills/ 下的 SKILL.md）
- CLI 完善：--resume、--list、config 子命令

### Phase 2 — 前端 MVP
**目标：React 聊天界面可用，宠物出现**

- Vite + React + TypeScript 项目初始化
- 聊天面板（消息流、流式渲染、工具调用卡片）
- WebSocket 实时通信
- 宠物渲染引擎（Canvas 帧动画）
- 第一个宠物角色（猫咪）
- 宠物状态机（思考中→工作中→空闲→睡觉）
- 设置面板（API 配置）
- 终端面板（xterm.js）

### Phase 3 — 平台能力
**目标：生态接入，安全执行**

- MCP 客户端（连接外部 MCP 服务）
- MCP 服务端（暴露 Codex 工具）
- bubblewrap 沙箱
- 审批确认弹窗
- Anthropic provider
- Ollama 本地模型

### Phase 4 — 完整体验
**目标：对标原版 Codex + 超越**

- 文件浏览器 + Diff 查看器
- 多宠物角色
- 宠物互动（点击、拖拽、喂食）
- 技能市场
- 上下文自动压缩
- 多模态输入
- 插件系统
- 守护进程 + 自动更新
- 会话搜索 + 导出
- 深色/浅色主题

---

## 当前仓库状态

```
分支: golang
最后提交: 8bbf15c "chore: add skills/ to .gitignore"
状态: CLI Agent 骨架可用（1000行 Go），前端/API/skills 均未做
技能: 34 个从 yeshenlougu/skills 已安装到 ~/.codex/skills/
```
