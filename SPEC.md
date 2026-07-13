# Codex Go 多模型架构设计

## 1. 现状

Codex Go 的 Provider 已是 cc-switch 代理池，具备以下能力：

- 多 Backend 权重路由（round_robin / fill_first / random）
- 自动故障转移 + 健康检查
- 健康检查已访问 `GET /models`（只检查状态码，未解析模型列表）

当前只有一个模型维度（chat），所有请求走同一个 Pool。

## 2. 设计原则

不在 Provider 之上另建一层 models.capabilities。Provider 本身就是能力声明——通过其 Backend 的模型列表自动表达支持哪些能力。

## 3. 自动检测模型类型

Pool 健康检查时解析 `/models` 响应，按模型名自动分类。

### 3.1 命名规则分类

| 模型名匹配 | 自动归类 | 说明 |
|---|---|---|
| `gpt-*` `claude-*` `deepseek-*` `gemini-*` `qwen-*` `glm-*` | chat | 所有 LLM 默认支持对话 |
| `gpt-4o` `gpt-5*` `claude-3.5*` `gemini-2*` | chat, vision | 多模态模型额外支持视觉 |
| `gpt-image-*` `dall-e-*` | image_gen | 图片生成专用 |
| `whisper-*` | audio_stt | 语音识别 |
| `tts-*` | audio_tts | 语音合成 |
| `text-embedding-*` | embedding | 向量嵌入 |
| `sora-*` `kling-*` `runway-*` | video_gen | 视频生成 |
| 未匹配 | chat | 默认兜底 |

### 3.2 手动覆盖

BackendConfig 的 models 字段可手动指定类型，覆盖自动检测结果：

```yaml
backends:
  - label: my-backend
    key: sk-xxx
    base_url: https://example.com/v1
    models:
      - name: gpt-5.5
        # type 留空 → 自动检测 → chat
      - name: my-custom-model
        type: image_gen              # 强制指定
        context_length: 131072
```

## 4. 数据结构

### 4.1 Config 层

```go
type BackendConfig struct {
    Key      string            `yaml:"key" json:"key"`
    Label    string            `yaml:"label" json:"label"`
    BaseURL  string            `yaml:"base_url" json:"base_url"`
    Weight   int               `yaml:"weight" json:"weight"`
    Models   []ModelEntry      `yaml:"models,omitempty" json:"models,omitempty"`
    Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

type ModelEntry struct {
    Name          string `yaml:"name" json:"name"`
    Type          string `yaml:"type,omitempty" json:"type,omitempty"`
    ContextLength int    `yaml:"context_length,omitempty" json:"context_length,omitempty"`
}
```

### 4.2 运行时 Pool 层

```go
type PoolEntry struct {
    // 现有字段：Key, Label, BaseURL, Weight, Health...

    // 新增：自动发现 + 配置合并的模型信息
    Models []ModelInfo `json:"models"`
}

type ModelInfo struct {
    Name string    `json:"name"`
    Type ModelType `json:"type"`
}

type ModelType string

const (
    ModelChat      ModelType = "chat"
    ModelVision    ModelType = "vision"
    ModelImageGen  ModelType = "image_gen"
    ModelVideoGen  ModelType = "video_gen"
    ModelAudioSTT  ModelType = "audio_stt"
    ModelAudioTTS  ModelType = "audio_tts"
    ModelEmbedding ModelType = "embedding"
)
```

### 4.3 按能力选择 Backend

```go
// SelectFor 返回支持指定能力类型的最佳 Backend
func (p *Pool) SelectFor(types ...ModelType) (*PoolEntry, ModelInfo, error)

// HasCapability 检查 Pool 是否支持某种能力
func (p *Pool) HasCapability(t ModelType) bool
```

## 5. 运行时路由

### 5.1 Chat 对话

```
用户输入 → agent.Run()
  → pool.SelectFor(ModelChat)
  → 从有 chat 模型的健康 backend 中按权重选择
  → 发送 /v1/chat/completions → 返回结果
  → 失败自动切换下一个
```

