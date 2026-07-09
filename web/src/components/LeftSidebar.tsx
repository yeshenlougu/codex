import { useState, useEffect } from 'react';
import type { SessionSummary } from '../lib/types';
import { listSessions, deleteSession } from '../lib/api';
import { Trash2, RefreshCw, Plus, FolderOpen, Settings as SettingsIcon, MessageSquare } from 'lucide-react';
import type { Page } from '../App';

interface Props {
  page: Page;
  sessionId: string;
  workspace: string;
  onNavigate: (p: Page) => void;
  onResumeSession: (id: string) => void;
  onNewSession: () => void;
  onWorkspaceChange: (w: string) => void;
}

export default function LeftSidebar(props: Props) {
  const { page, sessionId, workspace, onNavigate, onResumeSession, onNewSession, onWorkspaceChange } = props;
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    try { const data = await listSessions(); setSessions(data.sessions || []); } catch {}
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try { await deleteSession(id); setSessions((s) => s.filter((x) => x.id !== id)); } catch {}
  };

  // Group sessions by workspace directory (extracted from title or metadata)
  const grouped = sessions.reduce<Record<string, SessionSummary[]>>((acc, s) => {
    const ws = (s as any).workspace || 'default';
    if (!acc[ws]) acc[ws] = [];
    acc[ws].push(s);
    return acc;
  }, {});

  const workspaces = Object.keys(grouped).length > 0 ? Object.keys(grouped) : [workspace || 'default'];

  return (
    <aside className="left-sidebar">
      {/* Chat nav */}
      <div className={`ls-nav-item ${page === 'chat' ? 'active' : ''}`} onClick={() => onNavigate('chat')}>
        <MessageSquare size={15} />
        <span>Chat</span>
      </div>

      {/* Workspace selector */}
      <div className="ls-section">
        <div className="ls-section-header">
          <FolderOpen size={11} />
          <select
            className="ls-ws-select"
            value={workspace}
            onChange={(e) => onWorkspaceChange(e.target.value)}
          >
            {workspaces.map((ws) => (
              <option key={ws} value={ws}>{ws}</option>
            ))}
          </select>
          <button onClick={load} className="ls-icon-btn" title="Refresh">
            <RefreshCw size={10} />
          </button>
        </div>
      </div>

      {/* Sessions grouped by workspace */}
      <div className="ls-sessions">
        {loading ? (
          <div className="ls-empty">Loading...</div>
        ) : sessions.length === 0 ? (
          <div className="ls-empty">No sessions yet</div>
        ) : (
          workspaces.map((ws) => (
            <div key={ws} className="ls-group">
              <div className="ls-group-name">📁 {ws}</div>
              {(grouped[ws] || sessions.filter(s => (s as any).workspace === ws || !ws)).map((s) => (
                <div
                  key={s.id}
                  className={`ls-session ${s.id === sessionId ? 'active' : ''}`}
                  onClick={() => onResumeSession(s.id)}
                >
                  <div className="ls-session-info">
                    <div className="ls-session-title">{s.title || s.id.slice(-12)}</div>
                    <div className="ls-session-meta">{s.msg_count} msgs</div>
                  </div>
                  <button className="ls-delete-btn" onClick={(e) => handleDelete(s.id, e)} title="Delete">
                    <Trash2 size={10} />
                  </button>
                </div>
              ))}
            </div>
          ))
        )}
      </div>

      {/* New session button */}
      <button className="ls-new-btn" onClick={onNewSession}>
        <Plus size={12} /> New Session
      </button>

      {/* Spacer */}
      <div style={{ flex: 1 }} />

      {/* Settings at bottom */}
      <div
        className={`ls-nav-item ${page === 'settings' ? 'active' : ''}`}
        onClick={() => onNavigate('settings')}
        style={{ borderTop: '1px solid var(--border)', marginTop: 0 }}
      >
        <SettingsIcon size={15} />
        <span>Settings</span>
      </div>
    </aside>
  );
}
