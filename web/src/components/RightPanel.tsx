import { useState, useEffect, useCallback } from 'react';
import { Empty, Typography, Spin, Button, Input, Tooltip } from 'antd';
import {
  FolderOutlined, FileOutlined, AuditOutlined, CodeOutlined,
  GlobalOutlined, UnorderedListOutlined, CloseOutlined,
} from '@ant-design/icons';
import { listFiles, readFileContent, getTasks, execTerminal } from '../lib/api';
import type { RightTab } from '../App';
import type { Task } from '../lib/api';

const { Text } = Typography;

interface Props {
  tab: RightTab;
  onTabChange: (t: RightTab) => void;
  onClose: () => void;
}

interface FileNode {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
}

const TAB_ITEMS = [
  { key: 'review' as RightTab, label: '审阅', icon: <AuditOutlined />, shortcut: 'Ctrl+Shift+G' },
  { key: 'terminal' as RightTab, label: '终端', icon: <CodeOutlined /> },
  { key: 'browser' as RightTab, label: '浏览器', icon: <GlobalOutlined />, shortcut: 'Ctrl+T' },
  { key: 'files' as RightTab, label: '文件', icon: <FolderOutlined />, shortcut: 'Ctrl+P' },
  { key: 'sidetasks' as RightTab, label: '侧边任务', icon: <UnorderedListOutlined />, shortcut: 'Ctrl+Alt+S' },
];

export default function RightPanel({ tab, onTabChange, onClose }: Props) {
  return (
    <div style={{
      width: 320, display: 'flex', flexDirection: 'column',
      background: 'var(--bg-panel)', borderLeft: '1px solid var(--border)',
      flexShrink: 0,
    }}>
      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '6px 8px', borderBottom: '1px solid var(--border)',
      }}>
        <div style={{ display: 'flex', gap: 0 }}>
          {TAB_ITEMS.map(item => (
            <Tooltip key={item.key} title={item.shortcut || item.label}>
              <Button
                type="text"
                size="small"
                icon={item.icon}
                onClick={() => onTabChange(item.key)}
                style={{
                  color: tab === item.key ? 'var(--accent)' : 'var(--text-muted)',
                  background: tab === item.key ? 'var(--bg-active)' : 'transparent',
                  fontSize: 14,
                }}
              />
            </Tooltip>
          ))}
        </div>
        <Tooltip title="关闭 Ctrl+Alt+B">
          <Button type="text" size="small" icon={<CloseOutlined />} onClick={onClose}
            style={{ color: 'var(--text-muted)', fontSize: 12 }} />
        </Tooltip>
      </div>

      {/* Content */}
      <div style={{ flex: 1, overflow: 'hidden' }}>
        {tab === 'files' && <FilesPanel />}
        {tab === 'review' && <ReviewPanel />}
        {tab === 'terminal' && <TerminalPanel />}
        {tab === 'browser' && <BrowserPanel />}
        {tab === 'sidetasks' && <SideTasksPanel />}
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
                padding: '3px 8px', borderRadius: 4, cursor: 'pointer',
                fontSize: 11, color: node.isDirectory ? '#5e6ad2' : 'var(--text-secondary)',
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
          <div style={{
            padding: '4px 10px', fontSize: 10, color: 'var(--text-muted)',
            background: 'var(--bg-root)', borderBottom: '1px solid var(--border)',
          }}>
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

// Review panel — shows recent diffs/changes
function ReviewPanel() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ padding: '8px 12px', borderBottom: '1px solid var(--border)' }}>
        <Text strong style={{ fontSize: 12 }}>🔍 审阅更改</Text>
      </div>
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={<Text type="secondary" style={{ fontSize: 11 }}>暂无活动更改</Text>}
        style={{ padding: '40px 20px' }}
      >
        <Text type="secondary" style={{ fontSize: 10 }}>文件差异将在 Agent 会话期间出现</Text>
      </Empty>
    </div>
  );
}