### 5.2 图片生成

```
image_generate 工具
  → pool.SelectFor(ModelImageGen)
  → 过滤出有 image_gen 模型的健康 backend
  → 发送 /v1/images/generations，model = ModelInfo.Name
  → 失败切换下一个有此能力的 backend
```

### 5.3 视觉理解

```
vision 工具（看图片）
  → pool.SelectFor(ModelVision)
  → 过滤多模态 backend（gpt-4o, claude-3.5 等）
  → 发送带 image_url 的 /v1/chat/completions
```

### 5.4 无可用 Backend 时

返回明确错误："no backend available for image_gen. Add a backend that supports gpt-image-* or dall-e-* models."

## 6. 配置示例

```yaml
provider:
  pool_strategy: round_robin
  backends:
    - label: beecode-main
      key: sk-aaa
      base_url: https://beecode.cc/v1
      weight: 10
      # models 留空 → 启动时自动 GET /models 发现
      # 自动得到: gpt-5.5→chat, gpt-4o→chat+vision, gpt-image-2→image_gen

    - label: beecode-img
      key: sk-bbb
      base_url: https://beecode.cc/v1
      weight: 5
      models:
        - name: gpt-image-2
          type: image_gen
      # 声明此 backend 只用于图片生成

    - label: opencode-go
      key: ${OPENCODE_GO_API_KEY}
      base_url: https://opencode.ai/zen/go/v1
      weight: 8
      # 自动发现: deepseek-v4-pro→chat, qwen-*→chat...
```

## 7. 前端 UI

### 7.1 Backend 卡片（Settings → Backends）

```
┌─ beecode-main ────────────────────────────────┐
│  https://beecode.cc/v1          weight: 10     │
│  Models (auto-discovered from /models):        │
│    [💬 chat]   gpt-5.5, gpt-4o-mini            │
│    [👁️ vision] gpt-4o                           │
│    [🖼️ image]  gpt-image-2                      │
│  [Edit] [Duplicate] [Delete]                   │
└────────────────────────────────────────────────┘
```

### 7.2 能力总览面板（Settings → Overview）

```
Model Capabilities:

  💬 Chat        ✅ 可用 (3 backends)
                  beecode-main, opencode-go, shenfen

  👁️ Vision      ✅ 可用 (1 backend)
                  beecode-main (gpt-4o)

  🖼️ Image Gen   ✅ 可用 (2 backends)
                  beecode-main, beecode-img

  🎤 Speech STT  ❌ 未配置 — 添加 whisper-* 模型

  🔊 Speech TTS  ❌ 未配置 — 添加 tts-* 模型
```

## 8. 与 Hermes 架构对比

| 维度 | Hermes | Codex Go（新设计） |
|---|---|---|
| Provider 管理 | `providers:` YAML map，8 个独立条目 | `provider.backends[]` cc-switch 池，统一管理 |
| 能力声明 | 分散在 `auxiliary.vision` + `image_gen` 两处 | 自动从 `/models` 端点发现，Backend 模型列表即能力 |
| 新增能力 | 手动改 YAML 加 provider 条目 | 供应商加模型 → Pool 自动发现 |
| 故障转移 | 无（每个 provider 单点） | 内置：同能力多 Backend 自动切换 |
| Provider 复制 | 无内置支持 | Backend 级别 Duplicate 按钮 |
| 配置复杂度 | 高（需理解 auxiliary/image_gen 两层） | 低（一个 Provider 池覆盖所有能力） |

## 9. 实现路线

### Phase 1：Model 发现与分类（1-2天）

- Pool.probeBackend() 解析 `/models` JSON 响应
- 实现 detectModelType(name string) ModelType
- PoolEntry.Models 字段 + 自动发现逻辑
- API：`GET /api/backends/:label/models`

### Phase 2：能力路由（1天）

