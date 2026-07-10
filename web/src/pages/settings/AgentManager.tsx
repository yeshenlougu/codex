import { useState, useEffect, useCallback } from 'react';
import { Card, Button, Input, Tag, Space, message, Popconfirm, Modal } from 'antd';
import { PlusOutlined, CopyOutlined, EditOutlined, DeleteOutlined, CheckOutlined, CloseOutlined } from '@ant-design/icons';
import { listAgents, createAgent, deleteAgent, cloneAgent, updateAgent } from '../../lib/api';
import type { AgentProfile } from '../../lib/types';

export default function AgentManager() {
  const [agents, setAgents] = useState<AgentProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [newName, setNewName] = useState('');
  const [cloneTarget, setCloneTarget] = useState('');
  const [editing, setEditing] = useState<string | null>(null);
  const [editPrompt, setEditPrompt] = useState('');

  const load = useCallback(() => {
    setLoading(true);
    listAgents().then(a => setAgents(a.agents || [])).catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async () => {
    if (!newName.trim()) { message.warning('Name required'); return; }
    try { await createAgent(newName.trim()); message.success('Created'); setNewName(''); load(); }
    catch (e: any) { message.error(e.message); }
  };

  const handleDelete = async (name: string) => {
    try { await deleteAgent(name); message.success('Deleted'); load(); }
    catch (e: any) { message.error(e.message); }
  };

  const handleClone = async (sourceName: string) => {
    const name = cloneTarget || `${sourceName}-clone`;
    try { await cloneAgent(sourceName, name); message.success(`Cloned → "${name}"`); setCloneTarget(''); load(); }
    catch (e: any) { message.error(e.message); }
  };

  const startEdit = (agent: AgentProfile) => { setEditing(agent.name); setEditPrompt(agent.agent?.system_prompt || ''); };
  const saveEdit = async (name: string) => {
    try { await updateAgent(name, { agent: { max_turns: 0, system_prompt: editPrompt } } as any); message.success('Updated'); setEditing(null); load(); }
    catch (e: any) { message.error(e.message); }
  };

  if (loading) return <Card loading style={{ maxWidth: 680 }} />;

  return (
    <div style={{ maxWidth: 680, display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* Create */}
      <Card title="Create Agent" size="small">
        <Space.Compact style={{ width: '100%' }}>
          <Input value={newName} onChange={e => setNewName(e.target.value)}
            onPressEnter={handleCreate} placeholder="Agent name, e.g. python-expert" />
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>Create</Button>
        </Space.Compact>
        <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 6 }}>New agents clone from the built-in default.</div>
      </Card>

      {/* Clone target */}
      <Card title="Clone Name" size="small">
        <Input value={cloneTarget} onChange={e => setCloneTarget(e.target.value)}
          placeholder="Leave blank for auto-name (agent-clone)" />
      </Card>

      {/* Agent list */}
      {agents.length === 0 ? (
        <Card size="small"><div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: 16 }}>No agents yet. Create one above.</div></Card>
      ) : (
        agents.map(agent => (
          <Card
            key={agent.name}
            size="small"
            title={
              <Space>
                <span style={{ fontSize: 16 }}>{agent.avatar || '🤖'}</span>
                <span style={{ fontWeight: 600 }}>{agent.name}</span>
                <Tag color={agent.is_builtin ? 'default' : 'green'}>{agent.is_builtin ? 'built-in' : 'custom'}</Tag>
              </Space>
            }
            extra={
              <Space>
                {!agent.is_builtin && (
                  <>
                    <Button size="small" type="text" icon={<EditOutlined />} onClick={() => startEdit(agent)} />
                    <Popconfirm title={`Delete "${agent.name}"?`} onConfirm={() => handleDelete(agent.name)}>
                      <Button size="small" type="text" danger icon={<DeleteOutlined />} />
                    </Popconfirm>
                  </>
                )}
                <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => handleClone(agent.name)} />
              </Space>
            }
          >
            <p style={{ fontSize: 12, color: 'var(--text-tertiary)', marginBottom: 8 }}>
              {agent.description || 'No description'}
            </p>
            <Space wrap size={[4, 4]}>
              <Tag>{agent.model?.provider || 'openai'} / {agent.model?.model || 'gpt-4o'}</Tag>
              <Tag>max {agent.agent?.max_turns || 60} turns</Tag>
              {agent.mcp?.servers && agent.mcp.servers.length > 0 && (
                <Tag color="orange">{agent.mcp.servers.length} MCP</Tag>
              )}
              {agent.skills?.dirs && agent.skills.dirs.length > 0 && (
                <Tag color="purple">{agent.skills.dirs.length} skills</Tag>
              )}
            </Space>

            {editing === agent.name && (
              <div style={{ marginTop: 12, paddingTop: 12, borderTop: '1px solid var(--border)' }}>
                <div style={{ fontSize: 12, fontWeight: 500, marginBottom: 4 }}>System Prompt</div>
                <Input.TextArea rows={4} value={editPrompt} onChange={e => setEditPrompt(e.target.value)}
                  style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12 }} />
                <Space style={{ marginTop: 8 }}>
                  <Button size="small" type="primary" icon={<CheckOutlined />} onClick={() => saveEdit(agent.name)}>Save</Button>
                  <Button size="small" icon={<CloseOutlined />} onClick={() => setEditing(null)}>Cancel</Button>
                </Space>
              </div>
            )}
          </Card>
        ))
      )}
    </div>
  );
}
