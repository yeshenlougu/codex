# Codex Go Port — Specification & Roadmap

## 原版功能全量盘点（97 crates，按领域分类）

### 1. 核心 Agent 循环 `[已做基础版]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| think→act→observe 循环 | core | ✅ 已实现 |
| 流式响应处理 | core | ✅ 已实现 |
| 工具调用 + 结果注入 | core | ✅ 已实现 |
| 上下文压缩 | core | ❌ |
| 多模态输入（图片） | core | ❌ |
| 回滚/重试 | rollout | ❌ |
| Prompt 缓存 | protocol | ❌ |
| Reasoning effort | core | ✅ |

### 2. LLM Provider `[已做基础版]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| OpenAI Chat Completions | model-provider | ✅ |
| OpenAI Responses API | responses-api-proxy | ❌ |
| Anthropic Messages | model-provider | ❌ |
| Ollama 本地模型 | ollama | ❌ |
| LM Studio 本地模型 | lmstudio | ❌ |
| 模型列表发现 | models-manager | ❌ |
| 多 key 池 + 故障转移 | credential_pool | ❌ |

### 3. 工具系统 `[已做基础版]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| shell 命令执行 | shell-command | ✅ |
| read_file 读取文件 | file-system | ✅ |
| edit_file 查找替换 | apply-patch | ✅ |
| write_file 写入文件 | file-system | ❌ (刚加了但未集成) |
| grep 内容搜索 | file-search | ❌ (刚加了但未集成) |
| ls 目录列表 | file-system | ❌ (刚加了但未集成) |
| git 操作 | git-utils | ❌ (刚加了但未集成) |
| web_fetch 网页抓取 | (无对应) | ❌ (刚加了但未集成) |
| 文件监听 | file-watcher | ❌ |
| 上下文片段 | context-fragments | ❌ |
| 动态工具注册 | tools | ❌ |
| 并行工具执行 | core | ❌ |

### 4. 执行 & 沙箱 `[完全未做]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| 命令执行引擎 | exec | ❌ |
| 执行服务器 | exec-server | ❌ |
| 沙箱隔离 | sandboxing | ❌ |
| Linux bubblewrap | linux-sandbox/bwrap | ❌ |
| Windows 沙箱 | windows-sandbox-rs | ❌ |
| macOS seatbelt | (implied) | ❌ |
| 执行策略/审批 | execpolicy | ❌ |
| 权限提升 | shell-escalation | ❌ |
| 进程加固 | process-hardening | ❌ |

### 5. CLI / TUI `[已做基础版]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| 单次问答 | cli | ✅ --prompt |
| 交互模式 | cli | ✅ 交互对话 |
| 管道模式 | cli | ✅ 管道输入 |
| 会话持久化 | message-history | ❌ (刚加了 store) |
| 会话列表/恢复 | thread-store | ❌ (刚加了 store) |
| 全功能 TUI | tui | ❌ |
| 配置 CLI 子命令 | cli | ❌ --config |

### 6. App Server（守护进程）`[完全未做]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| JSON-RPC 服务 | app-server | ❌ |
| 后台守护进程 | app-server-daemon | ❌ |
| 多客户端连接 | app-server-transport | ❌ |
| WebSocket 通信 | websocket-client | ❌ |
| 自动更新 | app-server-daemon | ❌ |

### 7. MCP 协议 `[完全未做]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| MCP 客户端 | codex-mcp | ❌ |
| MCP 服务端 | mcp-server | ❌ |
| MCP 工具发现 | rmcp-client | ❌ |
| MCP OAuth 登录 | codex-mcp | ❌ |

### 8. 认证 `[部分已做]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| API Key 认证 | login | ✅ env/config |
| ChatGPT OAuth | chatgpt | ❌ |
| 密钥环存储 | keyring-store | ❌ |
| AWS 认证 | aws-auth | ❌ |

### 9. Skills / 插件系统 `[完全未做]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| Skill 定义 | skills | ❌ |
| 内置 Skill 集 | core-skills | ❌ |
| 插件接口 | plugin | ❌ |
| 内置插件 | core-plugins | ❌ |

### 10. 配置管理 `[已做基础版]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| YAML 配置 | config | ✅ |
| 环境变量 fallback | config | ✅ |
| 配置热加载 | config | ❌ |
| 云端配置 | cloud-config | ❌ |

### 11. 记忆/持久化 `[部分已做]`
| 功能 | Rust crate | Go 状态 |
|------|-----------|---------|
| 对话历史 | message-history | ❌ (刚加了 store) |
| 线程存储 | thread-store | ❌ (刚加了 store) |
| 记忆系统 | memories | ❌ |
| Agent 状态图 | agent-graph-store | ❌ |

### 12. 其他领域 `[完全未做]`
| 功能 | 说明 |
|------|------|
| analytics/feedback/otel | 遥测 + 反馈 |
| realtime-webrtc | 实时语音 |
| code-mode | 代码编辑模式 |
| cloud-tasks | 云端任务 |
| external-agent-migration | 外部 Agent 导入 |
| Python/TS SDK | 开发 SDK |

---

## 分阶段路线图

### Phase 1: 可用 Agent（当前 + 本周） `[进行中]`
目标：一个日常能用的命令行编程助手

- [x] Agent 核心循环 + 流式输出
- [x] shell / read_file / edit_file 工具
- [x] OpenAI 兼容 provider
- [ ] 会话持久化（save/load/list/delete）
- [ ] 扩展工具集成（write_file、grep、ls、git、web_fetch）
- [ ] CLI --resume / --list-sessions / --delete-session
- [ ] 多 key 池 + 自动故障转移
- [ ] Token 用量统计

### Phase 2: 健壮 Agent（下周）
目标：安全、可靠、可调试

- [ ] 审批机制：敏感操作（rm -rf、git push --force、修改 ~/.ssh）需确认
- [ ] 沙箱执行：bubblewrap 隔离 shell 命令
- [ ] 更好的错误处理 + 重试
- [ ] 日志系统
- [ ] 配置文件子命令 (`codex-go config set/get`)
- [ ] 模型列表子命令 (`codex-go models`)

### Phase 3: 生态 Agent（后续）
目标：可扩展、可编程

- [ ] MCP 客户端协议
- [ ] MCP 工具自动发现
- [ ] 插件系统（Go plugin 或 Yaml 定义）
- [ ] Skill 系统（类似 Hermes skills）
- [ ] WebSocket 通信
- [ ] 自定义 provider 接口

### Phase 4: 完整 Agent（远期）
目标：对标原版 Codex CLI

- [ ] TUI 界面（Bubble Tea）
- [ ] App Server 守护进程
- [ ] Anthropic provider
- [ ] Ollama / 本地模型
- [ ] 多模态输入（图片）
- [ ] 上下文自动压缩
- [ ] 记忆系统
- [ ] 文件监听 + 自动触发

---

## 当前仓库状态

```
分支: golang
模块: github.com/yeshenlougu/codex
Go 版本: 1.22+
外部依赖: gopkg.in/yaml.v3 (唯一)

目录结构:
cmd/codex/main.go          # CLI 入口
internal/
  agent/agent.go           # Agent 核心循环
  config/config.go         # 配置管理
  provider/openai.go       # LLM 客户端
  tool/
    tool.go                # 接口 + Registry
    tools.go               # shell, read_file, edit_file
    helpers.go             # 文件读写辅助
    extended.go            # write_file, grep, ls, git, web_fetch (未集成)
  session/store.go         # 会话持久化 (未集成)

待集成:
- session.Store 未连入 Agent/CLI
- extended tools 未注册到 tool.Registry
- 多 key 池逻辑完全未写
```