- Pool.SelectFor(types ...ModelType)
- 现有 chat 流程保持不动（渐进式改造）
- image_generate 工具接入 SelectFor(ModelImageGen)

### Phase 3：前端展示（1-2天）

- Backend 卡片：显示自动发现的模型列表
- 能力总览面板：汇总所有 Backend 的能力覆盖
- Duplicate Backend 功能

### Phase 4：Full 能力对接（2-3天）

- vision 工具 → SelectFor(ModelVision)
- video 工具 → SelectFor(ModelVideoGen)
- audio_stt / audio_tts / embedding 工具

---

## 10. CI/CD 自动构建管线

### 10.1 现状

- 无 `.github/workflows/` 目录（现仅有 Rust 项目遗留的 `.github/actions/`）
- Makefile 已覆盖 `build / build-all / desktop / release / test / clean`
- 跨平台编译能力完整（linux/mac/windows, amd64/arm64）

### 10.2 需要的 Workflow

#### CI (Pull Request) — `.github/workflows/ci.yml`

```
触发: push/PR → golang 分支
步骤:
  1. checkout
  2. setup-go 1.23+
  3. setup-node 22+（前端 tsc 检查）
  4. go vet ./...
  5. go test ./internal/... -count=1
  6. cd web && npm ci && npx tsc --noEmit
  7. make build（验证 CLI 可构建）
  8. make build-all（验证全平台交叉编译）
```

#### Release — `.github/workflows/release.yml`

```
触发: tag push v*.*.*
步骤:
  1. checkout
  2. setup-go + setup-node
  3. make build-all（全平台 CLI 二进制）
  4. make web（前端构建）
  5. 打包 tar.gz（每个平台 CLI + checksums.txt）
  6. make desktop（Windows portable + NSIS installer）
  7. make desktop-linux（.deb + AppImage + tar.gz）
  8. 创建 GitHub Release + 上传所有 artifacts
  9. (可选) 自动更新版本号 commit
```

#### 仅 PR 时不发版

CI workflow 确保只验证不发布。Release workflow 仅在 tag push 触发。

### 10.3 设计决策

- **不引入 Docker**：Go 静态编译 + CGO_ENABLED=0，无系统依赖
- **不引入 Bazel**：Go 项目用原生 go toolchain，不继承 Rust 项目的 Bazel
- **桌面打包**：仅 Windows/Linux 在 CI 可构建（macOS 需 GitHub Actions macOS runner）
- **checksums**：每个 release 附带 sha256 checksums.txt 验证完整性

---

## 11. spec / plan 工作流支持

### 11.1 现状

- 交互模式现有命令：`/exit` `/history` `/clear` `/save` `/sessions`
- 项目已有 `SPEC.md`（本文件），格式为编号章节 + 代码示例
- 用户偏好"spec-before-code"：先写 SPEC.md 再编码

### 11.2 功能设计

#### 11.2.1 交互式 slash 命令

| 命令 | 功能 | 示例 |
|------|------|------|
| `/spec [描述]` | 生成 SPEC.md，按模板输出 | `/spec 支持多语言 UI 切换` |
| `/plan [spec文件]` | 读 SPEC 生成实施计划（分阶段任务列表） | `/plan` (默认读 SPEC.md) |
| `/tasks` | 列出当前 plan 的任务状态（类似 todo） | `/tasks` |
| `/implement [task-id]` | 标记开始实现某个任务 | `/implement 3` |

#### 11.2.2 独立子命令

```bash
# 创建新 spec（从模板）
codex-go spec new <feature-name>

# 查看已有 spec
codex-go spec show [file]

# 从 spec 生成 plan
codex-go plan generate <spec-file>

# 列出 plan 任务
codex-go plan list
```

#### 11.2.3 实现方式

