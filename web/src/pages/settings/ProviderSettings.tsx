import { useState, useEffect, useCallback } from 'react';
import {
  Card, Select, Input, Typography, Button, message, Divider,
  Tag, Space, Tooltip, Statistic, Row, Col, Popconfirm,
  Modal, Form, InputNumber, Badge, Switch,
} from 'antd';
import {
  ApiOutlined, KeyOutlined, LinkOutlined, ThunderboltOutlined,
  SafetyCertificateOutlined, EyeInvisibleOutlined, EyeOutlined,
  ReloadOutlined, SettingOutlined, PlusOutlined, EditOutlined,
  DeleteOutlined, CopyOutlined, ExportOutlined, ImportOutlined,
  SyncOutlined, CheckCircleOutlined, CloseCircleOutlined,
  EyeOutlined as ViewIcon, PictureOutlined, AudioOutlined,
  SoundOutlined, CodeOutlined,
} from '@ant-design/icons';
import {
  getConfig, updateConfig, getBackends, addBackend,
  deleteBackend, updateBackend, probeBackends, importBackendsFile,
  getBackendsExportUrl, getCapabilities,
} from '../../lib/api';
import type { Config, CapabilityInfo } from '../../lib/types';

const { Text, Title, Paragraph } = Typography;

function SettingRow({ label, desc, children }: { label: string; desc?: string; children: React.ReactNode }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 0', gap: 24 }}>
      <div style={{ flex: 1, minWidth: 0 }}>
        <Text style={{ fontSize: 13, fontWeight: 500 }}>{label}</Text>
        {desc && <div><Text type="secondary" style={{ fontSize: 11 }}>{desc}</Text></div>}
      </div>
      <div style={{ flexShrink: 0 }}>{children}</div>
    </div>
  );
}

const CAPABILITY_ICONS: Record<string, React.ReactNode> = {
  chat: <CodeOutlined />, vision: <ViewIcon />, image_gen: <PictureOutlined />,
  video_gen: <ThunderboltOutlined />, audio_stt: <AudioOutlined />,
  audio_tts: <SoundOutlined />, embedding: <LinkOutlined />,
};
const CAPABILITY_LABELS: Record<string, string> = {
  chat: 'Chat', vision: 'Vision', image_gen: 'Image Gen',
  video_gen: 'Video', audio_stt: 'Speech STT', audio_tts: 'Speech TTS', embedding: 'Embedding',
};

