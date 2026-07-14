import { useState, useEffect, useCallback } from 'react';
import {
  Card, Button, Tag, Typography, Space, Row, Col, Modal, Form, Input,
  InputNumber, Select, message, Badge, Descriptions, Divider, Tooltip,
  Switch, Popconfirm, Statistic,
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, CopyOutlined,
  ReloadOutlined, ApiOutlined, CheckCircleOutlined, CloseCircleOutlined,
  SyncOutlined, ExportOutlined, ImportOutlined, ThunderboltOutlined,
  EyeOutlined, PictureOutlined, AudioOutlined, SoundOutlined,
  CodeOutlined, LinkOutlined,
} from '@ant-design/icons';
import {
  getBackends, addBackend, deleteBackend, updateBackend,
  probeBackends, importBackendsFile, getBackendsExportUrl,
  getCapabilities,
} from '../../lib/api';
import type { BackendPoolStatus, BackendConfig, CapabilityInfo } from '../../lib/types';

const { Text, Title, Paragraph } = Typography;

const CAPABILITY_ICONS: Record<string, React.ReactNode> = {
  chat: <CodeOutlined />,
  vision: <EyeOutlined />,
  image_gen: <PictureOutlined />,
  video_gen: <ThunderboltOutlined />,
  audio_stt: <AudioOutlined />,
  audio_tts: <SoundOutlined />,
  embedding: <LinkOutlined />,
};

const CAPABILITY_LABELS: Record<string, string> = {
  chat: 'Chat',
  vision: 'Vision',
  image_gen: 'Image Gen',
  video_gen: 'Video',
  audio_stt: 'Speech STT',
  audio_tts: 'Speech TTS',
  embedding: 'Embedding',
};

