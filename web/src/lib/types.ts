export interface Message {
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  tool_calls?: ToolCall[];
  tool_call_id?: string;
}
export interface ToolCall {
  id: string;
  type: 'function';
  function: { name: string; arguments: string };
}
export interface SessionSummary {
  id: string; created_at: string; updated_at: string;
  model: string; provider: string; title: string; msg_count: number;
}
export interface Session {
  id: string; created_at: string; updated_at: string;
  model: string; provider: string; title: string; messages: Message[];
}
export interface ChatResponse { session_id: string; content: string; turn_count: number; tool_calls: number; responding_agent?: string; }
export interface Config {
  provider: string; model: string; base_url: string;
  api_key_masked: string; reasoning_effort: string; max_turns: number;
  tool_count: number; active_sessions: number;
  pool_strategy: string; backend_count: number;
  system_prompt: string; wire_api: string;
}
export interface WSMessage {
  type: 'chunk' | 'tool_call' | 'done' | 'error' | 'session';
  session_id?: string; content?: string;
  tool_name?: string; tool_args?: string; tool_output?: string; error?: string;
}

// === Agent Profile types ===

export interface AgentProfile {
  name: string;
  description: string;
  avatar: string;
  is_builtin: boolean;
  model: AgentModelConfig;
  agent: AgentBehaviorConfig;
  tools: AgentToolsConfig;
  mcp?: AgentMCPConfig;
  skills?: AgentSkillsConfig;
  plugins?: AgentPluginsConfig;
  hooks?: AgentHooksConfig;
  subagents?: SubAgentRef[];
}

export interface AgentModelConfig {
  provider: string;
  model: string;
  reasoning_effort?: string;
}

export interface AgentBehaviorConfig {
  max_turns: number;
  system_prompt: string;
}

export interface AgentToolsConfig {
  shell: boolean;
  file_read: boolean;
  file_edit: boolean;
}

export interface AgentMCPConfig {
  servers: AgentMCPServer[];
}

export interface AgentMCPServer {
  name: string;
  command: string;
  args: string[];
  enabled: boolean;
}

export interface AgentSkillsConfig {
  dirs: string[];
}

export interface AgentPluginsConfig {
  dirs: string[];
}

export interface AgentHooksConfig {
  pre_tool: string;
  post_tool: string;
  on_session_start: string;
  on_session_end: string;
  post_tool_message: string;
}

export interface SubAgentRef {
  name: string;
  description: string;
}

// Backend pool types
export interface ModelInfo {
  name: string;
  type: string;
  auto: boolean;
}

export interface BackendStatus {
  label: string;
  base_url: string;
  weight: number;
  health: string;
  failures: number;
  successes: number;
  last_fail: string;
  last_success: string;
  cooldown: string;
  models: ModelInfo[];
  models_grouped?: Record<string, ModelInfo[]>;
}

export interface CapabilityInfo {
  type: string;
  label: string;
  icon: string;
  desc: string;
  enabled: boolean;
  backends?: BackendStatus[];
}
export interface BackendPoolStatus {
  backends: BackendStatus[];
  strategy: string;
  total: number;
  healthy: number;
}
export interface BackendConfig {
  key: string;
  label: string;
  base_url: string;
  provider?: string;
  weight: number;
}
export interface ImportResult {
  status: string;
  backends: BackendConfig[];
  strategy: string;
  count: number;
  file?: string;
}
