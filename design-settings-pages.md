# Codex Go — Settings 页面设计描述

## 通用结构
Settings 采用左右分栏：左侧 220px 子导航 + 右侧弹性内容区。顶部标题栏与主页面共用。

### Settings 子导航 (左侧 220px)
- 背景 var(--bg-panel), 右侧 1px 边框
- 顶部: "返回" 按钮 (arrow-left 图标 + 文字, 12px)
- 搜索框: 放大镜前缀图标, placeholder "搜索设置...", 6px 圆角
- 分类标题: 10px 大写加粗, letter-spacing 0.08em, 灰色
- 子项: 图标 + 文字, 12px, 6px 圆角, hover 灰底, active indigo 底色

### 分类结构
```
PERSONAL
  ├── General (setting 图标)
  └── Agents (team 图标)
INFRASTRUCTURE
  ├── Providers (api 图标)
  ├── Tools (tool 图标)
  └── Plugins (appstore 图标)
ACTIVITY
  ├── Scheduled (clock-circle 图标)
  └── Usage (bar-chart 图标)
```

---

## S1. General (常规) 页面

### 标题
"常规" (14px 加粗)

### Agent 行为 区域
- **模型 / Model**: 描述 "默认使用的模型名称" (11px 灰色)
  - 输入框值: "gpt-5.6-sol", 13px 字体, 6px 圆角, 1px 边框
- **推理强度 / Reasoning effort**: 描述 "模型推理深度 (low / medium / high / xhigh)"
  - 下拉选择框, 当前值 "High", 14px 下箭头
- **最大回合数 / Maximum turns**: 描述 "Agent 单次对话最大交互轮数"
  - 数字输入 60, 带 +/- 步进按钮

### 系统提示词 / System Prompt
- 大文本框, 占满宽度
- 内容示例: "You are Codex, an AI coding agent that runs in the terminal..."
- 等宽代码区域用 JetBrains Mono 字体显示
- 编辑模式 placeholder "输入自定义系统提示词..."

---

## S2. Agents 页面

### 标题区
- 标题 "Agent 配置 — 多 Agent 协作" (18px 加粗)
- 副标题 "Each agent can use a different model, provider, and tools configuration" (12px 灰色)
- 右上角: "+ 创建 Agent" 蓝色按钮

### Agent 卡片
- 全宽卡片, 6px 圆角, 1px 边框
- 卡片头: 机器人小图标 + agent 名称 + 蓝色 "built-in" 标签
- 右侧: Edit (铅笔图标) + Clone (复制图标) 按钮
- 内容体: "No system prompt configured" (斜体灰色)
- 底部标签: "cc-switch / gpt-4o" (蓝色胶囊) + "max 60 turns" (紫色胶囊)

### 空状态
- 无 agent 时显示 Empty 组件
- 引导文字 + "创建 Agent" 按钮

---

## S3. Providers 页面

### 标题区
- 标题 "供应商管理" (18px 加粗)
- 副标题 "管理 AI 供应商 — 多供应商支持 + 一键切换" (12px 灰色)
- 右上角: "+ 添加供应商" 蓝色按钮

### Provider 卡片 (当前唯一: cc-switch)
- 单 provider 时全宽, 多 provider 时半宽 (Col span)
- 6px 圆角, hover 提升 1px, 当前 active 左侧 3px indigo 竖线
- 标题行: emoji 图标 (🔌) + 名称 "cc-switch" (加粗)
- 标签: "合作" (黄色 pill) + "当前使用" (indigo pill)
- 右上角: HealthStatusIndicator 组件
  - Healthy: 绿色圆点 + "Healthy" 标签
  - 端点计数: "1/1 端点" (11px 灰色文字)
- 操作按钮行:
  - "切换到此供应商" (SwapOutlined)
  - "探测" (ThunderboltOutlined, 健康探测按钮)
  - 删除 (DeleteOutlined, 红色, 带 Popconfirm)
- 卡片体内:
  - "故障转移队列" 标签 (SafetyCertificateOutlined + 蓝色)
  - "活跃" 标签 (CheckCircleOutlined + indigo)

### 添加供应商 Modal
- 下拉选择预设 (OpenAI, Anthropic, DeepSeek, Groq 等)
- API Key 密码输入框
- 自定义名称输入框
- 预设信息卡片 (网站 URL + API Key 获取链接)

---

