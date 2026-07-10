import { useState, useEffect, useCallback } from 'react';
import { Tabs, Tree, Empty, Typography, Spin } from 'antd';
import { FolderOutlined, FileOutlined, FolderOpenOutlined } from '@ant-design/icons';
import { listFiles, readFileContent } from '../lib/api';
import type { RightTab } from '../App';

const { Text } = Typography;

interface Props {
  tab: RightTab;
  onTabChange: (t: RightTab) => void;
}

interface FileNode {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
}

export default function RightPanel({ tab, onTabChange }: Props) {
  const items = [
    { key: 'files', label: '📁 Files' },
    { key: 'changes', label: '📝 Changes' },
    { key: 'git', label: '🔀 Git' },
  ];

  return (
    <div style={{ width: 300, display: 'flex', flexDirection: 'column', background: 'var(--bg-panel)', borderLeft: '1px solid var(--border)', flexShrink: 0 }}>
      <Tabs
        activeKey={tab}
        onChange={k => onTabChange(k as RightTab)}
        items={items}
        size="small"
        tabBarStyle={{ marginBottom: 0, padding: '0 8px' }}
      />
      <div style={{ flex: 1, overflow: 'hidden' }}>
        {tab === 'files' && <FilesPanel />}
        {tab === 'changes' && <ChangesPanel />}
        {tab === 'git' && <GitPanel />}
      </div>
    </div>
  );
}

function FilesPanel() {
  const [cwd, setCwd] = useState('/home/ubuntu/app/codex');
  const [treeData, setTreeData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [fileContent, setFileContent] = useState('');

  const loadDir = useCallback(async (dir: string) => {
    setLoading(true);
    try {
      const r = await listFiles(dir);
      const nodes = (r.files || []).map((f: FileNode) => ({
        title: f.name,
        key: f.path,
        isLeaf: !f.is_dir,
        icon: f.is_dir ? <FolderOutlined style={{ color: '#5e6ad2' }} /> : <FileOutlined />,
        isDirectory: f.is_dir,
      }));
      setTreeData(nodes);
      setCwd(r.path || dir);
    } catch { setTreeData([]); }
    setLoading(false);
  }, []);

  useEffect(() => { loadDir(cwd); }, [cwd]);

  const handleSelect = async (keys: React.Key[]) => {
    if (keys.length === 0) return;
    const key = keys[0] as string;
    const node = treeData.find(n => n.key === key);
    if (!node) return;

    if (node.isDirectory) {
      setCwd(key);
      setSelectedFile(null);
      setFileContent('');
    } else {
      setSelectedFile(key);
      try {
        const r = await readFileContent(key);
        setFileContent(r.binary ? '[Binary file]' : (r.content || ''));
      } catch { setFileContent('[Error reading file]'); }
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ padding: '6px 10px', borderBottom: '1px solid var(--border)' }}>
        <Text type="secondary" style={{ fontSize: 11 }}>📁 {cwd}</Text>
      </div>
      <div style={{ flex: 1, overflowY: 'auto', padding: '4px' }}>
        {loading ? (
          <div style={{ textAlign: 'center', padding: 24 }}><Spin size="small" /></div>
        ) : (
          treeData.map(node => (
            <div
              key={node.key}
              onClick={() => handleSelect([node.key])}
              style={{
                display: 'flex', alignItems: 'center', gap: 6,
                padding: '4px 8px', borderRadius: 4, cursor: 'pointer',
                fontSize: 12, color: node.isDirectory ? '#5e6ad2' : 'var(--text-secondary)',
                background: selectedFile === node.key ? 'var(--bg-active)' : 'transparent',
              }}
            >
              {node.icon}
              <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{node.title}</span>
            </div>
          ))
        )}
      </div>
      {selectedFile && (
        <div style={{ height: '40%', borderTop: '1px solid var(--border)', display: 'flex', flexDirection: 'column' }}>
          <div style={{ padding: '4px 10px', fontSize: 10, color: 'var(--text-muted)', background: 'var(--bg-root)', borderBottom: '1px solid var(--border)' }}>
            {selectedFile.split('/').pop()}
          </div>
          <pre style={{
            flex: 1, overflow: 'auto', padding: '6px 10px', margin: 0,
            fontSize: 10, fontFamily: "'JetBrains Mono', monospace",
            color: 'var(--text-secondary)', whiteSpace: 'pre-wrap', lineHeight: 1.5,
          }}>{fileContent}</pre>
        </div>
      )}
    </div>
  );
}

function ChangesPanel() {
  return (
    <Empty
      image={Empty.PRESENTED_IMAGE_SIMPLE}
      description={<Text type="secondary">No active changes</Text>}
      style={{ padding: '40px 20px' }}
    >
      <Text type="secondary" style={{ fontSize: 11 }}>File diffs will appear during agent sessions</Text>
    </Empty>
  );
}

function GitPanel() {
  const [status, setStatus] = useState('');

  useEffect(() => {
    fetch('/api/git-status')
      .then(r => r.json())
      .then(d => setStatus(d.status || d.output || ''))
      .catch(() => setStatus('Git status unavailable'));
  }, []);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ padding: '6px 10px', fontWeight: 600, fontSize: 12, borderBottom: '1px solid var(--border)' }}>🔀 Git Status</div>
      <pre style={{
        flex: 1, overflow: 'auto', padding: '8px 10px', margin: 0,
        fontSize: 10, fontFamily: "'JetBrains Mono', monospace",
        color: 'var(--text-secondary)', whiteSpace: 'pre-wrap',
      }}>{status || 'Loading...'}</pre>
    </div>
  );
}
