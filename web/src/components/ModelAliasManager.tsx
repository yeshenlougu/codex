import { useState, useEffect, useCallback } from 'react';
import {
  Card, Table, Button, Input, Space, Typography, message, Popconfirm, Tag,
} from 'antd';
import { PlusOutlined, DeleteOutlined, LinkOutlined } from '@ant-design/icons';
import { listProviders } from '../../lib/api';

const { Text, Title } = Typography;

interface AliasRow {
  id: number;
  provider_id: string;
  alias: string;
  real_name: string;
}

export default function ModelAliasManager() {
  const [aliases, setAliases] = useState<AliasRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [newAlias, setNewAlias] = useState('');
  const [newReal, setNewReal] = useState('');
  const [adding, setAdding] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      // Get current provider ID first
      const provResp = await listProviders();
      const current = (provResp.providers || []).find((p: any) => p.is_current);
      if (!current) { setAliases([]); setLoading(false); return; }

      const resp = await fetch(`/api/v1/aliases`).then(r => r.json());
      // Fallback: try providers/:id/aliases 
      const resp2 = await fetch(`/api/providers/${current.id}/aliases`).then(r => r.json());
      const items = resp?.aliases || resp2?.aliases || [];
      setAliases(items);
    } catch {
      setAliases([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleAdd = async () => {
    if (!newAlias.trim() || !newReal.trim()) return;
    setAdding(true);
    try {
      const resp = await fetch('/api/providers/aliases', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ alias: newAlias.trim(), real_name: newReal.trim() }),
      });
      if (!resp.ok) throw new Error('Failed');
      setNewAlias('');
      setNewReal('');
      message.success('Alias added');
      load();
    } catch {
      message.error('Failed to add alias');
    } finally {
      setAdding(false);
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await fetch(`/api/providers/aliases/${id}`, { method: 'DELETE' });
      message.success('Alias deleted');
      load();
    } catch {
      message.error('Failed to delete');
    }
  };

  return (
    <Card
      size="small"
      title={<Space><LinkOutlined /><Text strong>Model Aliases</Text></Space>}
      extra={
        <Text type="secondary" style={{ fontSize: 11 }}>
          Map friendly names to real model names
        </Text>
      }
      style={{ marginTop: 16 }}
    >
      <Space style={{ marginBottom: 12, width: '100%' }} size={8}>
        <Input
          placeholder="Alias (e.g. gpt-5.5)"
          value={newAlias}
          onChange={e => setNewAlias(e.target.value)}
          style={{ width: 200 }}
          size="small"
          onPressEnter={handleAdd}
        />
        <Text type="secondary">→</Text>
        <Input
          placeholder="Real name (e.g. gpt-4o)"
          value={newReal}
          onChange={e => setNewReal(e.target.value)}
          style={{ width: 200 }}
          size="small"
          onPressEnter={handleAdd}
        />
        <Button
          type="primary"
          size="small"
          icon={<PlusOutlined />}
          onClick={handleAdd}
          loading={adding}
        >
          Add
        </Button>
      </Space>

      <Table
        dataSource={aliases}
        rowKey="id"
        loading={loading}
        size="small"
        pagination={false}
        locale={{ emptyText: 'No aliases configured' }}
        columns={[
          {
            title: 'Alias',
            dataIndex: 'alias',
            key: 'alias',
            render: (v: string) => <Tag color="blue">{v}</Tag>,
          },
          {
            title: 'Real Name',
            dataIndex: 'real_name',
            key: 'real_name',
            render: (v: string) => <Tag color="green">{v}</Tag>,
          },
          {
            title: '',
            key: 'actions',
            width: 50,
            render: (_: any, record: AliasRow) => (
              <Popconfirm
                title="Delete this alias?"
                onConfirm={() => handleDelete(record.id)}
              >
                <Button type="text" size="small" danger icon={<DeleteOutlined />} />
              </Popconfirm>
            ),
          },
        ]}
      />
    </Card>
  );
}
