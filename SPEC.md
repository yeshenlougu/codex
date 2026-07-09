# Codex Go 多模型架构设计

## 1. 架构概览

Codex Go 需要支持 8 种模型能力类型，每种可独立选择 Provider 和 Model。架构参考 Hermes 的 `providers` + `auxiliary` + `image_gen` 三层设计，简化为 Go 原生结构。

```
Config
├── providers: map[string]Provider    ← Provider 池（可复用、可复制）
├── models: ModelsConfig             ← 每种能力选择哪个 Provider + Model
├── tools: ToolsConfig
└── agent: AgentConfig
```

## 2. 模型能力类型

| 能力 ID | 名称 | 功能 | API 类型 |
|---------|------|------|---------|
| `chat` | LLM 对话 | 代码生成、问答、推理 | `/v1/chat/completions` |
| `vision` | 视觉理解 | 图片/截图分析 | `/v1/chat/completions` (multimodal) |
| `video` | 视频理解 | 视频内容分析 | `/v1/chat/completions` (multimodal + video) |
| `image_gen` | 图片生成 | DALL-E / GPT Image | `/v1/images/generations` |
| `video_gen` | 视频生成 | 文生视频 | 各厂商专用 API |
| `audio_stt` | 语音识别 | Speech-to-Text | `/v1/audio/transcriptions` |
| `audio_tts` | 语音合成 | Text-to-Speech | `/v1/audio/speech` |
| `embedding` | 向量嵌入 | 文本向量化 | `/v1/embeddings` |

## 3. Provider 模型

Provider 是独立的 API 端点配置，存储在 `providers` 池中，可被多个能力引用。

```yaml
providers:
  beecode-chat:
    label: "BeeCode Chat"
    api_key: "sk-xxx"
    base_url: "https://beecode.cc/v1"
    api_mode: "chat_completions"
    default_model: "gpt-5.5"
    headers:
      User-Agent: "beecode/1.0"
    models:
      - name: "gpt-5.5"
        context_length: 262144
    capabilities:
      - chat
      - vision
      - video

  beecode-img:
    label: "BeeCode Image"
    api_key: "sk-yyy"           # ← 不同 key
    base_url: "https://beecode.cc/v1"
    api_mode: "chat_completions"
    default_model: "gpt-image-2"
    headers:
      User-Agent: "beecode/1.0"
    capabilities:
      - image_gen

  opencode-go:
    label: "OpenCode Go"
    api_key: "${OPENCODE_GO_API_KEY}"
    base_url: "https://opencode.ai/zen/go/v1"
    api_mode: "chat_completions"
    default_model: "deepseek-v4-pro"
    capabilities:
      - chat
```

### Provider 字段

```go
type ProviderConfig struct {
    Label        string            `yaml:"label" json:"label"`
    APIKey       string            `yaml:"api_key" json:"api_key"`
    BaseURL      string            `yaml:"base_url" json:"base_url"`
    APIMode      string            `yaml:"api_mode" json:"api_mode"`
    DefaultModel string            `yaml:"default_model" json:"default_model"`
    Headers      map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
    Models       []ModelOverride   `yaml:"models,omitempty" json:"models,omitempty"`
    Capabilities []CapabilityID    `yaml:"capabilities" json:"capabilities"`
}

type ModelOverride struct {
    Name          string `yaml:"name" json:"name"`
    ContextLength int    `yaml:"context_length,omitempty" json:"context_length,omitempty"`
}
```

### Provider 能力声明

每个 Provider 声明自己支持哪些能力类型（`capabilities` 字段）。同一个 Provider 可以同时支持 `chat` + `vision`（如 GPT-4o），或者只支持 `image_gen`（如专用图生图 key）。

## 4. 能力配置

