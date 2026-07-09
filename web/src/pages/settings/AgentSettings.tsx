import { useState, useEffect, useCallback } from 'react';
import { getConfig, updateConfig } from '../../lib/api';
import type { Config } from '../../lib/types';

export default function AgentSettings() {
  const [cfg, setCfg] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState('');

  const load = useCallback(() => {
    getConfig().then(setCfg).catch(() => {}).finally(() => setLoading(false));
  }, []);

  useEffect(() => { load(); }, [load]);

  const save = async (updates: Record<string, unknown>) => {
    setMsg('');
    try {
      await updateConfig(updates);
      setMsg('✅ Saved');
      setTimeout(() => setMsg(''), 2000);
      load();
    } catch (e: any) {
      setMsg(`❌ ${e.message}`);
    }
  };

  if (loading) return <div className="p-4 text-sm text-[#8b949e]">Loading...</div>;
  if (!cfg) return <div className="p-4 text-sm text-red-400">Could not load config.</div>;

  const Field = ({ label, value, onChange, type = 'text', rows }: { label: string; value: string | number; onChange: (v: string) => void; type?: string; rows?: number }) => (
    <div className="mb-3">
      <label className="block text-xs text-[#8b949e] mb-1">{label}</label>
      {rows ? (
        <textarea value={value} onChange={e => onChange(e.target.value)} rows={rows}
          className="w-full bg-[#0d1117] border border-[#30363d] rounded px-3 py-2 text-xs text-[#e6edf3] font-mono resize-y" />
      ) : (
        <input type={type} value={value} onChange={e => onChange(e.target.value)}
          className="w-full bg-[#0d1117] border border-[#30363d] rounded px-3 py-2 text-xs text-[#e6edf3] font-mono" />
      )}
    </div>
  );

  const Select = ({ label, value, options, onChange }: { label: string; value: string; options: string[]; onChange: (v: string) => void }) => (
    <div className="mb-3">
      <label className="block text-xs text-[#8b949e] mb-1">{label}</label>
      <select value={value} onChange={e => onChange(e.target.value)}
        className="w-full bg-[#0d1117] border border-[#30363d] rounded px-3 py-2 text-xs text-[#e6edf3]">
        {options.map(o => <option key={o} value={o}>{o}</option>)}
      </select>
    </div>
  );

  return (
    <div className="overflow-auto max-h-full">
      <div className="p-5 space-y-2">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-sm font-semibold text-[#e6edf3]">🤖 Agent Settings</h2>
          {msg && <span className={`text-xs ${msg.startsWith('✅') ? 'text-green-400' : 'text-red-400'}`}>{msg}</span>}
        </div>

        <Select label="Provider" value={cfg.provider}
          options={['openai', 'anthropic', 'ollama']}
          onChange={v => save({ provider: v })} />

        <Field label="Model" value={cfg.model}
          onChange={v => save({ model: v })} />

        <Field label="Base URL" value={cfg.base_url}
          onChange={v => save({ base_url: v })} />

        <Select label="Reasoning Effort" value={cfg.reasoning_effort}
          options={['low', 'medium', 'high', 'xhigh']}
          onChange={v => save({ reasoning_effort: v })} />

        <Field label="Max Turns" value={cfg.max_turns} type="number"
          onChange={v => save({ max_turns: parseInt(v) || 30 })} />

        <Select label="Pool Strategy" value={cfg.pool_strategy || 'round_robin'}
          options={['round_robin', 'random', 'fill_first']}
          onChange={v => save({ pool_strategy: v })} />

        <Select label="Wire API" value={cfg.wire_api || 'chat_completions'}
          options={['chat_completions', 'responses']}
          onChange={v => save({ wire_api: v })} />

        <Field label="System Prompt" value={cfg.system_prompt || ''} rows={4}
          onChange={v => save({ system_prompt: v })} />

        <div className="text-[10px] text-[#8b949e] pt-2 border-t border-[#21262d] mt-4">
          Tools: {cfg.tool_count} · Active sessions: {cfg.active_sessions} · Backends: {cfg.backend_count}
        </div>
      </div>
    </div>
  );
}