```
交互模式 /spec:
  1. 用户输入 "/spec 功能描述"
  2. Agent 构造 prompt: "根据以下描述生成 SPEC.md..."
  3. AI 返回结构化 SPEC 内容
  4. 写入 SPEC-<feature>.md 到项目根目录
  5. 打印文件路径

交互模式 /plan:
  1. 用户输入 "/plan" 或 "/plan SPEC-xxx.md"
  2. Agent 读取指定 SPEC
  3. AI 分解为分阶段任务列表
  4. 写入 PLAN.md（格式：Phase → Tasks → 验收标准）
  5. 打印任务概览
```

### 11.3 数据格式

#### SPEC 模板（SPEC-<feature>.md）

```markdown
# <Feature Name>

## 1. 背景与动机

## 2. 目标

## 3. 设计方案

### 3.1 架构

### 3.2 数据结构

### 3.3 流程

## 4. 影响分析

## 5. 实施路线

### Phase 1: ...

### Phase 2: ...
```

#### PLAN 模板（PLAN.md）

```markdown
# Implementation Plan for <Feature>

## Phase 1: <Name>
- [ ] Task 1.1: <description> — 预计 1天
- [ ] Task 1.2: <description> — 预计 0.5天

## Phase 2: <Name>
- [ ] Task 2.1: <description>
...

## 验收标准
- [ ] 标准 1
- [ ] 标准 2
```

### 11.4 与现有系统集成

- `/spec` 和 `/plan` 复用现有 Agent.Run() —— 只是 prompt 构造不同
- SPEC/PLAN 文件写入用现有 write_file 工具
- 不新增 internal 包，在 main.go 的 `runInteractive()` 中加 case 分支

### 11.5 实现路线

#### Phase 1: CI/CD（1天）
- 创建 `.github/workflows/ci.yml`
- 创建 `.github/workflows/release.yml`
- 本地验证 CI 步骤（make test, make build-all）
- 推送到 GitHub 验证 workflow 触发

#### Phase 2: spec/plan（2天）
- `main.go`: 添加 `/spec` `/plan` `/tasks` `/implement` 交互命令
- CLI 子命令: `codex-go spec new/show` `codex-go plan generate/list`
- 模板文件放到 `~/.codex/templates/` 或在代码内嵌
- 集成测试（交互模式 mock）

#### Phase 3: 收尾（0.5天）
- 更新 README 中的命令列表
- 更新 Makefile help 文档

---

## 12. MCP 服务器安装管理（新增）

### 12.1 背景与动机

当前 MCP 服务器只能在 `config.yaml` 中静态配置，没有运行时 CRUD 能力。
参考 cc-switch（原始 Codex master）的 MCP 管理架构：
- SSOT 数据库存储所有 MCP 服务器定义
- CRUD API + UI 面板
- 按应用启用/禁用（Codex/Claude/Gemini/Hermes）
- 配置自动同步到各应用 live config
- MCP 向导和预设模板

### 12.2 目标

**后端：**
- MCP 服务器持久化存储（JSON 文件 → `~/.codex/mcp-servers.json`）
- CRUD API：`GET/POST /api/mcp/servers`、`GET/PUT/DELETE /api/mcp/servers/{id}`
- MCP 连接管理：启动/停止/重启 stdio 进程
- 安装后即时生效（热加载），无需重启整个应用

**前端：**
- MCP 服务器管理面板（"已安装"标签页，位于插件页面内）
- 三标签：插件 | 技能 | MCP
- 添加 MCP 服务器表单（Modal）：名称、命令、参数、环境变量
- 预设模板（filesystem、github、postgres、brave-search 等）
- 启用/禁用开关 + 连接状态指示
- 从现有配置导入（扫描 `~/.codex/config.yaml` 中已有 mcp.servers）

### 12.3 数据结构

#### 12.3.1 后端存储模型

