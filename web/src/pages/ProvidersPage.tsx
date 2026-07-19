import { Layout, Typography, Button } from 'antd';
import { ArrowLeftOutlined, ApiOutlined } from '@ant-design/icons';
import ProviderList from './settings/ProviderList';

const { Content } = Layout;
const { Title } = Typography;

interface Props {
  onBack?: () => void;
}

export default function ProvidersPage({ onBack }: Props) {
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
          <ApiOutlined style={{ fontSize: 22, color: '#5e6ad2' }} />
          <Title level={4} style={{ margin: 0 }}>
            Providers
          </Title>
        </div>
        <ProviderList onSelect={() => {}} />
      </Content>
    </Layout>
  );
}
