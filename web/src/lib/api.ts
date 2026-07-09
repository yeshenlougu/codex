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
export async function sendMessage(message: string, sessionId: string, opts?: { stream?: boolean }): Promise<ChatResponse> {
  return req('/chat', { method: 'POST', body: JSON.stringify({ message, session_id: sessionId, stream: false }) });
}
export async function streamMessage(
  message: string, sessionId: string,
  onChunk: (text: string) => void, onDone: (fullText: string) => void, onError: (err: string) => void,
) {
  const res = await fetch(BASE + '/chat', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message, session_id: sessionId, stream: true }),
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