```go
// MCPServerDef 存储在 ~/.codex/mcp-servers.json
type MCPServerDef struct {
    ID          string            `json:"id"`          // 唯一标识 (uuid)
    Name        string            `json:"name"`        // 显示名称
    Description string            `json:"description"` // 描述
    Command     string            `json:"command"`     // 执行命令 (npx, uvx, python, node...)
    Args        []string          `json:"args"`        // 命令参数
    Env         map[string]string `json:"env"`         // 环境变量
    Enabled     bool              `json:"enabled"`     // 是否启用
    Status      string            `json:"status"`      // "connected" | "disconnected" | "error"
    Error       string            `json:"error,omitempty"` // 错误信息
    ToolCount   int               `json:"tool_count"`  // 提供的工具数量
    CreatedAt   string            `json:"created_at"`
    UpdatedAt   string            `json:"updated_at"`
}
```

#### 12.3.2 API 设计

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/mcp/servers` | 列出所有 MCP 服务器 |
| POST | `/api/mcp/servers` | 添加并启动 MCP 服务器 |
| PUT | `/api/mcp/servers/{id}` | 更新 MCP 服务器（重启进程） |
| DELETE | `/api/mcp/servers/{id}` | 停止并删除 MCP 服务器 |
| POST | `/api/mcp/servers/{id}/restart` | 重启 MCP 服务器进程 |
| GET | `/api/mcp/presets` | 获取预设 MCP 模板列表 |

### 12.4 预设模板

```json
[
  {
    "name": "Filesystem",
    "description": "安全的文件系统操作（读写、搜索、编辑）",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/dir"],
    "env": {}
  },
  {
    "name": "GitHub",
    "description": "GitHub API 集成（仓库、Issues、PR 管理）",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github"],
    "env": {"GITHUB_PERSONAL_ACCESS_TOKEN": "<your-token>"}
  },
  {
    "name": "PostgreSQL",
    "description": "PostgreSQL 数据库查询（只读 Schema 检查）",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost/mydb"],
    "env": {}
  },
  {
    "name": "Brave Search",
    "description": "Brave Search API 网页和本地搜索",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-brave-search"],
    "env": {"BRAVE_API_KEY": "<your-api-key>"}
  },
  {
    "name": "Memory",
    "description": "基于知识图谱的持久记忆系统",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-memory"],
    "env": {}
  },
  {
    "name": "Puppeteer",
    "description": "浏览器自动化（截图、点击、表单填写）",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-puppeteer"],
    "env": {}
  },
  {
    "name": "Fetch",
    "description": "网页内容获取和转换为 Markdown",
    "command": "uvx",
    "args": ["mcp-server-fetch"],
    "env": {}
  },
  {
    "name": "Sequential Thinking",
    "description": "动态和反思性问题解决的思维工具",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-sequential-thinking"],
    "env": {}
  }
]
```

### 12.5 实现流程

```
用户点击 "添加 MCP 服务器" → Modal 表单
  → 填写 name/command/args/env
  → 或选择预设模板（自动填充）
  → POST /api/mcp/servers
    → Go 后端：保存到 mcp-servers.json
    → 启动 MCPClient stdio 进程
    → 调用 tools/list 获取工具
    → 包装为 tool.Tool 注册到 Agent registry
    → 返回状态（connected + tool_count）
  → 前端更新列表 + 显示连接状态

运行时更新：
  PUT /api/mcp/servers/{id}
    → 停止旧进程 → 更新配置 → 启动新进程
    → 重新加载工具到 registry
