import { useState, useEffect } from 'react';
import { getConfig } from '../../lib/api';
import type { Config } from '../../lib/types';

export default function SettingsPanel() {
  const [c, setC] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => { getConfig().then(setC).catch(()=>{}).finally(()=>setLoading(false)); }, []);

  if (loading) return <div className="h-full flex items-center justify-center"><p className="text-sm text-[#8b949e]">Loading...</p></div>;
  if (!c) return <div className="h-full flex items-center justify-center"><p className="text-sm text-[#8b949e]">Could not load config.</p></div>;

  const rows: [string,string][] = [
    ['Provider', c.provider], ['Model', c.model], ['Base URL', c.base_url],
    ['API Key', c.api_key_masked], ['Reasoning', c.reasoning_effort],
    ['Max Turns', String(c.max_turns)], ['Tools', String(c.tool_count)],
    ['Active Sessions', String(c.active_sessions)],
  ];

  return (
    <div className="h-full overflow-auto"><div className="max-w-lg mx-auto p-6">
      <h2 className="text-sm font-semibold text-[#e6edf3] mb-4">Configuration</h2>
      <div className="space-y-2">
        {rows.map(([k,v]) => (
          <div key={k} className="flex items-center justify-between py-2 px-3 bg-[#161b22] rounded border border-[#21262d]">
            <span className="text-xs text-[#8b949e]">{k}</span>
            <span className="text-xs font-mono text-[#e6edf3] max-w-[240px] truncate">{v}</span>
          </div>
        ))}
      </div>
      <p className="text-[10px] text-[#8b949e] mt-4">Edit at <code className="text-[#58a6ff]">~/.codex/config.yaml</code></p>
    </div></div>
  );
}
