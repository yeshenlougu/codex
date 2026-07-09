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
