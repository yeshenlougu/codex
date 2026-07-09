# Codex Go — Full-Stack AI Coding Agent

一个 Electron 桌面应用：主窗口是 React Web UI，桌宠是独立透明窗口。

## 架构

```
┌──────────────────────────────────────────────────┐
│  Codex Desktop (Electron)                         │
│                                                    │
│  ┌─────────────────────┐  ┌───────────────────┐  │
│  │  Main Window         │  │  Pet Window        │  │
│  │  React Web UI        │  │  透明 Canvas 像素动画 │  │
│  │  (Chat/Files/Term)   │  │  猫/狗/狐狸         │  │
│  └────────┬────────────┘  └────────┬──────────┘  │
│           │ HTTP + WS              │ HTTP polling  │
│           └───────────┬────────────┘              │
└───────────────────────┼───────────────────────────┘
                        │
┌───────────────────────┴───────────────────────────┐
│  Go Backend :1977                                  │
│  Agent Core · 8 Tools · Key Pool · Skills · API    │
└────────────────────────────────────────────────────┘
```

## 技术栈

Electron · React 18 + TypeScript + Vite + Tailwind · Go 1.22 · JSON 存储

## 目录结构

```
/app/codex/               Go 后端
├── cmd/codex/main.go     CLI + serve 模式
├── internal/
│   ├── agent/agent.go    Agent 循环 + skills
│   ├── api/              REST + WebSocket
│   └── provider/         OpenAI / Anthropic / Ollama
│   └── tool/             8 个工具
│   └── session/          JSON 持久化
│   └── skill/            34 技能加载器
│   └── config/           YAML + env

/app/codex-frontend/       React（Vite 构建 → Go 静态服务）
/app/codex-desktop/        Electron 桌面壳
├── main.js                主进程（双窗口 + tray + 快捷键）
├── pet.html               Canvas 像素宠物
└── package.json
```

## 路线图

### Phase 1 ✅ Go 后端
8 tools · key pool · JSON sessions · 34 skills · REST API · WebSocket · SSE streaming

### Phase 2 🚧 前端 + 桌宠
- ✅ React 项目 + Tailwind (tsc 零错误)
- ✅ Chat/Input/MessageBubble/Sidebar/Settings
- ✅ pet-state 端点 + IsThinking fix
- ✅ Electron 主进程（双窗口架构）
- ✅ Canvas 像素宠物（3 角色 + 状态机 + 眼睛跟踪）
- ❌ 验证 React 构建产物
- ❌ 端到端测试（启动 Go → Electron → 聊天）

### Phase 3 🚧 平台能力
- 🚧 Anthropic Messages provider (代码完成)
- 🚧 Ollama provider (代码完成)
- ❌ MCP 客户端/服务端
- ❌ bubblewrap 沙箱
- ❌ 审批确认

### Phase 4 ❌
文件浏览器 · Diff · 多模态 · 插件 · 会话搜索
