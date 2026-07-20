# Codex Go — 主 Chat 页面设计描述

## 概述
Codex Go 是一个 AI 编程助手桌面应用。主页面采用三栏布局：左侧边栏 (260px) + 中间对话区 (弹性) + 右侧面板 (可折叠)。

## 色彩系统 (Linear 风格)
- **Light 模式**: 背景 #ffffff, 面板 #f8f9fa, 文字 #1a1d23
- **Dark 模式**: 背景 #0c0c0d, 面板 #0f1011, 文字 #f7f8f8
- **主色调**: #5e6ad2 (Indigo), hover #4a55b8
- **边框**: rgba(0,0,0,0.08) light / rgba(255,255,255,0.06) dark
- **圆角**: 6px 全局, 4px 标签, 8px 卡片

## 字体
- 主字体: Inter, system-ui, -apple-system, sans-serif
- 代码字体: JetBrains Mono, monospace
- 字号: 14px body, 12px secondary, 10px label

---

## 一、顶部标题栏 (TitleBar)
- 高度 36px, 底部 1px 边框
- 左侧: 折叠按钮 (menu-fold 图标) + 应用 logo (16×16px, 圆角 3px) + "Codex Go" 文字 (11px, 600 字重)
- 右侧: 布局切换按钮 + 月亮图标 (暗色模式切换)

## 二、左侧边栏 (LeftSidebar, 260px 宽)
背景色 var(--bg-panel), 右侧 1px 边框

### 2.1 Logo 区
- "Codex" + "Go" 双行文字, 带下拉箭头 (down 图标)

### 2.2 主导航 (仅 1 项)
- "新建任务" 按钮: edit 图标 + 文字, 36px 高, hover 时灰底, active 时 indigo 底色 + 左侧 2px 竖线指示器

### 2.3 项目区 (Projects)
- 标题 "项目" (10px 大写 灰色) + folder-add 按钮
- 空状态: "无项目 — 打开文件夹"

### 2.4 对话历史 (Session History)
- 每个历史条目: 12px 标题 (单行省略) + 10px 元信息
- hover 时出现删除按钮 (红色 hover)
- active 状态: indigo 左边框 + 浅 indigo 背景

### 2.5 底部
- Settings 齿轮图标 (⚙, bulb 图标)
- 版本号 "v1.0" (10px, 灰色)
- 用户头像 + "Pro Plan"

## 三、主对话区 (ChatPage)

### 3.1 空状态欢迎页
- 居中显示
- 狐狸 emoji (🦊, 48px)
- 标题: "What should we build in default?" (22px, 600 字重)
- 提示文字: "💡 Use @agent-name to invoke a specific agent. 📋 Try /spec /plan /tasks /implement /execute"
- 4 张操作建议卡片 (2×2 网格):
  1. Explore & Understand Code — 放大镜图标
  2. Build New Features — 扳手图标
  3. Review & Suggest Changes — 文档图标
  4. Fix Issues & Failures — bug 图标
- 每张卡片: 浅色背景, 6px 圆角, hover 时提升 1px + 阴影

### 3.2 消息气泡
- 用户消息: 右对齐, 深色背景 #21262d, 圆角矩形
- AI 回复: 左对齐, 浅色背景, 左侧 2px indigo 竖线
- 代码块: 深色内嵌矩形, JetBrains Mono 字体
- 审批弹窗: 浮动卡片, "Approve" (indigo) / "Reject" (灰色) 按钮

### 3.3 输入区域
- "+" 按钮 (添加附件/上下文)
- 文本框 placeholder "要求后续变更", 最大高度 160px
- 发送按钮: 36×36px indigo 圆形, paper-plane 图标, disabled 时 0.4 透明度

### 3.4 状态栏
- 底部显示: 🤖 Default Agent · cc-switch / deepseek-v4-pro · 完全访问

## 四、右侧面板 (RightPanel)

### 4.1 图标导轨 (Icon Rail, 48px 宽)
6 个图标按钮 (36×36px):
- 代码审查 (review)
- 终端 (terminal)
- 浏览器 (browser)
- 文件 (files) — 当前选中, indigo 高亮
- 子任务 (sidetasks)
- 每个 hover 时灰色背景, active 时 indigo 底色

### 4.2 文件浏览器面板
- 路径显示: /home/ubuntu/app/codex
- 树形目录结构: .git, .github, cmd, desktop, dist, internal, node_modules, redesigned, screenshots, skills, test-results, web
- 文件: .gitignore, AGENTS.md, CHANGELOG.md, go.mod 等
- 每个节点可折叠/展开

---

## 交互规范
- 卡片 hover: transform translateY(-1px) + box-shadow
- 按钮: 0.12s transition, primary 按钮带 indigo 光晕
- 输入框 focus: 2px indigo 发光环
- 模态框 backdrop: blur(2px)
- 切换开关: 0.2s 平滑过渡
- 标签: 11px, 4px 圆角, 6px padding
- 滚动条: 6px 宽, 半透明

## 暗色模式
- 所有背景反转为深色 (#0c0c0d → #f7f8f8)
- 狐狸 emoji 带 indigo 发光滤镜
- 代码块背景更深
- 卡片阴影更重 (rgba(0,0,0,0.3))
