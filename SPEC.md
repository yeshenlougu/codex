# Codex Go — Full-Stack AI Coding Agent

一个 Electron 桌面应用：主窗口是 React Web UI，桌宠是独立透明窗口。
内置 cc-switch 多端点池，无需外部代理。

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
│  Agent Core · 8 Tools · Multi-Backend Pool · Skills│
│  Built-in cc-switch (health checks + failover)     │
└────────────────────────────────────────────────────┘
```

## 技术栈

Electron · React 18 + TypeScript + Vite + Tailwind · Go 1.22 · JSON 存储

## 目录结构

```
/app/codex/               Go 后端
├── cmd/codex/main.go     CLI + serve + provider 子命令
├── internal/
│   ├── agent/agent.go    Agent 循环 + 后端重试
│   ├── api/              REST + WebSocket + backends 健康面板
│   ├── provider/         OpenAI/Anthropic/Ollama + 多端点池(Pool)
│   ├── tool/             8 个工具
│   ├── session/          JSON 持久化
│   ├── skill/            98 技能加载器
│   ├── config/           YAML + env（支持 backends 配置）
│   ├── ccswitch/         cc-switch 配置导入/导出
│   ├── mcp/              MCP stdio JSON-RPC 客户端
│   ├── sandbox/          bubblewrap 沙箱 + 审批
│   ├── plugin/           JSON 插件系统
│   └── update/           GitHub Release 自动更新

/app/codex/web/           React（Vite 构建 → Go 静态服务）
/app/codex/desktop/       Electron 桌面壳
├── main.js               主进程（双窗口 + tray + 快捷键）
├── pet.html              Canvas 像素宠物
└── package.json
```

## 内置 cc-switch（多端点池）

无需启动外部 cc-switch 代理。配置多后端后，Codex 自动：

- 按策略路由请求（round_robin / random / fill_first）
- 健康检查：每 30s 探测不可用后端
- 故障转移：5xx/超时/限流自动切换后端，4xx 认证错误不重试
- 权重路由：高权重后端获得更多请求
- 冷却恢复：连续失败进入 cooldown（30s~5min），到期自动探活
- 健康面板：GET /api/backends 查看所有后端状态

### 配置示例

```yaml
provider:
  pool_strategy: round_robin
  backends:
    - key: sk-xxx
      label: "opencode-key1"
      base_url: https://opencode.ai/zen/go/v1
      weight: 2
    - key: sk-yyy
      label: "opencode-key2"
      base_url: https://opencode.ai/zen/go/v1
      weight: 1
```

### cc-switch 导入/导出

```bash
# 从 cc-switch 配置一键导入
codex-go provider import cc-switch.yaml

# 导出为 cc-switch 格式
codex-go provider export cc-switch.yaml

# 查看后端状态
codex-go provider status
```

## 路线图

### Phase 1 ✅ Go 后端
8 tools · key pool · JSON sessions · 98 skills · REST API · WebSocket · SSE streaming

### Phase 2 ✅ 前端 + 桌宠
React + Tailwind · xterm.js · Chat/Input/Sidebar/Settings · Electron 双窗口 · Canvas 像素宠物

### Phase 3 ✅ 平台能力
Anthropic/Ollama providers · MCP 客户端 · bubblewrap 沙箱 · 三级审批

### Phase 4 ✅ 文件浏览 + 更新 + 插件
FileTree/FileViewer/Diff · GitHub Release 自动更新 · 上下文压缩 · JSON 插件系统

### Phase 5 ✅ 内置 cc-switch
多端点池 · 健康检查 · 自动故障转移 · 权重路由 · cc-switch 配置导入/导出 · 健康面板 API
