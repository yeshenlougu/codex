# Codex Go — 各页面功能完整说明

---

## 01. Chat 主页

### 三栏布局
| 区域 | 功能 |
|------|------|
| **左侧边栏 (260px)** | 新建任务、项目区、对话历史、设置入口 |
| **中间对话区** | 欢迎页/消息流/输入框 |
| **右侧面板** | 文件浏览器/终端/浏览器/代码审查/子任务 |

### 左侧边栏功能
- **新建任务**: 创建新对话会话，生成时间戳 SessionID
- **项目区**: 显示已打开项目，支持 folder-add 打开新文件夹
- **对话历史**: 列出历史会话，点击恢复对话，hover 显示删除按钮
- **⚙ 图标**: 进入 Settings 页面
- **v1.0**: 版本号显示

### 中间对话区
- **欢迎页**: 狐狸 emoji + "What should we build in default?"
- **命令提示**: `@agent-name` 调用指定 Agent，`/spec /plan /tasks /implement /execute` 快速命令
- **快速开始卡片**: 4 张操作建议卡片 (Explore / Build / Review / Fix)
- **消息流**: 用户气泡(右对齐) + AI 回复(左对齐 indigo 左边框) + 代码块 + 审批弹窗
- **输入框**: "+" 按钮(添加上下文) + 文本输入 "要求后续变更" + 发送按钮
- **状态栏**: Agent 名称 · Provider / Model · 访问级别

### 右侧面板
- **图标导轨 (48px)**: 6 个工具图标 (review/terminal/browser/files/sidetasks)
- **文件浏览器**: 树形目录结构，展开/折叠，点击切换工作目录
- **终端**: 绿色 on black 嵌入式终端
- **浏览器预览**: Web 页面渲染
- **代码审查**: Diff 视图
- **子任务**: 并行子任务面板

---

## 02. Settings → General（常规）

### 左侧子导航
- Personal: General / Agents
- Infrastructure: Providers / Tools / Plugins
- Activity: Scheduled / Usage
- 搜索框: 搜索设置项

### 功能项
| 设置项 | 功能 | 当前值 |
|--------|------|--------|
| **模型 / Model** | 默认使用的模型名称 | `gpt-5.6-sol` |
| **推理强度** | 模型推理深度 (low/medium/high/xhigh) | `High` |
| **最大回合数** | Agent 单次对话最大交互轮数 | `60` |
| **系统提示词** | 可编辑的 Agent 系统级行为指令 | 完整 Codex 角色定义 |

---

## 03. Settings → Agents

### 功能
- **Agent 配置**: 多 Agent 协作管理
- **创建 Agent**: "+ 创建 Agent" 按钮 → 弹出表单配置新 Agent
- **Agent 卡片**:
  - 机器人图标 + Agent 名称
  - `built-in` 标签 (内置标记)
  - Provider / Model 胶囊标签
  - 最大回合数限制显示
  - **Edit** 按钮: 编辑 Agent 名称/系统提示词
  - **Clone** 按钮: 复制 Agent 配置
- **空状态**: 无 Agent 时引导创建

---

## 04. Settings → Providers

### 功能
- **供应商管理**: 管理 AI 供应商 — 多供应商支持 + 一键切换
- **添加供应商**: 预设列表(OpenAI/Anthropic/DeepSeek/Groq) 或自定义
- **Provider 卡片**:
  - Emoji 图标 + 供应商名称
  - 分类标签 (合作/官方/第三方)
  - `当前使用` 高亮标签 (indigo 左边框)
  - **HealthStatusIndicator**: 绿色 Healthy / 黄色 Degraded / 红色 Unhealthy
  - 端点计数: `1/1 端点`
  - **探测** 按钮: Thunderbolt 图标, 触发健康探测 `/models`
  - **切换** 按钮: 切换当前激活的供应商
  - **删除** 按钮: Popconfirm 确认后删除
- **配置 Modal**: API Key 输入 + Base URL + 预设选择
- **故障转移队列**: 标记参与自动故障转移
- **活跃标记**: CheckCircle 图标

---

## 05. Settings → Tools

### 功能
- **Tools Registry**: 管理 Agent 可用工具 — 启用/禁用 + 审批策略
- **统计卡片**: 总计 12 / 系统工具 5 / 可选工具 7 / 已启用 12
- **筛选器**: 下拉 "全部" / 搜索框 "搜索工具名或描述..."

