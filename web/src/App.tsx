import { useState, useCallback } from 'react';
import TitleBar from './components/TitleBar';
import LeftSidebar from './components/LeftSidebar';
import ChatPage from './pages/ChatPage';
import SettingsPage from './pages/SettingsPage';
import RightPanel from './components/RightPanel';

export type Page = 'chat' | 'settings';
export type RightTab = 'files' | 'changes' | 'git';

export default function App() {
  const [page, setPage] = useState<Page>('chat');
  const [rightTab, setRightTab] = useState<RightTab>('files');
  const [sessionId, setSessionId] = useState(() => {
    const n = new Date(); const p = (x: number) => String(x).padStart(2, '0');
    return `${n.getFullYear()}${p(n.getMonth() + 1)}${p(n.getDate())}-${p(n.getHours())}${p(n.getMinutes())}${p(n.getSeconds())}`;
  });
  const [workspace, setWorkspace] = useState('default');

  const newSession = useCallback(() => {
    const n = new Date(); const p = (x: number) => String(x).padStart(2, '0');
    setSessionId(`${n.getFullYear()}${p(n.getMonth() + 1)}${p(n.getDate())}-${p(n.getHours())}${p(n.getMinutes())}${p(n.getSeconds())}`);
    setPage('chat');
  }, []);

  const resumeSession = useCallback((id: string) => { setSessionId(id); setPage('chat'); }, []);

  return (
    <div className="app-root">
      <TitleBar />
      <div className="app-body">
        <LeftSidebar
          page={page}
          sessionId={sessionId}
          workspace={workspace}
          onNavigate={setPage}
          onResumeSession={resumeSession}
          onNewSession={newSession}
          onWorkspaceChange={setWorkspace}
        />
        {/* Center: main content */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
          {page === 'chat' && (
            <ChatPage sessionId={sessionId} workspace={workspace} />
          )}
          {page === 'settings' && <SettingsPage />}
        </div>
        {/* Right panel */}
        <RightPanel tab={rightTab} onTabChange={setRightTab} />
      </div>
      <div className="statusbar">
        <span>Codex Go v1.0.0</span>
        <span>Workspace: {workspace}</span>
        <span style={{ marginLeft: 'auto' }}>Session: {sessionId.slice(-12)}</span>
      </div>
    </div>
  );
}