```

### 12.6 与现有系统的集成

- 扩展现有 `mcp.MCPClient` 支持运行时启停（添加 `Start()` / `Stop()` 方法）
- 新增 `internal/api/handler_mcp.go` 处理程序
- Agent 的 registry 支持动态注册/注销工具
- 前端 PluginsPage 添加第三个 Tab "MCP"

---

## 13. Skill 技能安装与发现系统（新增）

### 13.1 背景与动机

当前 skill 系统只能被动扫描磁盘上的 SKILL.md 文件，无法安装、发现、更新。
参考 cc-switch 的 skill 管理架构：
- GitHub 仓库作为 skill 来源（Anthropic、ComposioHQ、cexll、JimLiu 等）
- 安装 → SSOT 目录（`~/.codex/skills/`）
- 内容 hash 变更检测实现自动更新
- 卸载时备份到 `~/.codex/skill-backups/`
- skills.sh 公共目录搜索集成

### 13.2 目标

**后端：**
- Skill 存储模型（`~/.codex/skills/` + `skill-store.json`）
- Skill 仓库管理（添加/删除 GitHub 仓库作为 skill 来源）
- 从 GitHub 仓库发现可安装 skill（扫描子目录中的 SKILL.md）
- Skill 安装：`git clone --depth 1 --filter=blob:none` 克隆到本地
- Skill 卸载：备份到 `~/.codex/skill-backups/` + 从技能注册表删除
- content hash 变更检测 + 更新提示
- skills.sh API 搜索集成

**前端：**
- 技能页面三区域：已安装 / 可发现 / 仓库管理
- "发现"标签：浏览 GitHub 仓库中的 skill，一键安装
- 安装按钮：显示安装进度（克隆→提取 SKILL.md→注册）
- 更新检测：显示可更新的 skill 数量 + 一键更新
- 仓库管理：添加/移除 skill 仓库 URL
- 导入已有 skill：扫描 `~/.agents/skills/` 等已知目录

### 13.3 数据结构

#### 13.3.1 后端模型

```go
// SkillStore 持久化到 ~/.codex/skill-store.json
type SkillStore struct {
    Skills []InstalledSkill `json:"skills"`
    Repos  []SkillRepo      `json:"repos"`
}

type InstalledSkill struct {
    ID          string `json:"id"`          // uuid
    Name        string `json:"name"`        // 从 SKILL.md 解析的名称
    Description string `json:"description"` // 从 SKILL.md 解析的描述
    Directory   string `json:"directory"`   // 安装目录名
    RepoOwner   string `json:"repo_owner"`  // GitHub 用户/组织
    RepoName    string `json:"repo_name"`   // GitHub 仓库名
    RepoBranch  string `json:"repo_branch"` // 分支名
    ReadmeURL   string `json:"readme_url"`  // README 链接
    ContentHash string `json:"content_hash"` // SKILL.md 内容 hash
    InstalledAt int64  `json:"installed_at"`
    UpdatedAt   int64  `json:"updated_at"`
    Enabled     bool   `json:"enabled"`     // 是否启用（注入 Agent）
}

type SkillRepo struct {
    Owner   string `json:"owner"`   // GitHub 用户/组织
    Name    string `json:"name"`    // 仓库名称
    Branch  string `json:"branch"`  // 默认分支
    Enabled bool   `json:"enabled"` // 是否启用扫描
}

// DiscoverableSkill 可发现的技能（从仓库扫描）
type DiscoverableSkill struct {
    Key         string `json:"key"`          // "owner/name:directory"
    Name        string `json:"name"`
    Description string `json:"description"`
    Directory   string `json:"directory"`
    RepoOwner   string `json:"repo_owner"`
    RepoName    string `json:"repo_name"`
    RepoBranch  string `json:"repo_branch"`
    ReadmeURL   string `json:"readme_url"`
    Installed   bool   `json:"installed"`    // 是否已安装
}
```

#### 13.3.2 API 设计

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/skills/installed` | 列出已安装的 skill |
| GET | `/api/skills/discover` | 从仓库发现可安装的 skill |
| POST | `/api/skills/install` | 安装 skill（body: {key, repo_owner, repo_name, directory}） |
| DELETE | `/api/skills/{id}` | 卸载 skill（备份后删除） |
| POST | `/api/skills/{id}/update` | 更新单个 skill |
| GET | `/api/skills/updates` | 检查所有已安装 skill 的更新 |
| GET | `/api/skills/repos` | 列出 skill 仓库 |
| POST | `/api/skills/repos` | 添加 skill 仓库 |
| DELETE | `/api/skills/repos/{owner}/{name}` | 移除 skill 仓库 |
| POST | `/api/skills/import` | 从已知路径导入 skill |

