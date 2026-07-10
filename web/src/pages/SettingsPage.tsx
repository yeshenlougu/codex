import { useState } from 'react';
import { ConfigProvider, theme, Layout, Menu, Typography } from 'antd';
import { RobotOutlined, TeamOutlined, ApiOutlined, ImportOutlined } from '@ant-design/icons';
import AgentSettings from './settings/AgentSettings';
import AgentManager from './settings/AgentManager';
import BackendManager from './settings/BackendManager';
import ImportExport from './settings/ImportExport';

const { Sider, Content } = Layout;
const { Title } = Typography;

type SubPage = 'agent' | 'agents' | 'backends' | 'import-export';

const items: { key: SubPage; label: string; icon: React.ReactNode }[] = [
  { key: 'agent', label: 'Agent', icon: <RobotOutlined /> },
  { key: 'agents', label: 'Agents', icon: <TeamOutlined /> },
  { key: 'backends', label: 'Backends', icon: <ApiOutlined /> },
  { key: 'import-export', label: 'Import & Export', icon: <ImportOutlined /> },
];

export default function SettingsPage() {
  const [sub, setSub] = useState<SubPage>('agent');

  return (
    <Layout style={{ height: '100%', background: 'transparent' }}>
      <Sider width={200} style={{ background: 'var(--bg-panel)', borderRight: '1px solid var(--border)' }}>
        <div style={{ padding: '12px 16px 8px' }}>
          <Typography.Text type="secondary" style={{ fontSize: 11, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            Settings
          </Typography.Text>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[sub]}
          onClick={({ key }) => setSub(key as SubPage)}
          items={items}
          style={{ background: 'transparent', border: 'none' }}
        />
      </Sider>
      <Content style={{ overflow: 'auto', padding: 24 }}>
        <Title level={5} style={{ marginBottom: 16, color: 'var(--text-primary)' }}>
          Settings / {items.find(x => x.key === sub)?.label}
        </Title>
        {sub === 'agent' && <AgentSettings />}
        {sub === 'agents' && <AgentManager />}
        {sub === 'backends' && <BackendManager />}
        {sub === 'import-export' && <ImportExport />}
      </Content>
    </Layout>
  );
}
