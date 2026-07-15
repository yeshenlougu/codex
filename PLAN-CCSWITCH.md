# PLAN-CCSWITCH: cc-switch 对齐实施计划

> 基于 SPEC-CCSWITCH.md，分 4 个 Phase 实施

## Phase 1: Provider 内核重构（P0，预计 3-4 天）

- [ ] **1.1 Provider 数据模型升级**
  - 新增 `config.Provider` 结构体：`ID, Name, Icon, IconColor, Category, Meta, Backends[]`
  - 旧 `BackendConfig` 增加 `ProviderID` 字段
  - 元数据结构：`ProviderMeta`（定价/限额/缓存/推理/端点自动选择）
  - 文件：`internal/config/config.go`

- [ ] **1.2 Provider 独立存储**
  - `~/.codex/providers.json` 存储所有 Provider
  - `store/provider_store.go`：Load/Save/CRUD
  - 首次启动迁移：`config.yaml` → `providers.json`

- [ ] **1.3 Provider CRUD API**
  - `handler_providers.go`：`listProviders / addProvider / updateProvider / deleteProvider`
  - 路由：`/api/providers` GET/POST，`/api/providers/:id` PUT/DELETE
  - 验证：名称唯一、必填字段校验

- [ ] **1.4 Backend 归属重构**
  - 修改 `handler_backends.go`：`/api/providers/:id/backends`
  - Backend CRUD 增加 `provider_id` 校验
  - `listConfigBackends()` → 按当前 Provider 过滤

- [ ] **1.5 供应商热切换**
  - `POST /api/providers/:id/switch` → 设置当前 Provider
  - `pool.go`：支持热重载 Backend 列表（无需重启）
  - TUI 侧更新：实时反映切换后的 Agent 配置

---

## Phase 2: 前端供应商管理（P1，预计 2-3 天）

- [ ] **2.1 ProviderList 组件**
  - 新建 `web/src/pages/settings/ProviderList.tsx`
  - 卡片网格布局：图标 + 名称 + 分类标签 + 健康状态气泡
  - 操作：切换 / 编辑 / 删除 / 探测

- [ ] **2.2 预设系统**
  - 新建 `web/src/lib/presets.ts`：从 cc-switch codexProviderPresets.ts 提取
  - 预设选择器：分类筛选（Official / Third-Party / Partner）
  - "从预设添加"按钮 → 弹窗选择 → 自动填充表单

- [ ] **2.3 Provider 表单增强**
  - 改造 ProviderSettings → ProviderEditor
  - 新增字段：图标选择、颜色选择、分类、备注
  - Backends 列表嵌入（已实现，需适配多 Provider）

- [ ] **2.4 健康状态指示器**
  - 新建 `web/src/components/HealthIndicator.tsx`
  - 三种状态：🟢 healthy / 🟡 degraded / 🔴 unhealthy
  - 实时轮询更新（30s 间隔）

- [ ] **2.5 SettingsPage 重组**
  - Infrastructure 分组：
    - Provider & Backends（Provider 列表 → 点进单个 Provider → Backends）
  - 新增 Provider 列表为入口

---

## Phase 3: 代理层增强（P2，预计 2-3 天）

- [ ] **3.1 CircuitBreaker 熔断器**
  - 新建 `internal/provider/circuit_breaker.go`
  - 三状态：Closed → Open（失败 N 次）→ HalfOpen（等待 T 秒后试探）
  - 集成到 Pool 的请求前检查 + 请求后更新

- [ ] **3.2 故障转移队列**
  - 新建 `internal/provider/router.go` — `ProviderRouter`
  - `SelectProviders()` → 按优先级排序的可用 Backend 列表
  - `FailoverSwitchManager` → 失败自动切换 + 去重
  - API：`POST /api/providers/:id/failover/toggle` 开关

- [ ] **3.3 模型映射器**
  - 新建 `internal/provider/model_mapper.go`
  - 支持别名映射：`claude-sonnet-4-6 → deepseek-v4-pro`
  - API：`PUT /api/providers/:id/backends/:label/models/map`

- [ ] **3.4 媒体过滤**
  - 新建 `internal/provider/media_sanitizer.go`
  - 检测请求是否包含图片 → 筛查 Backend 是否支持 vision
  - 不支持时移除图片内容 + 日志警告

- [ ] **3.5 协议适配**
  - 保留现有 `chat_completions` / `responses` 双模
  - 新增 Anthropic Messages 格式兼容（P3 孵化）

---

## Phase 4: 独立代理服务 + 用量（P3，预计 2-3 天）

- [ ] **4.1 独立 ProxyServer**
  - 参考 cc-switch hyper server 实现
  - 动态端口分配（默认 1978）
  - Graceful shutdown + 连接计数

- [ ] **4.2 用量统计基础**
  - 请求计数器 + 响应大小 + token 估算
  - 存储到 SQLite（`~/.codex/usage.db`）
  - API：`GET /api/usage/summary?from=&to=`

- [ ] **4.3 用量面板前端**
  - 新建 `web/src/components/UsagePanel.tsx`
  - 图表：日请求量 / token 消耗 / 成功率
  - Dashboard 入口（P3 优先级）

---

## 实施策略

| Phase | 策略 |
|---|---|
| P0 | 后端先行，API 就绪后再做前端 |
| P1 | 前端并行，基于 mock 数据开发 |
| P2 | 核心逻辑，逐个模块实现 + 单元测试 |
| P3 | 增量功能，按需推进 |

每个 Phase 结束：
- `make build` 通过
- `go vet ./...` 零错误
- Git 提交（带完整 commit message）
