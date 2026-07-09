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
}
export interface WSMessage {
  type: 'chunk' | 'tool_call' | 'done' | 'error' | 'session';
  session_id?: string; content?: string;
  tool_name?: string; tool_args?: string; tool_output?: string; error?: string;
}
