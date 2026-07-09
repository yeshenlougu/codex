/// <reference types="vite/client" />

interface ElectronAPI {
  minimize: () => void;
  maximize: () => void;
  close: () => void;
  isMaximized: () => Promise<boolean>;
  platform: string;
}

declare global {
  interface Window {
    electronAPI?: ElectronAPI;
  }
}

import { useTheme } from '../lib/ThemeContext';

export default function TitleBar() {
  const api = window.electronAPI;
  const isMac = api?.platform === 'darwin';
  const showControls = api && !isMac;
  const { theme, toggle } = useTheme();

  return (
    <div className="titlebar">
      <div className="titlebar-logo">
        <img src="/assets/app-icon.png" alt="Codex Go" className="titlebar-icon" />
        <span>Codex Go</span>
      </div>
      <div style={{ flex: 1 }} />
      <button
        className="titlebar-btn"
        onClick={toggle}
        title={theme === 'dark' ? 'Switch to light' : 'Switch to dark'}
        style={{ fontSize: 13, width: 32 }}
      >
        {theme === 'dark' ? '☀' : '☾'}
      </button>
      {showControls && (
        <div className="titlebar-win-controls">
          <button className="titlebar-btn" onClick={() => api!.minimize()} title="Minimize">─</button>
          <button className="titlebar-btn" onClick={() => api!.maximize()} title="Maximize">☐</button>
          <button className="titlebar-btn close" onClick={() => api!.close()} title="Close">✕</button>
        </div>
      )}
    </div>
  );
}
