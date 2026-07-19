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
  backends?: string[];
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

// === Multi-Provider types (§SPEC-CCSWITCH Phase 2) ===

export interface ProviderSummary {
  id: string;
  name: string;
  icon?: string;
  icon_color?: string;
  category?: string;
  backend_count: number;
  healthy_count: number;
  in_failover_queue: boolean;
  is_current: boolean;
}

export interface ProviderDetail {
  id: string;
  name: string;
  icon?: string;
  icon_color?: string;
  category?: string;
  notes?: string;
  in_failover_queue: boolean;
  backends: BackendConfig[];
  meta?: ProviderMeta;
  created_at: number;
}

export interface ProviderMeta {
  api_format?: string;
  cost_multiplier?: string;
  limit_daily_usd?: string;
  limit_monthly_usd?: string;
  is_full_url?: boolean;
  endpoint_auto_select?: boolean;
  prompt_cache_key?: string;
  max_output_tokens?: number;
  custom_user_agent?: string;
}

export interface ProviderPreset {
  name: string;
  category: string;
  icon?: string;
  icon_color?: string;
  website_url?: string;
  api_key_url?: string;
  base_url: string;
  default_model?: string;
  wire_api?: string;
  description?: string;
}

export interface ProviderListResponse {
  providers: ProviderSummary[];
  current: string;
}

export interface ProviderListDetailResponse {
  providers: ProviderDetail[];
  current: string;
}

export interface SwitchProviderResponse {
  status: string;
  current: string;
  backends: number;
}

export interface ProbeResult {
  label: string;
  status: string;
  error?: string;
}

export interface ProbeResponse {
  provider: string;
  results: ProbeResult[];
  total: number;
}
