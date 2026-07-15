import { useState, useEffect, useCallback } from 'react';
import {
  Card, Button, Typography, Tag, Space, Row, Col, message, Modal,
  Form, Input, Select, Popconfirm, Tooltip, Badge, Empty, Spin,
} from 'antd';
import {
  PlusOutlined, ApiOutlined, CheckCircleOutlined, CloseCircleOutlined,
  SwapOutlined, DeleteOutlined, EditOutlined, ReloadOutlined,
  ThunderboltOutlined, SafetyCertificateOutlined,
} from '@ant-design/icons';
import {
  listProviders, getProviderPresets, createFromPreset,
  switchProvider, deleteProvider, probeProvider,
} from '../../lib/api';
import type { ProviderSummary, ProviderPreset, ProviderListResponse } from '../../lib/types';

const { Text, Title } = Typography;

const PROVIDER_ICONS: Record<string, string> = {
  openai: '🟢', anthropic: '🟠', deepseek: '🔵', gemini: '🌟',
  ollama: '🦙', groq: '⚡', api: '🔌',
};
const CATEGORY_COLORS: Record<string, string> = {
  official: 'blue', third_party: 'green', partner: 'gold',
};
const CATEGORY_LABELS: Record<string, string> = {
  official: 'Official', third_party: '第三方', partner: '合作',
};