### 13.4 默认仓库配置

```json
[
  { "owner": "anthropics", "name": "skills", "branch": "main" },
  { "owner": "ComposioHQ", "name": "awesome-claude-skills", "branch": "master" },
  { "owner": "cexll", "name": "myclaude", "branch": "master" },
  { "owner": "JimLiu", "name": "baoyu-skills", "branch": "main" }
]
```

### 13.5 实现流程

```
Skill 安装流程:
  1. 前端 POST /api/skills/install { key: "anthropics/skills:pdf", repo_owner: "anthropics", repo_name: "skills", directory: "pdf" }
  2. 后端：
     a. git clone --depth 1 --filter=blob:none -b main \
        https://github.com/anthropics/skills.git /tmp/codex-skill-XXXX
     b. 复制 /tmp/.../skills/pdf/ → ~/.codex/skills/pdf/
     c. 解析 SKILL.md → name, description
     d. 计算 content hash
     e. 保存到 skill-store.json
     f. 重新加载 skill 注册表 → Agent 即时可用
  3. 前端更新已安装列表

Skill 更新检测:
  1. GET /api/skills/updates
  2. 后端对每个已安装 skill：
     a. git ls-remote 获取远程最新 commit SHA
     b. 对比本地 content hash
     c. 返回变更列表
  3. 前端显示 "N 个技能可更新"

Skill 卸载:
  1. DELETE /api/skills/{id}
  2. 后端：
     a. 将 skill 目录打包为 tar.gz → ~/.codex/skill-backups/
     b. 删除 ~/.codex/skills/{directory}/
     c. 从 skill-store.json 移除记录
     d. 重新加载 skill 注册表
  3. 前端更新已安装列表
```

---

## 14. 插件系统增强（增强）

### 14.1 当前能力

- `.plugin.json` 文件加载已实现
- 安装/卸载 API 已存在（JSON 存盘）
- 前端有分类展示（但列表硬编码）

### 14.2 需增强的功能

| 功能 | 描述 |
|------|------|
| 插件热加载 | 安装后无需重启，即时注册到 Agent tool registry |
| 真实子进程执行 | 安装的插件作为独立进程运行，通过 stdin/stdout JSON-RPC 通信 |
| 插件发现 | 从配置的仓库/GitHub 获取可安装插件列表 |
| 插件状态监控 | 显示插件进程状态（运行中/已停止/错误） |
| 插件配置编辑 | 在线编辑 .plugin.json 内容 |

### 14.3 插件执行模型

```go
// Plugin 接口扩展
type Plugin interface {
    tool.Tool
    Start() error    // 启动插件子进程
    Stop() error     // 停止插件子进程
    Status() string  // "running" | "stopped" | "error"
}

// PluginProcess 通过 stdin/stdout JSON-RPC 与插件通信
type PluginProcess struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser
    // 每次工具调用 = 发送 JSON 请求 → 读取 JSON 响应
}
```

---

## 15. 左侧边栏 UI 优化

### 15.1 当前问题

根据与原始 Codex 的对比分析：

1. **底部"新建任务"按钮多余** — 与顶部导航"新建任务"功能重复，且样式不一致
2. **设置按钮不固定** — 设置齿轮图标未固定在侧边栏底部，随内容滚动
3. **右侧面板结构** — 原始 Codex 的右面板菜单是垂直列表，而当前是水平 tab icon

### 15.2 目标布局

