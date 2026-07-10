import { useState, useEffect } from 'react';
import { Layout, Menu, Button, Modal, Input, Space, Typography } from 'antd';
import {
  MessageOutlined, SettingOutlined, PlusOutlined, FolderOpenOutlined,
  FolderAddOutlined, DeleteOutlined, HomeOutlined,
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

export default function LeftSidebar(props: Props) {
  const { page, sessionId, workspace, projects, onNavigate, onResumeSession, onNewSession, onWorkspaceChange, onProjectAdd } = props;
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [wsPath, setWsPath] = useState('/home/ubuntu');
  const [treeData, setTreeData] = useState<any[]>([]);

  const load = () => {
    listSessions().then(d => setSessions(d.sessions || [])).catch(() => {}).finally(() => setLoading(false));
  };
  useEffect(() => { load(); }, []);

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try { await deleteSession(id); setSessions(s => s.filter(x => x.id !== id)); } catch {}
  };

  // Browse directory
  const browseDir = async (dir: string) => {
    setWsPath(dir);
    try {
      const data = await listFiles(dir);
      const dirs = (data.files || [])
        .filter((f: any) => f.is_dir)
        .map((f: any) => ({
          title: f.name,
          key: f.path,
          isLeaf: false,
          icon: <FolderOpenOutlined />,
        }));
      setTreeData(dirs);
    } catch { setTreeData([]); }
  };

  const openModal = () => {
    setModalOpen(true);
    browseDir(wsPath);
  };

  const handleAddProject = () => {
    if (wsPath.trim()) {
      onProjectAdd(wsPath.trim());
      setModalOpen(false);
    }
  };

  // Build menu items
  const menuItems = [
    { key: 'chat', icon: <MessageOutlined />, label: 'Chat' },
  ];

  // Group sessions by workspace
  const grouped: Record<string, SessionSummary[]> = {};
  sessions.forEach(s => {
    const ws = (s as any).workspace || workspace || 'default';
    if (!grouped[ws]) grouped[ws] = [];
    grouped[ws].push(s);
  });

  return (
    <Sider width={260} style={{
      background: 'var(--bg-panel)',
      borderRight: '1px solid var(--border)',
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
    }}>
      {/* Brand */}
      <div style={{ padding: '10px 16px 6px' }}>
        <Text strong style={{ fontSize: 13, letterSpacing: -0.3 }}>
          <span style={{ color: 'var(--text-primary)' }}>Codex</span>{' '}
          <span style={{ color: '#5e6ad2' }}>Go</span>
        </Text>
      </div>

      {/* Main nav */}
      <Menu
        mode="inline"
        selectedKeys={[page]}
        onClick={({ key }) => onNavigate(key as Page)}
        items={menuItems}
        style={{ background: 'transparent', border: 'none' }}
      />

      {/* Projects section */}
      <div style={{ padding: '4px 12px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Text type="secondary" style={{ fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.08em' }}>
          Projects
        </Text>
        <Button type="text" size="small" icon={<FolderAddOutlined />} onClick={openModal} />
      </div>
      <div style={{ flex: '0 0 auto', maxHeight: 180, overflowY: 'auto', padding: '0 4px' }}>
        {projects.length === 0 ? (
          <Text type="secondary" style={{ fontSize: 10, padding: '4px 12px', display: 'block' }}>
            No projects — open a folder
          </Text>
        ) : (
          projects.map(proj => {
            const name = proj.split('/').filter(Boolean).pop() || proj;
            return (
              <div
                key={proj}
                onClick={() => { onWorkspaceChange(proj); onNavigate('chat'); }}
                style={{
                  display: 'flex', alignItems: 'center', gap: 8,
                  padding: '6px 10px', borderRadius: 6, cursor: 'pointer',
                  fontSize: 12, marginBottom: 1,
                  background: workspace === proj ? 'var(--bg-active)' : 'transparent',
                  color: workspace === proj ? 'var(--text-primary)' : 'var(--text-secondary)',
                  borderLeft: workspace === proj ? '2px solid #5e6ad2' : '2px solid transparent',
                }}
              >
                <FolderOpenOutlined style={{ fontSize: 13, opacity: 0.7 }} />
                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{name}</span>
              </div>
            );
          })
        )}
      </div>

      <div style={{ height: 1, background: 'var(--border)', margin: '4px 8px' }} />

      {/* Sessions */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '0 4px' }}>
        {loading ? (
          <Text type="secondary" style={{ fontSize: 11, padding: '8px 12px', display: 'block', textAlign: 'center' }}>Loading...</Text>
        ) : sessions.length === 0 ? (
          <Text type="secondary" style={{ fontSize: 11, padding: '8px 12px', display: 'block', textAlign: 'center' }}>No sessions</Text>
        ) : (
          Object.keys(grouped).map(ws => (
            <div key={ws}>
              <Text type="secondary" style={{ fontSize: 9, fontWeight: 700, textTransform: 'uppercase', padding: '6px 12px 2px', display: 'block' }}>
                📁 {ws === 'default' ? 'Default' : ws.split('/').pop() || ws}
              </Text>
              {grouped[ws].map(s => (
                <div
                  key={s.id}
                  onClick={() => onResumeSession(s.id)}
                  style={{
                    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                    padding: '4px 8px', borderRadius: 6, cursor: 'pointer', fontSize: 11,
                    background: s.id === sessionId ? 'var(--bg-active)' : 'transparent',
                    borderLeft: s.id === sessionId ? '2px solid #5e6ad2' : '2px solid transparent',
                    marginBottom: 1,
                  }}
                >
                  <div style={{ overflow: 'hidden' }}>
                    <div style={{ fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {s.title || s.id.slice(-12)}
                    </div>
                    <Text type="secondary" style={{ fontSize: 9 }}>{s.msg_count} msgs</Text>
                  </div>
                  <Button
                    type="text" size="small" danger
                    icon={<DeleteOutlined />}
                    onClick={(e) => handleDelete(s.id, e)}
                    style={{ opacity: 0.4 }}
                    className="ls-del-btn"
                  />
                </div>
              ))}
            </div>
          ))
        )}
      </div>

      {/* New session */}
      <div style={{ padding: '4px 8px' }}>
        <Button block size="small" icon={<PlusOutlined />} onClick={onNewSession}>
          New Session
        </Button>
      </div>

      {/* Settings */}
      <div style={{ borderTop: '1px solid var(--border)' }}>
        <Menu
          mode="inline"
          selectedKeys={[page]}
          onClick={({ key }) => onNavigate(key as Page)}
          items={[{ key: 'settings', icon: <SettingOutlined />, label: 'Settings' }]}
          style={{ background: 'transparent', border: 'none' }}
        />
      </div>

      {/* Open Folder Modal */}
      <Modal
        title="Open Folder as Project"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleAddProject}
        okText="Open"
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
            ⬆ Parent directory
          </Button>
          <div style={{ maxHeight: 240, overflowY: 'auto', border: '1px solid var(--border)', borderRadius: 6, padding: 4 }}>
            {treeData.length === 0 ? (
              <Text type="secondary" style={{ padding: 12, display: 'block', textAlign: 'center' }}>No directories</Text>
            ) : (
              treeData.map(d => (
                <div
                  key={d.key}
                  onClick={() => browseDir(d.key)}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 8,
                    padding: '6px 8px', borderRadius: 4, cursor: 'pointer',
                    fontSize: 12, color: '#5e6ad2',
                  }}
                  className="rp-file-item is-dir"
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
