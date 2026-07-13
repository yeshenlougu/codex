import { useState, useEffect, useMemo } from 'react';
import { Layout, Button, Modal, Input, Space, Typography, Tooltip } from 'antd';
import {
  FolderOpenOutlined, FolderAddOutlined, DeleteOutlined,
  HomeOutlined, EditOutlined, ClockCircleOutlined, AppstoreOutlined,
  SettingOutlined, DownOutlined, BulbOutlined,
} from '@ant-design/icons';
import type { SessionSummary } from '../lib/types';
import { listSessions, deleteSession, listFiles } from '../lib/api';
import type { Page } from '../App';

const { Sider } = Layout;
const { Text } = Typography;

interface Props {
  page: Page;
  sessionId: string;
  workspace: string;
  projects: string[];
  onNavigate: (p: Page) => void;
  onResumeSession: (id: string) => void;
  onNewSession: () => void;
  onWorkspaceChange: (w: string) => void;
  onProjectAdd: (path: string) => void;
}

// Project node with optional sub-paths (from workspace path structure)
interface ProjectNode {
  name: string;
  path: string;
  children: ProjectNode[];
}

function buildProjectTree(projects: string[]): ProjectNode[] {
  const root: ProjectNode[] = [];
  for (const p of projects) {
    const parts = p.replace(/\\/g, '/').split('/').filter(Boolean);
    if (parts.length === 0) continue;
    let level = root;
    let currentPath = '';
    for (let i = 0; i < parts.length; i++) {
      currentPath += (currentPath.endsWith('/') || i === 0 ? '' : '/') + parts[i];
      let existing = level.find(n => n.name === parts[i]);
      if (!existing) {
        existing = { name: parts[i], path: i === parts.length - 1 ? p : currentPath, children: [] };
        level.push(existing);
      }
      level = existing.children;
    }
  }
  return root;
}

