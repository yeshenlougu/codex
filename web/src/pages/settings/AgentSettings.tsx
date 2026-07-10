import { useState, useEffect, useCallback } from 'react';
import { Card, Select, Input, InputNumber, Typography, message, Switch, Divider } from 'antd';
import { getConfig, updateConfig } from '../../lib/api';
import type { Config } from '../../lib/types';

const { Text, Title } = Typography;

// A single settings row matching Codex reference: label + description + right control
function SettingRow({ label, desc, children }: { label: string; desc?: string; children: React.ReactNode }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      padding: '10px 0', gap: 24,
    }}>
      <div style={{ flex: 1, minWidth: 0 }}>
        <Text style={{ fontSize: 13, fontWeight: 500 }}>{label}</Text>
        {desc && <div><Text type="secondary" style={{ fontSize: 11 }}>{desc}</Text></div>}
      </div>
      <div style={{ flexShrink: 0 }}>{children}</div>
    </div>
  );
}

export default function AgentSettings() {
  const [cfg, setCfg] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(() => {
    getConfig().then(c => setCfg(c)).catch(() => {}).finally(() => setLoading(false));
  }, []);

  useEffect(() => { load(); }, [load]);

  const save = async (field: string, value: any) => {
    try {
      await updateConfig({ [field]: value });
      message.success({ content: '保存成功', key: field, duration: 1 });
    } catch (e: any) {
      message.error(e.message);
    }
  };

  if (loading) return <Card loading style={{ maxWidth: 640 }} />;
  if (!cfg) return <Card style={{ maxWidth: 640 }}><Text type="danger">Could not load config.</Text></Card>;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Title level={4} style={{ margin: 0, fontSize: 18 }}>常规</Title>

      {/* Provider & Model */}
      <Card size="small">
        <SettingRow label="提供者" desc="选择 AI 模型提供商">
          <Select value={cfg.provider} onChange={v => { save('provider', v); load(); }} style={{ width: 160 }}
            options={[
              { value: 'openai', label: 'OpenAI' },
              { value: 'anthropic', label: 'Anthropic' },
              { value: 'ollama', label: 'Ollama' },
              { value: 'custom', label: 'Custom' },
            ]} />
        </SettingRow>
        <Divider style={{ margin: 0 }} />
        <SettingRow label="模型" desc="使用的模型名称">
          <Input value={cfg.model} onChange={e => save('model', e.target.value)} style={{ width: 220 }} />
        </SettingRow>
        <Divider style={{ margin: 0 }} />
        <SettingRow label="Base URL" desc="API 端点地址">
          <Input value={cfg.base_url} onChange={e => save('base_url', e.target.value)}
            placeholder="https://api.openai.com/v1" style={{ width: 280 }} />
        </SettingRow>
      </Card>

      {/* Behavior */}
      <Card size="small" title="行为">
        <SettingRow label="推理强度" desc="模型推理深度 (low / medium / high / xhigh)">
          <Select value={cfg.reasoning_effort} onChange={v => save('reasoning_effort', v)} style={{ width: 120 }}
            options={[
              { value: 'low', label: 'Low' }, { value: 'medium', label: 'Medium' },
              { value: 'high', label: 'High' }, { value: 'xhigh', label: 'X-High' },
            ]} />
        </SettingRow>
        <Divider style={{ margin: 0 }} />
        <SettingRow label="最大回合数" desc="Agent 单次对话最大交互轮数">
          <InputNumber value={cfg.max_turns} min={1} max={200} onChange={v => save('max_turns', v)} style={{ width: 100 }} />
        </SettingRow>
        <Divider style={{ margin: 0 }} />
        <SettingRow label="池策略" desc="多 Backend 负载均衡策略">
          <Select value={cfg.pool_strategy || 'round_robin'} onChange={v => save('pool_strategy', v)} style={{ width: 140 }}
            options={[
              { value: 'round_robin', label: 'Round Robin' },
              { value: 'random', label: 'Random' },
              { value: 'fill_first', label: 'Fill First' },
            ]} />
        </SettingRow>
        <Divider style={{ margin: 0 }} />
        <SettingRow label="Wire API" desc="底层 API 协议 (chat_completions / responses)">
          <Select value={cfg.wire_api || 'chat_completions'} onChange={v => save('wire_api', v)} style={{ width: 160 }}
            options={[
              { value: 'chat_completions', label: 'Chat Completions' },
              { value: 'responses', label: 'Responses' },
            ]} />
        </SettingRow>
      </Card>

      {/* System Prompt */}
      <Card size="small" title="系统提示词">
        <Input.TextArea
          value={cfg.system_prompt || ''}
          onChange={e => save('system_prompt', e.target.value)}
          rows={5}
          style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12 }}
          placeholder="输入自定义系统提示词..."
        />
      </Card>

      {/* Stats */}
      <Card size="small" title="状态">
        <div style={{ display: 'flex', gap: 32 }}>
          <div>
            <Text type="secondary" style={{ fontSize: 11 }}>工具数</Text>
            <div style={{ fontSize: 20, fontWeight: 700, color: '#5e6ad2' }}>{cfg.tool_count}</div>
          </div>
          <div>
            <Text type="secondary" style={{ fontSize: 11 }}>活跃会话</Text>
            <div style={{ fontSize: 20, fontWeight: 700, color: '#5e6ad2' }}>{cfg.active_sessions}</div>
          </div>
          <div>
            <Text type="secondary" style={{ fontSize: 11 }}>Backends</Text>
            <div style={{ fontSize: 20, fontWeight: 700, color: '#5e6ad2' }}>{cfg.backend_count}</div>
          </div>
        </div>
      </Card>
    </div>
  );
}
