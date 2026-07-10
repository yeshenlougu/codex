import { useState, useEffect, useCallback } from 'react';
import { Card, Select, InputNumber, Input, Statistic, Row, Col, message, Space, Form } from 'antd';
import { getConfig, updateConfig } from '../../lib/api';
import type { Config } from '../../lib/types';

export default function AgentSettings() {
  const [cfg, setCfg] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [form] = Form.useForm();

  const load = useCallback(() => {
    getConfig().then(c => {
      setCfg(c);
      form.setFieldsValue(c);
    }).catch(() => {}).finally(() => setLoading(false));
  }, [form]);

  useEffect(() => { load(); }, [load]);

  const saveField = async (field: string, value: any) => {
    try {
      await updateConfig({ [field]: value });
      message.success({ content: 'Saved', key: field, duration: 1 });
    } catch (e: any) {
      message.error(e.message);
    }
  };

  if (loading) return <Card loading style={{ maxWidth: 680 }} />;
  if (!cfg) return <Card style={{ maxWidth: 680 }}><span style={{ color: 'var(--red)' }}>Could not load config.</span></Card>;

  return (
    <div style={{ maxWidth: 680, display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Form form={form} layout="horizontal" labelCol={{ span: 5 }} wrapperCol={{ span: 19 }} labelAlign="left" size="small">

        <Card title="Provider & Model" size="small">
          <Form.Item label="Provider" name="provider">
            <Select onChange={v => saveField('provider', v)} options={[
              { value: 'openai', label: 'OpenAI' }, { value: 'anthropic', label: 'Anthropic' },
              { value: 'ollama', label: 'Ollama' }, { value: 'custom', label: 'Custom' },
            ]} />
          </Form.Item>
          <Form.Item label="Model" name="model">
            <Input onBlur={e => saveField('model', e.target.value)} />
          </Form.Item>
          <Form.Item label="Base URL" name="base_url">
            <Input onBlur={e => saveField('base_url', e.target.value)} placeholder="https://api.openai.com/v1" />
          </Form.Item>
        </Card>

        <Card title="Behavior" size="small">
          <Form.Item label="Reasoning" name="reasoning_effort">
            <Select onChange={v => saveField('reasoning_effort', v)} options={[
              { value: 'low', label: 'Low' }, { value: 'medium', label: 'Medium' },
              { value: 'high', label: 'High' }, { value: 'xhigh', label: 'X-High' },
            ]} />
          </Form.Item>
          <Form.Item label="Max Turns" name="max_turns">
            <InputNumber min={1} max={200} onChange={v => saveField('max_turns', v)} style={{ width: 120 }} />
          </Form.Item>
          <Form.Item label="Pool Strategy" name="pool_strategy">
            <Select onChange={v => saveField('pool_strategy', v)} options={[
              { value: 'round_robin', label: 'Round Robin' },
              { value: 'random', label: 'Random' },
              { value: 'fill_first', label: 'Fill First' },
            ]} />
          </Form.Item>
          <Form.Item label="Wire API" name="wire_api">
            <Select onChange={v => saveField('wire_api', v)} options={[
              { value: 'chat_completions', label: 'Chat Completions' },
              { value: 'responses', label: 'Responses' },
            ]} />
          </Form.Item>
        </Card>

        <Card title="System Prompt" size="small">
          <Form.Item name="system_prompt" style={{ marginBottom: 0 }}>
            <Input.TextArea rows={5} onBlur={e => saveField('system_prompt', e.target.value)}
              style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12 }} />
          </Form.Item>
        </Card>

        <Card title="Status" size="small">
          <Row gutter={32}>
            <Col><Statistic title="Tools" value={cfg.tool_count} valueStyle={{ color: '#5e6ad2', fontSize: 20 }} /></Col>
            <Col><Statistic title="Sessions" value={cfg.active_sessions} valueStyle={{ color: '#5e6ad2', fontSize: 20 }} /></Col>
            <Col><Statistic title="Backends" value={cfg.backend_count} valueStyle={{ color: '#5e6ad2', fontSize: 20 }} /></Col>
          </Row>
        </Card>

      </Form>
    </div>
  );
}