## S4. Tools 页面

### 标题区
- 标题 "Tools Registry"
- 副标题 "管理 Agent 可用工具：启用/禁用、审批策略"

### 摘要卡片 (4 个统计)
- 总计 12 | 系统工具 5 | 可选工具 7 | 已启用 12
- 小卡片, 居中对齐, 数字大号加粗

### 筛选器
- 下拉: "全部" (系统工具/可选工具)
- 搜索框: "搜索工具名或描述..."

### 工具表格
- 列: Name | Description | Category | Risk | Approval | Enabled
- 系统工具 (灰色背景, 不可修改): read_file, write_file, edit_file, grep, ls
- 可选工具: shell (高风险, 审批开启), git, web_fetch, git_worktree, image_gen, browser, code_review
- Risk 标签: low=绿色, medium=黄色, high=红色
- Approval 开关: 控制是否需要审批
- Enabled 开关: 控制工具是否可用

---

## S5. Plugins 页面

### 标题区
- 三个 Tab: 插件 | 技能 | MCP (Tabs 组件)
- 默认 "插件" Tab 选中

### 插件 Tab
- 标题 "插件", 副标题 "在你常用的工具中与 Codex 协作"
- 搜索框 "搜索插件"
- 分类筛选: 精选 | 生产力 | 开发 | 通讯 | 全部 (Tag 按钮组)
- 已安装区: Computer Use (2个), Visualize — 卡片带 "已安装" 绿色标签
- 可选安装区: Chrome (让 Codex 控制浏览器) — blue "安装" 按钮
- 每个插件卡片: 图标 + 名称 + 描述 + 安装状态按钮

### Skills Tab
- GitHub 仓库列表 (4 repos)
- 已安装 skills 列表
- discoverSkills / installSkill / uninstallSkill 按钮
- 更新检测 Badge

### MCP Tab
- MCP 服务器列表 (server 名称 + 启动命令 + 状态 Badge)
- start/stop/restart 按钮
- 预设下拉 + "添加 MCP 服务器" 按钮

---

## S6. Scheduled 页面

### 标题区
- 标题 "已安排的任务"
- 副标题 "让 Codex 安排任务、设置提醒或监测更新"
- 右上角: "+ 创建" 蓝色按钮

### 建议任务卡片 (4 张)
- 每张卡片: 左侧彩色图标 + 标题 + 描述 + 时间 + 右侧箭头
- 卡片带 tag 标签 (每日/每周/监控)
- 1. 每日简报 — 日历图标, "工作日 8:00"
- 2. 每周回顾 — 趋势图标, "星期五 16:00"
- 3. 跟进监控 — 铃铛图标, "工作日 9:00"
- 4. 邮件摘要 — 信封图标, "每日 7:30"

### 自定义任务 (如有)
- 表格: Name | Cron | Agent | Status | Last Run | Next Run | Actions
- 启停开关, 编辑/删除图标
- 执行日志面板

---

## S7. Usage 页面

### 标题区
- 标题 "Usage Dashboard"
- 副标题 "API 调用用量统计与成本估算"
- 日期选择器: "最近 30 天" 下拉

### 摘要卡片 (4 个统计)
- Total Requests: 0 (大号数字 + 绿色趋势箭头)
- Input Tokens: 0
- Output Tokens: 0
- Estimated Cost: $0.0000
- 卡片间距 16px, 带虚线边框

### 每日用量汇总 表格
- 列: Provider | Model | Requests | Input | Output | Cost
- 空状态: "暂无用量数据，对话开始后自动记录"
- 行 hover 灰色背景

### 最近请求 表格
- 列: Model | Input | Output | Est Cost
- 空状态: "暂无调用记录"
- 行 hover 灰色背景

---

## 全局交互规范 (同主页面)
- 卡片 hover: transform translateY(-1px) + box-shadow
- Primary 按钮: indigo 背景 + 白色文字 + 1px 光晕阴影
- 输入框 focus: 2px indigo 发光环 (box-shadow)
- 1px 细线边框, 所有圆角 6px
- 标签/胶囊: 11px, 4px 圆角, 6px 水平 padding
- 健康状态指示器: 带颜色圆点 + 文字 + 脉冲动画 (healthy 状态)
- Loading 骨架屏: shimmer 渐变动画, 1.5s 周期
- 暗色模式: 所有背景反转为深色, 卡片阴影更重