// Terminal panel — simple command output
function TerminalPanel() {
  const [output, setOutput] = useState<string[]>(['Codex Go Terminal v1.0.0', '输入命令并按回车执行...', '']);
  const [cmd, setCmd] = useState('');
  const [executing, setExecuting] = useState(false);

  const handleCmd = async () => {
    const c = cmd.trim();
    if (!c || executing) return;
    setOutput(prev => [...prev, `$ ${c}`, '']);
    setCmd('');
    setExecuting(true);
    try {
      const res = await execTerminal(c);
      setOutput(prev => [...prev, res.output || '', '']);
    } catch (e: any) {
      setOutput(prev => [...prev, `Error: ${e.message}`, '']);
    }
    setExecuting(false);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ flex: 1, overflowY: 'auto' }}>
        <pre style={{
          padding: '8px 10px', margin: 0,
          fontSize: 10, fontFamily: "'JetBrains Mono', monospace",
          color: 'var(--text-secondary)', whiteSpace: 'pre-wrap', lineHeight: 1.6,
        }}>
          {output.join('\n')}
        </pre>
      </div>
      <div style={{ padding: '6px 8px', borderTop: '1px solid var(--border)', display: 'flex', gap: 6 }}>
        <span style={{ color: 'var(--accent)', fontSize: 12, lineHeight: '28px' }}>$</span>
        <Input
          size="small"
          value={cmd}
          onChange={e => setCmd(e.target.value)}
          onPressEnter={handleCmd}
          placeholder="输入命令..."
          style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 11 }}
        />
      </div>
    </div>
  );
}

// Browser panel — iframe web preview
function BrowserPanel() {
  const [url, setUrl] = useState('https://www.baidu.com');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ padding: '4px 6px', borderBottom: '1px solid var(--border)', display: 'flex', gap: 4 }}>
        <Input
          size="small"
          value={url}
          onChange={e => setUrl(e.target.value)}
          onPressEnter={() => {}}
          placeholder="输入 URL..."
          style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 11 }}
        />
        <Button size="small" onClick={() => setUrl(url)}>→</Button>
      </div>
      <div style={{ flex: 1, background: '#fff' }}>
        <iframe
          src={url}
          style={{ width: '100%', height: '100%', border: 'none' }}
          title="Browser"
          sandbox="allow-scripts allow-same-origin allow-forms"
        />
      </div>
    </div>
  );
}

// Side Tasks panel — current workflow tasks
function SideTasksPanel() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getTasks().then(res => setTasks(res.tasks || [])).catch(() => {}).finally(() => setLoading(false));
  }, []);

  const completed = tasks.filter(t => t.completed).length;
  const total = tasks.length;
  const progress = total > 0 ? Math.round((completed / total) * 100) : 0;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ padding: '8px 12px', borderBottom: '1px solid var(--border)' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <Text strong style={{ fontSize: 12 }}>📋 侧边任务</Text>
          {total > 0 && (
            <Text type="secondary" style={{ fontSize: 10 }}>
              第 {completed} / {total} 步
            </Text>
          )}
        </div>
        {total > 0 && (
          <div style={{
            height: 3, background: 'var(--border)', borderRadius: 2, marginTop: 6,
            overflow: 'hidden',
          }}>
            <div style={{
              height: '100%', width: `${progress}%`,
              background: 'var(--accent)', borderRadius: 2,
              transition: 'width 0.3s',
            }} />
          </div>
        )}
      </div>
      <div style={{ flex: 1, overflowY: 'auto', padding: '4px 8px' }}>
        {loading ? (
          <Spin size="small" style={{ display: 'block', textAlign: 'center', padding: 24 }} />
        ) : tasks.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={<Text type="secondary" style={{ fontSize: 11 }}>暂无任务</Text>}
            style={{ padding: '20px 0' }}
          />
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            {tasks.map((t, i) => (
              <div
                key={t.number}
                style={{
                  display: 'flex', alignItems: 'flex-start', gap: 8,
                  padding: '5px 8px', borderRadius: 4, fontSize: 11,
                  background: t.completed ? 'transparent' : 'var(--bg-active)',
                  opacity: t.completed ? 0.5 : 1,
                }}
              >
                <span style={{
                  width: 18, height: 18, borderRadius: '50%', flexShrink: 0, marginTop: 1,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: 10,
                  background: t.completed ? 'var(--green)' : 'var(--border)',
                  color: t.completed ? '#fff' : 'var(--text-muted)',
                }}>
                  {t.completed ? '✓' : t.number}
                </span>
                <div style={{ flex: 1 }}>
                  <div style={{
                    color: t.completed ? 'var(--text-muted)' : 'var(--text-primary)',
                    textDecoration: t.completed ? 'line-through' : 'none',
                    lineHeight: 1.5,
                  }}>
                    {t.content}
                  </div>
                  {t.phase && (
                    <Text type="secondary" style={{ fontSize: 9 }}>📍 {t.phase}</Text>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
