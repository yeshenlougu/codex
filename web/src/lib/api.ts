import type { SessionSummary, Session, ChatResponse, Config, BackendPoolStatus, BackendConfig, ImportResult, WSMessage } from './types';

const BASE = '/api';

async function req<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(BASE + url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  return res.json();
}

// Health
export async function getHealth(): Promise<{ status: string }> {
  return req('/health');
}

// Sessions
export async function listSessions(): Promise<{ sessions: SessionSummary[] }> {
  return req('/sessions');
}
export async function getSession(id: string): Promise<Session> {
  return req(`/sessions/${id}`);
}
export async function deleteSession(id: string): Promise<{ deleted: string }> {
  return req(`/sessions/${id}`, { method: 'DELETE' });
}
export async function createSession(id?: string): Promise<{ session_id: string }> {
  return req(`/sessions/${id || 'new'}`, { method: 'POST' });
}

// Chat
export async function sendMessage(message: string, sessionId: string): Promise<ChatResponse> {
  return req('/chat', { method: 'POST', body: JSON.stringify({ message, session_id: sessionId, stream: false }) });
}
export async function streamMessage(
  message: string, sessionId: string,
  onChunk: (text: string) => void, onDone: (fullText: string) => void, onError: (err: string) => void,
  agentName?: string,
) {
  const res = await fetch(BASE + '/chat', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message, session_id: sessionId, stream: true, agent_name: agentName || undefined }),
  });
  if (!res.ok) { const err = await res.json().catch(() => ({ error: res.statusText })); onError(err.error || `HTTP ${res.status}`); return; }
  const reader = res.body?.getReader(); if (!reader) { onError('No response body'); return; }
  const decoder = new TextDecoder(); let buffer = '', fullText = '';
  while (true) {
    const { done, value } = await reader.read(); if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n'); buffer = lines.pop() || '';
    for (const line of lines) {
      if (!line.startsWith('data: ')) continue;
      try {
        const msg: WSMessage = JSON.parse(line.slice(6));
        if (msg.type === 'chunk' && msg.content) { fullText += msg.content; onChunk(msg.content); }
        else if (msg.type === 'error') { onError(msg.error || 'Unknown error'); return; }
      } catch { /* skip */ }
    }
  }
  onDone(fullText);
}

// Config
export async function getConfig(): Promise<Config> { return req('/config'); }
export async function updateConfig(updates: Record<string, unknown>): Promise<{ status: string }> {
  return req('/config', { method: 'PUT', body: JSON.stringify(updates) });
}

// Backends
export async function getBackends(): Promise<BackendPoolStatus> { return req('/backends'); }
export async function addBackend(be: BackendConfig): Promise<{ status: string; backend: BackendConfig }> {
  return req('/backends', { method: 'POST', body: JSON.stringify(be) });
}
export async function deleteBackend(label: string): Promise<{ status: string }> {
  return req(`/backends/${encodeURIComponent(label)}`, { method: 'DELETE' });
}
export async function updateBackend(label: string, be: Partial<BackendConfig>): Promise<{ status: string }> {
  return req(`/backends/${encodeURIComponent(label)}`, { method: 'PUT', body: JSON.stringify(be) });
}
export async function importBackendsFile(file: File): Promise<ImportResult> {
  const form = new FormData(); form.append('file', file);
  const res = await fetch(BASE + '/backends/import', { method: 'POST', body: form });
  if (!res.ok) { const err = await res.json().catch(() => ({ error: res.statusText })); throw new Error(err.error || `HTTP ${res.status}`); }
  return res.json();
}
export async function probeBackends(): Promise<{ status: string; probed: number; message: string }> {
  return req('/backends/probe', { method: 'POST' });
}
export function getBackendsExportUrl(): string { return BASE + '/backends/export'; }

// Pet
export async function getPetState(): Promise<{ status: string; agents: number; thinking: number }> {
  return req('/pet-state');
}

// Files
export async function listFiles(dirPath?: string): Promise<{ path: string; files: FileEntry[] }> {
  const q = dirPath ? `?path=${encodeURIComponent(dirPath)}` : '';
  return req(`/files${q}`);
}
export async function readFileContent(filePath: string): Promise<{ path: string; size: number; binary: boolean; content: string; lines?: number }> {
  return req(`/files/content?path=${encodeURIComponent(filePath)}`);
}
export async function diffFiles(a: string, b: string): Promise<{ file_a: string; file_b: string; diff: DiffLine[] }> {
  return req(`/files/diff?a=${encodeURIComponent(a)}&b=${encodeURIComponent(b)}`);
}

interface FileEntry { name: string; path: string; is_dir: boolean; size: number; mod_time: string }
interface DiffLine { type: 'same' | 'add' | 'remove'; content: string; line_a?: number; line_b?: number }

// Model capabilities (auto-discovery)
export async function getCapabilities(): Promise<{ capabilities: import('./types').CapabilityInfo[] }> {
  return req('/capabilities');
}

export async function getBackendModels(): Promise<{ backends: import('./types').BackendStatus[] }> {
  return req('/backends/models');
}

// === Agent Profiles ===

export async function listAgents(): Promise<{ agents: import('./types').AgentProfile[] }> {
  return req('/agents');
}

