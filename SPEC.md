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
