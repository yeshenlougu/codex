/// <reference types="vite/client" />

interface ElectronAPI {
  minimize: () => void;
  maximize: () => void;
  close: () => void;
  isMaximized: () => Promise<boolean>;
  platform: string;
  selectFolder: () => Promise<string | null>;
  getDefaultPath: () => Promise<string>;
}

declare global {
  interface Window {
    electronAPI?: ElectronAPI;
  }
}

import { useTheme } from '../lib/ThemeContext';
import { Button, Tooltip } from 'antd';
import { MenuFoldOutlined, MenuUnfoldOutlined, SunOutlined, MoonOutlined } from '@ant-design/icons';

interface Props {
  rightPanelOpen: boolean;
  onToggleRight: () => void;
}

export default function TitleBar({ rightPanelOpen, onToggleRight }: Props) {
  const api = window.electronAPI;
  const showControls = api && api.platform !== 'darwin';
  const { theme, toggle } = useTheme();

  return (
    <div className="titlebar" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
      <div className="titlebar-logo">
        <img src="/assets/app-icon.png" alt="Codex Go" className="titlebar-icon" />
        <span>Codex Go</span>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
        <Tooltip title={rightPanelOpen ? '隐藏侧边栏 Ctrl+Alt+B' : '显示侧边栏 Ctrl+Alt+B'}>
          <Button
            type="text"
            size="small"
            icon={rightPanelOpen ? <MenuFoldOutlined /> : <MenuUnfoldOutlined />}
            onClick={onToggleRight}
            style={{ color: rightPanelOpen ? 'var(--accent)' : 'var(--text-muted)', fontSize: 12 }}
          />
        </Tooltip>
        <Button
          type="text"
          size="small"
          icon={theme === 'dark' ? <SunOutlined /> : <MoonOutlined />}
          onClick={toggle}
          style={{ fontSize: 12 }}
        />
        {showControls && (
          <div className="titlebar-win-controls" style={{ display: 'flex', marginLeft: 4 }}>
            <button className="titlebar-btn" onClick={() => api!.minimize()} title="Minimize">─</button>
            <button className="titlebar-btn" onClick={() => api!.maximize()} title="Maximize">☐</button>
            <button className="titlebar-btn close" onClick={() => api!.close()} title="Close">✕</button>
          </div>
        )}
      </div>
    </div>
  );
}