```yaml
models:
  chat:
    provider: "opencode-go"
    model: "deepseek-v4-pro"
    enabled: true

  vision:
    provider: "beecode-chat"
    model: "gpt-5.5"
    enabled: true

  image_gen:
    provider: "beecode-img"
    model: "gpt-image-2-medium"
    enabled: true

  video:
    provider: ""              # 空 = 继承 chat provider
    model: ""
    enabled: false            # 未启用

  audio_stt:
    provider: ""
    model: "whisper-1"
    enabled: false

  audio_tts:
    provider: ""
    model: "tts-1"
    enabled: false

  embedding:
    provider: ""
    model: "text-embedding-3-small"
    enabled: false
```

每个能力配置：
- `provider`: 引用 `providers` 池中的名称，空字符串 = 使用 `chat` 的 provider
- `model`: 覆盖 provider 的 default_model，空 = 使用 provider 默认值
- `enabled`: 是否启用该能力

## 5. Provider 复制 / 切换流程

这是用户强调的核心功能：**复制现有 Provider → 改几个参数 → 为新能力切换 Provider**。

### 5.1 典型用例

```
用户有 beecode-chat (key: sk-aaa, 用于对话)
需要为 image_gen 单独配置:

1. 复制 beecode-chat → beecode-img
2. 修改 api_key → sk-bbb（图生图专用 key）
3. 修改 default_model → gpt-image-2
4. 修改 capabilities → [image_gen]
5. 在 models.image_gen.provider 设为 "beecode-img"
```

### 5.2 Go 实现

```go
func (pm *ProviderManager) Duplicate(srcName, dstName string, overrides map[string]interface{}) error {
    src, ok := pm.providers[srcName]
    if !ok {
        return fmt.Errorf("provider %q not found", srcName)
    }
    
    // Deep copy
    dst := src
    dst.Label = src.Label + " (copy)"
    
    // Apply overrides
    if apiKey, ok := overrides["api_key"]; ok {
        dst.APIKey = apiKey.(string)
    }
    if model, ok := overrides["model"]; ok {
        dst.DefaultModel = model.(string)
    }
    
    pm.providers[dstName] = dst
    return pm.Save()
}
```

### 5.3 前端 UI 流程

```
Settings → Backends → Provider 列表
  ├── [Beecode Chat]  [Edit] [Duplicate] [Delete]
  ├── [Beecode Img]   [Edit] [Duplicate] [Delete]
  └── [+ Add Provider]
  
点击 [Duplicate]:
  → 弹出模态框: 新名称、覆盖哪些字段
  → 确认 → 自动创建新 provider
  
Settings → Models → 能力列表
  ├── 💬 Chat:        [opencode-go ▼] [deepseek-v4-pro ▼]
  ├── 👁️ Vision:      [beecode-chat ▼]  [gpt-5.5 ▼]
  ├── 🖼️ Image Gen:   [beecode-img ▼]   [gpt-image-2 ▼]
  ├── 🎥 Video:       [inherit ▼]        [--]
  └── ...
```

## 6. 与 Hermes 对比

| 特性 | Hermes | Codex Go |
|------|--------|----------|
| Provider 池 | `providers:` YAML map | `providers:` YAML map (相同) |
| 能力模型 | `auxiliary.vision` + `image_gen` 分两处 | `models:` 统一管理所有能力 |
| Provider 复制 | 无内置支持（手动改 YAML） | 内置 Duplicate API + UI |
| 能力声明 | 无（provider 不知道自己是干嘛的） | `capabilities` 字段显式声明 |
| 默认继承 | `provider: auto` | `provider: ""` = 继承 chat |
| API 模式 | 每个 provider 有 `api_mode` | 同左 |
| 模型覆盖 | `models.gpt-5.5.context_length` | 同左 |

## 7. 前端 UI 设计

### 7.1 Settings → Backends 页面

