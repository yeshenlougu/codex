import { useState, useEffect, useCallback } from 'react';
import {
  Card, Select, Input, Typography, Button, message, Divider,
  Tag, Space, Tooltip, Statistic, Row, Col, Popconfirm,
} from 'antd';
import {
  ApiOutlined, KeyOutlined, LinkOutlined, ThunderboltOutlined,
  SafetyCertificateOutlined, EyeInvisibleOutlined, EyeOutlined,
  ReloadOutlined, ArrowRightOutlined, SettingOutlined,
} from '@ant-design/icons';
import { getConfig, updateConfig } from '../../lib/api';
import type { Config } from '../../lib/types';

const { Text, Title, Paragraph } = Typography;

function SettingRow({ label, desc, children }: { label: string; desc?: string; children: React.ReactNode }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      padding: '12px 0', gap: 24,
    }}>
      <div style={{ flex: 1, minWidth: 0 }}>
        <Text style={{ fontSize: 13, fontWeight: 500 }}>{label}</Text>
        {desc && <div><Text type="secondary" style={{ fontSize: 11 }}>{desc}</Text></div>}
      </div>
      <div style={{ flexShrink: 0 }}>{children}</div>
    </div>
  );
}

export default function ProviderSettings({ onNavigateBackends }: { onNavigateBackends?: () => void }) {
  const [cfg, setCfg] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [showKey, setShowKey] = useState(false);
  const [apiKey, setApiKey] = useState('');
  const [saving, setSaving] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    getConfig().then(c => {
      setCfg(c);
      setApiKey('');
    }).catch(() => {}).finally(() => setLoading(false));
  }, []);

  useEffect(() => { load(); }, [load]);

  const save = async (field: string, value: any) => {
    try {
      await updateConfig({ [field]: value });
      message.success({ content: '保存成功', key: field, duration: 1 });
      load();
    } catch (e: any) {
      message.error(e.message);
    }
  };

  const saveApiKey = async () => {
    if (!apiKey.trim()) { message.warning('请输入 API Key'); return; }
    setSaving(true);
    try {
      await updateConfig({ api_key: apiKey.trim() });
      message.success('API Key 已保存');
      setApiKey('');
      setShowKey(false);
      load();
    } catch (e: any) {
      message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <Card loading style={{ maxWidth: 720 }} />;
  if (!cfg) return <Card style={{ maxWidth: 720 }}><Text type="danger">无法加载配置。</Text></Card>;

  const providerName = cfg.provider || 'cc-switch';
  const isPoolMode = providerName === 'cc-switch';

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, maxWidth: 720 }}>
      {/* Header */}
      <div>
        <Title level={4} style={{ margin: 0, fontSize: 18 }}>供应商管理</Title>
        <Text type="secondary" style={{ fontSize: 12 }}>
          配置 AI 供应商的连接参数与路由策略
        </Text>
      </div>

      {/* Provider Identity Card */}
      <Card size="small" style={{ borderLeft: '3px solid #5e6ad2' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 12 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{
              width: 40, height: 40, borderRadius: 10,
              background: 'linear-gradient(135deg, #5e6ad2, #7c8bf5)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
              <ApiOutlined style={{ fontSize: 20, color: '#fff' }} />
            </div>
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <Text strong style={{ fontSize: 15 }}>{providerName}</Text>
                <Tag color={isPoolMode ? 'blue' : 'green'} style={{ fontSize: 10 }}>
                  {isPoolMode ? 'cc-switch 代理池' : '直连模式'}
                </Tag>
              </div>
              <Text type="secondary" style={{ fontSize: 11 }}>
                {isPoolMode
                  ? `多 Backend 智能路由 · ${cfg.backend_count || 0} 个端点`
                  : `直连 ${cfg.base_url || '(未配置)'}`}
              </Text>
            </div>
          </div>
          <Select
            value={providerName}
            onChange={v => save('provider', v)}
            style={{ width: 180 }}
            size="middle"
            options={[
              { value: 'cc-switch', label: 'cc-switch (代理池)' },
              { value: 'openai', label: 'OpenAI' },
              { value: 'anthropic', label: 'Anthropic' },
              { value: 'deepseek', label: 'DeepSeek' },
              { value: 'ollama', label: 'Ollama (本地)' },
              { value: 'custom', label: 'Custom' },
            ]}
          />
        </div>
      </Card>

      {/* API Key Management */}
      <Card
        size="small"
        title={<span><KeyOutlined style={{ marginRight: 6 }} />API Key</span>}
        extra={
          <Tooltip title="重新加载">
            <Button type="text" size="small" icon={<ReloadOutlined />} onClick={load} />
          </Tooltip>
        }
      >
        <SettingRow label="当前 Key" desc="已保存的 API 密钥（脱敏显示）">
          <Space>
            <Text code style={{ fontSize: 12 }}>
              {cfg.api_key_masked || '(未设置)'}
            </Text>
            {cfg.api_key_masked && cfg.api_key_masked !== '(未设置)' && (
              <Tag color="green" style={{ fontSize: 10 }}>已配置</Tag>
            )}
          </Space>
        </SettingRow>
        <Divider style={{ margin: 0 }} />
        <div style={{ padding: '12px 0' }}>
          <Text style={{ fontSize: 13, fontWeight: 500 }}>更新 API Key</Text>
          <div style={{ margin: '8px 0 0', display: 'flex', gap: 8, alignItems: 'center' }}>
            <Input
              type={showKey ? 'text' : 'password'}
              value={apiKey}
              onChange={e => setApiKey(e.target.value)}
              placeholder="sk-..."
              style={{ flex: 1, maxWidth: 380 }}
              prefix={<KeyOutlined style={{ color: 'var(--text-muted)' }} />}
              suffix={
                <Button
                  type="text"
                  size="small"
                  icon={showKey ? <EyeInvisibleOutlined /> : <EyeOutlined />}
                  onClick={() => setShowKey(!showKey)}
                  style={{ border: 'none' }}
                />
              }
            />
            <Button
              type="primary"
              size="small"
              onClick={saveApiKey}
              loading={saving}
              icon={<SafetyCertificateOutlined />}
            >
              保存
            </Button>
          </div>
          <Text type="secondary" style={{ fontSize: 10, display: 'block', marginTop: 4 }}>
            密钥仅存储在本机 ~/.codex/config.yaml 中，不会上传到任何服务器
          </Text>
        </div>
      </Card>

      {/* Connection Settings (cc-switch pool mode) */}
      {isPoolMode && (
        <Card
          size="small"
          title={<span><ThunderboltOutlined style={{ marginRight: 6 }} />代理池策略</span>}
        >
          <SettingRow label="路由策略" desc="多 Backend 间的负载均衡方式">
            <Select
              value={cfg.pool_strategy || 'round_robin'}
              onChange={v => save('pool_strategy', v)}
              style={{ width: 160 }}
              options={[
                { value: 'round_robin', label: 'Round Robin' },
                { value: 'random', label: 'Random' },
                { value: 'fill_first', label: 'Fill First' },
              ]}
            />
          </SettingRow>
          <Divider style={{ margin: 0 }} />
          <SettingRow label="Wire API" desc="底层 API 协议格式">
            <Select
              value={cfg.wire_api || 'chat_completions'}
              onChange={v => save('wire_api', v)}
              style={{ width: 180 }}
              options={[
                { value: 'chat_completions', label: 'Chat Completions' },
                { value: 'responses', label: 'Responses' },
              ]}
            />
          </SettingRow>
          <Divider style={{ margin: 0 }} />
          <SettingRow label="Backend 端点" desc={`${cfg.backend_count || 0} 个已配置`}>
            <Button
              type="link"
              size="small"
              icon={<SettingOutlined />}
              onClick={onNavigateBackends}
              style={{ padding: 0 }}
            >
              管理 Backends <ArrowRightOutlined />
            </Button>
          </SettingRow>
        </Card>
      )}

      {/* Direct Connection Settings (non-pool mode) */}
      {!isPoolMode && (
        <Card
          size="small"
          title={<span><LinkOutlined style={{ marginRight: 6 }} />连接设置</span>}
        >
          <SettingRow label="Base URL" desc="API 端点地址">
            <Input
              value={cfg.base_url || ''}
              onChange={e => save('base_url', e.target.value)}
              placeholder="https://api.openai.com/v1"
              style={{ width: 320 }}
            />
          </SettingRow>
          <Divider style={{ margin: 0 }} />
          <SettingRow label="模型" desc="默认使用的模型名称">
            <Input
              value={cfg.model || ''}
              onChange={e => save('model', e.target.value)}
              placeholder="gpt-4o"
              style={{ width: 220 }}
            />
          </SettingRow>
        </Card>
      )}

      {/* Stats */}
      <Card size="small" title="供应商状态">
        <Row gutter={16}>
          <Col span={6}>
            <Statistic
              title="Backends"
              value={cfg.backend_count || 0}
              valueStyle={{ fontSize: 24, color: '#5e6ad2' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="活跃会话"
              value={cfg.active_sessions || 0}
              valueStyle={{ fontSize: 24, color: '#52c41a' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="工具数"
              value={cfg.tool_count || 0}
              valueStyle={{ fontSize: 24, color: '#faad14' }}
            />
          </Col>
          <Col span={6}>
            <Tooltip title="Pool Strategy">
              <Statistic
                title="策略"
                value={cfg.pool_strategy || 'round_robin'}
                valueStyle={{ fontSize: 14, color: 'var(--text-secondary)' }}
              />
            </Tooltip>
          </Col>
        </Row>
      </Card>
    </div>
  );
}
