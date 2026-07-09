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
export interface ChatResponse { session_id: string; content: string; turn_count: number; tool_calls: number; }
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

// Backend pool types
export interface BackendStatus {
  label: string;
  provider: string;
  base_url: string;
  weight: number;
  health: string;
  failures: number;
  successes: number;
  last_fail: string;
  last_success: string;
  cooldown: string;
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