export default function ProviderSettings() {
  // Provider state
  const [cfg, setCfg] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [showKey, setShowKey] = useState(false);
  const [apiKey, setApiKey] = useState('');
  const [saving, setSaving] = useState(false);

  // Backends state
  const [backends, setBackends] = useState<any[]>([]);
  const [backendTotal, setBackendTotal] = useState(0);
  const [backendHealthy, setBackendHealthy] = useState(0);
  const [capabilities, setCapabilities] = useState<CapabilityInfo[]>([]);
  const [beModalOpen, setBeModalOpen] = useState(false);
  const [beEditing, setBeEditing] = useState<string | null>(null);
  const [beImporting, setBeImporting] = useState(false);
  const [beForm] = Form.useForm();

  const loadAll = useCallback(async () => {
    setLoading(true);
    try {
      const [c, beData, capData] = await Promise.all([
        getConfig(),
        getBackends(),
        getCapabilities(),
      ]);
      setCfg(c);
      setBackends(beData.backends || []);
      setBackendTotal(beData.total || 0);
      setBackendHealthy(beData.healthy || 0);
      setCapabilities(capData.capabilities || []);
    } catch (e: any) {
      message.error('加载失败: ' + (e.message || ''));
    } finally { setLoading(false); }
  }, []);

  useEffect(() => { loadAll(); }, [loadAll]);

  // ── Provider save ──
  const save = async (field: string, value: any) => {
    try {
      await updateConfig({ [field]: value });
      message.success({ content: '保存成功', key: field, duration: 1 });
      loadAll();
    } catch (e: any) { message.error(e.message); }
  };

  const saveApiKey = async () => {
    if (!apiKey.trim()) { message.warning('请输入 API Key'); return; }
    setSaving(true);
    try {
      await updateConfig({ api_key: apiKey.trim() });
      message.success('API Key 已保存');
      setApiKey(''); setShowKey(false);
      loadAll();
    } catch (e: any) { message.error(e.message); }
    finally { setSaving(false); }
  };

  // ── Backend handlers ──
  const handleBeAdd = () => { setBeEditing(null); beForm.resetFields(); beForm.setFieldsValue({ weight: 5 }); setBeModalOpen(true); };
  const handleBeEdit = (label: string) => {
    const be = backends.find((b: any) => b.label === label);
    if (!be) return;
    setBeEditing(label);
    beForm.setFieldsValue({
      label: be.label, key: be.key || '', base_url: be.base_url || '',
      weight: be.weight || 5,
      models: (be.models || []).map((m: any) => `${m.name}:${m.type || 'chat'}`).join('\n'),
    });
    setBeModalOpen(true);
  };
  const handleBeDelete = async (label: string) => {
    try { await deleteBackend(label); message.success(`Backend "${label}" 已删除`); loadAll(); }
    catch (e: any) { message.error(e.message); }
  };
  const handleBeDuplicate = async (label: string) => {
    const be = backends.find((b: any) => b.label === label);
    if (!be) return;
    try {
      await addBackend({ label: `${label}-copy`, key: be.key || '', base_url: be.base_url || '', weight: be.weight || 5, models: be.models || [], headers: be.headers || {} });
      message.success(`已复制为 "${label}-copy"`); loadAll();
    } catch (e: any) { message.error(e.message); }
  };
  const handleBeSave = async () => {
    try {
      const values = await beForm.validateFields();
      const modelLines: string[] = (values.models || '').split('\n').filter(Boolean);
      const models = modelLines.map((line: string) => {
        const [name, type] = line.split(':');
        return { name: name.trim(), type: (type || 'chat').trim() };
      });
      const payload = { label: values.label, key: values.key, base_url: values.base_url, weight: values.weight, models, headers: {} };
      if (beEditing) { await updateBackend(beEditing, payload); message.success(`Backend "${beEditing}" 已更新`); }
      else { await addBackend(payload); message.success(`Backend "${values.label}" 已添加`); }
      setBeModalOpen(false); loadAll();
    } catch (e: any) { if (e.message) message.error(e.message); }
  };
  const handleBeProbe = async () => {
    try { const r = await probeBackends(); message.success(`探测了 ${r.probed} 个 Backends`); loadAll(); }
    catch (e: any) { message.error(e.message); }
  };
  const handleBeImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]; if (!file) return;
    setBeImporting(true);
    try { const r = await importBackendsFile(file); message.success(`导入了 ${r.imported} 个 Backends`); loadAll(); }
    catch (e: any) { message.error(e.message); }
    finally { setBeImporting(false); }
  };

  const getModelIcon = (type: string) => ({ chat:'💬', vision:'👁️', image_gen:'🖼️', video_gen:'🎬', audio_stt:'🎤', audio_tts:'🔊', embedding:'🧩' })[type] || '💬';
  const getStatusBadge = (health: string, latency?: number) => {
    if (health === 'healthy') return <Badge status="success" text={<span style={{color:'#52c41a'}}>Healthy{latency != null ? ` (${latency}ms)` : ''}</span>} />;
    if (health === 'degraded') return <Badge status="warning" text={<span style={{color:'#faad14'}}>Degraded</span>} />;
    return <Badge status="error" text={<span style={{color:'#ff4d4f'}}>Unhealthy</span>} />;
  };

  if (loading && !cfg) return <Card loading style={{ maxWidth: 860 }} />;
  if (!cfg) return <Card><Text type="danger">无法加载配置。</Text></Card>;

  const providerName = cfg.provider || 'cc-switch';
  const isPoolMode = providerName === 'cc-switch';

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, maxWidth: 860, paddingBottom: 24 }}>
      {/* Header */}
      <div>
        <Title level={4} style={{ margin: 0, fontSize: 18 }}>供应商管理</Title>
        <Text type="secondary" style={{ fontSize: 12 }}>配置 AI 供应商的连接参数与路由策略</Text>
      </div>

      {/* Provider Identity Card */}
      <Card size="small" style={{ borderLeft: '3px solid #5e6ad2' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 12 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{ width: 40, height: 40, borderRadius: 10, background: 'linear-gradient(135deg, #5e6ad2, #7c8bf5)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <ApiOutlined style={{ fontSize: 20, color: '#fff' }} />
            </div>
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <Text strong style={{ fontSize: 15 }}>{providerName}</Text>
                <Tag color={isPoolMode ? 'blue' : 'green'} style={{ fontSize: 10 }}>{isPoolMode ? 'cc-switch 代理池' : '直连模式'}</Tag>
              </div>
              <Text type="secondary" style={{ fontSize: 11 }}>
                {isPoolMode ? `多 Backend 智能路由 · ${backendTotal} 个端点` : `直连 ${cfg.base_url || '(未配置)'}`}
              </Text>
            </div>
          </div>
          <Select value={providerName} onChange={v => save('provider', v)} style={{ width: 180 }} size="middle"
            options={[
              { value: 'cc-switch', label: 'cc-switch (代理池)' },
              { value: 'openai', label: 'OpenAI' }, { value: 'anthropic', label: 'Anthropic' },
              { value: 'deepseek', label: 'DeepSeek' }, { value: 'ollama', label: 'Ollama (本地)' },
              { value: 'custom', label: 'Custom' },
            ]} />
        </div>
      </Card>

      {/* API Key */}
      <Card size="small" title={<span><KeyOutlined style={{ marginRight: 6 }} />API Key</span>}
        extra={<Tooltip title="重新加载"><Button type="text" size="small" icon={<ReloadOutlined />} onClick={loadAll} /></Tooltip>}>
        <SettingRow label="当前 Key" desc="已保存的 API 密钥（脱敏显示）">
          <Space>
            <Text code style={{ fontSize: 12 }}>{cfg.api_key_masked || '(未设置)'}</Text>
            {cfg.api_key_masked && cfg.api_key_masked !== '(未设置)' && <Tag color="green" style={{ fontSize: 10 }}>已配置</Tag>}
          </Space>
        </SettingRow>
        <Divider style={{ margin: 0 }} />
        <div style={{ padding: '12px 0' }}>
          <Text style={{ fontSize: 13, fontWeight: 500 }}>更新 API Key</Text>
          <div style={{ margin: '8px 0 0', display: 'flex', gap: 8, alignItems: 'center' }}>
            <Input type={showKey ? 'text' : 'password'} value={apiKey} onChange={e => setApiKey(e.target.value)}
              placeholder="sk-..." style={{ flex: 1, maxWidth: 380 }}
              prefix={<KeyOutlined style={{ color: 'var(--text-muted)' }} />}
              suffix={<Button type="text" size="small" icon={showKey ? <EyeInvisibleOutlined /> : <EyeOutlined />} onClick={() => setShowKey(!showKey)} style={{ border: 'none' }} />}
            />
            <Button type="primary" size="small" onClick={saveApiKey} loading={saving} icon={<SafetyCertificateOutlined />}>保存</Button>
          </div>
          <Text type="secondary" style={{ fontSize: 10, display: 'block', marginTop: 4 }}>密钥仅存储在本机 ~/.codex/config.yaml 中，不会上传到任何服务器</Text>
        </div>
      </Card>

      {/* Pool Strategy (cc-switch only) */}
      {isPoolMode && (
        <Card size="small" title={<span><ThunderboltOutlined style={{ marginRight: 6 }} />代理池策略</span>}>
          <SettingRow label="路由策略" desc="多 Backend 间的负载均衡方式">
            <Select value={cfg.pool_strategy || 'round_robin'} onChange={v => save('pool_strategy', v)} style={{ width: 160 }}
              options={[{ value: 'round_robin', label: 'Round Robin' }, { value: 'random', label: 'Random' }, { value: 'fill_first', label: 'Fill First' }]} />
          </SettingRow>
          <Divider style={{ margin: 0 }} />
          <SettingRow label="Wire API" desc="底层 API 协议格式">
            <Select value={cfg.wire_api || 'chat_completions'} onChange={v => save('wire_api', v)} style={{ width: 180 }}
              options={[{ value: 'chat_completions', label: 'Chat Completions' }, { value: 'responses', label: 'Responses' }]} />
          </SettingRow>
        </Card>
      )}

      {/* Direct mode settings */}
      {!isPoolMode && (
        <Card size="small" title={<span><LinkOutlined style={{ marginRight: 6 }} />连接设置</span>}>
          <SettingRow label="Base URL" desc="API 端点地址">
            <Input value={cfg.base_url || ''} onChange={e => save('base_url', e.target.value)} placeholder="https://api.openai.com/v1" style={{ width: 320 }} />
          </SettingRow>
          <Divider style={{ margin: 0 }} />
          <SettingRow label="模型" desc="默认使用的模型名称">
            <Input value={cfg.model || ''} onChange={e => save('model', e.target.value)} placeholder="gpt-4o" style={{ width: 220 }} />
          </SettingRow>
        </Card>
      )}

      {/* ═══════════════════════════════════════ */}
      {/* Backends — embedded under Provider       */}
      {/* ═══════════════════════════════════════ */}
      <Divider style={{ margin: '4px 0' }}>
        <Text type="secondary" style={{ fontSize: 11, fontWeight: 500, letterSpacing: '0.05em' }}>BACKENDS</Text>
      </Divider>

      {/* Backend toolbar + stats */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
        <Text strong style={{ fontSize: 14 }}>端点管理 — 属于 {providerName}</Text>
        <Space wrap>
          <Tooltip title="导入 YAML"><Button icon={<ImportOutlined />} size="small" onClick={() => document.getElementById('be-import')?.click()} loading={beImporting}>导入</Button></Tooltip>
          <input id="be-import" type="file" accept=".yaml,.yml" style={{ display: 'none' }} onChange={handleBeImport} />
          <Tooltip title="导出 YAML"><Button icon={<ExportOutlined />} size="small" href={getBackendsExportUrl()} target="_blank">导出</Button></Tooltip>
          <Tooltip title="探测所有 Backends"><Button icon={<SyncOutlined />} size="small" onClick={handleBeProbe}>Probe</Button></Tooltip>
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={handleBeAdd}>添加 Backend</Button>
        </Space>
      </div>

      <Row gutter={12}>
        <Col span={6}><Card size="small"><Statistic title="总计" value={backendTotal} /></Card></Col>
        <Col span={6}><Card size="small"><Statistic title="健康" value={backendHealthy} valueStyle={{ color: '#52c41a' }} suffix={`/ ${backendTotal}`} /></Card></Col>
        <Col span={6}><Card size="small"><Statistic title="策略" value={cfg.pool_strategy || 'round_robin'} valueStyle={{ fontSize: 14 }} /></Card></Col>
        <Col span={6}><Card size="small"><Statistic title="能力" value={capabilities.filter(c => (c as any).available || c.enabled).length} suffix={`/ ${capabilities.length}`} /></Card></Col>
      </Row>

      {/* Backend cards */}
      {backends.length === 0 ? (
        <Card>
          <div style={{ textAlign: 'center', padding: 32 }}>
            <ApiOutlined style={{ fontSize: 40, color: '#d9d9d9' }} />
            <Paragraph type="secondary" style={{ margin: '8px 0' }}>
              尚无 Backend 端点。添加第一个端点以开始路由请求。
            </Paragraph>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleBeAdd}>添加 Backend</Button>
          </div>
        </Card>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {backends.map((be: any) => (
            <Card key={be.label} size="small"
              title={<Space><ApiOutlined style={{ color: '#5e6ad2' }} /><Text strong>{be.label}</Text><Text type="secondary" style={{ fontSize: 12 }}>{be.base_url}</Text></Space>}
              extra={<Space><Tag>weight: {be.weight}</Tag>{getStatusBadge(be.health || be.status, be.latency)}</Space>}
              actions={[
                <Tooltip title="编辑"><Button type="text" icon={<EditOutlined />} onClick={() => handleBeEdit(be.label)} /></Tooltip>,
                <Tooltip title="复制"><Button type="text" icon={<CopyOutlined />} onClick={() => handleBeDuplicate(be.label)} /></Tooltip>,
                <Popconfirm title={`删除 "${be.label}"？`} onConfirm={() => handleBeDelete(be.label)}>
                  <Button type="text" danger icon={<DeleteOutlined />} />
                </Popconfirm>,
              ]}>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                {(be.models || []).map((m: any) => (
                  <Tag key={m.name} color="blue" style={{ margin: 0, fontSize: 11 }}>
                    {getModelIcon(m.type)} {m.name}{m.type ? ` [${m.type}]` : ''}
                  </Tag>
                ))}
                {(be.models || []).length === 0 && <Text type="secondary" style={{ fontSize: 11 }}>模型自动发现中...</Text>}
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* Capability Overview */}
      <Card size="small" title="能力总览 (Capability Overview)">
        <Row gutter={[10, 10]}>
          {capabilities.map(cap => {
            const avail = (cap as any).available ?? cap.enabled;
            const bks: string[] = (cap as any).backends ?? [];
            return (
              <Col span={8} key={cap.type}>
                <Card size="small" style={{ borderLeft: `3px solid ${avail ? '#52c41a' : '#ff4d4f'}`, background: avail ? '#f6ffed' : '#fff2f0' }}>
                  <Space>
                    {CAPABILITY_ICONS[cap.type]} <Text strong>{CAPABILITY_LABELS[cap.type] || cap.type}</Text>
                    {avail ? <Tag color="success" style={{ fontSize: 10 }}>{bks.length} backend{bks.length !== 1 ? 's' : ''}</Tag> : <Tag color="error" style={{ fontSize: 10 }}>未配置</Tag>}
                  </Space>
                  {avail && bks.length > 0 && (
                    <div style={{ marginTop: 4 }}>
                      <Text type="secondary" style={{ fontSize: 10 }}>{bks.join(', ')}</Text>
                    </div>
                  )}
                </Card>
              </Col>
            );
          })}
        </Row>
      </Card>

      {/* Stats footer */}
      <Card size="small" title="供应商状态">
        <Row gutter={16}>
          <Col span={6}><Statistic title="Backends" value={backendTotal} valueStyle={{ fontSize: 24, color: '#5e6ad2' }} /></Col>
          <Col span={6}><Statistic title="活跃会话" value={cfg.active_sessions || 0} valueStyle={{ fontSize: 24, color: '#52c41a' }} /></Col>
          <Col span={6}><Statistic title="工具数" value={cfg.tool_count || 0} valueStyle={{ fontSize: 24, color: '#faad14' }} /></Col>
          <Col span={6}><Statistic title="策略" value={cfg.pool_strategy || 'round_robin'} valueStyle={{ fontSize: 14 }} /></Col>
        </Row>
      </Card>

      {/* Add/Edit Backend Modal */}
      <Modal
        title={beEditing ? `编辑 Backend: ${beEditing}` : '添加 Backend'}
        open={beModalOpen}
        onCancel={() => setBeModalOpen(false)}
        onOk={handleBeSave}
        okText={beEditing ? '更新' : '添加'}
        width={560}
      >
        <Form form={beForm} layout="vertical">
          <Form.Item name="label" label="Label" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="e.g. beecode-main" />
          </Form.Item>
          <Form.Item name="key" label="API Key" rules={[{ required: true, message: '必填' }]}>
            <Input.Password placeholder="sk-..." />
          </Form.Item>
          <Form.Item name="base_url" label="Base URL" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="https://beecode.cc/v1" />
          </Form.Item>
          <Form.Item name="weight" label="Weight" rules={[{ required: true }]}>
            <InputNumber min={1} max={100} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="models" label="Models (每行: name:type)"
            extra="类型: chat, vision, image_gen, video_gen, audio_stt, audio_tts, embedding. 留空自动发现.">
            <Input.TextArea rows={4} placeholder="gpt-5.5:chat&#10;gpt-image-2:image_gen" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