export async function getAgent(name: string): Promise<import('./types').AgentProfile> {
  return req(`/agents/${encodeURIComponent(name)}`);
}

export async function createAgent(name: string, cloneFrom?: string): Promise<import('./types').AgentProfile> {
  return req('/agents', { method: 'POST', body: JSON.stringify({ name, clone_from: cloneFrom || 'default' }) });
}

export async function updateAgent(name: string, updates: Partial<import('./types').AgentProfile>): Promise<{ updated: string }> {
  return req(`/agents/${encodeURIComponent(name)}`, { method: 'PUT', body: JSON.stringify(updates) });
}

export async function deleteAgent(name: string): Promise<{ deleted: string }> {
  return req(`/agents/${encodeURIComponent(name)}`, { method: 'DELETE' });
}

export async function cloneAgent(sourceName: string, newName: string): Promise<import('./types').AgentProfile> {
  return req(`/agents/${encodeURIComponent(sourceName)}/clone`, { method: 'POST', body: JSON.stringify({ name: newName }) });
}

// === Spec/Plan/Tasks Workflow ===

export interface Task {
  number: number;
  content: string;
  completed: boolean;
  phase: string;
}

export async function getTasks(): Promise<{ content: string; tasks: Task[] }> {
  return req('/tasks');
}

export async function implementTask(taskNum: number): Promise<{ content: string }> {
  return req(`/implement/${taskNum}`, { method: 'POST' });
}

// executeTask runs a task from PLAN.md through the agent with SSE streaming.
// Returns SSE reader + the fetch response for abort.
export async function executeTask(
  taskNum: number,
  sessionId: string,
  agentName?: string,
  onChunk?: (chunk: string) => void,
  onDone?: (result: string) => void,
  onError?: (err: string) => void,
): Promise<Response> {
  const res = await fetch(BASE + `/execute/${taskNum}?session_id=${encodeURIComponent(sessionId)}${agentName ? '&agent_name=' + encodeURIComponent(agentName) : ''}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    onError?.(err.error || `HTTP ${res.status}`);
    return res;
  }
  // Read SSE stream
  const reader = res.body?.getReader();
  if (!reader) { onError?.('No response body'); return res; }
  const decoder = new TextDecoder();
  let buffer = '';
  (async () => {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue;
        try {
          const msg = JSON.parse(line.slice(6));
          if (msg.type === 'chunk') onChunk?.(msg.content || '');
          else if (msg.type === 'done') onDone?.(msg.content || '');
          else if (msg.type === 'task_info') onChunk?.(`\n📋 Task ${msg.task_num}: ${msg.content}\n`);
          else if (msg.type === 'error') onError?.(msg.error || 'Unknown error');
        } catch { /* skip */ }
      }
    }
  })();
  return res;
}

// approveCheck resolves a pending sandbox approval check.
export async function approveCheck(checkId: number, approved: boolean): Promise<{ status: string; check_id: string; action: string }> {
  return req(`/approve/${checkId}`, {
    method: 'POST',
    body: JSON.stringify({ check_id: checkId, approved }),
  });
}

export async function addAgentToSession(sessionId: string, agentName: string): Promise<{ session_id: string; agent: string; status: string }> {
  return req(`/sessions/${encodeURIComponent(sessionId)}/agents`, { method: 'POST', body: JSON.stringify({ agent_name: agentName }) });
}

export async function removeAgentFromSession(sessionId: string, agentName: string): Promise<{ session_id: string; agent: string; status: string }> {
  return req(`/sessions/${encodeURIComponent(sessionId)}/agents/${encodeURIComponent(agentName)}`, { method: 'DELETE' });
}

export async function listSessionAgents(sessionId: string): Promise<{ session_id: string; agents: string[] }> {
  return req(`/sessions/${encodeURIComponent(sessionId)}/agents`);
}

// === Schedules ===

export interface ScheduleTask {
  id: string; name: string; description: string; prompt: string;
  cron_expr: string; category: string; enabled: boolean;
  created_at: string; updated_at: string; next_run: string;
  last_run?: string; last_result?: string;
}

export async function listSchedules(): Promise<{ schedules: ScheduleTask[] }> {
  return req('/schedules');
}

export async function createSchedule(task: Partial<ScheduleTask>): Promise<ScheduleTask> {
  return req('/schedules', { method: 'POST', body: JSON.stringify(task) });
}

export async function deleteSchedule(id: string): Promise<{ deleted: string }> {
  return req(`/schedules/${id}`, { method: 'DELETE' });
}

// === Plugins ===

export async function listPlugins(): Promise<{ plugins: string[]; dir: string }> {
  return req('/plugins');
}

export async function installPlugin(def: Record<string, any>): Promise<any> {
  return req('/plugins/install', { method: 'POST', body: JSON.stringify(def) });
}

export async function uninstallPlugin(name: string): Promise<{ uninstalled: string }> {
  return req(`/plugins/${name}`, { method: 'DELETE' });
}

// === Skills ===

export interface SkillInfo { name: string; description: string; category: string; }

export async function listSkills(): Promise<{ skills: SkillInfo[] }> {
  return req('/skills');
}

// === Terminal ===

export async function execTerminal(command: string, workdir?: string): Promise<{ output: string; error?: string }> {
  return req('/terminal', { method: 'POST', body: JSON.stringify({ command, workdir }) });
}
