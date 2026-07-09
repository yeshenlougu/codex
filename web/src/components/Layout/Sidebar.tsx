import { useState, useEffect } from 'react';
import type { SessionSummary } from '../../lib/types';
import { listSessions, deleteSession } from '../../lib/api';
import { Trash2, RefreshCw } from 'lucide-react';

interface Props {
  sessionId: string;
  onResumeSession: (id: string) => void;
  onNewSession: () => void;
}

export default function Sidebar({ sessionId, onResumeSession, onNewSession }: Props) {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    try { const data = await listSessions(); setSessions(data.sessions || []); } catch {}
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try { await deleteSession(id); setSessions((s) => s.filter((s) => s.id !== id)); } catch {}
  };

  return (
    <aside className="w-56 shrink-0 border-r border-[#30363d] bg-[#0d1117] flex flex-col overflow-hidden">
      <div className="flex items-center justify-between px-3 py-2 border-b border-[#30363d]">
        <span className="text-[10px] text-[#8b949e] uppercase tracking-wider">Sessions</span>
        <button onClick={load} className="text-[#8b949e] hover:text-[#e6edf3] p-0.5 rounded" title="Refresh">
          <RefreshCw size={12} />
        </button>
      </div>
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="p-3 text-xs text-[#8b949e]">Loading...</div>
        ) : sessions.length === 0 ? (
          <div className="p-3 text-xs text-[#8b949e]">No sessions yet</div>
        ) : (
          sessions.map((s) => (
            <div key={s.id} onClick={() => onResumeSession(s.id)}
              className={`px-3 py-2 cursor-pointer group hover:bg-[#161b22] border-b border-[#21262d] transition-colors ${
                s.id === sessionId ? 'bg-[#58a6ff]/10 border-l-2 border-l-[#58a6ff]' : ''
              }`}>
              <div className="flex items-start justify-between">
                <div className="min-w-0 flex-1">
                  <p className="text-xs text-[#e6edf3] truncate">{s.title || s.id}</p>
                  <p className="text-[10px] text-[#8b949e] mt-0.5">{s.msg_count} msgs &middot; {s.model?.split('-').pop()}</p>
                </div>
                <button onClick={(e) => handleDelete(s.id, e)}
                  className="opacity-0 group-hover:opacity-100 text-[#f85149] hover:text-red-400 p-0.5 shrink-0 transition-opacity" title="Delete">
                  <Trash2 size={12} />
                </button>
              </div>
            </div>
          ))
        )}
      </div>
      <div className="p-2 border-t border-[#30363d]">
        <button onClick={onNewSession}
          className="w-full py-1.5 text-xs bg-[#21262d] hover:bg-[#30363d] text-[#e6edf3] rounded transition-colors">
          + New Session
        </button>
      </div>
    </aside>
  );
}