```
┌─────────────────────────────────┐
│  ChatGPT Codex                  │ ← 标题
│                                 │
│  ┌─ 新建任务  ─────────────────┐│ ← 导航区域
│  │  已安排                     ││
│  │  插件                       ││
│  └─────────────────────────────┘│
│                                 │
│  ┌─ 项目 ──────────────────────┐│
│  │  📁 plan-agentic-parser...  ││
│  │  📁 schoolNews              ││
│  │  📁 urbanLifeline           ││
│  │  📁 K12Study                ││
│  │  ┌─ 展开显示 ──────────────┐││
│  │  │  📄 分析知识库图谱...    │││
│  │  │  📄 生成数据库SQL...     │││
│  │  │  ...                     │││
│  │  └─────────────────────────┘││
│  └─────────────────────────────┘│
│                                 │
│  ┌─ 任务 ──────────────────────┐│
│  │  📄 回应问候                ││
│  │  📄 当前ceswitch的代理...   ││
│  └─────────────────────────────┘│
│                                 │
│  ───────────────────────────── │ ← 分隔线（固定）
│  ⚙ 设置            ? 帮助     │ ← 固定在底部
└─────────────────────────────────┘
```

### 15.3 实现要点

- **移除底部"新建任务"按钮** — 功能已在顶部导航的第一项
- **设置固定在底部** — 使用 `position: sticky; bottom: 0` + 分隔线
- **项目树支持展开子路径** — 针对 `F:\Project\urbanLifeline\ai-manager` 这类嵌套路径
- **会话列表归入项目下** — 类似原始 Codex 的"展开显示"
- **帮助图标** — 右下角添加 `?` 帮助入口

---

## 16. 实施路线总览

### Phase 1：MCP 服务器管理（2天）
- 后端：`handler_mcp.go` + MCP store（mcp-servers.json）+ MCPClient 启停
- 后端：预设模板 API
- 前端：MCP 标签页（添加/编辑/删除/状态显示）
- 前端：模板向导选择

### Phase 2：Skill 技能安装（2天）
- 后端：`handler_skills.go` 扩展（install/uninstall/update/discover）
- 后端：GitHub repo 扫描 + git clone
- 后端：skill-store.json 持久化
- 前端：技能"发现"标签 + 安装/卸载 + 更新检测

### Phase 3：插件系统增强（1天）
- 后端：PluginProcess 子进程管理 + 热加载
- 前端：插件状态实时监控
- 插件发现（从仓库获取可安装列表）

### Phase 4：Sidebar UI 优化（0.5天）
- 移除底部新建任务按钮
- 设置底部固定
- 项目树嵌套路径支持
- 右侧面板菜单风格优化

### Phase 5：收尾测试（0.5天）
- go vet + 测试
- 前端 tsc 检查
- make build 验证
- 端到端手动验证

总计：约 6 天

---

## 17. 与原始 Codex Master 对比总结

| 功能模块 | 原始 Codex (cc-switch) | 当前 Go 实现 | 本 SPEC 后 |
|----------|------------------------|-------------|-----------|
| MCP 服务器 CRUD | SQLite + get/upsert/delete | config.yaml 静态 | ✅ REST API + JSON store |
| MCP 服务器 UI | UnifiedMcpPanel + FormModal | 无 | ✅ 插件页内 MCP 标签 |
| MCP 预设模板 | mcpPresets 配置 | 无 | ✅ GET /api/mcp/presets |
| MCP 热加载 | sync to live config | 需重启 | ✅ Start/Stop/Restart |
| Skill 发现 | GitHub 仓库扫描 | 无 | ✅ GitHub API 扫描 |
| Skill 安装 | git clone → SSOT | 无 | ✅ git clone → ~/.codex/skills/ |
| Skill 卸载/备份 | backup → delete | 无 | ✅ tar.gz 备份 → 删除 |
| Skill 更新 | content hash 检测 | 无 | ✅ git ls-remote diff |
| 技能仓库管理 | add/remove repo | 无 | ✅ CRUD /api/skills/repos |
| 插件热加载 | 即时生效 | 需重启 | ✅ PluginProcess Start/Stop |
| 插件发现 | 仓库扫描 | 前端硬编码 | ✅ 仓库配置 |
| Sidebar 嵌套路径 | 展开子目录 | 扁平列表 | ✅ buildProjectTree |
| 设置固定底部 | sticky bottom | 随内容滚动 | ✅ position: sticky |
