import { useState, useEffect, useCallback } from 'react';
import { Card, Button, Input, Tag, Space, message, Popconfirm, Modal, Form, Select, InputNumber, Switch, Typography, Divider } from 'antd';
import {
  PlusOutlined, CopyOutlined, EditOutlined, DeleteOutlined,
  CheckOutlined, CloseOutlined, ToolOutlined, CodeOutlined,
  PictureOutlined, FileTextOutlined, FolderOpenOutlined,
} from '@ant-design/icons';
import { listAgents, createAgent, deleteAgent, cloneAgent, updateAgent, getBackends } from '../../lib/api';
import type { AgentProfile } from '../../lib/types';

const { Text, Paragraph } = Typography;

const TOOL_ICONS: Record<string, React.ReactNode> = {
  shell: <CodeOutlined />,
  file_read: <FileTextOutlined />,
  file_edit: <FolderOpenOutlined />,
  image_gen: <PictureOutlined />,
};

const TOOL_LABELS: Record<string, string> = {
  shell: 'Shell',
  file_read: 'File Read',
  file_edit: 'File Edit',
  image_gen: 'Image Gen',
};

export default function AgentManager() {
  const [agents, setAgents] = useState<AgentProfile[]>([]);
  const [backends, setBackends] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [editingAgent, setEditingAgent] = useState<AgentProfile | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [a, b] = await Promise.all([
        listAgents(),
        getBackends(),
      ]);
      setAgents(a.agents || []);
      setBackends(b.backends || []);
    } catch {
      // API may not be available
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async () => {
    try {
      const values = await createForm.validateFields();
      const agent = await createAgent(values.name);
      // Apply additional settings
      if (values.system_prompt) {
        await updateAgent(values.name, {
          agent: { max_turns: values.max_turns || 60, system_prompt: values.system_prompt },
          model: { provider: values.provider || 'cc-switch', model: values.model || 'deepseek-v4-pro' },
        } as any);
      }
      message.success(`Agent "${values.name}" created`);
      setCreateModalOpen(false);
      createForm.resetFields();
      load();
    } catch (e: any) { message.error(e.message); }
  };

  const handleDelete = async (name: string) => {
    try { await deleteAgent(name); message.success('Deleted'); load(); }
    catch (e: any) { message.error(e.message); }
  };

  const handleClone = async (sourceName: string) => {
    const name = `${sourceName}-clone`;
    try { await cloneAgent(sourceName, name); message.success(`Cloned → "${name}"`); load(); }
    catch (e: any) { message.error(e.message); }
  };

  const openEdit = (agent: AgentProfile) => {
    setEditingAgent(agent);
    editForm.setFieldsValue({
      system_prompt: agent.agent?.system_prompt || '',
      provider: agent.model?.provider || 'cc-switch',
      model: agent.model?.model || 'deepseek-v4-pro',
      max_turns: agent.agent?.max_turns || 60,
      temperature: (agent as any).model?.temperature || 0.7,
      max_tokens: (agent as any).model?.max_tokens || 4096,
      shell_enabled: agent.tools?.shell !== false,
      file_read_enabled: agent.tools?.file_read !== false,
      file_edit_enabled: agent.tools?.file_edit !== false,
    });
    setEditModalOpen(true);
  };

  const saveEdit = async () => {
    if (!editingAgent) return;
    try {
      const values = await editForm.validateFields();
      await updateAgent(editingAgent.name, {
        agent: { max_turns: values.max_turns, system_prompt: values.system_prompt },
        model: {
          provider: values.provider,
          model: values.model,
          temperature: values.temperature,
          max_tokens: values.max_tokens,
        },
        tools: {
          shell: values.shell_enabled,
          file_read: values.file_read_enabled,
          file_edit: values.file_edit_enabled,
        },
      } as any);
      message.success('Updated');
      setEditModalOpen(false);
      load();
    } catch (e: any) { message.error(e.message); }
  };

  if (loading) return <Card loading style={{ maxWidth: 780, margin: '20px 24px' }} />;

  return (
    <div style={{ height: '100%', overflow: 'auto', padding: '20px 24px' }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <Text strong style={{ fontSize: 18 }}>Agent 配置 — 多 Agent 协作</Text>
          <br />
          <Text type="secondary" style={{ fontSize: 12 }}>
            Each agent can use a different model, provider, and tools configuration
          </Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { createForm.resetFields(); setCreateModalOpen(true); }}>
          创建 Agent
        </Button>
      </div>

      {/* Agent Cards */}
      {agents.length === 0 ? (
        <Card>
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Text type="secondary">No agents created yet. Create your first agent above.</Text>
          </div>
        </Card>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          {agents.map(agent => (
            <Card
              key={agent.name}
              size="small"
              title={
                <Space>
                  <span style={{ fontSize: 20 }}>{agent.avatar || '🤖'}</span>
                  <Text strong>{agent.name}</Text>
                  <Tag color={agent.is_builtin ? 'blue' : 'green'}>
                    {agent.is_builtin ? 'built-in' : 'custom'}
                  </Tag>
                </Space>
              }
              extra={
                <Space>
                  <Button size="small" type="text" icon={<EditOutlined />} onClick={() => openEdit(agent)}>
                    Edit
                  </Button>
                  <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => handleClone(agent.name)}>
                    Clone
                  </Button>
                  {!agent.is_builtin && (
                    <Popconfirm title={`Delete "${agent.name}"?`} onConfirm={() => handleDelete(agent.name)}>
                      <Button size="small" type="text" danger icon={<DeleteOutlined />} />
                    </Popconfirm>
                  )}
                </Space>
              }
            >
              {/* System prompt preview */}
              <Paragraph
                type="secondary"
                ellipsis={{ rows: 2 }}
                style={{ fontSize: 12, marginBottom: 8, fontStyle: 'italic' }}
              >
                {agent.agent?.system_prompt || agent.description || 'No system prompt configured'}
              </Paragraph>

              {/* Config tags */}
              <Space wrap size={[4, 8]}>
                <Tag icon={<CodeOutlined />} color="blue">
                  {agent.model?.provider || 'cc-switch'} / {agent.model?.model || 'gpt-4o'}
                </Tag>
                <Tag color="purple">max {agent.agent?.max_turns || 60} turns</Tag>
                {agent.tools && (
                  <>
                    {agent.tools.shell && <Tag icon={TOOL_ICONS.shell}>{TOOL_LABELS.shell}</Tag>}
                    {agent.tools.file_read && <Tag icon={TOOL_ICONS.file_read}>{TOOL_LABELS.file_read}</Tag>}
                    {agent.tools.file_edit && <Tag icon={TOOL_ICONS.file_edit}>{TOOL_LABELS.file_edit}</Tag>}
                  </>
                )}
                {agent.mcp?.servers && agent.mcp.servers.length > 0 && (
                  <Tag color="orange">{agent.mcp.servers.length} MCP servers</Tag>
                )}
                {agent.skills?.dirs && agent.skills.dirs.length > 0 && (
                  <Tag color="purple">{agent.skills.dirs.length} skills</Tag>
                )}
              </Space>
            </Card>
          ))}
        </div>
      )}

      {/* Create Agent Modal */}
      <Modal
        title="Create Agent"
        open={createModalOpen}
        onCancel={() => setCreateModalOpen(false)}
        onOk={handleCreate}
        okText="Create"
        width={520}
      >
        <Form form={createForm} layout="vertical">
          <Form.Item name="name" label="Agent Name" rules={[{ required: true }]}>
            <Input placeholder="e.g. code-reviewer, test-writer" />
          </Form.Item>
          <Form.Item name="system_prompt" label="System Prompt">
            <Input.TextArea rows={4} placeholder="You are a helpful coding assistant..." />
          </Form.Item>
          <Form.Item name="provider" label="Provider" initialValue="cc-switch">
            <Select options={[
              { value: 'cc-switch', label: 'cc-switch 代理池 (auto-routing)' },
              ...backends.map((b: any) => ({ value: b.label, label: b.label })),
            ]} />
          </Form.Item>
          <Form.Item name="model" label="Model" initialValue="deepseek-v4-pro">
            <Input placeholder="deepseek-v4-pro" />
          </Form.Item>
          <Form.Item name="max_turns" label="Max Turns" initialValue={60}>
            <InputNumber min={1} max={500} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Agent Modal */}
      <Modal
        title={`Edit Agent: ${editingAgent?.name || ''}`}
        open={editModalOpen}
        onCancel={() => setEditModalOpen(false)}
        onOk={saveEdit}
        okText="Save"
        width={560}
      >
        <Form form={editForm} layout="vertical">
          <Form.Item name="system_prompt" label="System Prompt">
            <Input.TextArea rows={6} style={{ fontFamily: 'monospace', fontSize: 12 }} />
          </Form.Item>
          <Divider style={{ margin: '12px 0' }} />
          <Text strong style={{ fontSize: 13 }}>Model Configuration</Text>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginTop: 8 }}>
            <Form.Item name="provider" label="Provider">
              <Select options={[
                { value: 'cc-switch', label: 'cc-switch 代理池' },
                ...backends.map((b: any) => ({ value: b.label, label: b.label })),
              ]} />
            </Form.Item>
            <Form.Item name="model" label="Model">
              <Input />
            </Form.Item>
            <Form.Item name="temperature" label="Temperature">
              <InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="max_tokens" label="Max Tokens">
              <InputNumber min={100} max={131072} step={100} style={{ width: '100%' }} />
            </Form.Item>
          </div>
          <Divider style={{ margin: '12px 0' }} />
          <Text strong style={{ fontSize: 13 }}>Tools</Text>
          <div style={{ marginTop: 8 }}>
            <Form.Item name="shell_enabled" label="Shell" valuePropName="checked" style={{ marginBottom: 4 }}>
              <Switch />
            </Form.Item>
            <Form.Item name="file_read_enabled" label="File Read" valuePropName="checked" style={{ marginBottom: 4 }}>
              <Switch />
            </Form.Item>
            <Form.Item name="file_edit_enabled" label="File Edit" valuePropName="checked" style={{ marginBottom: 4 }}>
              <Switch />
            </Form.Item>
          </div>
        </Form>
      </Modal>
    </div>
  );
}
