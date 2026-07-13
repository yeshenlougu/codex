# Implementation Plan for MCP/Skill/Plugin Installation & Sidebar Optimization

> Based on SPEC.md sections 12-15

---

## Phase 1: MCP 服务器管理（2天）

- [ ] Task 1.1: 创建 `internal/store/mcp_store.go` — MCP 服务器 JSON 持久化存储（`~/.codex/mcp-servers.json`）— 预计 0.5天
- [ ] Task 1.2: 创建 `internal/api/handler_mcp.go` — MCP CRUD API（GET/POST/PUT/DELETE /api/mcp/servers, restart, presets）— 预计 0.5天
- [ ] Task 1.3: 扩展 `internal/mcp/client.go` — 添加 Start/Stop/Status 方法，支持运行时启停 — 预计 0.5天
- [ ] Task 1.4: 前端 MCP 标签页 — PluginsPage 添加第三个 Tab "MCP"，含列表/添加表单/预设模板选择/状态指示 — 预计 0.5天

## Phase 2: Skill 技能安装（2天）

- [ ] Task 2.1: 创建 `internal/store/skill_store.go` — Skill 商店持久化（`~/.codex/skill-store.json`）— 预计 0.5天
- [ ] Task 2.2: 创建 `internal/skill/installer.go` — GitHub 仓库扫描 + git clone + SKILL.md 解析 — 预计 0.5天
- [ ] Task 2.3: 扩展 `internal/api/handler_skills.go` — install/uninstall/discover/updates/repos API — 预计 0.5天
- [ ] Task 2.4: 前端技能增强 — 已安装列表 + 发现标签 + 仓库管理 + 安装/卸载/更新按钮 — 预计 0.5天

## Phase 3: 插件系统增强（1天）

- [ ] Task 3.1: 创建 `internal/plugin/process.go` — PluginProcess 子进程管理（Start/Stop/Status）— 预计 0.5天
- [ ] Task 3.2: 扩展 `internal/plugin/plugin.go` — 支持 process 类型插件（stdin/stdout JSON-RPC 通信）— 预计 0.25天
- [ ] Task 3.3: 前端插件页优化 — 移除硬编码列表，从 API 获取真实插件；状态监控 — 预计 0.25天

## Phase 4: Sidebar UI 优化（0.5天）

- [ ] Task 4.1: LeftSidebar 优化 — 移除底部"新建任务"按钮，设置固定底部（sticky），项目树嵌套路径支持 — 预计 0.3天
- [ ] Task 4.2: RightPanel 菜单优化 — 改为垂直列表样式（参考原始 Codex），添加键盘快捷键提示 — 预计 0.2天

---

## 验收标准

- [ ] MCP 服务器可通过 UI 添加/编辑/删除，安装后即时注册工具到 Agent
- [ ] 预设 MCP 模板可一键选择并自动填充表单
- [ ] Skill 可从 GitHub 仓库发现和安装
- [ ] Skill 更新检测功能正常，可一键更新
- [ ] Skill 卸载自动备份到 `~/.codex/skill-backups/`
- [ ] 插件安装后即时生效（热加载），显示运行状态
- [ ] 侧边栏"新建任务"按钮已移除
- [ ] 设置按钮固定在底部
- [ ] 项目树支持多级路径展开（如 F:/Project/urbanLifeline/ai-manager）
- [ ] `make build` 通过
- [ ] `go vet ./...` 无新增错误
- [ ] 前端 `npx tsc --noEmit` 无新增错误
