import { useState, useMemo } from 'react';
import { Layout, Input, Typography, Button } from 'antd';
import {
  ArrowLeftOutlined, SearchOutlined, SettingOutlined,
  TeamOutlined,
} from '@ant-design/icons';
import AgentSettings from './settings/AgentSettings';
import AgentManager from './settings/AgentManager';

const { Sider, Content } = Layout;
const { Text } = Typography;

type SubPage = 'general' | 'agents';

interface Category {
  label: string;
  items: { key: SubPage; label: string; icon: React.ReactNode }[];
}

const categories: Category[] = [
  {
    label: 'Personal',
    items: [
      { key: 'general', label: 'General', icon: <SettingOutlined /> },
      { key: 'agents', label: 'Agents', icon: <TeamOutlined /> },
    ],
  },
];

export default function SettingsPage() {
  const [sub, setSub] = useState<SubPage>('general');
  const [search, setSearch] = useState('');

  // Flatten all items for the menu
  const allItems = categories.flatMap(c => c.items);

  // Filter by search
  const visibleCategories = useMemo(() => {
    if (!search.trim()) return categories;
    const q = search.toLowerCase();
    return categories.map(c => ({
      ...c,
      items: c.items.filter(i => i.label.toLowerCase().includes(q) || i.key.includes(q)),
    })).filter(c => c.items.length > 0);
  }, [search]);

  return (
    <Layout style={{ height: '100%', background: 'transparent' }}>
      {/* Sidebar */}
      <Sider width={220} style={{
        background: 'var(--bg-panel)', borderRight: '1px solid var(--border)',
        display: 'flex', flexDirection: 'column', overflow: 'hidden',
      }}>
        {/* Search */}
        <div style={{ padding: '12px 12px 8px', display: 'flex', flexDirection: 'column', gap: 6 }}>
          <Input
            prefix={<SearchOutlined style={{ color: 'var(--text-muted)' }} />}
            placeholder="搜索设置..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            size="small"
            allowClear
          />
        </div>

        {/* Navigation groups */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '4px 0' }}>
          {visibleCategories.map(cat => (
            <div key={cat.label} style={{ marginBottom: 4 }}>
              <Text type="secondary" style={{
                fontSize: 10, fontWeight: 700, textTransform: 'uppercase',
                letterSpacing: '0.08em', padding: '8px 16px 4px', display: 'block',
              }}>
                {cat.label}
              </Text>
              {cat.items.map(item => (
                <div
                  key={item.key}
                  onClick={() => setSub(item.key)}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 8,
                    padding: '6px 16px', cursor: 'pointer', fontSize: 12,
                    margin: '0 6px', borderRadius: 6,
                    background: sub === item.key ? 'var(--bg-active)' : 'transparent',
                    color: sub === item.key ? 'var(--text-primary)' : 'var(--text-secondary)',
                    fontWeight: sub === item.key ? 500 : 400,
                  }}
                >
                  <span style={{ fontSize: 14, opacity: 0.7 }}>{item.icon}</span>
                  <span>{item.label}</span>
                </div>
              ))}
            </div>
          ))}
        </div>
      </Sider>

      {/* Content */}
      <Content style={{ overflow: 'auto', padding: 24, background: 'var(--bg-root)' }}>
        <div style={{ maxWidth: 640 }}>
          {sub === 'general' && <AgentSettings />}
          {sub === 'agents' && <AgentManager />}
        </div>
      </Content>
    </Layout>
  );
}
