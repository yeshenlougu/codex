import { useState, useCallback } from 'react';
import { ConfigProvider, theme } from 'antd';
import { ThemeProvider, useTheme } from './lib/ThemeContext';
import TitleBar from './components/TitleBar';
import LeftSidebar from './components/LeftSidebar';
import ChatPage from './pages/ChatPage';
import SettingsPage from './pages/SettingsPage';
import ScheduledPage from './pages/ScheduledPage';
import PluginsPage from './pages/PluginsPage';
import RightPanel from './components/RightPanel';

export type Page = 'chat' | 'settings' | 'scheduled' | 'plugins';
export type RightTab = 'review' | 'terminal' | 'browser' | 'files' | 'sidetasks';

function AppContent() {
  const { theme: currentTheme } = useTheme();
  const [page, setPage] = useState<Page>('chat');
  const [rightTab, setRightTab] = useState<RightTab>('files');
  const [rightOpen, setRightOpen] = useState(false);
  const [sessionId, setSessionId] = useState(() => {
    const n = new Date(); const p = (x: number) => String(x).padStart(2, '0');
    return `${n.getFullYear()}${p(n.getMonth() + 1)}${p(n.getDate())}-${p(n.getHours())}${p(n.getMinutes())}${p(n.getSeconds())}`;
  });
  const [workspace, setWorkspace] = useState('default');
  const [projects, setProjects] = useState<string[]>([]);

  const newSession = useCallback(() => {
    const n = new Date(); const p = (x: number) => String(x).padStart(2, '0');
    setSessionId(`${n.getFullYear()}${p(n.getMonth() + 1)}${p(n.getDate())}-${p(n.getHours())}${p(n.getMinutes())}${p(n.getSeconds())}`);
    setPage('chat');
  }, []);

  const resumeSession = useCallback((id: string) => { setSessionId(id); setPage('chat'); }, []);

  const addProject = useCallback((path: string) => {
    setProjects(prev => {
      if (prev.includes(path)) return prev;
      return [...prev, path];
    });
    setWorkspace(path);
    setPage('chat');
  }, []);

  return (
    <ConfigProvider
      theme={{
        algorithm: currentTheme === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          colorPrimary: '#5e6ad2',
          borderRadius: 6,
          fontFamily: "'Inter', system-ui, -apple-system, sans-serif",
        },
      }}
    >
    <div className="app-root">
      <TitleBar rightPanelOpen={rightOpen} onToggleRight={() => setRightOpen(v => !v)} />
      <div className="app-body">
        <LeftSidebar
          page={page}
          sessionId={sessionId}
          workspace={workspace}
          projects={projects}
          onNavigate={setPage}
          onResumeSession={resumeSession}
          onNewSession={newSession}
          onWorkspaceChange={setWorkspace}
          onProjectAdd={addProject}
        />
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
          {page === 'chat' && (
            <ChatPage sessionId={sessionId} workspace={workspace} />
          )}
          {page === 'settings' && <SettingsPage />}
          {page === 'scheduled' && <ScheduledPage />}
          {page === 'plugins' && <PluginsPage />}
        </div>
        {rightOpen && (
          <RightPanel tab={rightTab} onTabChange={setRightTab} onClose={() => setRightOpen(false)} />
        )}
      </div>
      <div className="statusbar">
        <span>Codex Go</span>
        <span style={{ marginLeft: 'auto' }}>🟢 在线</span>
      </div>
    </div>
    </ConfigProvider>
  );
}

export default function App() {
  return (
    <ThemeProvider>
      <AppContent />
    </ThemeProvider>
  );
}