export default function BackendsSettings() {
  const [backends, setBackends] = useState<any[]>([]);
  const [strategy, setStrategy] = useState('round_robin');
  const [total, setTotal] = useState(0);
  const [healthy, setHealthy] = useState(0);
  const [capabilities, setCapabilities] = useState<CapabilityInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<string | null>(null);
  const [form] = Form.useForm();
  const [importing, setImporting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [beData, capData] = await Promise.all([
        getBackends(),
        getCapabilities(),
      ]);
      setBackends(beData.backends || []);
      setStrategy(beData.strategy || 'round_robin');
      setTotal(beData.total || 0);
      setHealthy(beData.healthy || 0);
      setCapabilities(capData.capabilities || []);
    } catch (e: any) {
      message.error('Failed to load backends: ' + e.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleAdd = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ weight: 5, strategy: strategy });
    setModalOpen(true);
  };

  const handleEdit = (label: string) => {
    const be = backends.find((b: any) => b.label === label);
    if (!be) return;
    setEditing(label);
    form.setFieldsValue({
      label: be.label,
      key: be.key || '',
      base_url: be.base_url || '',
      weight: be.weight || 5,
      models: (be.models || []).map((m: any) => `${m.name}:${m.type || 'chat'}`).join('\n'),
    });
    setModalOpen(true);
  };

  const handleDelete = async (label: string) => {
    try {
      await deleteBackend(label);
      message.success(`Backend "${label}" deleted`);
      load();
    } catch (e: any) { message.error(e.message); }
  };

  const handleDuplicate = async (label: string) => {
    const be = backends.find((b: any) => b.label === label);
    if (!be) return;
    try {
      await addBackend({
        label: `${label}-copy`,
        key: be.key || '',
        base_url: be.base_url || '',
        weight: be.weight || 5,
        models: be.models || [],
        headers: be.headers || {},
      });
      message.success(`Duplicated as "${label}-copy"`);
      load();
    } catch (e: any) { message.error(e.message); }
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      const modelLines: string[] = (values.models || '').split('\n').filter(Boolean);
      const models = modelLines.map((line: string) => {
        const [name, type] = line.split(':');
        return { name: name.trim(), type: (type || 'chat').trim() };
      });

      const payload = {
        label: values.label,
        key: values.key,
        base_url: values.base_url,
        weight: values.weight,
        models,
        headers: {},
      };

      if (editing) {
        await updateBackend(editing, payload);
        message.success(`Backend "${editing}" updated`);
      } else {
        await addBackend(payload);
        message.success(`Backend "${values.label}" added`);
      }
      setModalOpen(false);
      load();
    } catch (e: any) {
      if (e.message) message.error(e.message);
    }
  };

  const handleProbe = async () => {
    try {
      const result = await probeBackends();
      message.success(`Probed ${result.probed} backends`);
      load();
    } catch (e: any) { message.error(e.message); }
  };

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setImporting(true);
    try {
      const result = await importBackendsFile(file);
      message.success(`Imported ${result.imported} backends`);
      load();
    } catch (e: any) { message.error(e.message); }
    finally { setImporting(false); }
  };

  const getModelIcon = (type: string) => {
    const icons: Record<string, string> = {
      chat: '💬', vision: '👁️', image_gen: '🖼️',
      video_gen: '🎬', audio_stt: '🎤', audio_tts: '🔊', embedding: '🧩',
    };
    return icons[type] || '💬';
  };

  const getStatusBadge = (health: string, latency?: number) => {
    if (health === 'healthy') {
      return <Badge status="success" text={<span style={{color:'#52c41a'}}>Healthy{latency != null ? ` (${latency}ms)` : ''}</span>} />;
    }
    if (health === 'degraded') {
      return <Badge status="warning" text={<span style={{color:'#faad14'}}>Degraded</span>} />;
    }
    return <Badge status="error" text={<span style={{color:'#ff4d4f'}}>Unhealthy</span>} />;
  };

  return (
    <div style={{ height: '100%', overflow: 'auto', padding: '20px 24px' }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>Provider Backends — cc-switch 代理池</Title>
          <Paragraph type="secondary" style={{ margin: '4px 0 0' }}>
            多 Backend 权重路由 · 自动故障转移 · 模型能力自动发现
          </Paragraph>
        </div>
        <Space>
          <Select value={strategy} style={{ width: 140 }} onChange={() => {}} disabled
            options={[
              { value: 'round_robin', label: 'Round Robin' },
              { value: 'fill_first', label: 'Fill First' },
              { value: 'random', label: 'Random' },
            ]}
          />
          <Tooltip title="Import YAML">
            <Button icon={<ImportOutlined />} onClick={() => document.getElementById('be-import')?.click()} loading={importing} />
          </Tooltip>
          <input id="be-import" type="file" accept=".yaml,.yml" style={{ display: 'none' }} onChange={handleImport} />
          <Tooltip title="Export YAML">
            <Button icon={<ExportOutlined />} href={getBackendsExportUrl()} target="_blank" />
          </Tooltip>
          <Tooltip title="Probe all backends">
            <Button icon={<SyncOutlined />} onClick={handleProbe}>Probe</Button>
          </Tooltip>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>添加 Backend</Button>
        </Space>
      </div>

      {/* Stats */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card size="small"><Statistic title="Total Backends" value={total} /></Card>
        </Col>
        <Col span={6}>
          <Card size="small"><Statistic title="Healthy" value={healthy} valueStyle={{ color: '#52c41a' }} suffix={`/ ${total}`} /></Card>
        </Col>
        <Col span={6}>
          <Card size="small"><Statistic title="Strategy" value={strategy} valueStyle={{ fontSize: 16 }} /></Card>
        </Col>
        <Col span={6}>
          <Card size="small"><Statistic title="Capabilities" value={capabilities.filter(c => c.enabled).length} suffix={`/ ${capabilities.length}`} /></Card>
        </Col>
      </Row>

      {/* Backend Cards */}
      {loading ? (
        <Card><Text type="secondary">Loading...</Text></Card>
      ) : backends.length === 0 ? (
        <Card>
          <div style={{ textAlign: 'center', padding: 40 }}>
            <ApiOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />
            <Paragraph type="secondary" style={{ marginTop: 12 }}>No backends configured. Add your first backend to start routing requests.</Paragraph>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>添加 Backend</Button>
          </div>
        </Card>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          {backends.map((be: any) => (
            <Card
              key={be.label}
              size="small"
              title={
                <Space>
                  <ApiOutlined style={{ color: '#5e6ad2' }} />
                  <Text strong>{be.label}</Text>
                  <Text type="secondary" style={{ fontSize: 12 }}>{be.base_url}</Text>
                </Space>
              }
              extra={
                <Space>
                  <Tag>weight: {be.weight}</Tag>
                  {getStatusBadge(be.health || be.status, be.latency)}
                </Space>
              }
              actions={[
                <Tooltip title="Edit"><Button type="text" icon={<EditOutlined />} onClick={() => handleEdit(be.label)} /></Tooltip>,
                <Tooltip title="Duplicate"><Button type="text" icon={<CopyOutlined />} onClick={() => handleDuplicate(be.label)} /></Tooltip>,
                <Popconfirm title={`Delete "${be.label}"?`} onConfirm={() => handleDelete(be.label)}>
                  <Tooltip title="Delete"><Button type="text" danger icon={<DeleteOutlined />} /></Tooltip>
                </Popconfirm>,
              ]}
            >
              {/* Models */}
              <div>
                <Text type="secondary" style={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                  Models {be.models_discovered ? '(auto-discovered from /models)' : '(manual config)'}
                </Text>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 6 }}>
                  {(be.models || []).map((m: any) => (
                    <Tag key={m.name} color="blue" style={{ margin: 0 }}>
                      {getModelIcon(m.type)} {m.type && <Text style={{ fontSize: 10, color: '#8c8c8c' }}>[{m.type}]</Text>} {m.name}
                    </Tag>
                  ))}
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* Capability Overview */}
      <Divider />
      <Card size="small" title="能力总览 (Capability Overview)">
        <Row gutter={[12, 12]}>
          {capabilities.map(cap => (
            <Col span={8} key={cap.type}>
              <Card size="small"
                style={{
                  borderLeft: `3px solid ${cap.available ? '#52c41a' : '#ff4d4f'}`,
                  background: cap.available ? '#f6ffed' : '#fff2f0',
                }}
              >
                <Space>
                  {CAPABILITY_ICONS[cap.type]}
                  <Text strong>{CAPABILITY_LABELS[cap.type] || cap.type}</Text>
                  {cap.enabled ? (
                    <Tag color="success">{(cap.backends || []).length} backend{(cap.backends || []).length !== 1 ? 's' : ''}</Tag>
                  ) : (
                    <Tag color="error">未配置</Tag>
                  )}
                </Space>
                {cap.enabled && cap.backends && cap.backends.length > 0 && (
                  <div style={{ marginTop: 4 }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>
                      {cap.backends.join(', ')}
                    </Text>
                  </div>
                )}
              </Card>
            </Col>
          ))}
          {capabilities.length === 0 && (
            <Col span={24}>
              <Text type="secondary">No capability data. Add backends and probe to auto-discover capabilities.</Text>
            </Col>
          )}
        </Row>
      </Card>

      {/* Add/Edit Modal */}
      <Modal
        title={editing ? `Edit Backend: ${editing}` : 'Add Backend'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        okText={editing ? 'Update' : 'Add'}
        width={560}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="label" label="Label" rules={[{ required: true, message: 'Required' }]}>
            <Input placeholder="e.g. beecode-main" />
          </Form.Item>
          <Form.Item name="key" label="API Key" rules={[{ required: true, message: 'Required' }]}>
            <Input.Password placeholder="sk-..." />
          </Form.Item>
          <Form.Item name="base_url" label="Base URL" rules={[{ required: true, message: 'Required' }]}>
            <Input placeholder="https://beecode.cc/v1" />
          </Form.Item>
          <Form.Item name="weight" label="Weight" rules={[{ required: true }]}>
            <InputNumber min={1} max={100} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="models" label="Models (one per line: name:type)"
            extra="Types: chat, vision, image_gen, video_gen, audio_stt, audio_tts, embedding. Leave blank for auto-discovery."
          >
            <Input.TextArea rows={4} placeholder="gpt-5.5:chat&#10;gpt-image-2:image_gen" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
