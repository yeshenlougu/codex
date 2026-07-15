# SPEC-CCSWITCH: cc-switch 供应商与代理架构对齐

> 基于 `/home/ubuntu/app/cc-switch` (main, commit f6e37ed9) 的分析

## 1. cc-switch 架构总览

cc-switch 是一个**多应用、多供应商的本地代理层**，核心职责：

```
┌─────────────────────────────────────────────────────┐
│                    cc-switch                         │
│                                                      │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐        │
│  │  Codex   │   │  Claude  │   │  Gemini  │  ...   │
│  │  app     │   │  app     │   │  app     │        │
│  └────┬─────┘   └────┬─────┘   └────┬─────┘        │
│       │               │               │              │
│  ┌────▼───────────────▼───────────────▼─────┐       │
│  │         ProviderManager                  │       │
│  │  IndexMap<id, Provider> + current        │       │
│  └────┬─────────────────────────────────────┘       │
│       │                                              │
│  ┌────▼─────────────────────────────────────┐       │
│  │         ProxyServer (port 15721)          │       │
│  │  ┌──────────┐  ┌────────────────────┐    │       │
│  │  │ Router   │  │  Protocol Adapters │    │       │
│  │  │ (failover│  │  Codex/Chat/Resp/  │    │       │
│  │  │  circuit)│  │  Anthropic/Gemini  │    │       │
│  │  └──────────┘  └────────────────────┘    │       │
│  └────┬─────────────────────────────────────┘       │
│       │                                              │
│  ┌────▼─────────────────────────────────────┐       │
│  │      上游 API (OpenAI / Anthropic / ...)  │       │
│  └──────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────┘
```

**关键设计**：
- **每应用一个当前供应商**：Codex 切换到供应商 A，Claude 同时用供应商 B
- **代理接管 Live Config**：切换时直接改写各应用的配置文件
- **故障转移队列**：供应商按优先级排列，失败自动切换下一个
- **协议适配层**：Codex Responses ↔ Chat Completions ↔ Anthropic Messages 互转

---

## 2. Provider 数据模型（对标分析）

### 2.1 cc-switch Provider 结构

```rust
// /home/ubuntu/app/cc-switch/src-tauri/src/provider.rs
pub struct Provider {
    pub id: String,                    // UUID
    pub name: String,                  // 显示名称
    pub settings_config: Value,        // 各应用特定的配置 JSON
    pub website_url: Option<String>,
    pub category: Option<String>,      // "official" | "third_party" | "partner"
    pub created_at: Option<i64>,
    pub sort_index: Option<usize>,
    pub notes: Option<String>,
    pub meta: Option<ProviderMeta>,    // 元数据（不写入 live config）
    pub icon: Option<String>,          // 图标标识
    pub icon_color: Option<String>,    // 图标颜色
    pub in_failover_queue: bool,       // 是否参与故障转移
}

pub struct ProviderMeta {
    pub custom_endpoints: HashMap<String, CustomEndpoint>,
    pub common_config_enabled: Option<bool>,
    pub usage_script: Option<UsageScript>,     // 用量查询脚本
    pub endpoint_auto_select: Option<bool>,     // 自动选择最佳端点
    pub is_partner: Option<bool>,
    pub cost_multiplier: Option<String>,
    pub limit_daily_usd: Option<String>,
    pub limit_monthly_usd: Option<String>,
    pub api_format: Option<String>,             // "anthropic" | "openai_chat" | "openai_responses"
    pub auth_binding: Option<AuthBinding>,       // 认证来源
    pub is_full_url: Option<bool>,
    pub prompt_cache_key: Option<String>,
    pub prompt_cache_routing: Option<String>,
    pub codex_chat_reasoning: Option<CodexChatReasoningConfig>,
    pub max_output_tokens: Option<u64>,
    pub custom_user_agent: Option<String>,
    pub local_proxy_request_overrides: Option<LocalProxyRequestOverrides>,
    // ... 30+ 字段
}
```

### 2.2 Codex Go 当前 BackendConfig 结构

```go
// /home/ubuntu/app/codex/internal/config/config.go
type BackendConfig struct {
    Key      string            `yaml:"key" json:"key"`
    Label    string            `yaml:"label" json:"label"`
    BaseURL  string            `yaml:"base_url" json:"base_url"`
    Weight   int               `yaml:"weight" json:"weight"`
    Models   []ModelEntry      `yaml:"models,omitempty" json:"models,omitempty"`
    Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}
```

### 2.3 差距对比

| 维度 | cc-switch | Codex Go 当前 | 优先级 |
|---|---|---|---|
| 供应商标识 | UUID + name + icon + color | 仅 Label 字符串 | P1 |
| 供应商分类 | official/third_party/partner | 无 | P1 |
| 多应用支持 | 每 app 独立 current provider | 全局单一 provider 名 | P2 |
| 元数据 | 30+ 字段（定价/限额/缓存/推理等） | 无 | P2 |
| 认证绑定 | ProviderConfig / ManagedAccount / OAuth | 仅 API Key | P2 |
| 端点管理 | 每供应商多端点 + 自动选择 | 多 Backend（已有） | ✅ |
| 故障转移队列 | 显式 in_failover_queue 标记 | Weight 隐式优先级 | P1 |
| 用量查询 | UsageScript（JS 脚本驱动） | 无 | P3 |
| 供应商切换 | Hot-switch + Live config 接管 | 改 config.yaml 需重启 | P1 |
| 预设系统 | 50+ 预设模板（auth + config.toml） | 无 | P2 |