export default function ProviderList({ onSelect }: { onSelect?: (id: string) => void }) {
  const [providers, setProviders] = useState<ProviderSummary[]>([]);
  const [current, setCurrent] = useState('');
  const [presets, setPresets] = useState<ProviderPreset[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedPreset, setSelectedPreset] = useState<string>('');
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [pData, presetData] = await Promise.all([
        listProviders(),
        getProviderPresets(),
      ]);
      setProviders(pData.providers || []);
      setCurrent(pData.current);
      setPresets(presetData.presets || []);
    } catch (e: any) {
      message.error('加载供应商失败: ' + (e.message || ''));
    } finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleSwitch = async (id: string) => {
    try {
      const r = await switchProvider(id);
      message.success(`已切换到该供应商 (${r.backends} 个端点)`);
      load();
    } catch (e: any) { message.error(e.message); }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteProvider(id);
      message.success('已删除');
      load();
    } catch (e: any) { message.error(e.message); }
  };

  const handleProbe = async (id: string) => {
    try {
      const r = await probeProvider(id);
      const healthy = r.results.filter(x => x.status === 'healthy').length;
      message.success(`探测完成: ${healthy}/${r.total} 健康`);
    } catch (e: any) { message.error(e.message); }
  };

  const handleAddFromPreset = async () => {
    try {
      const values = await form.validateFields();
      await createFromPreset(values.preset, values.name || undefined, values.api_key || '');
      message.success(`供应商 "${values.name || values.preset}" 已添加`);
      setModalOpen(false);
      form.resetFields();
      load();
    } catch (e: any) {
      if (e.message) message.error(e.message);
    }
  };

  const selectedPresetData = presets.find(p => p.name === selectedPreset);

  if (loading) return (
    <div style={{ display: 'flex', justifyContent: 'center', padding: 60 }}>
      <Spin size="large" />
    </div>
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, maxWidth: 860 }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
        <div>
          <Title level={4} style={{ margin: 0, fontSize: 18 }}>供应商管理</Title>
          <Text type="secondary" style={{ fontSize: 12 }}>管理 AI 供应商 — 多供应商支持 + 一键切换</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
          添加供应商
        </Button>
      </div>

      {/* Provider Cards */}
      {providers.length === 0 ? (
        <Card>
          <Empty
            image={<ApiOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />}
            description="暂无供应商。点击「添加供应商」从预设创建，或手动配置。"
          >
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
              添加供应商
            </Button>
          </Empty>
        </Card>
      ) : (
        <Row gutter={[16, 16]}>
          {providers.map(p => {
            const isCurrent = p.id === current;
            const categoryColor = CATEGORY_COLORS[p.category || ''] || 'default';
            const categoryLabel = CATEGORY_LABELS[p.category || ''] || p.category || '';
            const iconEmoji = PROVIDER_ICONS[p.icon || ''] || '🔌';

            return (
              <Col span={12} key={p.id}>
                <Card
                  size="small"
                  hoverable
                  style={{
                    borderLeft: isCurrent ? '3px solid #5e6ad2' : '3px solid transparent',
                    background: isCurrent ? 'var(--bg-active)' : undefined,
                  }}
                  onClick={() => onSelect?.(p.id)}
                  title={
                    <Space>
                      <span style={{ fontSize: 18 }}>{iconEmoji}</span>
                      <Text strong>{p.name}</Text>
                      {categoryLabel && <Tag color={categoryColor} style={{ fontSize: 10 }}>{categoryLabel}</Tag>}
                      {isCurrent && <Tag color="#5e6ad2" style={{ fontSize: 10 }}>当前使用</Tag>}
                    </Space>
                  }
                  extra={
                    <Space>
                      <Badge status={p.backend_count > 0 ? 'success' : 'default'} text={`${p.backend_count} 端点`} />
                    </Space>
                  }
                  actions={[
                    !isCurrent && (
                      <Tooltip title="切换到此供应商" key="switch">
                        <Button type="text" size="small" icon={<SwapOutlined />}
                          onClick={e => { e.stopPropagation(); handleSwitch(p.id); }}>
                          切换
                        </Button>
                      </Tooltip>
                    ),
                    <Tooltip title="健康探测" key="probe">
                      <Button type="text" size="small" icon={<ThunderboltOutlined />}
                        onClick={e => { e.stopPropagation(); handleProbe(p.id); }} />
                    </Tooltip>,
                    !isCurrent && (
                      <Popconfirm title={`删除 "${p.name}"？`} onConfirm={() => handleDelete(p.id)} key="del">
                        <Button type="text" size="small" danger icon={<DeleteOutlined />}
                          onClick={e => e.stopPropagation()} />
                      </Popconfirm>
                    ),
                  ].filter(Boolean)}
                >
                  <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                    {p.in_failover_queue && (
                      <Tag icon={<SafetyCertificateOutlined />} color="blue" style={{ fontSize: 10 }}>
                        故障转移队列
                      </Tag>
                    )}
                    {isCurrent && (
                      <Tag icon={<CheckCircleOutlined />} color="#5e6ad2" style={{ fontSize: 10 }}>
                        活跃
                      </Tag>
                    )}
                  </div>
                </Card>
              </Col>
            );
          })}
        </Row>
      )}

      {/* Add Provider Modal (from preset) */}
      <Modal
        title="添加供应商"
        open={modalOpen}
        onCancel={() => { setModalOpen(false); form.resetFields(); }}
        onOk={handleAddFromPreset}
        okText="添加"
        width={500}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 12 }}>
          <Form.Item name="preset" label="选择预设供应商" rules={[{ required: true, message: '请选择' }]}>
            <Select
              placeholder="选择一个预设..."
              onChange={v => setSelectedPreset(v)}
              showSearch
              optionFilterProp="label"
              options={presets.map(p => ({
                value: p.name,
                label: `${PROVIDER_ICONS[p.icon || ''] || '🔌'} ${p.name}${p.category ? ` (${CATEGORY_LABELS[p.category] || p.category})` : ''}`,
              }))}
            />
          </Form.Item>

          {selectedPresetData && (
            <Card size="small" style={{ marginBottom: 12, background: '#fafafa' }}>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', fontSize: 12 }}>
                <Tag color={CATEGORY_COLORS[selectedPresetData.category] || 'default'}>
                  {CATEGORY_LABELS[selectedPresetData.category] || selectedPresetData.category}
                </Tag>
                <Text type="secondary">{selectedPresetData.base_url}</Text>
                {selectedPresetData.default_model && (
                  <Tag>{selectedPresetData.default_model}</Tag>
                )}
              </div>
              {selectedPresetData.website_url && (
                <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 4 }}>
                  {selectedPresetData.website_url}
                  {selectedPresetData.api_key_url && (
                    <> · <a href={selectedPresetData.api_key_url} target="_blank" rel="noopener">获取 API Key ↗</a></>
                  )}
                </Text>
              )}
            </Card>
          )}

          <Form.Item name="name" label="名称 (可选)" extra="留空则使用预设名称">
            <Input placeholder="自定义名称" />
          </Form.Item>

          <Form.Item name="api_key" label="API Key" extra="用于该供应商的认证密钥">
            <Input.Password placeholder="sk-..." />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