### 工具表格
| 工具 | 类别 | 风险 | 审批 | 启用 |
|------|------|------|------|------|
| `read_file` | system | low | Off | On |
| `write_file` | system | low | Off | On |
| `edit_file` | system | medium | Off | On |
| `grep` | system | low | Off | On |
| `ls` | system | low | Off | On |
| `shell` | optional | high | On | On |
| `git` | optional | medium | Off | On |
| `web_fetch` | optional | medium | Off | On |

- 系统工具灰色背景不可修改
- **Approval 开关**: 控制使用该工具是否需要用户审批
- **Enabled 开关**: 控制工具是否可用
- **Risk 标签**: low(绿) / medium(黄) / high(红)

---

## 06. Settings → Plugins

### Tab 结构
| Tab | 功能 |
|-----|------|
| **插件** | 插件市场 + 已安装列表 |
| **技能** | GitHub Skills 管理 |
| **MCP** | MCP 服务器管理 |

### 插件 Tab 功能
- 搜索 "搜索插件"
- 分类: 精选/生产力/开发/通讯/全部
- **已安装**: Computer Use, Visualize 卡片 + "已安装" 标签
- **可安装**: Chrome (浏览器控制) + "安装" 按钮

### Skills Tab 功能
- GitHub 仓库列表 (4 repos)
- discoverSkills 发现新技能
- installSkill / uninstallSkill 安装卸载
- checkSkillUpdates 更新检测

### MCP Tab 功能
- MCP 服务器列表 (名称 + 启动命令 + 状态 Badge)
- start/stop/restart 按钮
- 预设下拉 + "添加 MCP 服务器" 按钮

---

## 07. Settings → Scheduled

### 功能
- **已安排的任务**: 让 Codex 安排任务、设置提醒或监测更新
- **"+ 创建"**: 新建定时任务 (名称/Cron/Agent/启停)

### 建议任务模板
| 任务 | 标签 | 描述 | 时间 |
|------|------|------|------|
| 每日简报 | 每日 | 日历、未读邮件、优先事项摘要 | 工作日 8:00 |
| 每周回顾 | 每周 | 整理工作为简明的状态更新 | 星期五 16:00 |
| 跟进监控 | 监控 | 查看邮箱和日历活动，标记需关注事项 | 工作日 9:00 |
| 邮件摘要 | 每日 | 汇总未读邮件按优先级排序 | 每日 7:30 |

### 自定义任务管理
- 表格列: Name / Cron / Agent / Status(开关) / Last Run / Next Run / Actions
- 执行日志面板: 最近运行记录 (时间/状态/输出预览)
- 展开查看完整日志

---

## 08. Settings → Usage

### 功能
- **Usage Dashboard**: API 调用用量统计与成本估算
- **日期选择器**: "最近 30 天" 下拉

### 摘要卡片
| 指标 | 说明 |
|------|------|
| **Total Requests** | 总请求数 + 绿色趋势箭头 |
| **Input Tokens** | 输入 Token 总量 |
| **Output Tokens** | 输出 Token 总量 |
| **Estimated Cost** | 估算费用 `$0.0000` |

### 每日用量汇总 表格
- 列: Provider / Model / Requests / Input / Output / Cost
- 空状态: "暂无用量数据，对话开始后自动记录"

### 最近请求 表格
- 列: Model / Input / Output / Est Cost
- 空状态: "暂无调用记录"
- 数据源自异步 logUsage() → usage_logs 表

---

## 全局交互规范

| 效果 | 说明 |
|------|------|
| 卡片 hover | `translateY(-1px)` + box-shadow 提升 |
| Primary 按钮 | indigo #5e6ad2 背景 + 白色文字 + 光晕阴影 |
| 输入框 focus | 2px indigo 发光环 |
| 健康状态 healthy | 绿色脉冲动画 |
| Loading 骨架屏 | shimmer 1.5s 渐变动画 |
| 模态框 | backdrop-filter blur(2px) |
| 切换开关 | 0.2s 平滑过渡 |
| 标签 | 11px, 4px 圆角, 6px padding |
| 全局圆角 | 6px |
