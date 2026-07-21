import { useState, useCallback, useEffect, Component } from 'react';
import { ConfigProvider, theme, Result, Button } from 'antd';
import { ThemeProvider, useTheme } from './lib/ThemeContext';
import TitleBar from './components/TitleBar';
import LeftSidebar from './components/LeftSidebar';
import ChatPage from './pages/ChatPage';
import SettingsPage from './pages/SettingsPage';
import RightPanel from './components/RightPanel';

export type Page = 'chat' | 'settings';
export type RightTab = 'review' | 'terminal' | 'browser' | 'files' | 'sidetasks';

// ErrorBoundary prevents the entire app from going blank on render error
class ErrorBoundary extends Component<{ children: React.ReactNode }, { hasError: boolean; error: Error | null }> {
  constructor(props: any) {
    super(props);
    this.state = { hasError: false, error: null };
  }
  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }
  render() {
    if (this.state.hasError) {
      return (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', padding: 40 }}>
          <Result
            status="warning"
            title="页面出错"
            subTitle={this.state.error?.message || '未知错误'}
            extra={
              <Button type="primary" onClick={() => { this.setState({ hasError: false, error: null }); window.location.reload(); }}>
                刷新页面
              </Button>
            }
          />
        </div>
      );
    }
    return this.props.children;
  }
}

function AppContent() {
  const { theme: currentTheme } = useTheme();
  const [page, setPage] = useState<Page>('chat');
  const [rightTab, setRightTab] = useState<RightTab>('files');
  const [rightOpen, setRightOpen] = useState(true);
  const [leftOpen, setLeftOpen] = useState(true);
  const [sessionId, setSessionId] = useState(() => {
    const n = new Date(); const p = (x: number) => String(x).padStart(2, '0');
    return `${n.getFullYear()}${p(n.getMonth() + 1)}${p(n.getDate())}-${p(n.getHours())}${p(n.getMinutes())}${p(n.getSeconds())}`;
  });
  const [workspace, setWorkspace] = useState('default');
  const [projects, setProjects] = useState<string[]>([]);

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const mod = e.ctrlKey || e.metaKey;
      if (mod && e.altKey && e.key === 'b') { e.preventDefault(); setRightOpen(v => !v); return; }
      if (mod && e.altKey && e.key === 's') { e.preventDefault(); setRightTab('sidetasks'); setRightOpen(true); return; }
      if (mod && e.shiftKey && e.key === 'G') { e.preventDefault(); setRightTab('review'); setRightOpen(true); return; }
      if (mod && e.key === 't') { e.preventDefault(); setRightTab('browser'); setRightOpen(true); return; }
      if (mod && e.key === 'p') { e.preventDefault(); setRightTab('files'); setRightOpen(true); return; }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  const newSession = useCallback(() => {
    const n = new Date(); const p = (x: number) => String(x).padStart(2, '0');
    setSessionId(`${n.getFullYear()}${p(n.getMonth() + 1)}${p(n.getDate())}-${n.getHours()}${p(n.getMinutes())}${p(n.getSeconds())}`);
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

  // Hide sidebars on Settings page
  const isFullPage = page !== 'chat';

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
      <TitleBar
        leftOpen={leftOpen}
        rightPanelOpen={rightOpen}
        onToggleLeft={() => setLeftOpen(v => !v)}
        onToggleRight={() => setRightOpen(v => !v)}
      />
      <div className="app-body">
        {!isFullPage && leftOpen && (
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
        )}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
          {page === 'chat' && (
            <ChatPage sessionId={sessionId} workspace={workspace} onNavigate={setPage} />
          )}
          {page === 'settings' && <SettingsPage onBack={() => setPage('chat')} />}
        </div>
        {!isFullPage && rightOpen && (
          <RightPanel tab={rightTab} onTabChange={setRightTab} onClose={() => setRightOpen(false)} />
        )}
      </div>
    </div>
    </ConfigProvider>
  );
}

export default function App() {
  return (
    <ThemeProvider>
      <ErrorBoundary>
        <AppContent />
      </ErrorBoundary>
    </ThemeProvider>
  );
}
