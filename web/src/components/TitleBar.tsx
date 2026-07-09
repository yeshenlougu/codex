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

export default function TitleBar() {
  const api = window.electronAPI;
  const isMac = api?.platform === 'darwin';
  const showControls = api && !isMac;

  return (
    <div className="titlebar">
      <div className="titlebar-logo">
        <img src="/assets/app-icon.png" alt="Codex Go" className="titlebar-icon" />
        <span>Codex Go</span>
      </div>
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
