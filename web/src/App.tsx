import { useState, useCallback } from 'react';
import AppShell from './components/Layout/AppShell';
import ChatPanel from './components/Chat/ChatPanel';
import SettingsPanel from './components/Settings/SettingsPanel';
import PetStatus from './components/Pet/PetStatus';

type Panel = 'chat' | 'settings' | 'pet' | 'terminal' | 'files';

export default function App() {
  const [panel, setPanel] = useState<Panel>('chat');
  const [sessionId, setSessionId] = useState(() => {
    const n = new Date(); const p = (n:number)=>String(n).padStart(2,'0');
    return `${n.getFullYear()}${p(n.getMonth()+1)}${p(n.getDate())}-${p(n.getHours())}${p(n.getMinutes())}${p(n.getSeconds())}`;
  });

  const newSesh = useCallback(() => {
    const n = new Date(); const p = (n:number)=>String(n).padStart(2,'0');
    setSessionId(`${n.getFullYear()}${p(n.getMonth()+1)}${p(n.getDate())}-${p(n.getHours())}${p(n.getMinutes())}${p(n.getSeconds())}`);
    setPanel('chat');
  }, []);

  const resume = useCallback((id: string) => { setSessionId(id); setPanel('chat'); }, []);

  const tabs: Panel[] = ['chat', 'terminal', 'files', 'settings', 'pet'];
  const labels: Record<Panel,string> = { chat: '💬 Chat', terminal: '⚡ Term', files: '📁 Files', settings: '⚙️ Config', pet: '🐱 Pet' };

  return (
    <div className="h-screen flex flex-col bg-[#0d1117]">
      <header className="h-10 flex items-center px-4 border-b border-[#30363d] bg-[#161b22] shrink-0">
        <span className="text-[#58a6ff] font-bold text-sm">🐱 Codex Go</span>
        <span className="text-[#8b949e] text-xs ml-3">Session: {sessionId.slice(-8)}</span>
        <nav className="flex gap-1 ml-auto">
          {tabs.map(t => (
            <button key={t} onClick={() => setPanel(t)}
              className={`px-3 py-1 text-xs rounded ${panel===t ? 'bg-[#58a6ff]/20 text-[#58a6ff]' : 'text-[#8b949e] hover:bg-[#21262d]'}`}>
              {labels[t]}
            </button>
          ))}
        </nav>
        <button onClick={newSesh} className="ml-3 px-2 py-1 text-xs bg-[#238636] hover:bg-[#2ea043] text-white rounded">+ New</button>
      </header>
      <main className="flex-1 overflow-hidden">
        <AppShell sessionId={sessionId} onResumeSession={resume} onNewSession={newSesh}>
          {panel === 'chat' && <ChatPanel sessionId={sessionId} />}
          {panel === 'terminal' && <div className="flex items-center justify-center h-full text-[#8b949e]"><div className="text-center"><div className="text-4xl mb-3">⚡</div><p className="text-sm">Terminal coming soon</p></div></div>}
          {panel === 'files' && <div className="flex items-center justify-center h-full text-[#8b949e]"><div className="text-center"><div className="text-4xl mb-3">📁</div><p className="text-sm">File browser coming soon</p></div></div>}
          {panel === 'settings' && <SettingsPanel />}
          {panel === 'pet' && <PetStatus />}
        </AppShell>
      </main>
      <footer className="h-6 flex items-center px-3 text-[10px] text-[#8b949e] bg-[#161b22] border-t border-[#30363d] shrink-0">
        <span>Codex Go · API :1977</span>
        <span className="ml-auto">{panel}</span>
      </footer>
    </div>
  );
}
