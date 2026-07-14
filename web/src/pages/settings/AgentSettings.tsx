import { useState, useEffect, useCallback } from 'react';
import { Card, Select, Input, InputNumber, Typography, message } from 'antd';
import { getConfig, updateConfig } from '../../lib/api';
import type { Config } from '../../lib/types';

const { Text, Title } = Typography;

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

      {/* Agent Behavior */}
      <Card size="small" title="Agent 行为">
        <SettingRow label="模型" desc="默认使用的模型名称">
          <Input value={cfg.model || ''} onChange={e => save('model', e.target.value)}
            placeholder="deepseek-v4-pro" style={{ width: 220 }} />
        </SettingRow>
        <Card size="small" style={{ marginTop: 0, border: 'none', boxShadow: 'none' }}>
          <SettingRow label="推理强度" desc="模型推理深度 (low / medium / high / xhigh)">
            <Select value={cfg.reasoning_effort || 'high'} onChange={v => save('reasoning_effort', v)} style={{ width: 120 }}
              options={[
                { value: 'low', label: 'Low' }, { value: 'medium', label: 'Medium' },
                { value: 'high', label: 'High' }, { value: 'xhigh', label: 'X-High' },
              ]} />
          </SettingRow>
        </Card>
        <div style={{ padding: '0' }}>
          <SettingRow label="最大回合数" desc="Agent 单次对话最大交互轮数">
            <InputNumber value={cfg.max_turns || 60} min={1} max={200}
              onChange={v => v && save('max_turns', v)} style={{ width: 100 }} />
          </SettingRow>
        </div>
      </Card>

      {/* System Prompt */}
      <Card size="small" title="系统提示词">
        <Input.TextArea
          value={cfg.system_prompt || ''}
          onChange={e => save('system_prompt', e.target.value)}
          rows={6}
          style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12 }}
          placeholder="输入自定义系统提示词..."
        />
      </Card>

      {/* Stats */}
      <Card size="small" title="状态">
        <div style={{ display: 'flex', gap: 32 }}>
          <div>
            <Text type="secondary" style={{ fontSize: 11 }}>工具数</Text>
            <div style={{ fontSize: 20, fontWeight: 700, color: '#5e6ad2' }}>{cfg.tool_count || 0}</div>
          </div>
          <div>
            <Text type="secondary" style={{ fontSize: 11 }}>活跃会话</Text>
            <div style={{ fontSize: 20, fontWeight: 700, color: '#5e6ad2' }}>{cfg.active_sessions || 0}</div>
          </div>
          <div>
            <Text type="secondary" style={{ fontSize: 11 }}>Backends</Text>
            <div style={{ fontSize: 20, fontWeight: 700, color: '#5e6ad2' }}>{cfg.backend_count || 0}</div>
          </div>
        </div>
      </Card>
    </div>
  );
}
