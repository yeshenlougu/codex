import { useState } from 'react';
import AgentSettings from './settings/AgentSettings';
import BackendManager from './settings/BackendManager';
import ImportExport from './settings/ImportExport';

type SubPage = 'agent' | 'backends' | 'import-export';

const subNav: { id: SubPage; label: string; icon: string }[] = [
  { id: 'agent', label: 'Agent', icon: '🤖' },
  { id: 'backends', label: 'Backends', icon: '🔌' },
  { id: 'import-export', label: 'Import / Export', icon: '📦' },
];

export default function SettingsPage() {
  const [sub, setSub] = useState<SubPage>('agent');

  return (
    <>
      <aside className="sidebar">
        <div className="sidebar-header">
          <h3>Settings</h3>
        </div>
        <div style={{ padding: '4px 6px' }}>
          {subNav.map((item) => (
            <div
              key={item.id}
              className={`session-item ${sub === item.id ? 'active' : ''}`}
              onClick={() => setSub(item.id)}
              style={{ padding: '8px 10px' }}
            >
              <div className="session-info">
                <div className="session-title">
                  {item.icon} {item.label}
                </div>
              </div>
            </div>
          ))}
        </div>
      </aside>
      <div className="settings-container" style={{ flex: 1 }}>
        <h2 style={{ fontSize: 18, fontWeight: 600, color: 'var(--text-primary)', marginBottom: 16 }}>
          {subNav.find((x) => x.id === sub)?.label}
        </h2>
        {sub === 'agent' && <AgentSettings />}
        {sub === 'backends' && <BackendManager />}
        {sub === 'import-export' && <ImportExport />}
      </div>
    </>
  );
}
