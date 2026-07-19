import { Layout, Typography, Button } from 'antd';
import { ArrowLeftOutlined, TeamOutlined } from '@ant-design/icons';
import AgentManager from './settings/AgentManager';

const { Content } = Layout;
const { Title } = Typography;

interface Props {
  onBack?: () => void;
}

export default function AgentsPage({ onBack }: Props) {
  return (
    <Layout style={{ height: '100%', background: 'transparent' }}>
      <Content style={{
        padding: '24px 32px',
        overflow: 'auto',
        background: 'var(--bg-app)',
      }}>
        <div style={{
          display: 'flex', alignItems: 'center', gap: 12,
          marginBottom: 20,
        }}>
          {onBack && (
            <Button
              type="text"
              icon={<ArrowLeftOutlined />}
              onClick={onBack}
              style={{ fontSize: 14 }}
            />
          )}
          <TeamOutlined style={{ fontSize: 22, color: '#5e6ad2' }} />
          <Title level={4} style={{ margin: 0 }}>
            Agents
          </Title>
        </div>
        <AgentManager />
      </Content>
    </Layout>
  );
}