```
┌──────────────────────────────────────────────────────────┐
│  Backends                                    [+ Add New] │
│                                                          │
│  ┌─ BeeCode Chat ──────────────────────────────────────┐ │
│  │  Base URL:  https://beecode.cc/v1                    │ │
│  │  API Key:   sk-xxx...xxx                             │ │
│  │  Model:     gpt-5.5                                  │ │
│  │  API Mode:  chat_completions                         │ │
│  │  Supports:  💬 chat  👁️ vision                       │ │
│  │  [Edit] [Duplicate] [Delete]                         │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─ BeeCode Image ─────────────────────────────────────┐ │
│  │  Base URL:  https://beecode.cc/v1                    │ │
│  │  API Key:   sk-yyy...yyy                             │ │
│  │  Model:     gpt-image-2                              │ │
│  │  Supports:  🖼️ image_gen                             │ │
│  │  [Edit] [Duplicate] [Delete]                         │ │
│  └─────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

### 7.2 Settings → Models 页面

```
┌──────────────────────────────────────────────────────────┐
│  Model Capabilities                                      │
│                                                          │
│  💬 Chat (LLM)                              [Enabled ✓]  │
│  ┌──────────────────────────────────────────────────────┐│
│  │ Provider: [opencode-go          ▼]                   ││
│  │ Model:    [deepseek-v4-pro      ▼]                   ││
│  │ Reason:   [high ▼]                                   ││
│  └──────────────────────────────────────────────────────┘│
│                                                          │
│  👁️ Vision                                    [Enabled ✓] │
│  ┌──────────────────────────────────────────────────────┐│
│  │ Provider: [beecode-chat          ▼]                   ││
│  │ Model:    [gpt-5.5              ▼]                   ││
│  └──────────────────────────────────────────────────────┘│
│                                                          │
│  🖼️ Image Generation                          [Enabled ✓] │
│  ┌──────────────────────────────────────────────────────┐│
│  │ Provider: [beecode-img           ▼]                   ││
│  │ Model:    [gpt-image-2-medium   ▼]                   ││
│  └──────────────────────────────────────────────────────┘│
│                                                          │
│  🎥 Video Understanding                     [Enabled -]  │
│  🎤 Speech-to-Text                          [Enabled -]  │
│  🔊 Text-to-Speech                          [Enabled -]  │
│  📊 Embedding                                [Enabled -]  │
└──────────────────────────────────────────────────────────┘
```

## 8. 后端 API

### 8.1 Provider CRUD

```
GET    /api/providers              → 列出所有 provider
GET    /api/providers/:name        → 获取单个
POST   /api/providers              → 创建新 provider
PUT    /api/providers/:name        → 更新
DELETE /api/providers/:name        → 删除
POST   /api/providers/:name/duplicate  → 复制并创建
```

### 8.2 Model 配置

```
GET    /api/models                 → 获取所有能力配置
PUT    /api/models/:capability     → 更新单个能力
POST   /api/models/validate        → 验证配置（测试连接）
```

### 8.3 运行时路由

```go
func (s *Server) resolveProvider(cap CapabilityID) (*ProviderConfig, error) {
    capCfg := s.config.Models[cap]
    
    // 1. 如果指定了 provider，直接使用
    if capCfg.Provider != "" {
        prov, ok := s.config.Providers[capCfg.Provider]
        if !ok {
            return nil, fmt.Errorf("provider %q not found", capCfg.Provider)
        }
        // 应用 model 覆盖
        if capCfg.Model != "" {
            prov.DefaultModel = capCfg.Model
        }
        return &prov, nil
    }
    
    // 2. 继承 chat provider
    return s.resolveProvider("chat")
}
```

## 9. 实现路线

### Phase 1: Provider 池重构（1-2天）
- 重写 `internal/config/config.go` → 支持多 provider 池
- 添加 Provider CRUD API (`handler_providers.go`)
- 配置文件迁移：`provider:` → `providers:`

### Phase 2: 多能力模型配置（1天）
- 添加 `ModelsConfig` 结构
- 能力路由逻辑 (`resolveProvider`)
- Models API (`handler_models.go`)

### Phase 3: Provider 复制功能（0.5天）
- Duplicate API
- 前端 Duplicate 按钮 + 模态框

### Phase 4: 前端 Settings 页面升级（1-2天）  
- BackendManager → 改为 Provider 卡片列表
- 新建 ModelsSettings 能力配置页
- Duplicate 模态框

### Phase 5: 工具集对接（2-3天）
- image_generate 工具对接 image_gen provider
- vision 工具对接 vision provider
- 后续：video/audio/embedding 工具
