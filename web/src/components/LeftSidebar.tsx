import { useState, useEffect, useMemo } from 'react';
import { Layout, Button, Modal, Input, Space, Typography, Tooltip, Badge } from 'antd';
import {
  FolderOpenOutlined, FolderAddOutlined, DeleteOutlined,
  HomeOutlined, EditOutlined, SettingOutlined, DownOutlined, BulbOutlined,
  MessageOutlined, FolderOutlined, ClockCircleOutlined, CaretDownOutlined,
  SearchOutlined, CloseOutlined, ReloadOutlined,
} from '@ant-design/icons';
import type { SessionSummary, AgentProfile } from '../lib/types';
import { listSessions, deleteSession, listFiles, listAgents } from '../lib/api';
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

/** Relative time formatter (e.g. "2分钟前", "昨天") */
function relativeTime(ts: string): string {
  if (!ts) return '';
  const d = new Date(ts);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return '刚刚';
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}分钟前`;
  const hour = Math.floor(min / 60);
  if (hour < 24) {
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const that = new Date(d.getFullYear(), d.getMonth(), d.getDate());
    const days = Math.floor((today.getTime() - that.getTime()) / 86400000);
    if (days === 0) return `${hour}小时前`;
    if (days === 1) return '昨天';
    if (days < 7) return `${days}天前`;
  }
  return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' });
}

/** Group sessions by date */
function groupByDate(sessions: SessionSummary[]): { label: string; items: SessionSummary[] }[] {
  const groups: Record<string, SessionSummary[]> = {};
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 86400000);
  const weekAgo = new Date(today.getTime() - 7 * 86400000);

  for (const s of sessions) {
    const d = new Date(s.updated_at || s.created_at);
    const that = new Date(d.getFullYear(), d.getMonth(), d.getDate());
    let key: string;
    if (that.getTime() >= today.getTime()) key = '今天';
    else if (that.getTime() >= yesterday.getTime()) key = '昨天';
    else if (that.getTime() >= weekAgo.getTime()) key = '本周';
    else key = d.toLocaleDateString('zh-CN', { month: 'long' });
    if (!groups[key]) groups[key] = [];
    groups[key].push(s);
  }
  // Order: Today, Yesterday, This week, then month groups
  const order = ['今天', '昨天', '本周'];
  const result: { label: string; items: SessionSummary[] }[] = [];
  for (const k of order) {
    if (groups[k]) { result.push({ label: k, items: groups[k] }); delete groups[k]; }
  }
  for (const k of Object.keys(groups).sort()) {
    result.push({ label: k, items: groups[k] });
  }
  return result;
}

export default function LeftSidebar(props: Props) {
  const { page, sessionId, workspace, projects, onNavigate, onResumeSession, onWorkspaceChange, onProjectAdd } = props;
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [wsPath, setWsPath] = useState('/');
  const [treeData, setTreeData] = useState<any[]>([]);
  const [expandedProjects, setExpandedProjects] = useState<Record<string, boolean>>({});
  const [showAllSessions, setShowAllSessions] = useState(false);
  const [agents, setAgents] = useState<AgentProfile[]>([]);
  const [sectionsCollapsed, setSectionsCollapsed] = useState<Record<string, boolean>>({});

  // Get OS-aware default path on mount
  useEffect(() => {
    if (window.electronAPI) {
      window.electronAPI.getDefaultPath().then(p => setWsPath(p)).catch(() => setWsPath('/'));
    } else {
      setWsPath('/');
    }
  }, []);

  const load = () => {
    setLoading(true);
    listSessions().then(d => setSessions(d.sessions || [])).catch(() => {}).finally(() => setLoading(false));
  };
  useEffect(() => { load(); }, []);

  // Load agents
  useEffect(() => {
    listAgents().then(a => setAgents(a.agents || [])).catch(() => {});
  }, []);

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
    if (window.electronAPI?.selectFolder) {
      try {
        const folder = await window.electronAPI.selectFolder();
        if (folder) { onProjectAdd(folder); }
      } catch {}
      return;
    }
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

  const toggleSection = (section: string) => {
    setSectionsCollapsed(prev => ({ ...prev, [section]: !prev[section] }));
  };

  // Build project tree
  const projectTree = useMemo(() => buildProjectTree(projects), [projects]);

  // Group sessions by date
  const sessionGroups = useMemo(() => groupByDate(sessions), [sessions]);

  // Flattened visible sessions (limited unless showAll)
  const visibleSessions = showAllSessions ? sessions : sessions.slice(0, 8);

  // Agent count
  const agentCount = agents.length;
  const workspaceName = workspace.split('/').filter(Boolean).pop() || workspace;

  const renderProjectNode = (node: ProjectNode, depth: number = 0): React.ReactNode => {
    const isExpanded = expandedProjects[node.path] !== false;
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
            transition: 'background 0.12s',
          }}
        >
          {hasChildren && (
            <CaretDownOutlined style={{ fontSize: 8, transform: isExpanded ? 'rotate(0deg)' : 'rotate(-90deg)', transition: 'transform 0.15s' }} />
          )}
          {!hasChildren && <span style={{ width: 8, flexShrink: 0 }} />}
          <FolderOutlined style={{ fontSize: 13, opacity: isActive ? 0.9 : 0.6 }} />
          <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{node.name}</span>
        </div>
        {hasChildren && isExpanded && node.children.map(child => renderProjectNode(child, depth + 1))}
      </div>
    );
  };

  // Nav items
  const navItems: { key: Page; label: string; icon: React.ReactNode }[] = [
    { key: 'chat', label: '新建任务', icon: <EditOutlined /> },
  ];

  return (
    <Sider width={240} style={{
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
              transition: 'all 0.12s',
            }}
          >
            <span style={{ fontSize: 14, opacity: page === item.key ? 1 : 0.6 }}>{item.icon}</span>
            <span>{item.label}</span>
          </div>
        ))}
      </div>

      <div style={{ height: 1, background: 'var(--border)', margin: '4px 8px' }} />

      {/* ── Workspace section (collapsible) ── */}
      <div
        onClick={() => toggleSection('workspace')}
        style={{
          padding: '4px 12px', display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          cursor: 'pointer', userSelect: 'none',
        }}
      >
        <Text type="secondary" style={{ fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.08em' }}>
          工作区
        </Text>
        <CaretDownOutlined style={{
          fontSize: 10, color: 'var(--text-muted)',
          transform: sectionsCollapsed['workspace'] ? 'rotate(-90deg)' : 'rotate(0deg)',
          transition: 'transform 0.15s',
        }} />
      </div>

      {!sectionsCollapsed['workspace'] && (
        <>
          {/* Current workspace */}
          <div style={{
            display: 'flex', alignItems: 'center', gap: 6,
            padding: '4px 12px', margin: '0 4px',
          }}>
            <FolderOpenOutlined style={{ fontSize: 13, color: '#5e6ad2' }} />
            <Text style={{ fontSize: 11, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }}>
              {workspaceName}
            </Text>
            {agentCount > 0 && (
              <Badge count={agentCount} size="small" style={{ backgroundColor: '#5e6ad2' }} title={`${agentCount} agents`} />
            )}
          </div>

          {/* Projects section */}
          <div style={{ padding: '2px 12px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <Text type="secondary" style={{ fontSize: 10, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em' }}>
              项目
            </Text>
            <Tooltip title="打开文件夹">
              <Button type="text" size="small" icon={<FolderAddOutlined />} onClick={openModal}
                style={{ fontSize: 11, width: 24, height: 24, padding: 0 }} />
            </Tooltip>
          </div>
          <div style={{ flex: '0 0 auto', maxHeight: 120, overflowY: 'auto', padding: '0 2px' }}>
            {projectTree.length === 0 ? (
              <Text type="secondary" style={{ fontSize: 10, padding: '4px 14px', display: 'block' }}>
                无项目
              </Text>
            ) : (
              projectTree.map(node => renderProjectNode(node))
            )}
          </div>
        </>
      )}

      <div style={{ height: 1, background: 'var(--border)', margin: '4px 8px' }} />

      {/* ── History section (collapsible) ── */}
      <div
        onClick={() => toggleSection('history')}
        style={{
          padding: '4px 12px', display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          cursor: 'pointer', userSelect: 'none',
        }}
      >
        <Text type="secondary" style={{ fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.08em' }}>
          历史
        </Text>
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <Tooltip title="刷新">
            <Button type="text" size="small" icon={<ReloadOutlined />}
              onClick={(e) => { e.stopPropagation(); load(); }}
              style={{ fontSize: 10, width: 20, height: 20, padding: 0 }} />
          </Tooltip>
          <CaretDownOutlined style={{
            fontSize: 10, color: 'var(--text-muted)',
            transform: sectionsCollapsed['history'] ? 'rotate(-90deg)' : 'rotate(0deg)',
            transition: 'transform 0.15s',
          }} />
        </div>
      </div>

      {!sectionsCollapsed['history'] && (
        <div style={{ flex: 1, overflowY: 'auto', padding: '0 2px', minHeight: 0 }}>
          {loading ? (
            <Text type="secondary" style={{ fontSize: 11, padding: '12px 14px', display: 'block', textAlign: 'center' }}>加载中...</Text>
          ) : visibleSessions.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 11, padding: '12px 14px', display: 'block', textAlign: 'center' }}>暂无记录</Text>
          ) : (
            <>
              {showAllSessions ? (
                // Grouped view
                sessionGroups.map(group => (
                  <div key={group.label}>
                    <Text type="secondary" style={{
                      fontSize: 9, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.08em',
                      padding: '6px 14px 2px', display: 'block',
                    }}>
                      <ClockCircleOutlined style={{ marginRight: 4 }} />{group.label}
                    </Text>
                    {group.items.map(s => renderSessionItem(s))}
                  </div>
                ))
              ) : (
                // Simple list (first 8)
                visibleSessions.map(s => renderSessionItem(s))
              )}
              {sessions.length > 8 && (
                <div
                  onClick={() => setShowAllSessions(!showAllSessions)}
                  style={{
                    padding: '5px 14px', cursor: 'pointer', fontSize: 11,
                    color: 'var(--accent)', textAlign: 'center',
                    borderTop: '1px solid var(--border)', marginTop: 4,
                  }}
                >
                  {showAllSessions ? `收起 · 显示最近 8 条` : `展开全部 · 共 ${sessions.length} 条`}
                </div>
              )}
            </>
          )}
        </div>
      )}

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
            transition: 'color 0.15s',
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

  // Helper: render single session item
  function renderSessionItem(s: SessionSummary) {
    const trimmed = (s.title || '').trim();
    const displayTitle = trimmed || s.id.slice(-12);
    const timeStr = relativeTime(s.updated_at || s.created_at);

    return (
      <div
        key={s.id}
        onClick={() => onResumeSession(s.id)}
        style={{
          display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between',
          padding: '5px 10px', borderRadius: 6, cursor: 'pointer', fontSize: 11,
          margin: '0 4px 1px',
          background: s.id === sessionId ? 'var(--bg-active)' : 'transparent',
          borderLeft: s.id === sessionId ? '2px solid #5e6ad2' : '2px solid transparent',
          transition: 'background 0.1s',
        }}
      >
        <div style={{ overflow: 'hidden', flex: 1, minWidth: 0 }}>
          <div style={{
            fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis',
            whiteSpace: 'nowrap', fontSize: 11, lineHeight: 1.4,
          }}>
            <MessageOutlined style={{ fontSize: 10, marginRight: 4, opacity: 0.5 }} />
            {displayTitle}
          </div>
          {timeStr && (
            <Text type="secondary" style={{ fontSize: 9, display: 'block', marginTop: 1 }}>
              {timeStr}
            </Text>
          )}
        </div>
        <Button
          type="text" size="small" danger
          icon={<CloseOutlined />}
          onClick={(e) => handleDelete(s.id, e)}
          style={{
            opacity: 0, fontSize: 9, width: 18, height: 18, padding: 0, flexShrink: 0,
            transition: 'opacity 0.15s',
          }}
          className="ls-del-btn"
        />
      </div>
    );
  }
}