export default function LeftSidebar(props: Props) {
  const { page, sessionId, workspace, projects, onNavigate, onResumeSession, onNewSession, onWorkspaceChange, onProjectAdd } = props;
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [wsPath, setWsPath] = useState('/');
  const [treeData, setTreeData] = useState<any[]>([]);
  const [expandedProjects, setExpandedProjects] = useState<Record<string, boolean>>({});

  const isElectron = !!window.electronAPI;

  // Get OS-aware default path on mount
  useEffect(() => {
    if (window.electronAPI) {
      window.electronAPI.getDefaultPath().then(p => setWsPath(p)).catch(() => setWsPath('/'));
    } else {
      // Web fallback: use reasonable default
      setWsPath('/');
    }
  }, []);

  const load = () => {
    listSessions().then(d => setSessions(d.sessions || [])).catch(() => {}).finally(() => setLoading(false));
  };
  useEffect(() => { load(); }, []);

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try { await deleteSession(id); setSessions(s => s.filter(x => x.id !== id)); } catch {}
  };

  const browseDir = async (dir: string) => {
    setWsPath(dir);
    try {
      const data = await listFiles(dir);
      const dirs = (data.files || [])
        .filter((f: any) => f.is_dir)
        .map((f: any) => ({ title: f.name, key: f.path, isLeaf: false, icon: <FolderOpenOutlined /> }));
      setTreeData(dirs);
    } catch { setTreeData([]); }
  };

  const openModal = async () => {
    // In Electron: use native folder picker
    if (window.electronAPI?.selectFolder) {
      try {
        const folder = await window.electronAPI.selectFolder();
        if (folder) {
          onProjectAdd(folder);
        }
      } catch { /* fall back to modal */ }
      return;
    }
    // In browser: show manual path input modal
    setModalOpen(true);
    browseDir(wsPath);
  };

  const handleAddProject = () => {
    if (wsPath.trim()) {
      onProjectAdd(wsPath.trim());
      setModalOpen(false);
    }
  };

  const toggleProject = (path: string) => {
    setExpandedProjects(prev => ({ ...prev, [path]: !prev[path] }));
  };

  // Build project tree
  const projectTree = useMemo(() => buildProjectTree(projects), [projects]);

  // Get recent tasks (sessions) up to 5
  const recentTasks = sessions.slice(0, 5);

  const renderProjectNode = (node: ProjectNode, depth: number = 0): React.ReactNode => {
    const isExpanded = expandedProjects[node.path] !== false; // default expanded
    const hasChildren = node.children.length > 0;
    const isActive = workspace === node.path || workspace.startsWith(node.path + '/');
    
    return (
      <div key={node.path}>
        <div
          onClick={() => {
            if (hasChildren) { toggleProject(node.path); }
            onWorkspaceChange(node.path);
            onNavigate('chat');
          }}
          style={{
            display: 'flex', alignItems: 'center', gap: 6,
            padding: '4px 10px', paddingLeft: 10 + depth * 14,
            borderRadius: 6, cursor: 'pointer', fontSize: 12,
            margin: '0 4px 1px',
            background: isActive ? 'var(--bg-active)' : 'transparent',
            color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)',
            fontWeight: isActive ? 500 : 400,
          }}
        >
          {hasChildren && (
            <DownOutlined style={{ fontSize: 8, transform: isExpanded ? 'rotate(0deg)' : 'rotate(-90deg)', transition: '0.15s' }} />
          )}
          {!hasChildren && <span style={{ width: 8, flexShrink: 0 }} />}
          <FolderOpenOutlined style={{ fontSize: 13, opacity: 0.7 }} />
          <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{node.name}</span>
        </div>
        {hasChildren && isExpanded && node.children.map(child => renderProjectNode(child, depth + 1))}
      </div>
    );
  };

  // Nav items matching Codex
  const navItems: { key: Page; label: string; icon: React.ReactNode }[] = [
    { key: 'chat', label: '新建任务', icon: <EditOutlined /> },
    { key: 'scheduled', label: '已安排', icon: <ClockCircleOutlined /> },
    { key: 'plugins', label: '插件', icon: <AppstoreOutlined /> },
  ];

  return (
    <Sider width={230} style={{
      background: 'var(--bg-panel)',
      borderRight: '1px solid var(--border)',
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
    }}>
      {/* Brand */}
      <div style={{ padding: '10px 14px 4px', display: 'flex', alignItems: 'center', gap: 4 }}>
        <Text style={{ fontSize: 13, fontWeight: 600, letterSpacing: -0.3, color: 'var(--text-primary)' }}>
          Codex
        </Text>
        <Text style={{ fontSize: 13, fontWeight: 600, letterSpacing: -0.3, color: '#5e6ad2' }}>
          Go
        </Text>
        <DownOutlined style={{ fontSize: 9, marginLeft: 2, opacity: 0.4 }} />
      </div>

      {/* Nav items */}
      <div style={{ padding: '2px 4px' }}>
        {navItems.map(item => (
          <div
            key={item.key}
            onClick={() => onNavigate(item.key)}
            style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '6px 12px', borderRadius: 6, cursor: 'pointer',
              fontSize: 12, margin: '1px 0',
              background: page === item.key ? 'var(--bg-active)' : 'transparent',
              color: page === item.key ? 'var(--text-primary)' : 'var(--text-secondary)',
              fontWeight: page === item.key ? 500 : 400,
            }}
          >
            <span style={{ fontSize: 14, opacity: page === item.key ? 1 : 0.6 }}>{item.icon}</span>
            <span>{item.label}</span>
          </div>
        ))}
      </div>

      <div style={{ height: 1, background: 'var(--border)', margin: '4px 8px' }} />

      {/* Projects section */}
      <div style={{ padding: '2px 12px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Text type="secondary" style={{ fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.08em' }}>
          项目
        </Text>
        <Tooltip title="打开文件夹">
          <Button type="text" size="small" icon={<FolderAddOutlined />} onClick={openModal} style={{ fontSize: 12 }} />
        </Tooltip>
      </div>
      <div style={{ flex: '0 0 auto', maxHeight: 160, overflowY: 'auto', padding: '0 2px' }}>
        {projectTree.length === 0 ? (
          <Text type="secondary" style={{ fontSize: 10, padding: '4px 14px', display: 'block' }}>
            无项目 — 打开文件夹
          </Text>
        ) : (
          projectTree.map(node => renderProjectNode(node))
        )}
      </div>

      <div style={{ height: 1, background: 'var(--border)', margin: '4px 8px' }} />

      {/* Recent tasks (conversations) */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '0 2px' }}>
        {loading ? (
          <Text type="secondary" style={{ fontSize: 11, padding: '8px 14px', display: 'block', textAlign: 'center' }}>加载中...</Text>
        ) : recentTasks.length === 0 ? (
          <Text type="secondary" style={{ fontSize: 11, padding: '8px 14px', display: 'block', textAlign: 'center' }}>暂无任务</Text>
        ) : (
          recentTasks.map(s => (
            <div
              key={s.id}
              onClick={() => onResumeSession(s.id)}
              style={{
                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                padding: '5px 10px', borderRadius: 6, cursor: 'pointer', fontSize: 11,
                margin: '0 4px 1px',
                background: s.id === sessionId ? 'var(--bg-active)' : 'transparent',
                borderLeft: s.id === sessionId ? '2px solid #5e6ad2' : '2px solid transparent',
              }}
            >
              <div style={{ overflow: 'hidden', flex: 1 }}>
                <div style={{ fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 11 }}>
                  {s.title || s.id.slice(-12)}
                </div>
              </div>
              <Button
                type="text" size="small" danger
                icon={<DeleteOutlined />}
                onClick={(e) => handleDelete(s.id, e)}
                style={{ opacity: 0, fontSize: 10 }}
                className="ls-del-btn"
              />
            </div>
          ))
        )}
        {sessions.length > 5 && (
          <Text type="secondary" style={{ fontSize: 10, padding: '4px 14px', display: 'block', cursor: 'pointer' }}>
            展开显示
          </Text>
        )}
      </div>

      {/* Bottom — sticky */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '8px 14px 6px', borderTop: '1px solid var(--border)',
        marginTop: 'auto',
      }}>
        <div
          onClick={() => onNavigate('settings')}
          style={{
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer', fontSize: 18,
            color: page === 'settings' ? 'var(--accent)' : 'var(--text-muted)',
          }}
        >
          <SettingOutlined />
        </div>
        <Text type="secondary" style={{ fontSize: 9 }}>
          <BulbOutlined style={{ marginRight: 3 }} />v1.0
        </Text>
      </div>

      {/* Open Folder Modal */}
      <Modal
        title="打开文件夹作为项目"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleAddProject}
        okText="打开"
        cancelText="取消"
        width={480}
      >
        <Space direction="vertical" style={{ width: '100%' }} size={12}>
          <Input
            value={wsPath}
            onChange={e => setWsPath(e.target.value)}
            onPressEnter={() => browseDir(wsPath)}
            placeholder="/home/ubuntu/projects/my-app"
            prefix={<HomeOutlined />}
          />
          <Button size="small" onClick={() => {
            const parent = wsPath.split('/').slice(0, -1).join('/') || '/';
            browseDir(parent);
          }}>
            ⬆ 上级目录
          </Button>
          <div style={{ maxHeight: 240, overflowY: 'auto', border: '1px solid var(--border)', borderRadius: 6, padding: 4 }}>
            {treeData.length === 0 ? (
              <Text type="secondary" style={{ padding: 12, display: 'block', textAlign: 'center' }}>无目录</Text>
            ) : (
              treeData.map((d: any) => (
                <div
                  key={d.key}
                  onClick={() => browseDir(d.key)}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 8,
                    padding: '6px 8px', borderRadius: 4, cursor: 'pointer',
                    fontSize: 12, color: '#5e6ad2',
                  }}
                >
                  <FolderOpenOutlined /> {d.title}
                </div>
              ))
            )}
          </div>
        </Space>
      </Modal>
    </Sider>
  );
}
