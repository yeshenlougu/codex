import { useState, useEffect, useCallback } from 'react';
import { listFiles, readFileContent } from '../lib/api';
import type { RightTab } from '../App';

interface Props {
  tab: RightTab;
  onTabChange: (t: RightTab) => void;
}

interface FileInfo { name: string; path: string; is_dir: boolean; size: number }

const tabs: { id: RightTab; label: string; icon: string }[] = [
  { id: 'files', label: 'Files', icon: '📁' },
  { id: 'changes', label: 'Changes', icon: '📝' },
  { id: 'git', label: 'Git', icon: '🔀' },
];

export default function RightPanel({ tab, onTabChange }: Props) {
  return (
    <aside className="right-panel">
      {/* Tab bar */}
      <div className="rp-tabs">
        {tabs.map((t) => (
          <button
            key={t.id}
            className={`rp-tab ${tab === t.id ? 'active' : ''}`}
            onClick={() => onTabChange(t.id)}
          >
            {t.icon} {t.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="rp-content">
        {tab === 'files' && <FilesPanel />}
        {tab === 'changes' && <ChangesPanel />}
        {tab === 'git' && <GitPanel />}
      </div>
    </aside>
  );
}

// ============ Files Panel ============

function FilesPanel() {
  const [cwd, setCwd] = useState('.');
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(false);

  const loadDir = useCallback(async (dir: string) => {
    setLoading(true);
    try { const r = await listFiles(dir); setFiles(r.files || []); setCwd(r.path || dir); } catch { setFiles([]); }
    setLoading(false);
  }, []);

  useEffect(() => { loadDir(cwd); }, [cwd]);

  const handleClick = async (f: FileInfo) => {
    if (f.is_dir) { setCwd(f.path); setSelected(null); setContent(''); }
    else {
      setSelected(f.path); setLoading(true);
      try {
        const r = await readFileContent(f.path);
        setContent(r.binary ? '[Binary]' : (r.content || ''));
      } catch { setContent('Error'); }
      setLoading(false);
    }
  };

  return (
    <div className="rp-files">
      <div className="rp-files-path">📁 {cwd}</div>
      <div className="rp-files-list">
        {files.map((f) => (
          <div
            key={f.name}
            onClick={() => handleClick(f)}
            className={`rp-file-item ${selected === f.path ? 'active' : ''} ${f.is_dir ? 'is-dir' : ''}`}
          >
            <span>{f.is_dir ? '📁' : '📄'}</span>
            <span className="rp-file-name">{f.name}</span>
          </div>
        ))}
      </div>
      {selected && (
        <div className="rp-file-preview">
          <div className="rp-preview-header">{selected.split('/').pop()}</div>
          <pre className="rp-preview-content">{content || (loading ? 'Loading...' : 'Select a file')}</pre>
        </div>
      )}
    </div>
  );
}

// ============ Changes Panel ============

function ChangesPanel() {
  return (
    <div className="rp-placeholder">
      <div className="rp-placeholder-icon">📝</div>
      <p className="rp-placeholder-title">No active changes</p>
      <p className="rp-placeholder-desc">File diffs will appear here during agent sessions</p>
    </div>
  );
}

// ============ Git Panel ============

function GitPanel() {
  const [status, setStatus] = useState<string>('');

  useEffect(() => {
    fetch('/api/git-status')
      .then(r => r.json())
      .then(d => setStatus(d.status || d.output || ''))
      .catch(() => setStatus('Git status unavailable'));
  }, []);

  return (
    <div className="rp-git">
      <div className="rp-git-header">🔀 Git Status</div>
      <pre className="rp-git-output">{status || 'Loading...'}</pre>
    </div>
  );
}
