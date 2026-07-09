# Codex Go — Full-Stack AI Coding Agent + Desktop Pet

## 项目目标

基于 `openai/codex` 的完整功能，用 **Go + Node/React + Electron** 重写，并增加 **Desktop Pet（桌面宠物）** 系统。

## 技术栈

后端: Go 1.22+ · net/http · gorilla/websocket
Web 前端: React 18 + TypeScript + Vite + Tailwind CSS
桌宠: Electron（独立透明桌面窗口，Canvas 像素动画）
数据: JSON 文件（会话持久化）
LLM: OpenAI 兼容 API（多 key 池 + 故障转移）

## 架构

```
┌──────────────────────────────────────────────────┐
│  Desktop Pet (Electron)                           │
│  独立透明窗口 · Canvas 像素动画 · 状态机驱动       │
│  polling http://localhost:1977/api/pet-state      │
└───────────────────────┬──────────────────────────┘
                        │ HTTP polling
┌───────────────────────┴──────────────────────────┐
│  Go Backend :1977                                 │
│  api/     HTTP + WebSocket handlers               │
│  agent/   think→act→observe 循环                  │
│  provider/  OpenAI client + 多 key 池             │
│  tool/     shell/read/write/grep/ls/git/web_fetch │
│  session/  JSON 文件持久化                        │
│  skill/    34 个技能自动加载                      │
│  config/   YAML + env                             │
└───────────────────────┬──────────────────────────┘
                        │ REST + WebSocket
┌───────────────────────┴──────────────────────────┐
│  React Web UI                                     │
│  💬 Chat Panel · 📁 Files · ⚡ Terminal · ⚙️ Settings│
└──────────────────────────────────────────────────┘
```

## 目录结构

```
/app/codex/                  Go 后端 + CLI
├── cmd/codex/main.go        CLI 入口 (+ serve 模式)
├── internal/
│   ├── agent/agent.go       Agent 核心循环
│   ├── api/                 HTTP/WebSocket API
│   │   ├── server.go        路由 + CORS
│   │   ├── handler_chat.go  聊天 + SSE 流式
│   │   ├── handler_session.go  会话 CRUD
│   │   ├── handler_pet.go   Pet 状态端点
│   │   └── ws.go            WebSocket hub
│   ├── provider/
│   │   ├── openai.go        OpenAI 兼容流式客户端
│   │   └── pool.go          多 key 池 (fill_first/round_robin)
│   ├── tool/
│   │   ├── tool.go          接口 + Registry
│   │   ├── tools.go         shell/read_file/edit_file
│   │   ├── extended.go      write_file/grep/ls/git/web_fetch
│   │   └── helpers.go       文件辅助
│   ├── session/store.go     JSON 文件会话持久化
│   ├── skill/registry.go    技能加载 + YAML frontmatter 解析
│   └── config/config.go     YAML 配置 + 环境变量
│
/app/codex-frontend/          React Web 界面
├── src/
│   ├── App.tsx              主应用
│   ├── index.css            Tailwind + 主题
│   ├── lib/types.ts         TypeScript 类型
│   ├── lib/api.ts           REST 客户端
│   ├── hooks/
│   │   ├── useWebSocket.ts  WebSocket 连接
│   │   └── useSessions.ts   会话管理
│   └── components/
│       ├── Layout/           布局 (AppShell, Sidebar)
│       ├── Chat/             聊天 (ChatPanel, MessageBubble, InputBar)
│       ├── Terminal/         终端 (xterm.js)
│       ├── Files/            文件浏览器
│       └── Settings/         设置面板
│
/app/codex-pet/               Electron 桌面宠物
├── main.js                  Electron 主进程 (透明窗口 + tray)
├── pet.html                 Canvas 像素宠物 (猫/狗/狐狸)
└── package.json
```

---

## 分阶段路线图 + 完成状态

### Phase 1 — 后端夯实 ✅ 已完成

- ✅ 会话持久化（JSON 文件，自动保存/列表/恢复/删除）
- ✅ 8 个工具: shell, read_file, edit_file, write_file, grep, ls, git, web_fetch
- ✅ 多 key 池 + 故障转移 (fill_first + round_robin, 5min cooldown)
- ✅ HTTP REST API (/health, /api/chat, /api/sessions, /api/config)
- ✅ WebSocket 流式推送
- ✅ SSE 流式聊天端点
- ✅ 技能加载器（读取 ~/.codex/skills/ 下 SKILL.md，含 YAML frontmatter）
- ✅ CLI 完善 (--resume, --list, --delete, --serve, /history, /clear, /sessions)

### Phase 2 — 前端 MVP + 桌宠 🚧 进行中

- 🚧 Vite + React + TypeScript + Tailwind 项目初始化 ✅
- 🚧 聊天面板（消息流、流式渲染、工具调用卡片）
- 🚧 WebSocket 实时通信
- 🚧 桌面宠物渲染引擎（Electron 独立窗口 + Canvas 像素动画）
- 🚧 宠物状态机（idle / thinking / working / sleeping / eating）
- 🚧 3 个宠物角色（猫 / 狗 / 狐狸）+ 眼睛跟踪
- ❌ 设置面板（API 配置）
- ❌ 终端面板（xterm.js）
- ❌ 文件浏览器

### Phase 3 — 平台能力 ❌

- ❌ MCP 客户端（连接外部 MCP 服务）
- ❌ MCP 服务端（暴露 Codex 工具）
- ❌ bubblewrap 沙箱
- ❌ 审批确认弹窗
- ❌ Anthropic Messages provider
- ❌ Ollama 本地模型 provider

### Phase 4 — 完整体验 ❌

- ❌ 文件浏览器 + Diff 查看器
- ❌ 宠物互动增强（拖拽移动、连续点击反应）
- ❌ 技能市场
- ❌ 上下文自动压缩
- ❌ 多模态输入
- ❌ 插件系统
- ❌ 守护进程 + 自动更新
- ❌ 会话搜索 + 导出

---

## 当前仓库状态

```
分支: golang
最后提交: Phase 1 complete (API server + WebSocket + skills + 8 tools + key pool)
Go 后端: ~2000 行, 编译通过, API 全部端点已测试通过
Web 前端: 项目初始化完成, 依赖安装完成
桌宠: Electron 项目初始化, 主进程 + pet.html Canvas 动画完成
技能: 34 个已安装到 ~/.codex/skills/
```