---

## 3. 代理架构（对标分析）

### 3.1 cc-switch ProxyServer

```
ProxyServer (hyper HTTP, port 15721)
├── ProviderRouter
│   ├── select_providers() → 按优先级排序
│   ├── CircuitBreaker (每供应商熔断器)
│   └── FailoverSwitchManager (去重 + 自动切换)
├── Forwarder
│   ├── forward_with_retry() — 重试 + 故障转移
│   ├── Protocol Adapters
│   │   ├── Codex Responses ↔ Chat Completions
│   │   ├── Codex → Anthropic (streaming + transform)
│   │   ├── Gemini integration
│   │   └── OpenCode (OMO) integration
│   ├── ModelMapper (模型名映射)
│   ├── MediaSanitizer (图片能力检测 + 过滤)
│   ├── ThinkingRectifier (reasoning 签名修正)
│   └── CacheInjector (prompt cache 注入)
└── Usage
    ├── Calculator (token 用量计算)
    ├── Logger (请求日志)
    └── Parser (响应解析)
```

### 3.2 Codex Go 当前代理架构

```
Codex CLI (内嵌 HTTP)
├── cc-switch Pool (provider/pool.go)
│   ├── SelectFor(ModelType) — 按能力选择
│   ├── LoadBalance(strategy) — round_robin/random/fill_first
│   └── probeBackend() — /models 健康检查
└── Agent
    ├── openai.go — Chat Completions 直连
    ├── image_gen.go — 图片生成
    └── No protocol transform layer
```

### 3.3 差距对比

| 维度 | cc-switch | Codex Go 当前 | 优先级 |
|---|---|---|---|
| 独立代理进程 | 独立 hyper HTTP server | 内嵌于 CLI | P2 |
| 熔断器 | CircuitBreaker + 自动恢复 | 无 | P1 |
| 故障转移 | 显式队列 + 自动切换 | fill_first 自然切换 | P1 |
| 协议转换 | 完整 6 向转换矩阵 | 仅 chat_completions / responses | P2 |
| 模型映射 | ModelMapper（别名→真实模型） | 无 | P2 |
| 媒体过滤 | MediaSanitizer（图片能力检测） | 无 | P2 |
| 思考修正 | ThinkingRectifier | 无 | P3 |
| Prompt Cache | CacheInjector | 无 | P3 |
| 用量统计 | 完整计算+日志+面板 | 无 | P3 |
| 流式处理 | SSE + Anthropic streaming | 基础 SSE | ✅ |
| 动态端口 | 固定 15721 | 1977（可配置） | ✅ |

---

## 4. 前端 UI（对标分析）

### 4.1 cc-switch 前端组件树

```
App.tsx
├── ProviderCard.tsx          ← 供应商卡片（图标+名称+状态+操作）
├── AddProviderDialog.tsx     ← 添加供应商对话框（预设选择+表单）
├── EditProviderDialog.tsx    ← 编辑供应商
├── ProviderForm.tsx          ← 供应商表单（根据 app type 动态渲染）
│   ├── CodexFormFields.tsx   ← Codex 特有字段
│   ├── ClaudeFormFields.tsx  ← Claude 特有字段
│   ├── OpenCodeFormFields.tsx
│   ├── HermesFormFields.tsx
│   └── ...
├── HealthStatusIndicator.tsx ← 健康状态气泡
├── ProviderAdvancedConfig.tsx ← 高级配置（定价/限额/端点）
├── ProfileSwitcher.tsx       ← App Profile 切换
└── UsageDashboard.tsx        ← 用量面板
```

### 4.2 Codex Go 当前前端

```
SettingsPage.tsx
├── AgentSettings.tsx         ← 通用 Agent 设置
├── AgentManager.tsx          ← Agent Profile 管理
└── ProviderSettings.tsx      ← 供应商+Backends（刚合并）
    ├── Provider 身份卡
    ├── API Key
    ├── 代理池策略
    └── Backends 管理（嵌入式）
```

### 4.3 差距对比

| 维度 | cc-switch | Codex Go 当前 | 优先级 |
|---|---|---|---|
| 供应商列表 | 卡片网格，图标+名称+状态 | 无独立列表（合并一页） | P1 |
| 供应商分类 | official/third_party/partner 标签 | 无 | P1 |
| 预设系统 | 50+ 预设，一键添加 | 无 | P1 |
| 供应商切换 | 一键切换 + 实时生效 | 下拉改 provider 字段 | P1 |
| 健康状态 | 实时气泡 Badge | 无（仅 Backend 卡片有） | P1 |
| 高级配置 | 定价/限额/缓存/推理/端点 | 无 | P2 |
| 用量面板 | UsageDashboard + Coding Plan | 无 | P3 |
| 表单动态化 | 按 app type 渲染不同表单 | 通用 Backend 表单 | P2 |
| 图标+颜色 | 预设图标系统 | 无 | P2 |

