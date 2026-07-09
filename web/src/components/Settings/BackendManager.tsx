import { useState, useEffect, useCallback } from 'react';
import { getBackends, addBackend, deleteBackend, probeBackends } from '../../lib/api';
import type { BackendStatus } from '../../lib/types';

export default function BackendManager() {
  const [backends, setBackends] = useState<BackendStatus[]>([]);
  const [strategy, setStrategy] = useState('');
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [msg, setMsg] = useState('');

  // New backend form
  const [newLabel, setNewLabel] = useState('');
  const [newKey, setNewKey] = useState('');
  const [newUrl, setNewUrl] = useState('');

  const load = useCallback(async () => {
    try {
      const data = await getBackends();
      setBackends(data.backends);
      setStrategy(data.strategy);
    } catch (e) {}
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const doAdd = async () => {
    if (!newLabel || !newKey || !newUrl) return;
    setMsg('');
    try {
      await addBackend({ label: newLabel, key: newKey, base_url: newUrl, weight: 1 });
      setNewLabel(''); setNewKey(''); setNewUrl(''); setAdding(false);
      load();
      setMsg('✅ Backend added');
    } catch (e: any) { setMsg(`❌ ${e.message}`); }
  };

  const doDelete = async (label: string) => {
    if (!confirm(`Remove backend "${label}"?`)) return;
    try {
      await deleteBackend(label);
      load();
      setMsg(`✅ ${label} removed`);
    } catch (e: any) { setMsg(`❌ ${e.message}`); }
  };

  const doProbe = async () => {
    try { await probeBackends(); setMsg('🔍 Health probe triggered'); load(); }
    catch (e: any) { setMsg(`❌ ${e.message}`); }
  };

  if (loading) return <div className="p-4 text-sm text-[#8b949e]">Loading...</div>;

  const healthColor = (h: string) =>
    h === 'healthy' ? 'bg-green-500' : h === 'degraded' ? 'bg-yellow-500' : h === 'unhealthy' ? 'bg-red-500' : 'bg-gray-500';

  return (
    <div className="overflow-auto max-h-full">
      <div className="p-5 space-y-2">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-sm font-semibold text-[#e6edf3]">🔌 Backends</h2>
          <div className="flex gap-2">
            <button onClick={doProbe} className="px-2 py-1 text-[10px] bg-[#21262d] hover:bg-[#30363d] text-[#8b949e] rounded">
              🔍 Probe
            </button>
            <button onClick={() => setAdding(!adding)}
              className="px-2 py-1 text-[10px] bg-[#238636] hover:bg-[#2ea043] text-white rounded">
              + Add
            </button>
          </div>
        </div>

        {msg && <div className={`text-xs mb-2 ${msg.startsWith('✅') ? 'text-green-400' : msg.startsWith('🔍') ? 'text-blue-400' : 'text-red-400'}`}>{msg}</div>}

        {adding && (
          <div className="bg-[#161b22] border border-[#30363d] rounded p-3 mb-3 space-y-2">
            <input placeholder="Label" value={newLabel} onChange={e => setNewLabel(e.target.value)}
              className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-xs text-[#e6edf3]" />
            <input placeholder="API Key" value={newKey} onChange={e => setNewKey(e.target.value)} type="password"
              className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-xs text-[#e6edf3]" />
            <input placeholder="Base URL (https://...)" value={newUrl} onChange={e => setNewUrl(e.target.value)}
              className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-xs text-[#e6edf3]" />
            <div className="flex gap-2">
              <button onClick={doAdd} className="px-3 py-1 text-xs bg-[#238636] hover:bg-[#2ea043] text-white rounded">Save</button>
              <button onClick={() => setAdding(false)} className="px-3 py-1 text-xs bg-[#21262d] hover:bg-[#30363d] text-[#8b949e] rounded">Cancel</button>
            </div>
          </div>
        )}

        <div className="space-y-1">
          {backends.length === 0 && (
            <div className="text-xs text-[#8b949e] py-4 text-center">
              No backends configured. Add one above, or import from cc-switch.
            </div>
          )}
          {backends.map(be => (
            <div key={be.label} className="flex items-center gap-2 py-2 px-3 bg-[#161b22] rounded border border-[#21262d] group">
              <span className={`w-2 h-2 rounded-full shrink-0 ${healthColor(be.health)}`} title={be.health} />
              <div className="flex-1 min-w-0">
                <div className="text-xs text-[#e6edf3] truncate">{be.label}</div>
                <div className="text-[10px] text-[#8b949e] truncate">{be.base_url}</div>
              </div>
              <div className="text-[10px] text-[#8b949e] shrink-0">
                {be.successes > 0 && <span className="text-green-400 mr-1">{be.successes}✓</span>}
                {be.failures > 0 && <span className="text-red-400">{be.failures}✗</span>}
              </div>
              <button onClick={() => doDelete(be.label)}
                className="opacity-0 group-hover:opacity-100 px-1 text-[10px] text-red-400 hover:text-red-300 transition-opacity">✕</button>
            </div>
          ))}
        </div>

        <div className="text-[10px] text-[#8b949e] pt-2 border-t border-[#21262d]">
          Strategy: {strategy || 'none'} · {backends.length} total
        </div>
      </div>
    </div>
  );
}
