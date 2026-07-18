import { useState, useEffect, useCallback } from 'react';
import { Table, Switch, Tag, Card, Typography, Space, message, Input, Select } from 'antd';
import { ToolOutlined, SafetyCertificateOutlined, AppstoreOutlined } from '@ant-design/icons';

const { Text, Title } = Typography;

interface ToolRow {
  name: string;
  description: string;
  category: string;
  risk: 'low' | 'medium' | 'high';
  approval_required: boolean;
  enabled: boolean;
  sort_order: number;
}

export default function ToolsPage() {
  const [tools, setTools] = useState<ToolRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (filter) params.set('category', filter);
      const res = await fetch(`/api/tools?${params}`).then(r => r.json());
      setTools(res.tools || []);
    } catch { message.error('加载工具列表失败'); }
    finally { setLoading(false); }
  }, [filter]);

  useEffect(() => { load(); }, [load]);

  const toggleEnabled = async (name: string, enabled: boolean) => {
    try {
      await fetch(`/api/tools/${name}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
      });
      setTools(prev => prev.map(t => t.name === name ? { ...t, enabled } : t));
      message.success(`${name} ${enabled ? '已启用' : '已禁用'}`);
    } catch { message.error('更新失败'); }
  };

  const toggleApproval = async (name: string, required: boolean) => {
    try {
      await fetch(`/api/tools/${name}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ approval_required: required }),
      });
      setTools(prev => prev.map(t => t.name === name ? { ...t, approval_required: required } : t));
      message.success(`${name} 审批${required ? '已开启' : '已关闭'}`);
    } catch { message.error('更新失败'); }
  };

  const riskColor = (r: string) => ({ low: 'green', medium: 'orange', high: 'red' })[r] || 'default';

  const filtered = tools.filter(t =>
    t.name.toLowerCase().includes(search.toLowerCase()) ||
    t.description.toLowerCase().includes(search.toLowerCase())
  );

  const cols = [
    { title: 'Name', dataIndex: 'name', key: 'name', width: 140, render: (v: string) => <Text code>{v}</Text> },
    { title: 'Description', dataIndex: 'description', key: 'desc', ellipsis: true },
    {
      title: 'Category', dataIndex: 'category', key: 'cat', width: 100,
      render: (v: string) => <Tag color={v === 'system' ? 'blue' : 'default'}>{v}</Tag>,
    },
    {
      title: 'Risk', dataIndex: 'risk', key: 'risk', width: 90,
      render: (v: string) => <Tag color={riskColor(v)}>{v}</Tag>,
    },
    {
      title: 'Approval', key: 'approval', width: 90,
      render: (_: any, r: ToolRow) => (
        <Switch
          checked={r.approval_required}
          onChange={v => toggleApproval(r.name, v)}
          disabled={r.category === 'system'}
          size="small"
          checkedChildren="✓"
          unCheckedChildren="✗"
        />
      ),
    },
    {
      title: 'Enabled', key: 'enabled', width: 90,
      render: (_: any, r: ToolRow) => (
        <Switch
          checked={r.enabled}
          onChange={v => toggleEnabled(r.name, v)}
          disabled={r.category === 'system'}
          size="small"
        />
      ),
    },
  ];

  const systemCount = tools.filter(t => t.category === 'system').length;
  const optionalCount = tools.filter(t => t.category === 'optional').length;
  const enabledCount = tools.filter(t => t.enabled).length;

  return (
    <div style={{ padding: '16px 24px', maxWidth: 960 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}><ToolOutlined style={{ marginRight: 8 }} />Tools Registry</Title>
          <Text type="secondary">管理 Agent 可用工具：启用/禁用、审批策略</Text>
        </div>
        <Space>
          <Select
            value={filter}
            onChange={setFilter}
            style={{ width: 120 }}
            allowClear
            placeholder="分类筛选"
            options={[
              { value: '', label: '全部' },
              { value: 'system', label: '系统工具' },
              { value: 'optional', label: '可选工具' },
            ]}
          />
          <Input.Search
            placeholder="搜索工具名或描述..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            style={{ width: 220 }}
            allowClear
          />
        </Space>
      </div>

      {/* Summary cards */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 16 }}>
        <Card size="small" style={{ flex: 1 }}>
          <Text type="secondary" style={{ fontSize: 11 }}>总计</Text>
          <div><Text strong style={{ fontSize: 22 }}>{tools.length}</Text></div>
        </Card>
        <Card size="small" style={{ flex: 1 }}>
          <Text type="secondary" style={{ fontSize: 11 }}>系统工具</Text>
          <div><Text strong style={{ fontSize: 22, color: '#1677ff' }}>{systemCount}</Text></div>
        </Card>
        <Card size="small" style={{ flex: 1 }}>
          <Text type="secondary" style={{ fontSize: 11 }}>可选工具</Text>
          <div><Text strong style={{ fontSize: 22 }}>{optionalCount}</Text></div>
        </Card>
        <Card size="small" style={{ flex: 1 }}>
          <Text type="secondary" style={{ fontSize: 11 }}>已启用</Text>
          <div><Text strong style={{ fontSize: 22, color: '#52c41a' }}>{enabledCount}</Text></div>
        </Card>
      </div>

      <Card size="small">
        <Table
          dataSource={filtered}
          columns={cols}
          rowKey="name"
          size="small"
          loading={loading}
          pagination={{ pageSize: 20, size: 'small' }}
          locale={{ emptyText: '无匹配工具' }}
        />
      </Card>

      <div style={{ marginTop: 12 }}>
        <Text type="secondary" style={{ fontSize: 11 }}>
          <AppstoreOutlined style={{ marginRight: 4 }} />
          系统工具（蓝色标签）始终启用，不可禁用。高风险工具需审批方可执行。
        </Text>
      </div>
    </div>
  );
}
