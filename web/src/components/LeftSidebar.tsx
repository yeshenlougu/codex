import { useState, useEffect, useCallback } from 'react';
import type { SessionSummary } from '../lib/types';
import { listSessions, deleteSession, listFiles } from '../lib/api';
import { Trash2, RefreshCw, Plus, FolderOpen, Settings as SettingsIcon, MessageSquare, FolderPlus } from 'lucide-react';
import type { Page } from '../App';

interface Props {
  page: Page;
  sessionId: string;
  workspace: string;
  projects: string[];
  onNavigate: (p: Page) => void;
  onResumeSession: (id: string) => void;
  onNewSession: () => void;
  onWorkspaceChange: (w: string) => void;
  onProjectAdd: (path: string) => void;
}

export default function LeftSidebar(props: Props) {
  const { page, sessionId, workspace, projects, onNavigate, onResumeSession, onNewSession, onWorkspaceChange, onProjectAdd } = props;
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [showWsModal, setShowWsModal] = useState(false);
  const [wsPath, setWsPath] = useState('');
  const [wsDirList, setWsDirList] = useState<{ name: string; path: string; is_dir: boolean }[]>([]);

  const load = async () => {
    try { const data = await listSessions(); setSessions(data.sessions || []); } catch {}
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try { await deleteSession(id); setSessions((s) => s.filter((x) => x.id !== id)); } catch {}
  };

  // Browse directory via files API
  const browseDir = useCallback(async (dir: string) => {
    try {
      const data = await listFiles(dir);
      setWsPath(dir);
      setWsDirList((data.files || []).filter((f: any) => f.is_dir).map((f: any) => ({
        name: f.name, path: f.path, is_dir: true
      })));
    } catch {
      setWsDirList([]);
    }
  }, []);

  const openWsModal = () => {
    setShowWsModal(true);
    browseDir(wsPath || '/home/ubuntu');
  };

  const handleAddProject = () => {
    const path = wsPath.trim();
    if (path) {
      onProjectAdd(path);
      onWorkspaceChange(path);
      setShowWsModal(false);
    }
  };

  // Group sessions by workspace
  const grouped = sessions.reduce<Record<string, SessionSummary[]>>((acc, s) => {
    const ws = (s as any).workspace || workspace || 'default';
    if (!acc[ws]) acc[ws] = [];
    acc[ws].push(s);
    return acc;
  }, {});

  return (
    <aside className="left-sidebar">
      {/* Chat nav */}
      <div className={`ls-nav-item ${page === 'chat' ? 'active' : ''}`} onClick={() => onNavigate('chat')}>
        <MessageSquare size={15} />
        <span>Chat</span>
      </div>

      {/* Projects section */}
      <div className="ls-projects">
        <div className="ls-projects-header">
          <span className="ls-projects-title">Projects</span>
          <button className="ls-project-add" onClick={openWsModal} title="Open folder">
            <FolderPlus size={14} />
          </button>
        </div>
        {projects.map((proj) => {
          const name = proj.split('/').filter(Boolean).pop() || proj;
          const isActive = workspace === proj;
          return (
            <div key={proj}>
              <div
                className={`ls-project-item ${isActive ? 'active' : ''}`}
                onClick={() => onWorkspaceChange(proj)}
              >
                <span className="ls-project-icon">📁</span>
                <span className="ls-project-name">{name}</span>
              </div>
              {isActive && <div className="ls-project-path">{proj}</div>}
            </div>
          );
        })}
        {projects.length === 0 && (
          <div className="ls-empty" style={{ padding: '6px 8px', fontSize: 10 }}>
            No projects — open a folder to start
          </div>
        )}
      </div>

      {/* Separator */}
      <div style={{ height: 1, background: 'var(--border)', margin: '2px 8px' }} />

      {/* Sessions */}
      <div className="ls-sessions">
        {loading ? (
          <div className="ls-empty">Loading...</div>
        ) : sessions.length === 0 ? (
          <div className="ls-empty">No sessions yet</div>
        ) : (
          Object.keys(grouped).map((ws) => (
            <div key={ws} className="ls-group">
              <div className="ls-group-name">📁 {ws === 'default' ? 'Default' : ws.split('/').pop() || ws}</div>
              {grouped[ws].map((s) => (
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

      {/* New session */}
      <button className="ls-new-btn" onClick={onNewSession}>
        <Plus size={12} /> New Session
      </button>

      <div style={{ flex: 1 }} />

      {/* Settings */}
      <div
        className={`ls-nav-item ${page === 'settings' ? 'active' : ''}`}
        onClick={() => onNavigate('settings')}
        style={{ borderTop: '1px solid var(--border)', marginTop: 0 }}
      >
        <SettingsIcon size={15} />
        <span>Settings</span>
      </div>

      {/* Workspace Modal */}
      {showWsModal && (
        <div className="ws-modal-overlay" onClick={() => setShowWsModal(false)}>
          <div className="ws-modal" onClick={e => e.stopPropagation()}>
            <div className="ws-modal-title">Open Folder as Project</div>
            
            {/* Path input */}
            <input
              className="input"
              value={wsPath}
              onChange={e => { setWsPath(e.target.value); }}
              onKeyDown={e => { if (e.key === 'Enter') { browseDir(wsPath); } }}
              placeholder="/home/ubuntu/projects/my-app"
              style={{ width: '100%' }}
            />

            {/* Parent dir navigation */}
            {wsPath && (
              <button className="btn" style={{ alignSelf: 'flex-start', fontSize: 11 }}
                onClick={() => {
                  const parent = wsPath.split('/').slice(0, -1).join('/') || '/';
                  browseDir(parent);
                }}>
                ⬆ Parent directory
              </button>
            )}

            {/* Directory listing */}
            <div style={{ maxHeight: 240, overflowY: 'auto', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: 4 }}>
              {wsDirList.length === 0 ? (
                <div className="settings-hint" style={{ padding: 12, textAlign: 'center' }}>No directories found</div>
              ) : (
                wsDirList.map((d) => (
                  <div
                    key={d.path}
                    className="rp-file-item is-dir"
                    onClick={() => browseDir(d.path)}
                    style={{ cursor: 'pointer' }}
                  >
                    📁 <span className="rp-file-name">{d.name}</span>
                  </div>
                ))
              )}
            </div>

            {/* Actions */}
            <div className="ws-modal-actions">
              <button className="btn" onClick={() => setShowWsModal(false)}>Cancel</button>
              <button className="btn btn-primary" onClick={handleAddProject}>Open</button>
            </div>
          </div>
        </div>
      )}
    </aside>
  );
}