---

## 5. 实施路线

### Phase 1: 供应商内核重构（P0，3-4天）

```
目标：Provider 从"单一池"变为"多供应商管理系统"
```

| 任务 | 说明 |
|---|---|
| 1.1 Provider 数据模型升级 | Config 新增 Provider 结构体（id/name/icon/category/meta），替换当前单一字段 |
| 1.2 Provider CRUD API | `/api/providers` GET/POST/PUT/DELETE + switch |
| 1.3 Provider 存储 | `~/.codex/providers.json`（独立于 config.yaml） |
| 1.4 Backend 归属 | Backend 关联到 Provider ID，不再全局平铺 |
| 1.5 供应商切换 API | `POST /api/providers/switch` + 热切换（无需重启） |

### Phase 2: 前端供应商管理（P1，2-3天）

```
目标：供应商列表页 + 预设系统 + 状态指示
```

| 任务 | 说明 |
|---|---|
| 2.1 ProviderList 页面 | 卡片网格布局，图标+名称+分类+健康状态+操作 |
| 2.2 Provider 预设系统 | 从 cc-switch 预设数据（50+ 供应商）生成配置模板 |
| 2.3 供应商表单 | 分类筛选（Official/ThirdParty/Partner），一键添加 |
| 2.4 健康状态指示 | HealthStatusIndicator 组件（healthy/degraded/unhealthy） |
| 2.5 供应商详情/编辑 | 显示关联 Backends、高级配置、用量信息 |

### Phase 3: 代理层增强（P2，2-3天）

```
目标：熔断器 + 故障转移队列 + 协议适配
```

| 任务 | 说明 |
|---|---|
| 3.1 CircuitBreaker | 每 Backend 熔断器（半开/全开/关闭 + 自动恢复） |
| 3.2 故障转移队列 | ProviderRouter 按优先级选择 + 自动切换 |
| 3.3 模型映射 | ModelMapper 支持别名→真实模型映射 |
| 3.4 媒体过滤 | 图片能力检测 + 请求体过滤 |
| 3.5 协议适配增强 | Codex Responses ↔ Chat Completions 互转 |

### Phase 4: 独立代理服务（P3，2-3天）

```
目标：cc-switch 风格独立代理进程
```

| 任务 | 说明 |
|---|---|
| 4.1 独立 ProxyServer | hyper HTTP server，动态端口 |
| 4.2 Live Config 接管 | 切换时直接改写 ~/.codex/config.toml |
| 4.3 用量统计 | 请求计数 + token 计算 + 日志 |
| 4.4 用量面板 | UsageDashboard 前端组件 |
| 4.5 流式增强 | 多格式 SSE 支持 + 静默超时 |

---

## 6. 不做的部分

| 功能 | 理由 |
|---|---|
| Claude/Gemini/OpenCode 应用支持 | Codex Go 仅服务于自身 |
| Claude Desktop 配置接管 | 同上 |
| OAuth 托管账号 | 本期复杂度太高 |
| JS 用量查询脚本引擎 | 本期不做，用 Go 原生实现替代 |
| 多 App Profile 切换 | 单体应用不需要 |

---

## 7. 数据迁移

```
迁移路径：
  config.yaml provider.backends[] 
    → providers.json providers[].backends[]
  
  旧：provider: { pool_strategy, backends: [...] }
  新：providers: [{ id, name, backends: [...] }]
  
  兼容：首次启动检测旧格式 → 自动迁移到新格式
```

---

## 8. API 设计

### 8.1 Provider CRUD

```
GET    /api/providers            → 列表（含健康状态摘要）
POST   /api/providers            → 创建
PUT    /api/providers/:id        → 更新
DELETE /api/providers/:id        → 删除
POST   /api/providers/:id/switch → 切换为当前
POST   /api/providers/:id/probe  → 健康探测
```

### 8.2 Provider Presets

```
GET    /api/providers/presets    → 预设模板列表
POST   /api/providers/from-preset → 从预设创建
```

### 8.3 Backends（归属到 Provider）

```
GET    /api/providers/:id/backends       → 某 Provider 的 Backend 列表
POST   /api/providers/:id/backends       → 添加 Backend
PUT    /api/providers/:id/backends/:label → 更新
DELETE /api/providers/:id/backends/:label → 删除
POST   /api/providers/:id/backends/probe → 全量探测
```

---

## 9. 完成标准

- [ ] Provider 列表页展示多供应商卡片（含图标/分类/健康状态）
- [ ] 从预设一键添加供应商
- [ ] Backend 明确归属到特定 Provider
- [ ] 供应商切换即时生效
- [ ] 熔断器 + 故障转移正常工作
- [ ] 构建通过 + go vet 零错误
