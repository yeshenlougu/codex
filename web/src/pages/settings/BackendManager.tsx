import { useState, useEffect, useCallback } from 'react';
import { getBackends, addBackend, deleteBackend, probeBackends, getBackendModels } from '../../lib/api';
import type { BackendStatus, ModelInfo } from '../../lib/types';

const TYPE_ICONS: Record<string, string> = {
  chat: '💬', vision: '👁️', image_gen: '🖼️', video_gen: '🎥',
  audio_stt: '🎤', audio_tts: '🔊', embedding: '📊',
};

export default function BackendManager() {
  const [backends, setBackends] = useState<BackendStatus[]>([]);
  const [strategy, setStrategy] = useState('');
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [msg, setMsg] = useState('');

  const [newLabel, setNewLabel] = useState('');
  const [newKey, setNewKey] = useState('');
  const [newUrl, setNewUrl] = useState('');

  const load = useCallback(async () => {
    try {
      // Use the models endpoint to get rich backend info
      const data = await getBackendModels();
      setBackends(data.backends);
      // Also get strategy from normal backends endpoint
      const be = await getBackends();
      setStrategy(be.strategy);
    } catch {
      // Fallback: use normal backends endpoint
      try {
        const data = await getBackends();
        setBackends(data.backends);
        setStrategy(data.strategy);
      } catch {}
    }
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
      setMsg('✅ Backend added — models will be auto-discovered');
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
    try { await probeBackends(); setMsg('🔍 Health probe + model discovery triggered'); load(); }
    catch (e: any) { setMsg(`❌ ${e.message}`); }
  };

  if (loading) return <div className="p-4 text-sm" style={{ color: 'var(--text-muted)' }}>Loading...</div>;

  const healthColor = (h: string) =>
    h === 'healthy' ? '#27a644' : h === 'degraded' ? '#d19a00' : h === 'unhealthy' ? '#e5484d' : '#62666d';

  return (
    <div style={{ overflow: 'auto', maxHeight: '100%' }}>
      <div style={{ padding: '16px 20px' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
          <h2 style={{ fontSize: 14, fontWeight: 600, color: 'var(--text-primary)' }}>🔌 Backends</h2>
          <div style={{ display: 'flex', gap: 8 }}>
            <button onClick={doProbe} className="btn" style={{ fontSize: 11 }}>
              🔍 Probe & Discover
            </button>
            <button onClick={() => setAdding(!adding)} className="btn btn-primary" style={{ fontSize: 11 }}>
              + Add
            </button>
          </div>
        </div>

        {msg && (
          <div style={{ fontSize: 11, marginBottom: 8, color: msg.startsWith('✅') ? 'var(--green)' : msg.startsWith('🔍') ? 'var(--accent)' : 'var(--red)' }}>
            {msg}
          </div>
        )}

        {adding && (
          <div style={{ background: 'var(--bg-panel)', border: '1px solid var(--border)', borderRadius: 'var(--radius-md)', padding: 12, marginBottom: 12 }}>
            <input placeholder="Label" value={newLabel} onChange={e => setNewLabel(e.target.value)}
              className="input" style={{ width: '100%', marginBottom: 8 }} />
            <input placeholder="API Key" value={newKey} onChange={e => setNewKey(e.target.value)} type="password"
              className="input" style={{ width: '100%', marginBottom: 8 }} />
            <input placeholder="Base URL (https://...)" value={newUrl} onChange={e => setNewUrl(e.target.value)}
              className="input" style={{ width: '100%', marginBottom: 8 }} />
            <div style={{ display: 'flex', gap: 8 }}>
              <button onClick={doAdd} className="btn btn-primary" style={{ fontSize: 11 }}>Save</button>
              <button onClick={() => setAdding(false)} className="btn" style={{ fontSize: 11 }}>Cancel</button>
            </div>
          </div>
        )}

        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {backends.length === 0 && (
            <div style={{ fontSize: 12, color: 'var(--text-muted)', padding: '24px 0', textAlign: 'center' }}>
              No backends configured. Add one above, or import from cc-switch.
            </div>
          )}
          {backends.map(be => (
            <div key={be.label} style={{
              background: 'var(--bg-panel)', border: '1px solid var(--border)',
              borderRadius: 'var(--radius-md)', padding: '12px 14px',
            }}>
              {/* Header */}
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                <span style={{
                  width: 8, height: 8, borderRadius: '50%', flexShrink: 0,
                  background: healthColor(be.health),
                }} />
                <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)', flex: 1 }}>
                  {be.label}
                </span>
                <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>
                  {be.successes > 0 && <span style={{ color: 'var(--green)', marginRight: 6 }}>{be.successes}✓</span>}
                  {be.failures > 0 && <span style={{ color: 'var(--red)' }}>{be.failures}✗</span>}
                </span>
                <button onClick={() => doDelete(be.label)}
                  style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', fontSize: 12, padding: '2px 4px' }}
                  title="Remove">✕</button>
              </div>

              {/* URL */}
              <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 6, fontFamily: 'JetBrains Mono, monospace' }}>
                {be.base_url}
              </div>

              {/* Models (grouped by type) */}
              {be.models && be.models.length > 0 && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                  {be.models_grouped ? (
                    Object.entries(be.models_grouped).map(([type, models]) => (
                      <div key={type} style={{ display: 'flex', alignItems: 'flex-start', gap: 6 }}>
                        <span style={{
                          fontSize: 10, fontWeight: 600, color: 'var(--accent)',
                          background: 'var(--accent-dim)', padding: '1px 6px', borderRadius: 3,
                          flexShrink: 0, minWidth: 80, textAlign: 'center',
                        }}>
                          {TYPE_ICONS[type] || '📦'} {type}
                        </span>
                        <span style={{ fontSize: 10, color: 'var(--text-secondary)', lineHeight: 1.6 }}>
                          {models.map((m: ModelInfo, i: number) => (
                            <span key={m.name}>
                              {i > 0 && ', '}
                              <span style={{ fontFamily: 'JetBrains Mono, monospace' }}>{m.name}</span>
                              {m.auto && <span style={{ color: 'var(--text-muted)', fontSize: 9 }}> auto</span>}
                            </span>
                          ))}
                        </span>
                      </div>
                    ))
                  ) : (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                      {be.models.map((m: ModelInfo) => (
                        <span key={m.name} style={{
                          fontSize: 10, padding: '2px 8px', borderRadius: 'var(--radius-lg)',
                          background: m.auto ? 'var(--bg-hover)' : 'var(--accent-dim)',
                          border: '1px solid var(--border)',
                          color: m.auto ? 'var(--text-secondary)' : 'var(--accent)',
                          fontFamily: 'JetBrains Mono, monospace',
                        }}>
                          {TYPE_ICONS[m.type] || '📦'} {m.name}
                          {!m.auto && <span style={{ fontSize: 8, marginLeft: 3 }}>✎</span>}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {(!be.models || be.models.length === 0) && (
                <div style={{ fontSize: 10, color: 'var(--text-muted)', fontStyle: 'italic' }}>
                  Models not yet discovered — click "Probe & Discover"
                </div>
              )}
            </div>
          ))}
        </div>

        <div style={{ fontSize: 10, color: 'var(--text-muted)', paddingTop: 8, borderTop: '1px solid var(--border)', marginTop: 12 }}>
          Strategy: {strategy || 'none'} · {backends.length} total
        </div>
      </div>
    </div>
  );
}
