import { useState } from 'react';
import AgentSettings from './AgentSettings';
import BackendManager from './BackendManager';
import ImportExport from './ImportExport';

type Tab = 'agent' | 'backends' | 'import';

export default function SettingsPanel() {
  const [tab, setTab] = useState<Tab>('agent');
  const tabs: { id: Tab; label: string }[] = [
    { id: 'agent', label: '🤖 Agent' },
    { id: 'backends', label: '🔌 Backends' },
    { id: 'import', label: '📦 Export' },
  ];

  return (
    <div className="h-full flex flex-col">
      <nav className="flex gap-0 border-b border-[#30363d] bg-[#161b22] shrink-0">
        {tabs.map(t => (
          <button key={t.id} onClick={() => setTab(t.id)}
            className={`px-4 py-2 text-xs border-b-2 transition-colors ${
              tab === t.id
                ? 'border-[#58a6ff] text-[#58a6ff] bg-[#58a6ff]/5'
                : 'border-transparent text-[#8b949e] hover:text-[#e6edf3] hover:bg-[#21262d]'
            }`}>
            {t.label}
          </button>
        ))}
      </nav>
      <div className="flex-1 overflow-hidden">
        {tab === 'agent' && <AgentSettings />}
        {tab === 'backends' && <BackendManager />}
        {tab === 'import' && <ImportExport />}
      </div>
    </div>
  );
}
