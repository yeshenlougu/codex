# Codex Go 多模型架构设计

## 1. 现状

Codex Go 的 Provider 已是 cc-switch 代理池：

- 多 Backend 权重路由（round_robin / fill_first / random）
- 自动故障转移 + 健康检查
- 健康检查已访问 `GET /models`（只检查状态码，未解析模型列表）

当前只有一个模型维度（chat），所有请求走同一个 Pool。

## 2. 设计原则

**不在 Provider 之上另建一层 models.capabilities。**
Provider 本身就是能力声明——通过其 Backend 的模型列表自动表达支持哪些能力。

## 3. 自动检测模型类型

Pool 健康检查时解析 `/models` 响应，按模型名自动分类：

- `gpt-*` `claude-*` `deepseek-*` `gemini-*` `qwen-*` → **chat**
- `gpt-4o` `gpt-5*` `claude-3.5*` → **chat + vision**
- `gpt-image-*` `dall-e-*` → **image_gen**
- `whisper-*` → **audio_stt**
- `tts-*` → **audio_tts**
- `text-embedding-*` → **embedding**
- `sora-*` `runway-*` → **video_gen**
- 未匹配 → **chat**（默认，所有 LLM 都能对话）

### 手动覆盖

`BackendConfig` 加可选字段：

```yaml
backends:
  - key: sk-xxx
    base_url: https://beecode.cc/v1
    models:
      - name: gpt-5.5
      - name: my-custom-model
        type: image_gen     # 手动指定
        context_length: 131072
```

自动检测优先，`type` 字段只覆盖自动判断错误的。

## 4. Go 数据结构

```go
// BackendConfig 增加 Models 字段
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

// ModelType 能力枚举
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

## 5. Pool 改造

PoolEntry 存储自动发现的模型信息：

```go
type PoolEntry struct {
    // 现有字段...
    Models []ModelInfo    `json:"models"`  // 自动发现 + 配置合并
}

type ModelInfo struct {
    Name string    `json:"name"`
    Type ModelType `json:"type"`
}

// 按能力筛选可用 Backend
func (p *Pool) SelectFor(models ...ModelType) (*PoolEntry, ModelInfo, bool)
```

## 6. 运行时路由

```
image_generate 工具调用
  → pool.SelectFor(ModelImageGen)
  → 只从有 image_gen 模型的健康 backend 中选择
  → 发送请求到选中 backend，model 名用 ModelInfo.Name
  → 失败自动切换到下一个有此能力的 backend
```

如果没有任何 backend 支持请求的能力类型：

- 返回明确错误："no backend available for image_gen"
- 前端显示：此能力未配置，请添加支持 image_gen 的 Backend

## 7. Provider 复制流程

```
用户操作：Backend 列表 → 选中 beecode → [Duplicate]
  → 弹出窗口：新 Label、覆盖 Key、修改 Models
  → 保存为新 Backend → Pool 自动重新发现模型类型
```

不需要"复制整个 Provider"——在同一个 Provider 下加 Backend 即可。cc-switch 的权重路由自然处理多 Backend 分发。

## 8. 配置示例

```yaml
provider:
  base_url: https://beecode.cc/v1
  pool_strategy: round_robin
  backends:
    - label: beecode-main
      key: sk-aaa
      base_url: https://beecode.cc/v1
      weight: 10
      # models 留空 → 自动从 /models 端点发现
      # 自动分类: gpt-5.5→chat, gpt-image-2→image_gen

    - label: beecode-img-only
      key: sk-bbb
      base_url: https://beecode.cc/v1
      weight: 5
      models:
        - name: gpt-image-2
          type: image_gen     # 声明此 backend 只提供 image_gen
```

前端 Settings 页面自动显示每个 Backend 的模型列表和能力标签：

- beecode-main: 💬 gpt-5.5 👁️ gpt-4o 🖼️ gpt-image-2
- beecode-img-only: 🖼️ gpt-image-2

## 9. 前端 UI

### Settings → Backends 页面

每个 Backend 卡片展示自动发现的模型列表，按类型分组：

```
┌─ beecode-main ──────────────────────────────────────┐
│  https://beecode.cc/v1              weight: 10       │
│  Models (auto-discovered):                           │
│    💬 chat:    gpt-5.5, gpt-4o-mini                   │
│    👁️ vision:  gpt-4o                                 │
│    🖼️ image:   gpt-image-2                            │
│  [Edit] [Duplicate] [Delete]                         │
└─────────────────────────────────────────────────────┘
```

### 能力总览面板

汇总所有 Backend 的能力覆盖：

```
能力覆盖:
  💬 Chat:       ✅ 3 backends (beecode-main, opencode-go, shenfen)
  👁️ Vision:     ✅ 1 backend  (beecode-main)
  🖼️ Image Gen:  ✅ 2 backends (beecode-main, beecode-img-only)
  🎤 Speech STT: ❌ 未配置
  🔊 Speech TTS: ❌ 未配置
```

## 10. 与之前 SPEC 的关键差异

- **之前**: 额外建 `models.capabilities` 层，Provider 上声明能力
- **现在**: Provider = cc-switch 池，Backend 的模型列表自动表达能力，零额外配置
- **之前**: 需要手动复制 Provider 并改 capabilities
- **现在**: 同 Provider 下加 Backend 即可，Pool 自动处理路由和故障转移
- **之前**: 两层抽象（Provider + Capability）
- **现在**: 一层抽象（Pool 自动发现 → 工具按需选择 Backend）

## 11. 实现路线

### Phase 1: Model 发现与分类 (1-2天)

- `Pool.probeBackend()` 解析 `/models` JSON 响应
- 实现 `detectModelType(name string) ModelType`
- `PoolEntry` 增加 `Models []ModelInfo`
- API: `GET /api/backends/:label/models`

### Phase 2: 能力路由 (1天)

- `Pool.SelectFor(types ...ModelType)` 按能力选择 Backend
- `agent.Run()` 中根据工具类型调用对应的 SelectFor
- 当前只有 chat → 渐进式，不改现有流程

### Phase 3: 前端展示 (1-2天)

- Backend 卡片展示模型列表（按类型分组）
- 能力总览面板
- Duplicate Backend 按钮

### Phase 4: image_gen 工具对接 (1天)

- image_generate 工具调用 `SelectFor(ModelImageGen)`
- 对接现有 image_gen provider 逻辑
- 测试：BeeCode 生图 key 通过 Backend pool 路由
