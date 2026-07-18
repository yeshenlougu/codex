import { useState, useEffect, useCallback } from 'react';
import { Card, Table, Typography, Statistic, Row, Col, Select, Space, Tag, Spin } from 'antd';
import { BarChartOutlined, ApiOutlined, ThunderboltOutlined, DollarOutlined } from '@ant-design/icons';

const { Text, Title } = Typography;

interface UsageSummary {
  provider_id: string;
  provider_name: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  request_count: number;
  cost_est: number;
}

interface UsageLog {
  provider_id: string;
  backend_id: number;
  model: string;
  input_tokens: number;
  output_tokens: number;
  cost_est: number;
}

export default function UsagePage() {
  const [summary, setSummary] = useState<UsageSummary[]>([]);
  const [logs, setLogs] = useState<UsageLog[]>([]);
  const [totals, setTotals] = useState({ input: 0, output: 0, requests: 0, cost: 0 });
  const [loading, setLoading] = useState(true);
  const [days, setDays] = useState(30);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const to = new Date().toISOString().slice(0, 10);
      const from = new Date(Date.now() - days * 86400000).toISOString().slice(0, 10);

      const [sumRes, logRes] = await Promise.all([
        fetch(`/api/usage/summary?from=${from}&to=${to}`).then(r => r.json()),
        fetch(`/api/usage?limit=200`).then(r => r.json()),
      ]);

      setSummary(sumRes.summary || []);
      setLogs(logRes.logs || []);
      setTotals({
        input: sumRes.total_input || 0,
        output: sumRes.total_output || 0,
        requests: sumRes.total_requests || 0,
        cost: sumRes.total_cost || 0,
      });
    } catch {
      // API may not be available
    } finally {
      setLoading(false);
    }
  }, [days]);

  useEffect(() => { load(); }, [load]);

  const formatTokens = (n: number) => {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
    return String(n);
  };

  const summaryCols = [
    { title: 'Provider', dataIndex: 'provider_name', key: 'provider', render: (v: string, r: UsageSummary) => v || r.provider_id?.slice(0, 8) || '-' },
    { title: 'Model', dataIndex: 'model', key: 'model', render: (v: string) => v || '-' },
    { title: 'Requests', dataIndex: 'request_count', key: 'requests', render: (n: number) => <Text strong>{n}</Text> },
    { title: 'Input', dataIndex: 'input_tokens', key: 'input', render: (n: number) => formatTokens(n) },
    { title: 'Output', dataIndex: 'output_tokens', key: 'output', render: (n: number) => formatTokens(n) },
    { title: 'Cost', dataIndex: 'cost_est', key: 'cost', render: (n: number) => <Text type={n > 0 ? 'warning' : 'secondary'}>${n.toFixed(4)}</Text> },
  ];

  const logCols = [
    { title: 'Model', dataIndex: 'model', key: 'model', width: 140 },
    { title: 'Input', dataIndex: 'input_tokens', key: 'input', render: (n: number) => formatTokens(n) },
    { title: 'Output', dataIndex: 'output_tokens', key: 'output', render: (n: number) => formatTokens(n) },
    { title: 'Est Cost', dataIndex: 'cost_est', key: 'cost', render: (n: number) => `$${n.toFixed(6)}` },
  ];

  return (
    <div style={{ padding: '16px 24px', maxWidth: 1000 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}><BarChartOutlined style={{ marginRight: 8 }} />Usage Dashboard</Title>
          <Text type="secondary">API 调用用量统计与成本估算</Text>
        </div>
        <Select value={days} onChange={setDays} style={{ width: 140 }}
          options={[
            { value: 1, label: '今天' }, { value: 7, label: '最近 7 天' },
            { value: 30, label: '最近 30 天' }, { value: 90, label: '最近 90 天' },
          ]}
        />
      </div>

      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}><Spin size="large" /></div>
      ) : (
        <>
          {/* Totals */}
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col span={6}>
              <Card size="small"><Statistic title="Total Requests" value={totals.requests} prefix={<ApiOutlined />} /></Card>
            </Col>
            <Col span={6}>
              <Card size="small"><Statistic title="Input Tokens" value={formatTokens(totals.input)} prefix={<ThunderboltOutlined />} /></Card>
            </Col>
            <Col span={6}>
              <Card size="small"><Statistic title="Output Tokens" value={formatTokens(totals.output)} prefix={<ThunderboltOutlined />} /></Card>
            </Col>
            <Col span={6}>
              <Card size="small"><Statistic title="Est Cost" value={`$${totals.cost.toFixed(4)}`} prefix={<DollarOutlined />} valueStyle={{ color: totals.cost > 0 ? '#faad14' : undefined }} /></Card>
            </Col>
          </Row>

          {/* Daily Summary */}
          <Card size="small" title="每日用量汇总" style={{ marginBottom: 16 }}>
            <Table
              dataSource={summary}
              columns={summaryCols}
              rowKey={(r: UsageSummary) => `${r.provider_id}-${r.model}`}
              size="small"
              pagination={{ pageSize: 20, size: 'small' }}
              locale={{ emptyText: '暂无用量数据。开始对话后自动记录。' }}
            />
          </Card>

          {/* Recent Logs */}
          <Card size="small" title={`最近请求 (${logs.length})`}>
            <Table
              dataSource={logs}
              columns={logCols}
              rowKey={(_: UsageLog, i: number) => String(i)}
              size="small"
              pagination={{ pageSize: 15, size: 'small' }}
              locale={{ emptyText: '暂无调用记录' }}
            />
          </Card>
        </>
      )}
    </div>
  );
}
